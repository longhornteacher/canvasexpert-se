package doctor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "playbooks"), 0755); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		"AGENTS.md":          "Read the task playbook.",
		"playbooks/setup.md": "Setup guidance.",
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeCourses(t *testing.T, root string, courses []domain.Course) {
	t.Helper()
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: courses}, ""); err != nil {
		t.Fatal(err)
	}
}

func passedLoad() (platform.Credential, error) {
	return platform.Credential{Origin: "https://school.example", Token: "synthetic"}, nil
}

func passedSelfTest() error { return nil }

func find(r Report, name string) Check {
	for _, check := range r.Checks {
		if check.Name == name {
			return check
		}
	}
	return Check{}
}

func TestDoctorValidEmptyInstallDistinguishesUnconfiguredCourses(t *testing.T) {
	root := fixture(t)
	writeCourses(t, root, nil)
	r := run(root, os.Args[0], "synthetic", passedLoad, passedSelfTest)
	if r.Failed {
		t.Fatalf("unexpected failure: %+v", r.Checks)
	}
	if got := find(r, "course_config").Status; got != "not_configured" {
		t.Fatalf("empty courses status = %s", got)
	}
	if got := find(r, "saved_origin").Status; got != "pass" {
		t.Fatalf("origin status = %s", got)
	}
	if len(r.Checks) != 8 {
		t.Fatalf("got %d checks, want fixed eight", len(r.Checks))
	}
}

func TestDoctorAuthenticatedCanvasCheckCategoriesAndPrivacy(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"success", 200, `{"id":42,"name":"private account name"}`, "pass"},
		{"rejected", 401, `{"errors":["private response"]}`, "fail"},
		{"outage", 503, `{"message":"private outage details"}`, "not_verified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/users/self" || r.Header.Get("Authorization") != "Bearer synthetic-token" {
					t.Errorf("unexpected request: %s %q", r.URL.Path, r.Header.Get("Authorization"))
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			old := http.DefaultTransport
			http.DefaultTransport = srv.Client().Transport
			defer func() { http.DefaultTransport = old }()
			err := authenticatedCheck(srv.URL, "synthetic-token")
			var load = func() (platform.Credential, error) {
				return platform.Credential{Origin: srv.URL, Token: "synthetic-token"}, nil
			}
			r := runMode(fixture(t), os.Args[0], "test", false, load, passedSelfTest, func(string, string) error { return err }, func(string) bool { return true })
			if got := find(r, "canvas_connection").Status; got != tc.want {
				t.Fatalf("status=%s, want %s (err %v)", got, tc.want, err)
			}
			b, _ := json.Marshal(r)
			if strings.Contains(string(b), "private account name") || strings.Contains(string(b), "private response") || strings.Contains(string(b), "private outage details") || strings.Contains(string(b), "synthetic-token") {
				t.Fatalf("report leaked private data: %s", b)
			}
		})
	}
}

func TestDoctorOfflineSkipsNetworkChecksAndReportsHubStates(t *testing.T) {
	root := fixture(t)
	writeCourses(t, root, nil)
	loadCalls, liveCalls, hubCalls := 0, 0, 0
	load := func() (platform.Credential, error) {
		loadCalls++
		return platform.Credential{Origin: "https://school.example", Token: "synthetic"}, nil
	}
	live := func(string, string) error { liveCalls++; return nil }
	hub := func(string) bool { hubCalls++; return true }
	r := runMode(root, os.Args[0], "test", true, load, passedSelfTest, live, hub)
	if loadCalls != 1 || liveCalls != 0 || hubCalls != 0 || find(r, "canvas_connection").Status != "not_verified" || find(r, "hub").Status != "not_verified" {
		t.Fatalf("offline made/check failed: calls %d/%d/%d, %+v", loadCalls, liveCalls, hubCalls, r.Checks)
	}
	for _, tc := range []struct {
		name    string
		healthy bool
		want    string
	}{{"healthy", true, "pass"}, {"stale", false, "not_verified"}, {"absent", false, "not_verified"}} {
		t.Run(tc.name, func(t *testing.T) {
			got := runMode(root, os.Args[0], "test", false, load, passedSelfTest, func(string, string) error { return nil }, func(string) bool { return tc.healthy })
			if find(got, "hub").Status != tc.want {
				t.Fatalf("hub=%+v", find(got, "hub"))
			}
		})
	}
}

func TestDoctorMalformedCoursesDoesNotStopIndependentChecks(t *testing.T) {
	root := fixture(t)
	if err := os.MkdirAll(filepath.Join(root, "context"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("not a course document"), 0644); err != nil {
		t.Fatal(err)
	}
	r := run(root, os.Args[0], "synthetic", passedLoad, passedSelfTest)
	if !r.Failed || find(r, "course_config").Status != "fail" || find(r, "saved_origin").Status != "not_verified" || find(r, "credential_round_trip").Status != "pass" {
		t.Fatalf("independent results missing: %+v", r)
	}
}

func TestDoctorUnavailableConfiguredSyncReportsRulesFailure(t *testing.T) {
	root := fixture(t)
	writeCourses(t, root, []domain.Course{{Nickname: "ELA", ID: 1}})
	syncRoot := filepath.Join(t.TempDir(), "offline-sync")
	if _, err := workspace.SetRulesLocation(root, syncRoot, nil, ""); err == nil {
		t.Fatal("expected missing Sync directory to be rejected")
	}
	// Bind directly as an existing configured Install would be bound; doctor
	// must report the unavailable authority rather than use local copies.
	if err := os.MkdirAll(filepath.Join(root, ".cse"), 0700); err != nil {
		t.Fatal(err)
	}
	binding := `{"sync_root":"` + strings.ReplaceAll(syncRoot, `\`, `\\`) + `"}`
	if err := os.WriteFile(filepath.Join(root, ".cse", "rules-location.json"), []byte(binding), 0600); err != nil {
		t.Fatal(err)
	}
	r := run(root, os.Args[0], "synthetic", passedLoad, passedSelfTest)
	if find(r, "active_rules").Status != "fail" || find(r, "course_config").Status != "pass" || find(r, "saved_origin").Status != "pass" {
		t.Fatalf("unavailable Sync did not remain an independent result: %+v", r)
	}
}

func TestDoctorOriginMismatchNeverIncludesToken(t *testing.T) {
	root := fixture(t)
	writeCourses(t, root, []domain.Course{{Nickname: "ELA", ID: 1}})
	load := func() (platform.Credential, error) {
		return platform.Credential{Origin: "https://different.example", Token: "secret-synthetic-token"}, nil
	}
	r := run(root, os.Args[0], "synthetic", load, passedSelfTest)
	encoded, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if find(r, "saved_origin").Status != "fail" || strings.Contains(string(encoded), "secret-synthetic-token") {
		t.Fatalf("origin result leaked or was not flagged: %s", encoded)
	}
}
