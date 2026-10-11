package canvas

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCourseUploadStorageIsolationMultipartAndCompletion(t *testing.T) {
	for _, response := range []string{"redirect", "created-location", "direct-json", "same-origin"} {
		t.Run(response, func(t *testing.T) {
			var canvasURL string
			known := false
			storageCalls, completions := 0, 0
			storageHandler := func(w http.ResponseWriter, r *http.Request) {
				storageCalls++
				if r.Header.Get("Authorization") != "" {
					t.Error("PAT sent to storage")
				}
				reader, e := r.MultipartReader()
				if e != nil {
					t.Fatal(e)
				}
				var names []string
				for {
					p, e := reader.NextPart()
					if e == io.EOF {
						break
					}
					if e != nil {
						t.Fatal(e)
					}
					names = append(names, p.FormName())
					b, _ := io.ReadAll(p)
					if p.FormName() == "file" && string(b) != "fixture" {
						t.Error("wrong bytes")
					}
					if p.FormName() == "signed" && string(b) != "opaque-policy" {
						t.Error("changed upload parameters")
					}
				}
				if strings.Join(names, ",") != "signed,file" {
					t.Errorf("file must be last: %v", names)
				}
				if response == "direct-json" {
					_, _ = io.WriteString(w, `{"id":9,"size":7,"url":"https://signed.example/?SECRET=1"}`)
					return
				}
				w.Header().Set("Location", canvasURL+"/api/v1/files/9/create_success?uuid=SECRET")
				if response == "created-location" {
					w.WriteHeader(http.StatusCreated)
				} else {
					w.WriteHeader(http.StatusMovedPermanently)
				}
			}
			storage := httptest.NewTLSServer(http.HandlerFunc(storageHandler))
			defer storage.Close()
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/storage" {
					storageHandler(w, r)
					return
				}
				if r.Header.Get("Authorization") != "Bearer SYNTHETIC-PAT" {
					t.Error("Canvas request missing PAT")
				}
				if r.URL.Path == "/api/v1/courses/1/files" {
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body["on_duplicate"] != "rename" || body["parent_folder_id"] != float64(4) || body["size"] != float64(7) {
						t.Errorf("wrong initiation: %v", body)
					}
					u := storage.URL
					if response == "same-origin" {
						u = canvasURL + "/storage"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"upload_url": u, "upload_params": map[string]string{"signed": "opaque-policy"}})
					return
				}
				completions++
				if !known {
					t.Error("completion before returned ID persistence")
				}
				if r.Method != "GET" || r.URL.Path != "/api/v1/files/9/create_success" {
					t.Errorf("wrong completion %s %s", r.Method, r.URL.Path)
				}
				_, _ = io.WriteString(w, `{"id":9,"size":7,"url":"https://signed.example/?SECRET=1"}`)
			}))
			defer server.Close()
			canvasURL = server.URL
			c := testClient(t, server)
			c.limit = 1 // completing must release the storage request's slot
			file, e := c.UploadCourseFile(context.Background(), CourseFileUpload{CourseID: 1, FolderID: 4, Name: "fixture.pdf", ContentType: "application/pdf", Size: 7, Source: strings.NewReader("fixture"), Known: func(f File) error { known = f.ID == 9; return nil }})
			if e != nil || file.ID != 9 || !known || storageCalls != 1 {
				t.Fatalf("upload: %#v calls=%d err=%v", file, storageCalls, e)
			}
			if response != "direct-json" && completions != 1 {
				t.Errorf("completion count %d", completions)
			}
			b, _ := json.Marshal(file)
			if strings.Contains(string(b), "SECRET") {
				t.Error("signed URL persisted")
			}
		})
	}
}

func TestCourseUploadRejectsCompletionEscapeAndRetainsKnownID(t *testing.T) {
	for _, mode := range []string{"external", "impersonation", "wrong-path", "wrong-id-response", "lost-response"} {
		t.Run(mode, func(t *testing.T) {
			known := int64(0)
			writes := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/courses/1/files":
					writes++
					_ = json.NewEncoder(w).Encode(map[string]any{"upload_url": "https://" + r.Host + "/storage", "upload_params": map[string]string{}})
				case "/storage":
					writes++
					if r.Header.Get("Authorization") != "" {
						t.Error("PAT at storage")
					}
					_, _ = io.Copy(io.Discard, r.Body)
					location := "https://" + r.Host + "/api/v1/files/9/create_success"
					if mode == "external" {
						location = "https://evil.example/api/v1/files/9/create_success"
					}
					if mode == "impersonation" {
						location += "?as_user_id=5"
					}
					if mode == "wrong-path" {
						location = "https://" + r.Host + "/api/v1/courses/2/files"
					}
					w.Header().Set("Location", location)
					w.WriteHeader(http.StatusFound)
				default:
					if mode == "lost-response" {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					_, _ = io.WriteString(w, `{"id":10,"size":7}`)
				}
			}))
			defer server.Close()
			c := testClient(t, server)
			file, e := c.UploadCourseFile(context.Background(), CourseFileUpload{CourseID: 1, Name: "fixture", Size: 7, Source: strings.NewReader("fixture"), Known: func(f File) error { known = f.ID; return nil }})
			requireKind(t, e, "uncertain")
			if writes != 2 {
				t.Errorf("write replay: %d", writes)
			}
			if mode == "wrong-id-response" || mode == "lost-response" {
				if known != 9 || file.ID != 9 {
					t.Error("lost known completion ID")
				}
			} else if known != 0 {
				t.Error("accepted escaped completion")
			}
		})
	}
}
