package localui

import (
	"context"
	"html/template"
	"math/rand/v2"
	"net/http"
	"path/filepath"

	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

func (h *Hub) reqCtx() (context.Context, context.CancelFunc) {
	base := h.ctx
	if base == nil {
		base = context.Background()
	}
	return context.WithTimeout(base, 60*time.Second)
}

func (h *Hub) canvasClient() (*canvas.Client, error) {
	credential, err := platform.LoadCredential()
	if err != nil {
		return nil, err
	}
	return canvas.New(credential.Origin, credential.Token)
}

func (h *Hub) fetchCourses(ctx context.Context) ([]workflow.Course, string, error) {
	if h.listCoursesFn != nil {
		return h.listCoursesFn(ctx)
	}
	c, err := h.canvasClient()
	if err != nil {
		return nil, "", err
	}
	list, err := workflow.Courses(ctx, c)
	return list, c.Origin(), err
}

func (h *Hub) fetchRoster(ctx context.Context, courseID int64) ([]workflow.Enrollment, string, error) {
	if h.rosterFn != nil {
		return h.rosterFn(ctx, courseID)
	}
	c, err := h.canvasClient()
	if err != nil {
		return nil, "", err
	}
	list, err := workflow.StudentEnrollments(ctx, c, courseID)
	return list, c.Origin(), err
}

func (h *Hub) readConfig() (domain.CourseConfig, string, error) {
	var config domain.CourseConfig
	body, err := workspace.ReadDocument(filepath.Join(h.root, "context", "courses.md"), &config)
	return config, body, err
}

func (h *Hub) takeFlash() (message, notice string) {
	message, notice = h.flashErr, h.flash
	h.flash, h.flashErr = "", ""
	return
}

func (h *Hub) redirect(w http.ResponseWriter, r *http.Request, to, notice, problem string) {
	h.flash, h.flashErr = notice, problem
	http.Redirect(w, r, to, http.StatusSeeOther)
}

func (h *Hub) needWorkspace(w http.ResponseWriter) bool {
	if h.root == "" {
		h.home(w, 400, "Launch with the CE-SE installation first; for development use cse --workspace PATH ui.", "")
		return true
	}
	return false
}
func (h *Hub) render(w http.ResponseWriter, status int, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = t.Execute(w, data)
}

func randomAlias(available []string) string { return available[rand.IntN(len(available))] }
