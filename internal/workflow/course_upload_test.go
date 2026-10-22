package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/canvas"
)

type uploadFixtureState struct {
	file                                    canvas.File
	writes, uploads                         int
	wrongFolder, folderLocked, rejectAccess bool
	completion                              func()
	initiate                                func()
}

func uploadWorkflowFixture(t *testing.T) (*Service, string, *uploadFixtureState) {
	t.Helper()
	unlocked, visible := false, false
	state := &uploadFixtureState{file: canvas.File{ID: 9, FolderID: 4, Size: 7, DisplayName: "fixture (1).pdf", Filename: "fixture-1.pdf", ContentType: "application/pdf", Locked: &unlocked, Hidden: &visible, VisibilityLevel: "inherit", UpdatedAt: "2026-10-10T00:00:00Z", UploadStatus: "success"}}
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "GET" {
			state.writes++
		}
		switch r.URL.Path {
		case "/api/v1/courses/1/files":
			if state.initiate != nil {
				state.initiate()
			}
			state.uploads++
			state.file.ID = int64(8 + state.uploads)
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["on_duplicate"] != "rename" {
				t.Error("upload would overwrite")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"upload_url": "https://" + r.Host + "/storage", "upload_params": map[string]string{"signed": "SECRET-POLICY"}})
		case "/storage":
			if r.Header.Get("Authorization") != "" {
				t.Error("PAT leaked into storage")
			}
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("Location", fmt.Sprintf("https://%s/api/v1/files/%d/create_success?uuid=SECRET-UUID", r.Host, state.file.ID))
			w.WriteHeader(http.StatusFound)
		case "/api/v1/courses/1/folders/4":
			id := int64(1)
			if state.wrongFolder {
				id = 2
			}
			_ = json.NewEncoder(w).Encode(courseAssetFolder{ID: 4, ContextID: id, ContextType: "Course", Locked: state.folderLocked})
		default:
			if strings.HasSuffix(r.URL.Path, "/create_success") && state.completion != nil {
				state.completion()
			}
			if r.Method == "PUT" {
				if state.rejectAccess {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				var b struct {
					Locked     bool   `json:"locked"`
					Hidden     bool   `json:"hidden"`
					Visibility string `json:"visibility_level"`
				}
				_ = json.NewDecoder(r.Body).Decode(&b)
				state.file.Locked, state.file.Hidden = &b.Locked, &b.Hidden
					if b.Visibility != "" {
						state.file.VisibilityLevel = b.Visibility
					}
			}
			// Include unsafe response-only values to prove they are never receipts.
			b, _ := json.Marshal(state.file)
			var response map[string]any
			_ = json.Unmarshal(b, &response)
			response["url"] = "https://storage.example/file?signature=SECRET-DOWNLOAD"
			_ = json.NewEncoder(w).Encode(response)
		}
	})
	path := filepath.Join(s.Root, "fixture.pdf")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	return s, path, state
}

func TestCourseUploadReceiptReuseChangedSourceAndExplicitNewUpload(t *testing.T) {
	s, path, state := uploadWorkflowFixture(t)
	opts := CourseUploadOptions{Access: "course"}
	state.completion = func() {
		var rec CourseUploadReceipt
		b, e := os.ReadFile(path + ".cse-upload-test.json")
		if e != nil || json.Unmarshal(b, &rec) != nil || rec.File.ID != 9 {
			t.Error("returned ID not persisted before completion")
		}
	}
	first, e := s.UploadCourseFile(context.Background(), "test", path, opts)
	if e != nil || first.Status != "completed" || first.Receipt.File.ID != 9 {
		t.Fatalf("first upload: %#v %v", first, e)
	}
	b, _ := os.ReadFile(first.Path)
	if strings.Contains(string(b), "SECRET") || strings.Contains(string(b), "FIXTURE-ONLY-TOKEN") || !strings.Contains(string(b), "fixture (1).pdf") {
		t.Errorf("unsafe/incomplete receipt: %s", b)
	}
	writes := state.writes
	second, e := s.UploadCourseFile(context.Background(), "TEST", path, opts)
	if e != nil || second.Status != "completed" || second.Receipt.File.ID != 9 || state.writes != writes {
		t.Fatalf("receipt reuse: %#v writes=%d %v", second, state.writes, e)
	}
	if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	_, e = s.UploadCourseFile(context.Background(), "TEST", path, opts)
	if e == nil || !strings.Contains(e.Error(), "source bytes changed") || state.writes != writes {
		t.Fatal("changed bytes silently uploaded")
	}
	state.completion = nil
	opts.NewUpload = true
	third, e := s.UploadCourseFile(context.Background(), "TEST", path, opts)
	if e != nil || third.Status != "completed" || third.Path == first.Path || third.Receipt.File.ID != 10 {
		t.Fatalf("explicit new upload: %#v %v", third, e)
	}
	old, _ := os.ReadFile(first.Path)
	if string(old) != string(b) {
		// Reuse may refresh its explanatory message but the mapping must stay.
		var rec CourseUploadReceipt
		_ = json.Unmarshal(old, &rec)
		if rec.File.ID != 9 || rec.SHA256 == third.Receipt.SHA256 {
			t.Error("new upload destroyed old mapping")
		}
	}
}

func TestCourseUploadUncertainReceiptIsVerifiedAndAccessRetried(t *testing.T) {
	for _, mode := range []string{"transfer", "access"} {
		t.Run(mode, func(t *testing.T) {
			s, path, state := uploadWorkflowFixture(t)
			opts := CourseUploadOptions{Access: "hidden"}
			if mode == "access" {
				state.rejectAccess = true
			}
			first, e := s.UploadCourseFile(context.Background(), "TEST", path, opts)
			if mode == "access" && e == nil {
				t.Fatal("uncertain access was reported success")
			}
			if mode == "transfer" {
				var rec CourseUploadReceipt
				b, _ := os.ReadFile(first.Path)
				_ = json.Unmarshal(b, &rec)
				rec.Status = "uncertain"
				b, _ = json.Marshal(rec)
				_ = os.WriteFile(first.Path, b, 0600)
			}
			writes := state.writes
			again, e := s.UploadCourseFile(context.Background(), "TEST", path, opts)
			if mode == "transfer" && (e != nil || again.Status != "completed" || state.writes != writes) {
				t.Fatalf("a known file with an unclear receipt should be verified, not uploaded again: %#v writes %d err %v", again, state.writes, e)
			}
			if mode == "access" && e == nil {
				t.Fatalf("access is still rejected, so the retry must still report failure: %#v", again)
			}
		})
	}
}

func TestCourseUploadCompletedFileIsReportedAsCanvasHasItWithoutWrite(t *testing.T) {
	for _, mode := range []string{"size", "filename", "modified", "access", "availability", "wrong-course"} {
		t.Run(mode, func(t *testing.T) {
			s, path, state := uploadWorkflowFixture(t)
			opts := CourseUploadOptions{Access: "course"}
			if _, e := s.UploadCourseFile(context.Background(), "TEST", path, opts); e != nil {
				t.Fatal(e)
			}
			writes := state.writes
			switch mode {
			case "size":
				state.file.Size = 8
			case "filename":
				state.file.Filename = "teacher-renamed.pdf"
			case "modified":
				state.file.UpdatedAt = "2026-10-11T00:00:00Z"
			case "access":
				state.file.VisibilityLevel = "public"
			case "availability":
				state.file.UploadStatus = "deleted"
			case "wrong-course":
				state.wrongFolder = true
			}
			for n := 0; n < 2; n++ {
				got, e := s.UploadCourseFile(context.Background(), "TEST", path, opts)
				if mode == "wrong-course" {
					if e == nil || got.Status != "held" || state.writes != writes {
						t.Fatalf("a file outside the course must not be used: %#v err=%v writes=%d", got, e, state.writes)
					}
				} else if e != nil || got.Status != "completed" || state.writes != writes {
					t.Fatalf("a changed file should be reported as Canvas has it, with no writes: %#v err=%v writes=%d", got, e, state.writes)
				}
			}
		})
	}
}

func TestCourseUploadChangedDuringTransferStillCompletesAndInvalidReceiptDoesNotReplay(t *testing.T) {
	s, path, state := uploadWorkflowFixture(t)
	state.initiate = func() {
		if e := os.WriteFile(path, []byte("changed"), 0600); e != nil {
			t.Error(e)
		}
	}
	report, e := s.UploadCourseFile(context.Background(), "TEST", path, CourseUploadOptions{Access: "course"})
	if e != nil || report.Status != "completed" || report.Receipt.File.ID != 9 {
		t.Fatalf("a source edited during transfer should still complete with the known ID: %#v %v", report, e)
	}
	writes := state.writes
	state.initiate = nil
	// A completed receipt with a lost ID is malformed, not permission to create.
	report.Receipt.Status = "completed"
	report.Receipt.File.ID = 0
	b, _ := json.Marshal(report.Receipt)
	if e = os.WriteFile(report.Path, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.UploadCourseFile(context.Background(), "TEST", path, CourseUploadOptions{Access: "course"}); e == nil || state.writes != writes {
		t.Fatal("malformed known receipt caused new upload")
	}
}

func TestCourseUploadDestinationPrivacyAndSourceBoundaries(t *testing.T) {
	for _, mode := range []string{"wrong-folder", "locked-no-folder", "locked-unlocked-folder", "missing-access", "source-receipt", "source-case-receipt"} {
		t.Run(mode, func(t *testing.T) {
			s, path, state := uploadWorkflowFixture(t)
			opts := CourseUploadOptions{Access: "course"}
			switch mode {
			case "wrong-folder":
				opts.FolderID = 4
				state.wrongFolder = true
			case "locked-no-folder":
				opts.Access = "locked"
			case "locked-unlocked-folder":
				opts.Access = "locked"
				opts.FolderID = 4
			case "missing-access":
				opts.Access = ""
			case "source-receipt":
				opts.ReceiptPath = path
			case "source-case-receipt":
				// SameFile also protects an alternate spelling of the source on Windows.
				opts.ReceiptPath = strings.ToUpper(path)
				if _, e := os.Stat(opts.ReceiptPath); e != nil {
					t.Skip("case-sensitive filesystem")
				}
			}
			_, e := s.UploadCourseFile(context.Background(), "TEST", path, opts)
			if e == nil || state.writes != 0 {
				t.Fatalf("unsafe upload boundary err=%v writes=%d", e, state.writes)
			}
			b, _ := os.ReadFile(path)
			if string(b) != "fixture" {
				t.Error("source overwritten")
			}
		})
	}
	s, path, state := uploadWorkflowFixture(t)
	state.folderLocked = true
	report, e := s.UploadCourseFile(context.Background(), "TEST", path, CourseUploadOptions{Access: "locked", FolderID: 4})
	if e != nil || report.Status != "completed" || report.Receipt.File.Locked == nil || !*report.Receipt.File.Locked {
		t.Fatalf("locked upload: %#v %v", report, e)
	}
}
