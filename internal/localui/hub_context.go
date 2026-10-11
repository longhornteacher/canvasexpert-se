package localui

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

// maxContextText bounds each teacher-written box so a stray paste cannot bloat
// the agent's always-read settings files.
const maxContextText = 8000

const gradingBody = "Edited from the CE-SE hub Settings page. `voice` is how feedback should sound; `feedback_guidance` holds grading habits and rubric preferences. Put student-specific notes in `students.md`. Never store a Canvas token here.\n"

const notesBody = "Edited from the CE-SE hub Students page. Each note names a student by stable `student_ref` (or an unambiguous `name`, with optional `course`). Notes are private teacher context and must never be copied into student-facing feedback.\n"

func gradingPath(root string) string { return filepath.Join(root, "context", "grading.md") }
func notesPath(root string) string   { return filepath.Join(root, "context", "students.md") }

// readGrading tolerates a missing file so a fresh workspace still shows the form.
func readGrading(root string) (domain.GradingConfig, string, error) {
	config := domain.GradingConfig{Schema: 1}
	body, err := workspace.ReadDocument(gradingPath(root), &config)
	if errors.Is(err, os.ErrNotExist) {
		return domain.GradingConfig{Schema: 1}, gradingBody, nil
	}
	return config, body, err
}

func readNotes(root string) (domain.StudentNotes, string, error) {
	notes := domain.StudentNotes{Schema: 1}
	body, err := workspace.ReadDocument(notesPath(root), &notes)
	if errors.Is(err, os.ErrNotExist) {
		return domain.StudentNotes{Schema: 1}, notesBody, nil
	}
	return notes, body, err
}

// hubNote is the note the hub edits for one student: referenced by stable ref
// and not scoped to a course, so it follows the student everywhere.
func hubNote(n domain.StudentNote, ref string) bool {
	return n.StudentRef == ref && n.Course == ""
}

// notesByRef gathers each student's hub-editable note text.
func notesByRef(notes domain.StudentNotes) map[string]string {
	out := map[string]string{}
	for _, n := range notes.Notes {
		if n.StudentRef != "" && n.Course == "" {
			if prev := out[n.StudentRef]; prev != "" {
				out[n.StudentRef] = prev + "\n\n" + n.Note
			} else {
				out[n.StudentRef] = n.Note
			}
		}
	}
	return out
}

// cleanText normalizes browser line endings and trims; ok is false when too long.
func cleanText(raw string) (string, bool) {
	text := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	return text, len([]rune(text)) <= maxContextText && !strings.ContainsRune(text, 0)
}

// saveNotes replaces the hub notes for the edited students and leaves every
// other note (name-based or course-scoped, written by hand) untouched.
func saveNotes(root string, edited map[string]string) error {
	notes, body, err := readNotes(root)
	if err != nil {
		return err
	}
	kept := notes.Notes[:0:0]
	for _, n := range notes.Notes {
		if _, changing := edited[n.StudentRef]; changing && hubNote(n, n.StudentRef) {
			continue
		}
		kept = append(kept, n)
	}
	for ref, text := range edited {
		if text != "" {
			kept = append(kept, domain.StudentNote{StudentRef: ref, Note: text})
		}
	}
	notes.Notes = kept
	return workspace.WriteDocument(notesPath(root), &notes, body)
}

func (h *Hub) saveGrading(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) || h.needWorkspace(w) {
		return
	}
	config, body, err := readGrading(h.root)
	if err != nil {
		h.redirect(w, r, "/settings", "", "Could not read context/grading.md: "+err.Error())
		return
	}
	voice, okVoice := cleanText(r.PostForm.Get("voice"))
	guidance, okGuidance := cleanText(r.PostForm.Get("feedback_guidance"))
	if !okVoice || !okGuidance {
		h.redirect(w, r, "/settings", "", "Each box holds up to 8,000 characters.")
		return
	}
	config.Schema, config.Voice, config.FeedbackGuidance = 1, voice, guidance
	if err = workspace.WriteDocument(gradingPath(h.root), &config, body); err != nil {
		h.redirect(w, r, "/settings", "", "Could not save context/grading.md: "+err.Error())
		return
	}
	h.redirect(w, r, "/settings", "Feedback voice saved. The agent uses it next time it writes feedback.", "")
}

// whoami asks Canvas which account the saved token belongs to.
func (h *Hub) whoami(r *http.Request) (string, error) {
	if h.whoamiFn != nil {
		return h.whoamiFn(r.Context())
	}
	c, err := h.canvasClient()
	if err != nil {
		return "", err
	}
	ctx, cancel := h.reqCtx()
	defer cancel()
	var user workflow.CanvasUser
	if err = c.Get(ctx, "/api/v1/users/self", &user); err != nil {
		return "", err
	}
	return user.Name, nil
}

func (h *Hub) testCanvas(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	name, err := h.whoami(r)
	if err != nil {
		h.canvasUser = ""
		var ce *canvas.Error
		switch {
		case errors.As(err, &ce) && ce.Kind == "authentication":
			h.redirect(w, r, "/settings", "", "Canvas rejected the saved token. It may have expired or been deleted in Canvas. Use Replace token to connect again.")
		case errors.As(err, &ce):
			h.redirect(w, r, "/settings", "", "Canvas test failed: "+ce.Error()+".")
		default:
			h.redirect(w, r, "/settings", "", "Canvas test failed: "+err.Error())
		}
		return
	}
	h.canvasUser = name
	h.redirect(w, r, "/settings", "Canvas works. The token belongs to "+name+".", "")
}

func (h *Hub) forgetCanvas(w http.ResponseWriter, r *http.Request) {
	if !h.checkPost(w, r) {
		return
	}
	if r.PostForm.Get("confirm") != "on" {
		h.redirect(w, r, "/settings", "", "Tick the box to confirm before removing the token.")
		return
	}
	forget := h.forgetFn
	if forget == nil {
		forget = platform.DeleteCredential
	}
	if err := forget(); err != nil {
		h.redirect(w, r, "/settings", "", "Windows could not remove the token: "+err.Error())
		return
	}
	h.canvasUser = ""
	h.redirect(w, r, "/settings", "Token removed from this computer. Your CE-SE files are untouched.", "")
}
