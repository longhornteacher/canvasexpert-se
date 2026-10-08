package localui

import (
	"canvassidekick/internal/workspace"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestStudentEditorOriginAndDuplicateAlias(t *testing.T) {
	s, e := workspace.OpenStudents(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, _ := s.Upsert("https://school.example", 1, "Ana", "Ana", "", "")
	b, _ := s.Upsert("https://school.example", 2, "Bob", "Bob", "", "")
	h := &studentHandler{origin: "http://127.0.0.1:1234", secret: "nonce", store: s, done: make(chan struct{})}
	form := url.Values{"request_secret": {"nonce"}, "alias_" + a.Ref: {"Same"}, "alias_" + b.Ref: {"Same"}}
	request := func(origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, h.origin+"/students", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request("http://attacker.example"); w.Code != 403 {
		t.Fatal("cross-origin allowed")
	}
	if w := request(h.origin); w.Code != 400 || s.Rows[0].Alias != a.Alias {
		t.Fatal("duplicate alias changed store")
	}
	form.Set("alias_"+a.Ref, "New Alias")
	form.Set("alias_"+b.Ref, b.Alias)
	form.Set("monitored_"+a.Ref, "on")
	if w := request(h.origin); w.Code != 200 || !s.Rows[0].Monitored || s.Rows[0].Ref != a.Ref {
		t.Fatal("edit retargeted identity")
	}
}
