package workflow

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectionImagesDownloadedBesidePacket(t *testing.T) {
	var s *Service
	var base string
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/files/77":
			fmt.Fprintf(w, `{"url":"%s/signed/77","display_name":"code.png","filename":"code.png"}`, base)
		case "/signed/77":
			if r.Header.Get("Authorization") != "" {
				t.Error("signed download carried the Canvas token")
			}
			_, _ = w.Write([]byte("PNGDATA"))
		default:
			http.NotFound(w, r)
		}
	})
	base = server.URL
	html := fmt.Sprintf(`<p>Copy this:</p><img src="%s/courses/1/files/77/preview" alt="code"><img src="%s/courses/1/files/77/download"><img src="https://elsewhere.example/pic.png">`, base, base)
	items := s.downloadDirectionImages(context.Background(), "courses/TEST/assignments/10-x/pulled/1", html)
	if len(items) != 1 || items[0].Status != "downloaded" || items[0].ID != 77 {
		t.Fatalf("unexpected images: %+v", items)
	}
	b, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(items[0].Path)))
	if err != nil || string(b) != "PNGDATA" {
		t.Fatalf("image not saved: %q %v", b, err)
	}
}
