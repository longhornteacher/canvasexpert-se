package localui

import (
	"bytes"
	"context"
	"html/template"
	"math/rand/v2"
	"net/http"

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
	load := platform.LoadCredential
	if h.loadCredentialFn != nil {
		load = h.loadCredentialFn
	}
	credential, err := load()
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
	c, err := h.clientForCourse(ctx, courseID)
	if err != nil {
		return nil, "", err
	}
	list, err := workflow.StudentEnrollments(ctx, c, courseID)
	return list, c.Origin(), err
}

// hubKeys lets the workflow router reach the same key seams the Team page uses.
type hubKeys struct{ h *Hub }

func (k hubKeys) Operator() (platform.Credential, error) {
	if k.h.loadCredentialFn != nil {
		return k.h.loadCredentialFn()
	}
	return platform.LoadCredential()
}
func (k hubKeys) Teammate(id int64) (platform.Credential, error) { return k.h.loadTeammate(id) }

// clientForCourse is your own client for your courses and the owner's key for a
// teammate's course, by the same rule the commands use.
func (h *Hub) clientForCourse(ctx context.Context, courseID int64) (*canvas.Client, error) {
	c, err := h.canvasClient()
	if err != nil {
		return nil, err
	}
	config, _, err := h.readConfig()
	if err != nil {
		return c, nil
	}
	for _, course := range config.Courses {
		if course.ID != courseID || course.Owner == "" {
			continue
		}
		team, err := workspace.ReadTeam(h.root)
		if err != nil {
			return nil, err
		}
		service := &workflow.Service{Root: h.root, Client: c, Config: config, Team: team, Keys: hubKeys{h}, InstallRoot: h.root}
		scoped, err := service.ForCourse(ctx, course)
		if err != nil {
			return nil, err
		}
		return scoped.Client, nil
	}
	return c, nil
}

func (h *Hub) readConfig() (domain.CourseConfig, string, error) {
	var config domain.CourseConfig
	body, err := workspace.ReadTeacherDocument(h.root, "context/courses.md", &config)
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

// render executes into a buffer so a template error sends a plain 500 instead
// of a truncated page with a success status.
func (h *Hub) render(w http.ResponseWriter, status int, t *template.Template, data any) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		http.Error(w, "This page could not be displayed.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func randomAlias(available []string) string { return available[rand.IntN(len(available))] }
