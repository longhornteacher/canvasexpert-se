package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
)

func TestSelectRequiresNewNamedExcludedFixture(t *testing.T) {
	for _, body := range []string{
		`{"id":42,"course_id":121046,"name":"[CSE 0.1b DISPOSABLE] synthetic","grading_type":"points","omit_from_final_grade":true,"post_to_sis":false}`,
		`{"id":42,"course_id":121046,"name":"Actual coursework","grading_type":"points","omit_from_final_grade":true,"post_to_sis":false}`,
		`{"id":42,"course_id":121046,"name":"[CSE UNATTENDED] synthetic","grading_type":"points","omit_from_final_grade":false,"post_to_sis":false}`,
		`{"id":42,"course_id":121046,"name":"[CSE UNATTENDED] synthetic","grading_type":"points","omit_from_final_grade":true,"post_to_sis":true}`,
		`{"id":42,"course_id":121046,"name":"[CSE UNATTENDED] synthetic","grading_type":"points","omit_from_final_grade":true}`,
		`{"id":42,"course_id":1,"name":"[CSE UNATTENDED] synthetic","grading_type":"points","omit_from_final_grade":true,"post_to_sis":false}`,
	} {
		p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" {
				t.Fatal("select wrote to Canvas")
			}
			return response(r, body), nil
		})
		p.state.AssignmentID, p.state.Title = 0, ""
		if e := p.selectFixture(context.Background(), []string{"--assignment", "42"}); e == nil {
			t.Fatalf("selected invalid fixture: %s", body)
		}
		if p.state.AssignmentID != 0 {
			t.Fatal("invalid identity persisted")
		}
	}
}

func TestSelectPersistsRubricAndPublication(t *testing.T) {
	p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/rubrics/") {
			return response(r, `{"associations":[{"id":7,"association_id":42,"association_type":"Assignment","rubric_id":6,"use_for_grading":false}]}`), nil
		}
		return response(r, `{"id":42,"course_id":121046,"name":"[CSE UNATTENDED] synthetic","published":true,"grading_type":"points","omit_from_final_grade":true,"post_to_sis":false,"rubric_settings":{"id":6},"rubric":[{"id":"c1","points":100}]}`), nil
	})
	p.state.AssignmentID, p.state.Title = 0, ""
	if e := p.selectFixture(context.Background(), []string{"--assignment", "42"}); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(p.path)
	if e != nil {
		t.Fatal(e)
	}
	var saved probeState
	if e = json.Unmarshal(b, &saved); e != nil {
		t.Fatal(e)
	}
	if saved.AssignmentID != 42 || saved.RubricID != 6 || saved.AssociationID != 7 || saved.CriterionID != "c1" || !saved.Published || !saved.PublicationAuthorized {
		t.Fatalf("identity not persisted: %+v", saved)
	}
}

func TestConfigureOnlyUnpublishedNamedFixtureAndNarrowPatch(t *testing.T) {
	writes := 0
	p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) {
		safe := writes > 0
		if r.Method == "PUT" {
			writes++
			b, e := io.ReadAll(r.Body)
			if e != nil {
				t.Fatal(e)
			}
			if string(b) != `{"assignment":{"omit_from_final_grade":true,"post_to_sis":false,"submission_types":["online_upload"]}}` {
				t.Fatalf("unexpected patch: %s", b)
			}
			safe = true
		}
		if safe {
			return response(r, `{"id":42,"course_id":121046,"name":"[CSE UNATTENDED] synthetic","grading_type":"points","omit_from_final_grade":true,"post_to_sis":false}`), nil
		}
		return response(r, `{"id":42,"course_id":121046,"name":"[CSE UNATTENDED] synthetic","grading_type":"points","omit_from_final_grade":false,"post_to_sis":true}`), nil
	})
	p.state.AssignmentID, p.state.Title = 0, ""
	if e := p.configureFixture(context.Background(), []string{"--assignment", "42", "--submission-types", "online_upload"}); e != nil {
		t.Fatal(e)
	}
	if writes != 1 || p.state.AssignmentID != 42 {
		t.Fatal("fixture configuration not scoped/persisted")
	}
}

func TestUnsafeOrPublishedFixtureCannotBeWritten(t *testing.T) {
	for _, operation := range []string{"publish", "configure"} {
		writes := 0
		p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" {
				writes++
			}
			if operation == "configure" {
				return response(r, `{"id":42,"course_id":121046,"name":"[CSE UNATTENDED] synthetic","published":true,"grading_type":"points","omit_from_final_grade":true,"post_to_sis":false}`), nil
			}
			return response(r, `{"id":42,"course_id":121046,"name":"[CSE UNATTENDED] synthetic","published":false,"grading_type":"points","omit_from_final_grade":false,"post_to_sis":false}`), nil
		})
		p.state.Title = unattendedPrefix + " synthetic"
		var e error
		if operation == "publish" {
			e = p.publication(context.Background(), true)
		} else {
			e = p.configureFixture(context.Background(), nil)
		}
		if e == nil || writes != 0 {
			t.Fatal("unsafe/published fixture reached a write")
		}
	}
}

func TestUncertainModuleCannotReplayAndReturnedIDSurvivesReadFailure(t *testing.T) {
	calls := 0
	p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method == "POST" {
			return response(r, `{"id":8,"name":"[CSE UNATTENDED] module"}`), nil
		}
		return response(r, `{}`), nil
	})
	p.state.Outcomes["create-module"] = domain.ActionOutcome{Status: "uncertain"}
	if e := p.createModule(context.Background(), []string{"--name", "[CSE UNATTENDED] module"}); e == nil || calls != 0 {
		t.Fatal("uncertain module create replayed")
	}
	delete(p.state.Outcomes, "create-module")
	if e := p.createModule(context.Background(), []string{"--name", "[CSE UNATTENDED] module"}); e == nil {
		t.Fatal("unexpected read accepted")
	}
	b, e := os.ReadFile(p.path)
	if e != nil {
		t.Fatal(e)
	}
	var saved probeState
	if e = json.Unmarshal(b, &saved); e != nil {
		t.Fatal(e)
	}
	if saved.ModuleID != 8 {
		t.Fatal("module ID lost after failed read")
	}
	if e := p.createModule(context.Background(), []string{"--name", "[CSE UNATTENDED] module"}); e == nil || calls != 2 {
		t.Fatal("known module created again")
	}
}

func TestReadModuleListsFreshPaginatedMembershipWithoutContent(t *testing.T) {
	pages := 0
	p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" {
			t.Fatal("module membership read performed a write")
		}
		if strings.HasSuffix(r.URL.Path, "/items") {
			pages++
			if r.URL.Query().Get("page") == "2" {
				return response(r, `[{"id":12,"module_id":8,"type":"Assignment","content_id":43,"title":"PRIVATE TITLE","url":"https://example.com/private"}]`), nil
			}
			out := response(r, `[{"id":11,"module_id":8,"type":"Assignment","content_id":42,"title":"PRIVATE TITLE"}]`)
			out.Header.Set("Link", "<"+allowedOrigin+"/api/v1/courses/121046/modules/8/items?page=2>; rel=\"next\"")
			return out, nil
		}
		if r.URL.Path != "/api/v1/courses/121046/modules/8" {
			t.Fatal("read broadened module scope", r.URL.Path)
		}
		return response(r, `{"id":8,"name":"[CSE UNATTENDED] module","published":false,"items_count":2,"items":[{"id":999}]}`), nil
	})
	p.state.ModuleID, p.state.ModuleName = 8, unattendedPrefix+" module"
	read, write, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	oldStdout := os.Stdout
	os.Stdout = write
	e = p.readModule(context.Background())
	os.Stdout = oldStdout
	write.Close()
	b, readErr := io.ReadAll(read)
	read.Close()
	if e != nil || readErr != nil {
		t.Fatal(e, readErr)
	}
	var result struct {
		Items []struct {
			ID        int64  `json:"id"`
			ContentID int64  `json:"content_id"`
			Type      string `json:"type"`
		} `json:"items"`
	}
	if json.Unmarshal(b, &result) != nil || pages != 2 || len(result.Items) != 2 || result.Items[0].ContentID != 42 || result.Items[1].ContentID != 43 || result.Items[1].Type != "Assignment" {
		t.Fatal("fresh full module membership was not reported", string(b), pages)
	}
	if strings.Contains(string(b), "PRIVATE") || strings.Contains(string(b), "example.com") || strings.Contains(string(b), "999") {
		t.Fatal("module read exposed content or used stale embedded items")
	}
}

func TestReadModuleRejectsForeignItem(t *testing.T) {
	p := fixtureProbe(t, func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/items") {
			return response(r, `[{"id":11,"module_id":9,"type":"Assignment","content_id":42}]`), nil
		}
		return response(r, `{"id":8,"name":"[CSE UNATTENDED] module","items_count":1}`), nil
	})
	p.state.ModuleID, p.state.ModuleName = 8, unattendedPrefix+" module"
	if e := p.readModule(context.Background()); e == nil {
		t.Fatal("foreign module membership accepted")
	}
}
