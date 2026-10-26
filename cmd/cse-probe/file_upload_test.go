package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/canvas"
)

func syntheticProbeFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic-response.txt")
	if e := os.WriteFile(path, []byte("synthetic evidence"), 0600); e != nil {
		t.Fatal(e)
	}
	return path
}

func uploadFixtureResponse(r *http.Request) *http.Response {
	if strings.HasSuffix(r.URL.Path, "student_view_student") {
		return response(r, `{"id":5}`)
	}
	return response(r, `{"id":42,"course_id":900001,"name":"[CSE 0.1b DISPOSABLE] synthetic","published":false,"points_possible":20,"grading_type":"points","submission_types":["online_upload"]}`)
}

func TestFileUploadStorageOmitsTokenAndFinalizesExactCanvasFile(t *testing.T) {
	storageCalls, submissions, finalizations := 0, 0, 0
	fn := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "storage.example" {
			storageCalls++
			if r.Header.Get("Authorization") != "" || r.Method != "POST" {
				t.Fatal("storage received credential or wrong method")
			}
			reader, e := r.MultipartReader()
			if e != nil {
				t.Fatal(e)
			}
			var names []string
			for {
				part, e := reader.NextPart()
				if e == io.EOF {
					break
				}
				if e != nil {
					t.Fatal(e)
				}
				names = append(names, part.FormName())
				b, _ := io.ReadAll(part)
				if part.FormName() == "policy" && string(b) != "signed-policy" || part.FormName() == "file" && string(b) != "synthetic evidence" {
					t.Fatal("signed parameter or file bytes changed")
				}
			}
			if strings.Join(names, ",") != "key,policy,file" {
				t.Fatal("file was not last", names)
			}
			out := response(r, "")
			out.StatusCode = 301
			out.Header.Set("Location", allowedOrigin+"/api/v1/files/99/create_success?uuid=signed-fixture")
			return out, nil
		}
		if r.Header.Get("Authorization") != "Bearer SYNTHETIC-ONLY-SECRET" {
			t.Fatal("Canvas credential missing")
		}
		if strings.HasSuffix(r.URL.Path, "/create_success") {
			finalizations++
			if r.Method != "GET" || r.URL.Query().Get("as_user_id") != "5" {
				t.Fatal("invalid finalization request")
			}
			return response(r, `{"id":99,"size":18}`), nil
		}
		if r.Method == "POST" {
			if r.URL.Query().Get("as_user_id") != "5" {
				t.Fatal("file write omitted exact Test Student impersonation")
			}
			if strings.HasSuffix(r.URL.Path, "/files") {
				if r.URL.Path != "/api/v1/courses/900001/assignments/42/submissions/5/files" {
					t.Fatal("wrong initiation scope")
				}
				return response(r, `{"upload_url":"https://storage.example/upload","upload_params":{"key":"fixture","policy":"signed-policy"}}`), nil
			}
			submissions++
			var body struct {
				Submission struct {
					Type  string  `json:"submission_type"`
					Files []int64 `json:"file_ids"`
				} `json:"submission"`
			}
			if e := json.NewDecoder(r.Body).Decode(&body); e != nil || body.Submission.Type != "online_upload" || len(body.Submission.Files) != 1 || body.Submission.Files[0] != 99 {
				t.Fatal("wrong file submission payload")
			}
			return response(r, `{"id":77,"user_id":5,"assignment_id":42,"attempt":1,"submission_type":"online_upload","attachments":[{"id":99}]}`), nil
		}
		if strings.HasSuffix(r.URL.Path, "/submissions/5") {
			return response(r, `{"id":77,"user_id":5,"assignment_id":42,"attempt":1,"submission_type":"online_upload","attachments":[{"id":99}]}`), nil
		}
		return uploadFixtureResponse(r), nil
	})
	p := fixtureProbe(t, fn)
	old := http.DefaultTransport
	http.DefaultTransport = fn
	t.Cleanup(func() { http.DefaultTransport = old })
	args := []string{"--file", syntheticProbeFile(t)}
	if e := p.submitFile(context.Background(), args); e != nil {
		t.Fatal(e)
	}
	if e := p.submitFile(context.Background(), args); e != nil {
		t.Fatal(e)
	}
	if storageCalls != 1 || submissions != 1 || finalizations != 1 {
		t.Fatal("successful file submission replayed", storageCalls, submissions, finalizations)
	}
	b, e := os.ReadFile(p.path)
	if e != nil || strings.Contains(string(b), "signed-policy") || strings.Contains(string(b), "signed-fixture") || strings.Contains(string(b), "SYNTHETIC-ONLY-SECRET") {
		t.Fatal("upload authorization persisted")
	}
}

func TestDeniedFileInitiationIsRecordedAndNotReplayed(t *testing.T) {
	writes := 0
	p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" {
			writes++
			out := response(r, `{}`)
			out.StatusCode = 403
			return out, nil
		}
		return uploadFixtureResponse(r), nil
	})
	args := []string{"--file", syntheticProbeFile(t)}
	if p.submitFile(context.Background(), args) == nil || p.submitFile(context.Background(), args) == nil || writes != 1 {
		t.Fatal("permission denial was hidden or replayed")
	}
	for key, outcome := range p.state.Outcomes {
		if strings.HasPrefix(key, "file-upload:") && outcome.Status != "failed" {
			t.Fatal("denial not persisted")
		}
	}
}

func TestFailedUploadConfirmationRetainsKnownFileIDAndHoldsReplay(t *testing.T) {
	writes, storageCalls := 0, 0
	fn := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "storage.example" {
			storageCalls++
			out := response(r, "")
			out.StatusCode = 301
			out.Header.Set("Location", allowedOrigin+"/api/v1/files/99/create_success?uuid=fixture")
			return out, nil
		}
		if strings.HasSuffix(r.URL.Path, "/create_success") {
			out := response(r, `{}`)
			out.StatusCode = 403
			return out, nil
		}
		if r.Method == "POST" {
			writes++
			return response(r, `{"upload_url":"https://storage.example/upload","upload_params":{}}`), nil
		}
		return uploadFixtureResponse(r), nil
	})
	p := fixtureProbe(t, fn)
	old := http.DefaultTransport
	http.DefaultTransport = fn
	t.Cleanup(func() { http.DefaultTransport = old })
	args := []string{"--file", syntheticProbeFile(t)}
	if p.submitFile(context.Background(), args) == nil || p.submitFile(context.Background(), args) == nil || writes != 1 || storageCalls != 1 {
		t.Fatal("failed confirmation replayed upload")
	}
	for key, outcome := range p.state.Outcomes {
		if strings.HasPrefix(key, "file-upload:") && (outcome.Status != "uncertain" || outcome.ID != 99) {
			t.Fatal("pending file identity not preserved", outcome)
		}
	}
}

func TestFileUploadTransportFailureIsUncertainAndRedirectIsNotFollowed(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("sensitive storage URL omitted")
	})}
	directive := probeUploadDirective{URL: "https://storage.example/upload", Params: map[string]json.RawMessage{}}
	_, _, e := sendProbeFile(context.Background(), client, directive, "synthetic.txt", "text/plain", []byte("x"))
	var ce *canvas.Error
	if !errors.As(e, &ce) || !ce.Uncertain || strings.Contains(e.Error(), "sensitive") {
		t.Fatal("transport failure was not safely uncertain")
	}
}

func TestUploadFinalizationRejectsWrongOriginPathAndStudent(t *testing.T) {
	for _, raw := range []string{"https://storage.example/api/v1/files/99/create_success", allowedOrigin + "/api/v1/courses/900001/assignments", allowedOrigin + "/api/v1/files/99/create_success?as_user_id=6", allowedOrigin + "/api/v1/files/99/create_success?as_user_id=5&as_user_id=6", "http://pearland.instructure.com/api/v1/files/99", allowedOrigin + "/api/v1/files/0"} {
		if _, e := probeFinalizePath(raw, 5); e == nil {
			t.Fatal("accepted unsafe finalization", raw)
		}
	}
	path, e := probeFinalizePath(allowedOrigin+"/api/v1/files/99/create_success?uuid=fixture", 5)
	u, _ := url.Parse(path)
	if e != nil || u.Query().Get("as_user_id") != "5" || u.Query().Get("uuid") != "fixture" {
		t.Fatal("valid finalization changed signed confirmation")
	}
}
