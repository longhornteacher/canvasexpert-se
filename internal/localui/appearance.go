package localui

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"

	"canvasexpertse/internal/workspace"
)

const appearancePath = "context/appearance.md"

type appearanceDocument struct {
	Schema int    `yaml:"schema"`
	Mode   string `yaml:"mode,omitempty"`
}

func readAppearance(root string) (appearanceDocument, string, error) {
	// Resolve first: an unavailable configured UserSync is never a missing
	// optional document and must never fall back to the inactive local copy.
	p, err := workspace.ResolveTeacherPath(root, appearancePath)
	if err != nil {
		return appearanceDocument{}, "", err
	}
	var doc appearanceDocument
	body, err := workspace.ReadDocument(p, &doc)
	if os.IsNotExist(err) {
		return appearanceDocument{Schema: 1}, "", nil
	}
	if err == nil && doc.Mode != "" && doc.Mode != "light" && doc.Mode != "dark" {
		err = fmt.Errorf("unsupported appearance mode")
	}
	return doc, body, err
}

func saveAppearance(root, mode string) error {
	if mode != "light" && mode != "dark" {
		return fmt.Errorf("unsupported appearance mode")
	}
	doc, body, err := readAppearance(root)
	if err != nil {
		return err
	}
	doc.Schema, doc.Mode = 1, mode
	return workspace.WriteTeacherDocument(root, appearancePath, doc, body)
}

// appearanceData belongs to one response. Shared CSS and template functions
// stay immutable; each new page reads the active teacher authority afresh.
type appearanceData struct {
	Mode, Secret, Problem string
	Enabled               bool
}

func appearanceFor(root, secret string) appearanceData {
	a := appearanceData{Secret: secret, Enabled: root != ""}
	if root == "" {
		return a
	} // Explicitly rootless wrappers follow the OS.
	doc, _, err := readAppearance(root)
	if err != nil {
		a.Problem = "Appearance preference could not be read; using your computer setting."
	} else {
		a.Mode = doc.Mode
	}
	return a
}

func (a appearanceData) Control() template.HTML {
	if !a.Enabled {
		return ""
	}
	label := "Light / Dark"
	if a.Mode == "light" {
		label = "Light"
	}
	if a.Mode == "dark" {
		label = "Dark"
	}
	return template.HTML(fmt.Sprintf(`<div class="appearance"><button type="button" role="switch" aria-label="Dark appearance" aria-checked="%t" data-appearance-toggle data-request-secret="%s"><span data-appearance-label>%s</span></button><span class="appearance-status" role="status" aria-live="polite">%s</span></div>`, a.Mode == "dark", template.HTMLEscapeString(a.Secret), label, template.HTMLEscapeString(a.Problem)))
}

// The narrow route runs before all ordinary form actions, including review
// build/post/test and student/credential saves. It never serializes those forms.
func serveAppearance(w http.ResponseWriter, r *http.Request, root, origin, secret string) bool {
	if r.URL.Path != "/_cse/appearance" {
		return false
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return true
	}
	if "http://"+r.Host != origin || r.Header.Get("Origin") != origin {
		http.Error(w, "Form origin rejected", http.StatusForbidden)
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return true
	}
	if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("request_secret")), []byte(secret)) != 1 {
		http.Error(w, "Form request rejected", http.StatusForbidden)
		return true
	}
	if len(r.PostForm) != 2 || len(r.PostForm["mode"]) != 1 || len(r.PostForm["request_secret"]) != 1 || (r.PostForm.Get("mode") != "light" && r.PostForm.Get("mode") != "dark") {
		http.Error(w, "Invalid appearance choice", http.StatusBadRequest)
		return true
	}
	if root == "" {
		http.Error(w, "Appearance has no UserInstall", http.StatusBadRequest)
		return true
	}
	mode := r.PostForm.Get("mode")
	if err := saveAppearance(root, mode); err != nil {
		http.Error(w, "Appearance could not be saved", http.StatusInternalServerError)
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Mode string `json:"mode"`
	}{mode})
	return true
}

// Terminal responses carry all palette information inline; no save control or
// asset request can outlive the one-shot server.
func appearanceHTML(root string) string {
	return `<html lang="en" data-appearance="` + appearanceFor(root, "").Mode + `">`
}
