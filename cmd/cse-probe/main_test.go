package main

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"canvassidekick/internal/canvas"
	"canvassidekick/internal/domain"
	"canvassidekick/internal/workflow"
)

func TestDisposableAssignmentGuardRejectsRealPublishedAndWrongScope(t *testing.T) {
	s := probeState{Origin: allowedOrigin, CourseID: courseID, AssignmentID: 42, Title: prefix + " synthetic"}
	a := workflow.Assignment{ID: 42, CourseID: courseID, Name: s.Title, GradingType: "points"}
	if e := validateAssignment(a, s); e != nil {
		t.Fatal(e)
	}
	published := a
	published.Published = true
	authorized := s
	authorized.PublicationAuthorized = true
	if e := validateAssignment(published, authorized); e != nil {
		t.Fatal("explicitly authorized exact published probe refused")
	}
	published.ID = 3697517
	if e := validateAssignment(published, authorized); e == nil {
		t.Fatal("publication authorization escaped exact probe")
	}
	for _, change := range []func(*workflow.Assignment){func(a *workflow.Assignment) { a.Published = true }, func(a *workflow.Assignment) { a.Name = "Actual classroom assignment" }, func(a *workflow.Assignment) { a.CourseID = 1 }, func(a *workflow.Assignment) { a.ID = 3697517 }, func(a *workflow.Assignment) { a.Name = prefix + " another probe" }} {
		b := a
		change(&b)
		if e := validateAssignment(b, s); e == nil {
			t.Fatal("accepted outside disposable scope")
		}
	}
}
func TestPointsCannotEscapeProbeScale(t *testing.T) {
	for _, v := range []string{"-1", "21", "NaN", "Infinity", "1e20"} {
		if _, e := boundedDecimal(v, "20"); e == nil {
			t.Fatalf("accepted %s", v)
		}
	}
	for _, v := range []string{"0", "15.123456", "20"} {
		if _, e := boundedDecimal(v, "20"); e != nil {
			t.Fatal(e)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixtureProbe(t *testing.T, fn roundTripFunc) *probe {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = fn
	c, e := canvas.New(allowedOrigin, "SYNTHETIC-ONLY-SECRET")
	http.DefaultTransport = old
	if e != nil {
		t.Fatal(e)
	}
	return &probe{c: c, path: filepath.Join(t.TempDir(), "state.json"), state: probeState{Origin: allowedOrigin, CourseID: courseID, TestStudentID: 5, AssignmentID: 42, Title: prefix + " synthetic", Outcomes: map[string]domain.ActionOutcome{}}}
}
func response(r *http.Request, text string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(text)), Request: r}
}
func TestSubmissionGuardRechecksBuiltinAndNeverWritesOtherStudent(t *testing.T) {
	writes := 0
	p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" {
			writes++
		}
		if strings.HasSuffix(r.URL.Path, "student_view_student") {
			return response(r, `{"id":6}`), nil
		}
		return response(r, `{"id":42,"course_id":121046,"name":"[CSE 0.1b DISPOSABLE] synthetic","published":false,"points_possible":20,"grading_type":"points","submission_types":["online_text_entry"]}`), nil
	})
	if e := p.submit(context.Background(), nil, false); e == nil {
		t.Fatal("submission retargeted unexpected built-in user")
	}
	if writes != 0 {
		t.Fatal("unsafe target sent a write")
	}
}
func TestUncertainCreateCannotReplay(t *testing.T) {
	calls := 0
	p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) { calls++; return response(r, `{}`), nil })
	p.state.AssignmentID = 0
	p.state.Outcomes["create"] = domain.ActionOutcome{Status: "uncertain"}
	if e := p.create(context.Background()); e == nil {
		t.Fatal("uncertain create replayed")
	}
	if calls != 0 {
		t.Fatal("uncertain create reached transport")
	}
}

func TestExplicitPublicationPersistsScopedAuthorization(t *testing.T) {
	writes := 0
	p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) {
		if r.Method == "PUT" {
			writes++
			b, e := io.ReadAll(r.Body)
			if e != nil {
				t.Fatal(e)
			}
			if string(b) != `{"assignment":{"published":true}}` {
				t.Fatal("publication modified unrelated settings")
			}
			return response(r, `{"id":42,"course_id":121046,"name":"[CSE 0.1b DISPOSABLE] synthetic","published":true,"points_possible":20,"grading_type":"points","submission_types":["online_text_entry"]}`), nil
		}
		return response(r, `{"id":42,"course_id":121046,"name":"[CSE 0.1b DISPOSABLE] synthetic","published":false,"points_possible":20,"grading_type":"points","submission_types":["online_text_entry"]}`), nil
	})
	if e := p.publication(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	if writes != 1 || !p.state.PublicationAuthorized {
		t.Fatal("explicit scoped publication not recorded")
	}
}
func TestDeniedUnpublishTruthfullyKeepsPublishedScope(t *testing.T) {
	p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) {
		if r.Method == "PUT" {
			out := response(r, `{"errors":"Cannot unpublish after submissions"}`)
			out.StatusCode = 403
			return out, nil
		}
		return response(r, `{"id":42,"course_id":121046,"name":"[CSE 0.1b DISPOSABLE] synthetic","published":true,"points_possible":20,"grading_type":"points","submission_types":["online_text_entry"]}`), nil
	})
	p.state.PublicationAuthorized = true
	e := p.publication(context.Background(), false)
	if e == nil || !strings.Contains(e.Error(), "may remain published") || !p.state.PublicationAuthorized || p.state.Outcomes["unpublish"].Status != "failed" {
		t.Fatal("denied unpublish claimed cleanup or revoked actual state")
	}
}
