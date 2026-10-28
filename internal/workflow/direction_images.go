package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

const maxDirectionImages = 10

var (
	imgTag      = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	imgSrc      = regexp.MustCompile(`(?is)\bsrc\s*=\s*["']([^"']+)["']`)
	imgFileLink = regexp.MustCompile(`/files/(\d+)`)
)

// downloadDirectionImages saves Canvas-hosted images embedded in an assignment's
// directions (for example a reference image students must reproduce) beside the
// packet, so the agent can read them with read_workspace_asset. Failures are
// recorded on the entry, never fatal: the agent then holds what it cannot check.
func (s *Service) downloadDirectionImages(ctx context.Context, relDir, descriptionHTML string) []domain.Attachment {
	var items []domain.Attachment
	seen := map[int64]bool{}
	for _, tag := range imgTag.FindAllString(descriptionHTML, -1) {
		src := imgSrc.FindStringSubmatch(tag)
		if src == nil {
			continue
		}
		m := imgFileLink.FindStringSubmatch(src[1])
		if m == nil {
			continue
		}
		id, _ := strconv.ParseInt(m[1], 10, 64)
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		if len(items) >= maxDirectionImages {
			break
		}
		items = append(items, s.downloadDirectionImage(ctx, relDir, id))
	}
	return items
}

func (s *Service) downloadDirectionImage(ctx context.Context, relDir string, id int64) domain.Attachment {
	item := domain.Attachment{ID: id, Name: fmt.Sprint(id), Status: "failed"}
	var file struct {
		URL         string `json:"url"`
		DisplayName string `json:"display_name"`
		Filename    string `json:"filename"`
	}
	if e := s.Client.Get(ctx, fmt.Sprintf("/api/v1/files/%d", id), &file); e != nil || file.URL == "" {
		item.Reason = "Canvas did not return the image file"
		return item
	}
	item.Name = file.Filename
	if item.Name == "" {
		item.Name = file.DisplayName
	}
	if item.Name == "" {
		item.Name = fmt.Sprint(id)
	}
	rel := filepath.Join(relDir, "directions", fmt.Sprintf("%d-%s", id, workspace.SafeFilename(item.Name)))
	path, e := workspace.ConstrainPath(s.Root, rel)
	if e == nil {
		e = os.MkdirAll(filepath.Dir(path), 0755)
	}
	var f *os.File
	if e == nil {
		f, e = os.CreateTemp(filepath.Dir(path), ".cse-download-*")
	}
	if e != nil {
		item.Reason = e.Error()
		return item
	}
	tmp := f.Name()
	e = s.Client.Download(ctx, file.URL, f)
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(tmp, path)
	}
	if e != nil {
		os.Remove(tmp)
		item.Reason = e.Error()
		return item
	}
	item.Path = filepath.ToSlash(rel)
	item.Status = "downloaded"
	return item
}
