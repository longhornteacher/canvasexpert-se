package localui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

func newTestHub() *Hub {
	h := &Hub{origin: "http://127.0.0.1:4444", secret: "nonce", quit: make(chan struct{})}
	h.loadCredentialFn = func() (platform.Credential, error) {
		return platform.Credential{}, platform.ErrNoCredential
	}
	h.setup = &setupHandler{origin: h.origin, secret: h.secret, persistent: true, hub: true, done: make(chan struct{})}
	return h
}

func hubPost(h *Hub, path string, form url.Values, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, h.origin+path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", origin)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func makeWorkspace(t *testing.T) string {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "context"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "context", "courses.md"), []byte("---\nschema: 1\ninstance: \"\"\ncourses: []\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestHubRejectsForeignHostAndServesNavigation(t *testing.T) {
	h := newTestHub()
	r := httptest.NewRequest(http.MethodGet, "http://rebind.example/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal("foreign Host accepted")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/", nil))
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, `href="/courses"`) || !strings.Contains(body, `href="/settings"`) || !strings.Contains(body, `href="/how"`) || !strings.Contains(body, `href="/" aria-current="page">Home</a>`) || strings.Contains(body, "SE = ") || strings.Contains(body, "/pokemonify") {
		t.Fatal("home page incomplete")
	}
}

func TestHubFormsNeedOriginAndSecret(t *testing.T) {
	h := newTestHub()
	for _, path := range []string{"/settings", "/quit", "/rules-location"} {
		if w := hubPost(h, path, url.Values{"request_secret": {"nonce"}, "path": {"x"}}, "http://attacker.example"); w.Code != 403 {
			t.Fatalf("%s cross-origin allowed", path)
		}
		if w := hubPost(h, path, url.Values{"request_secret": {"wrong"}, "path": {"x"}}, h.origin); w.Code != 403 {
			t.Fatalf("%s wrong secret allowed", path)
		}
	}
	select {
	case <-h.quit:
		t.Fatal("unauthenticated quit")
	default:
	}
}

func TestUnavailableSyncShowsPathAndKeepsSettingsCorrectionForm(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	syncRoot := t.TempDir()
	if _, err := workspace.SetRulesLocation(h.root, syncRoot, nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(syncRoot, syncRoot+"-offline"); err != nil {
		t.Fatal(err)
	}
	page := hubGet(h, "/settings?section=connections").Body.String()
	if !strings.Contains(page, "Unavailable") || !strings.Contains(page, syncRoot) || !strings.Contains(page, `name="sync_root" value="`+syncRoot+`"`) {
		t.Fatal("unavailable UserSync path or correction control is missing")
	}
	home := hubGet(h, "/").Body.String()
	if !strings.Contains(home, "Unavailable") || !strings.Contains(home, syncRoot) || !strings.Contains(home, "old copy instead") {
		t.Fatal("home summary must show unavailable UserSync and its configured path")
	}
	if strings.Contains(home, "This PC only") {
		t.Fatal("home summary must not imply fallback to local rules")
	}
	test := hubPost(h, "/rules/test", url.Values{"request_secret": {h.secret}, "sync_root": {t.TempDir()}}, h.origin)
	result := hubGet(h, test.Header().Get("Location")).Body.String()
	if !strings.Contains(result, "Folder check: Unavailable") || !strings.Contains(result, syncRoot) || !strings.Contains(result, "old copy instead") {
		t.Fatal("testing an unavailable UserSync folder must preserve its saved authority and explain the failure")
	}
	if _, _, err := readGrading(h.root); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("grading read fell back to stale UserInstall copy: %v", err)
	}
	if w := hubPost(h, "/rules-location", url.Values{"request_secret": {"wrong"}, "sync_root": {""}}, h.origin); w.Code != 403 {
		t.Fatal("rules location accepted wrong secret")
	}
	if _, err := os.Stat(filepath.Join(h.root, ".cse", "rules-location.json")); err != nil {
		t.Fatal("unauthorized form altered or removed the binding")
	}
}

func TestHomeShowsResolvedLocalLocations(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	page := hubGet(h, "/").Body.String()
	if !strings.Contains(page, "This PC only") || !strings.Contains(page, "Your settings are saved on this PC. To use them on another PC") {
		t.Fatal("home summary missing local rules status")
	}
	if strings.Contains(page, `action="/rules-location"`) {
		t.Fatal("home summary must not duplicate editable Settings controls")
	}
}

func TestHubQuitSignalsShutdown(t *testing.T) {
	h := newTestHub()
	if w := hubPost(h, "/quit", url.Values{"request_secret": {"nonce"}}, h.origin); w.Code != 200 {
		t.Fatal("quit refused")
	}
	select {
	case <-h.quit:
	default:
		t.Fatal("quit not signalled")
	}
}

func TestHubCannotChangeInstallation(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	original := h.root
	for _, route := range []string{"/workspace", "/browse", "/browse-cancel"} {
		w := hubPost(h, route, url.Values{"request_secret": {"nonce"}, "path": {t.TempDir()}}, h.origin)
		if w.Code != http.StatusNotFound || h.root != original {
			t.Fatalf("%s changed installation or is still available: %d", route, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/settings?section=system", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), original) || strings.Contains(w.Body.String(), `action="/workspace"`) || strings.Contains(w.Body.String(), `action="/browse"`) {
		t.Fatal("settings must show a read-only installation")
	}
}

func TestHubDevelopmentLaunchExplainsExplicitRoot(t *testing.T) {
	h := newTestHub()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/settings?section=system", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "--workspace") {
		t.Fatal("missing explicit development launch guidance")
	}
}

func TestHubOpeningExistingWorkspaceAddsMissingSeedOnly(t *testing.T) {
	root := makeWorkspace(t)
	grading := filepath.Join(root, "context", "grading.md")
	edited := []byte("my own grading habits")
	if err := os.WriteFile(grading, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	notice := addMissingSeed(root)
	if !strings.Contains(notice, "playbooks/score.md") {
		t.Fatalf("notice does not name the restored guide: %q", notice)
	}
	if _, err := os.Stat(filepath.Join(root, "playbooks", "score.md")); err != nil {
		t.Fatal("missing guide was not restored")
	}
	if got, _ := os.ReadFile(grading); string(got) != string(edited) {
		t.Fatal("edited settings file changed")
	}
	if again := addMissingSeed(root); again != "" {
		t.Fatalf("second open should add nothing, got %q", again)
	}
}

func TestPlaybookReplaceKeepsOldCopyAndRefusesSettings(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	addMissingSeed(h.root)
	guide := filepath.Join(h.root, "playbooks", "score.md")
	if err := os.WriteFile(guide, []byte("my edits"), 0o644); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"request_secret": {"nonce"}, "file": {"playbooks/score.md"}, "confirm": {"on"}}
	if w := hubPost(h, "/playbook/replace", form, h.origin); w.Code != 303 {
		t.Fatalf("replace not accepted: %d", w.Code)
	}
	old, _ := filepath.Glob(filepath.Join(h.root, "playbook-old", "*", "playbooks", "score.md"))
	if len(old) != 1 {
		t.Fatalf("old copy not kept: %v", old)
	}
	if b, _ := os.ReadFile(old[0]); string(b) != "my edits" {
		t.Fatal("old copy changed")
	}
	if seed, _ := seedBytes("playbooks/score.md"); !strings.EqualFold(string(seed), readString(t, guide)) {
		t.Fatal("seed copy not written")
	}
	grading := filepath.Join(h.root, "context", "grading.md")
	if err := os.WriteFile(grading, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	form.Set("file", "context/grading.md")
	hubPost(h, "/playbook/replace", form, h.origin)
	if readString(t, grading) != "mine" || !strings.Contains(h.flashErr, "can't be replaced") {
		t.Fatal("settings file was replaced or not refused")
	}
}

func TestPlaybookReplaceUsesActiveSyncAndArchivesThere(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	addMissingSeed(h.root)
	syncRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(h.root, "playbooks", "score.md"), []byte("custom UserInstall version"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := hubPost(h, "/rules-location", url.Values{"request_secret": {"nonce"}, "sync_root": {syncRoot}}, h.origin); w.Code != 303 {
		t.Fatalf("enable UserSync: %d", w.Code)
	}
	active := filepath.Join(syncRoot, "playbooks", "score.md")
	if err := os.WriteFile(active, []byte("custom shared version"), 0o644); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"request_secret": {"nonce"}, "file": {"playbooks/score.md"}, "confirm": {"on"}}
	if w := hubPost(h, "/playbook/replace", form, h.origin); w.Code != 303 {
		t.Fatalf("replace shared playbook: %d", w.Code)
	}
	old, _ := filepath.Glob(filepath.Join(syncRoot, "playbook-old", "*", "playbooks", "score.md"))
	if len(old) != 1 {
		t.Fatalf("shared old copy not kept: %v", old)
	}
	if b, _ := os.ReadFile(old[0]); string(b) != "custom shared version" {
		t.Fatal("wrong authority was archived")
	}
	if b, _ := os.ReadFile(filepath.Join(h.root, "playbooks", "score.md")); string(b) != "custom UserInstall version" {
		t.Fatal("inactive local copy changed")
	}
}

func TestBindingCustomizedSyncOffersComparisonWithoutReplacing(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	syncRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(syncRoot, "AGENTS.md"), []byte("teacher's existing instructions"), 0o644); err != nil {
		t.Fatal(err)
	}
	addMissingSeed(syncRoot)
	custom := filepath.Join(syncRoot, "playbooks", "setup.md")
	if err := os.WriteFile(custom, []byte("teacher setup preferences"), 0o644); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"request_secret": {"nonce"}, "sync_root": {syncRoot}}
	if w := hubPost(h, "/rules-location", form, h.origin); w.Code != 303 {
		t.Fatalf("bind existing UserSync: %d", w.Code)
	}
	if !strings.Contains(h.flash, "newer version of some instruction files") || !strings.Contains(h.flash, "Settings, Files") {
		t.Fatalf("missing adoption notice: %q", h.flash)
	}
	if got := readString(t, custom); got != "teacher setup preferences" {
		t.Fatalf("binding replaced customized UserSync file: %q", got)
	}
	if got := readString(t, filepath.Join(syncRoot, "AGENTS.md")); got != "teacher's existing instructions" {
		t.Fatalf("binding replaced existing AGENTS: %q", got)
	}
	page := hubGet(h, "/settings?section=files").Body.String()
	if !strings.Contains(page, "Get the new version") || !strings.Contains(page, "changed by you, or older than this version") {
		t.Fatal("existing comparison and explicit adoption UI not shown")
	}
}

func TestBindingEmptySyncInitializesWithoutAdoptionNotice(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	syncRoot := t.TempDir()
	form := url.Values{"request_secret": {"nonce"}, "sync_root": {syncRoot}}
	if w := hubPost(h, "/rules-location", form, h.origin); w.Code != 303 {
		t.Fatalf("bind empty UserSync: %d", w.Code)
	}
	if strings.Contains(h.flash, "seed-managed instructions") {
		t.Fatalf("empty UserSync should initialize cleanly: %q", h.flash)
	}
	if rows := playbookRows(h.root); len(rows) == 0 {
		t.Fatal("comparison rows missing after empty UserSync initialization")
	} else {
		for _, row := range rows {
			if row.Replaceable && row.State != "same" {
				t.Fatalf("empty UserSync seed %s = %s, want same", row.Path, row.State)
			}
		}
	}
}

func readString(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestHowPageShowsPaths(t *testing.T) {
	h := newTestHub()
	h.programDir, h.root, h.version = `C:\Prog`, `C:\Work`, "0.1b"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, h.origin+"/how", nil))
	if body := w.Body.String(); w.Code != 200 || !strings.Contains(body, `C:\Work`) {
		t.Fatal("how page missing paths")
	}
}

func TestGuideUsesSixNativeExpandableTopicsAndExplainsStorageScopes(t *testing.T) {
	h := newTestHub()
	w := hubGet(h, "/how")
	body := w.Body.String()
	if w.Code != http.StatusOK || strings.Count(body, `class="card guide-topic"`) != 7 {
		t.Fatalf("guide should render six native expandable topics, status=%d", w.Code)
	}
	if strings.Contains(body, `popovertarget=`) || !strings.Contains(body, `<details class="card guide-topic"`) {
		t.Fatal("guide topics should use native details disclosures")
	}
	for _, want := range []string{"Update CanvasExpert", "OneDrive or Google Drive", "Hide student names"} {
		if !strings.Contains(body, want) {
			t.Errorf("guide missing storage distinction %q", want)
		}
	}
}

func TestHomeShowsCommandsFromTeacherContext(t *testing.T) {
	h := newTestHub()
	h.root = makeWorkspace(t)
	body := hubGet(h, "/").Body.String()
	for _, want := range []string{`href="/settings?section=connections">Canvas and other PCs</a>`, "Scoring session", "Start a scoring session for one of my courses."} {
		if !strings.Contains(body, want) {
			t.Errorf("Home without a commands file missing %q", want)
		}
	}
	p := filepath.Join(h.root, "context", "commands.md")
	if err := os.WriteFile(p, []byte("---\nschema: 1\ncommands:\n  - name: Friday sweep\n    prompt: \"Find missing work in {course}.\"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body = hubGet(h, "/").Body.String()
	if !strings.Contains(body, "Friday sweep") || !strings.Contains(body, `data-prompt="Find missing work in {course}."`) || strings.Contains(body, "Scoring session") {
		t.Error("Home did not use the teacher's commands")
	}
	if err := os.WriteFile(p, []byte("---\nschema: 1\ncommands:\n  - name: No prompt\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body = hubGet(h, "/").Body.String()
	if !strings.Contains(body, "Scoring session") || !strings.Contains(body, "could not be read") {
		t.Error("Home did not fall back to the starter commands with a note")
	}
}
