package workflow

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectionImagesDownloadedBesidePacket(t *testing.T) {
	storageReads := 0
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storageReads++
		if r.Header.Get("Authorization") != "" {
			t.Error("storage download carried the Canvas token")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	defer storage.Close()
	canvasDownloads := 0
	var s *Service
	var base string
	s, server := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/files/77":
			fmt.Fprintf(w, `{"url":"%s/files/77/download","display_name":"code.png","filename":"code.png"}`, base)
		case "/files/77/download":
			canvasDownloads++
			if r.Header.Get("Authorization") != "Bearer FIXTURE-ONLY-TOKEN" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, storage.URL+"/signed/77", http.StatusFound)
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
	if canvasDownloads != 1 || storageReads != 1 {
		t.Fatalf("wrong download path: Canvas=%d storage=%d", canvasDownloads, storageReads)
	}
}
