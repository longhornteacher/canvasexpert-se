package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workspace"
)

type quizReportProgress struct {
	WorkflowState string `json:"workflow_state"`
	Results       struct {
		URL string `json:"url"`
	} `json:"results"`
}
type quizReportRow struct {
	Student struct {
		ID          int64  `json:"id"`
		Attempt     int    `json:"attempt"`
		SubmittedAt string `json:"submitted_at"`
	} `json:"student_data"`
	Responses []struct {
		ItemID   json.Number     `json:"item_id"`
		ItemType string          `json:"item_type"`
		Answer   json.RawMessage `json:"answer"`
		Score    *json.Number    `json:"score"`
	} `json:"item_responses"`
}
// writtenQuizItem is one entry of the New Quizzes items list. Canvas nests the
// question type and body under "entry"; the outer id is the quiz-entry id and
// entry.id is the item id, so either may be the id other reports use.
type writtenQuizItem struct {
	ID     json.Number `json:"id"`
	Points json.Number `json:"points_possible"`
	Entry struct {
		ID   json.Number `json:"id"`
		Type string      `json:"interaction_type_slug"`
		Body string      `json:"item_body"`
	} `json:"entry"`
	Type string `json:"-"`
	Body string `json:"-"`
}
type newQuizRow struct {
	Attempt     int
	SubmittedAt string
	Responses   []domain.WrittenResponse
	Files       []domain.WrittenFile
	FileItemIDs []string
	AutoScored     *domain.AutoScored
	AutoScoredNote string
}

// newQuizResponses reads Canvas's Student Analysis report and joins only the
// requested assignment's written items. The signed result file is fetched by
// the Canvas client's credential-free Download path.
func newQuizResponses(ctx context.Context, c *canvas.Client, courseID, assignmentID int64) (map[int64]newQuizRow, error) {
	var launched struct {
		Progress struct {
			URL string `json:"url"`
		} `json:"progress"`
	}
	path := fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d/reports", courseID, assignmentID)
	if err := c.Write(ctx, "POST", path, map[string]any{"quiz_report": map[string]string{"report_type": "student_analysis", "format": "json"}}, &launched); err != nil {
		return nil, err
	}
	progressURL, err := canvasAPIPath(c, launched.Progress.URL)
	if err != nil {
		return nil, err
	}
	var progress quizReportProgress
	deadline := time.Now().Add(5 * time.Minute)
	for {
		if err = c.Get(ctx, progressURL, &progress); err != nil {
			return nil, err
		}
		if progress.WorkflowState == "completed" {
			break
		}
		if progress.WorkflowState == "failed" || progress.WorkflowState == "error" {
			return nil, fmt.Errorf("Student Analysis report failed")
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("Student Analysis report did not complete")
		}
		t := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
	if progress.Results.URL == "" {
		return nil, fmt.Errorf("Student Analysis report has no results URL")
	}
	var buf bytes.Buffer
	if err = c.DownloadUnauthenticated(ctx, progress.Results.URL, &buf); err != nil {
		return nil, err
	}
	var rows []quizReportRow
	if err = json.Unmarshal(buf.Bytes(), &rows); err != nil {
		return nil, fmt.Errorf("invalid Student Analysis results")
	}
	var items []writtenQuizItem
	if err = c.List(ctx, fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d/items?per_page=100", courseID, assignmentID), &items); err != nil {
		return nil, err
	}
	prompts := map[string]writtenQuizItem{}
	fileItemIDs := []string{}
	var autoItems []writtenQuizItem
	for _, item := range items {
		item.Type, item.Body = item.Entry.Type, item.Entry.Body
		typ := strings.ToLower(item.Type)
		if typ != "essay" && typ != "file-upload" && typ != "file_upload" {
			if p, e := domain.ParseDecimal(item.Points.String()); e == nil && p.Sign() > 0 {
				autoItems = append(autoItems, item)
			}
		}
		for _, id := range []json.Number{item.ID, item.Entry.ID} {
			if id == "" {
				continue
			}
			prompts[id.String()] = item
			if typ == "file-upload" || typ == "file_upload" {
				fileItemIDs = append(fileItemIDs, id.String())
			}
		}
	}
	out := map[int64]newQuizRow{}
	for _, row := range rows {
		if row.Student.ID <= 0 || row.Student.Attempt < 1 {
			continue
		}
		entry := out[row.Student.ID]
		if entry.Attempt != 0 && row.Student.Attempt < entry.Attempt {
			continue
		}
		if entry.Attempt != row.Student.Attempt { entry = newQuizRow{} }
		entry.Attempt, entry.SubmittedAt = row.Student.Attempt, row.Student.SubmittedAt
		entry.FileItemIDs = append([]string(nil), fileItemIDs...)
		entry.AutoScored, entry.AutoScoredNote = autoScoredTotal(autoItems, row)
		for _, response := range row.Responses {
			id := response.ItemID.String()
			item, ok := prompts[id]
			if !ok || (strings.ToLower(item.Type) != "essay" && strings.ToLower(item.Type) != "file-upload" && strings.ToLower(item.Type) != "file_upload") {
				continue
			}
			var answer string
			if len(response.Answer) > 0 && string(response.Answer) != "null" {
				_ = json.Unmarshal(response.Answer, &answer)
			}
			if strings.ToLower(item.Type) == "file-upload" || strings.ToLower(item.Type) == "file_upload" {
				if answer != "" {
					entry.Files = append(entry.Files, domain.WrittenFile{ItemID: id, Name: answer, Status: "not-retrieved", Reason: "native file result unavailable"})
				}
				continue
			}
			entry.Responses = append(entry.Responses, domain.WrittenResponse{ItemID: id, Prompt: render.HTMLText(item.Body), Type: "essay", HTML: answer, Text: render.HTMLText(answer)})
		}
		out[row.Student.ID] = entry
	}
	return out, nil
}

// autoScoredTotal sums one report row's auto-scored items. Any item without a
// numeric score leaves the total unknown rather than guessed.
func autoScoredTotal(items []writtenQuizItem, row quizReportRow) (*domain.AutoScored, string) {
	if len(items) == 0 {
		return nil, ""
	}
	scores := map[string]string{}
	for _, response := range row.Responses {
		if response.Score != nil {
			scores[response.ItemID.String()] = response.Score.String()
		}
	}
	earned, possible := new(big.Rat), new(big.Rat)
	missing := 0
	for _, item := range items {
		score, ok := scores[item.ID.String()]
		if !ok {
			score, ok = scores[item.Entry.ID.String()]
		}
		d, e := domain.ParseDecimal(score)
		if !ok || e != nil {
			missing++
			continue
		}
		earned.Add(earned, d.Rat())
		possible.Add(possible, domain.MustDecimal(item.Points.String()).Rat())
	}
	if missing > 0 {
		return nil, fmt.Sprintf("%d of %d auto-scored items have no score in Canvas's report", missing, len(items))
	}
	return &domain.AutoScored{Earned: domain.RoundRat(earned, 2), Possible: domain.RoundRat(possible, 2), Items: len(items)}, ""
}

func (s *Service) downloadNewQuizFiles(ctx context.Context, relDir, studentRef string, userID int64, courseID, assignmentID int64, attempt int, row newQuizRow, budget *int64) []domain.WrittenFile {
	files, err := s.Client.NewQuizFiles(ctx, courseID, assignmentID, userID, attempt, sliceSet(row.FileItemIDs))
	if err != nil {
		for i := range row.Files {
			row.Files[i].Status = "not-retrieved"
			row.Files[i].Reason = err.Error()
		}
		return row.Files
	}
	result := append([]domain.WrittenFile(nil), row.Files...)
	used := make([]bool, len(result))
	nameCounts := map[string]int{}
	for _, f := range files {
		base := workspace.SafeFilename(path.Base(strings.ReplaceAll(f.Name, "\\", "/")))
		nameCounts[base]++
		item := s.saveNewQuizFile(ctx, relDir, studentRef, attempt, f, nameCounts[base], budget)
		replaced := false
		for i, old := range result {
			if !used[i] && old.ItemID == f.ItemID && old.Name == f.Name {
				result[i], used[i], replaced = item, true, true
				break
			}
		}
		if !replaced {
			result = append(result, item)
		}
	}
	return result
}

func (s *Service) saveNewQuizFile(ctx context.Context, relDir, studentRef string, attempt int, f canvas.NewQuizFile, duplicate int, budget *int64) domain.WrittenFile {
	item := domain.WrittenFile{ItemID: f.ItemID, Name: f.Name, Size: f.Size, Status: "not-retrieved"}
	if f.Size < 0 || f.Size > 50<<20 {
		item.Reason = "too large"
		return item
	}
	if *budget+f.Size > 500<<20 {
		item.Reason = "pull size limit reached"
		return item
	}
	base := path.Base(strings.ReplaceAll(f.Name, "\\", "/"))
	name := workspace.SafeFilename(base)
	dir := filepath.Join(relDir, "students", studentRef, "files", fmt.Sprintf("attempt-%d", attempt))
	for n := duplicate; ; n++ {
		if n < 1 {
			n = 1
		}
		candidate := name
		if n > 1 {
			ext := filepath.Ext(name)
			candidate = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), n, ext)
		}
		rel := filepath.Join(dir, candidate)
		dest, e := workspace.ConstrainPath(s.Root, rel)
		if e != nil {
			item.Reason = "unsafe destination"
			return item
		}
		st, e := os.Stat(dest)
		if os.IsNotExist(e) {
			item.Path = filepath.ToSlash(rel)
			break
		}
		if e == nil && st.Size() == f.Size {
			item.Path = filepath.ToSlash(rel)
			item.Status = "downloaded"
			return item
		}
	}
	dest, e := workspace.ConstrainPath(s.Root, filepath.FromSlash(item.Path))
	if e != nil {
		item.Path = ""
		item.Reason = "unsafe destination"
		return item
	}
	if e = os.MkdirAll(filepath.Dir(dest), 0755); e != nil {
		item.Path = ""
		item.Reason = "file write failed"
		return item
	}
	partial := dest + ".partial"
	out, e := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		item.Path = ""
		item.Reason = "partial file could not be created"
		return item
	}
	e = s.Client.DownloadUnauthenticated(ctx, f.URL, &limitedWriter{W: out, Limit: f.Size})
	closeErr := out.Close()
	if e == nil {
		e = closeErr
	}
	if e == nil {
		st, se := os.Stat(partial)
		if se != nil || st.Size() != f.Size {
			e = fmt.Errorf("download size mismatch")
		}
	}
	if e == nil {
		e = os.Rename(partial, dest)
	}
	if e != nil {
		_ = os.Remove(partial)
		item.Path = ""
		item.Reason = "download failed"
		return item
	}
	item.Status = "downloaded"
	item.Reason = ""
	*budget += f.Size
	return item
}

type limitedWriter struct {
	W     *os.File
	Limit int64
	Count int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.Count+int64(len(p)) > w.Limit {
		return 0, fmt.Errorf("declared size exceeded")
	}
	n, e := w.W.Write(p)
	w.Count += int64(n)
	return n, e
}
func sliceSet(ids []string) map[string]bool {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func unsupportedNewQuizSubmission(s Submission) string {
	if s.Excused {
		return "excused work skipped"
	}
	if s.AssignmentVisible != nil && !*s.AssignmentVisible {
		return "work not currently assigned to student"
	}
	if s.SubmittedAt == nil || s.Attempt < 1 {
		return "missing work has no actual submission; no automatic zero"
	}
	return ""
}

func canvasAPIPath(c *canvas.Client, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid Canvas report URL")
	}
	base, _ := url.Parse(c.Origin())
	if u.IsAbs() {
		if !strings.EqualFold(u.Scheme, base.Scheme) || !strings.EqualFold(u.Host, base.Host) {
			return "", fmt.Errorf("report progress URL is outside Canvas")
		}
	} else {
		u = base.ResolveReference(u)
	}
	if !strings.HasPrefix(u.Path, "/api/") {
		return "", fmt.Errorf("invalid Canvas report progress path")
	}
	return u.String(), nil
}
