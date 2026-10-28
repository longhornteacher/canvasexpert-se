package workflow

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

const readRubricFixture = `{"id":31,"title":"Evidence","points_possible":4,"free_form_criterion_comments":true,"data":[{"id":"REMOTE-CRITERION","description":"Support ideas","long_description":"Use precise examples.","points":4,"criterion_use_range":true,"ratings":[{"id":"REMOTE-RATING","points":4,"description":"Clear","long_description":"Specific evidence."},{"id":"REMOTE-ZERO","points":0,"description":"Missing"}]}],"assessments":[{"private":"STUDENT ASSESSMENT"}],"associations":[{"id":8181,"private":"ASSOCIATION"}]}`

func TestRubricReadsPaginateAndExportPortableDraft(t *testing.T) {
	listPages, usagePages := 0, 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatalf("rubric read wrote %s", r.Method)
		}
		if r.URL.Query().Has("include[]") {
			t.Fatal("requested assessments or associations")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/courses/1/rubrics":
			listPages++
			if r.URL.Query().Get("page") == "2" {
				fmt.Fprint(w, `[{"id":32,"title":"Another","points_possible":0,"data":[]}]`)
				return
			}
			w.Header().Set("Link", `<https://`+r.Host+`/api/v1/courses/1/rubrics?page=2>; rel="next"`)
			fmt.Fprint(w, `[{"id":31,"title":"Evidence","points_possible":4}]`)
		case "/api/v1/courses/1/rubrics/31":
			fmt.Fprint(w, readRubricFixture)
		case "/api/v1/courses/1":
			fmt.Fprint(w, `{"id":1,"account_id":51}`)
		case "/api/v1/accounts/51/rubrics":
			http.Error(w, `{"error":"PRIVATE IDENTITY"}`, 403)
		case "/api/v1/courses/1/rubrics/31/used_locations":
			usagePages++
			if r.URL.Query().Get("page") == "2" {
				fmt.Fprint(w, `[{"id":1,"name":"PRIVATE COURSE","assignments":[{"id":502,"title":"PRIVATE TITLE"}]},{"id":2,"assignments":[{"id":503}]}]`)
				return
			}
			w.Header().Set("Link", `<https://`+r.Host+`/api/v1/courses/1/rubrics/31/used_locations?page=2>; rel="next"`)
			fmt.Fprint(w, `[{"id":1,"name":"PRIVATE COURSE","assignments":[{"id":501}]}]`)
		default:
			t.Errorf("unexpected rubric read %s", r.URL)
			http.NotFound(w, r)
		}
	})
	r, e := s.ListRubrics(context.Background(), "TEST", true)
	if e != nil || listPages != 2 || len(r.Rubrics) != 2 || *r.Rubrics[0].CriteriaCount != 1 || *r.Rubrics[1].CriteriaCount != 0 || len(r.Limitations) != 1 || !strings.Contains(r.Limitations[0], "can't see school rubrics") {
		t.Fatalf("list: %+v %v", r, e)
	}
	if strings.Contains(fmt.Sprint(r), "PRIVATE") {
		t.Fatal("raw account error leaked")
	}
	first, e := s.PullRubric(context.Background(), "TEST", 31)
	if e != nil || first.UsedCourses == nil || *first.UsedCourses != 2 || *first.UsedAssignments != 3 || usagePages != 2 {
		t.Fatalf("pull: %+v %v", first, e)
	}
	var d domain.RubricLibraryDocument
	if _, e = workspace.ReadDocument(first.Path, &d); e != nil {
		t.Fatal(e)
	}
	c := d.Rubric.Criteria[0]
	if !d.Rubric.FreeFormCriterionComments || c.ID != "criterion-1" || !c.UseRange || c.LongDescription != "Use precise examples." || c.Ratings[0].ID != "rating-1" || c.Ratings[0].LongDescription != "Specific evidence." || c.Points.String() != "4" {
		t.Fatalf("lost rubric content: %+v", d)
	}
	before, _ := os.ReadFile(first.Path)
	for _, private := range []string{"REMOTE-", "STUDENT ASSESSMENT", "ASSOCIATION", "PRIVATE", "association_id", "use_for_grading"} {
		if strings.Contains(string(before), private) {
			t.Fatalf("export contains nonportable data %q", private)
		}
	}
	if e = os.WriteFile(first.Path, append(before, []byte("Unsent teacher edits.\n")...), 0600); e != nil {
		t.Fatal(e)
	}
	second, e := s.PullRubric(context.Background(), "TEST", 31)
	if e != nil || first.Path == second.Path {
		t.Fatalf("pull did not create a fresh draft: %+v %v", second, e)
	}
	after, _ := os.ReadFile(first.Path)
	if !strings.Contains(string(after), "Unsent teacher edits.") {
		t.Fatal("pull replaced unsent draft")
	}
}

func TestRubricReadsKeepUnknownCountsAndUsableDefinition(t *testing.T) {
	for _, usage := range []string{`{"unexpected":"PRIVATE"}`, `[{"id":1}]`, `[{"id":1,"assignments":[{}]}]`} {
		t.Run(usage, func(t *testing.T) {
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Fatal("unexpected write")
				}
				switch r.URL.Path {
				case "/api/v1/courses/1/rubrics":
					fmt.Fprint(w, `[{"id":32,"title":"Unknown"}]`)
				case "/api/v1/courses/1/rubrics/32":
					fmt.Fprint(w, `{"id":32,"title":"Unknown"}`)
				case "/api/v1/courses/1/rubrics/31":
					fmt.Fprint(w, readRubricFixture)
				case "/api/v1/courses/1/rubrics/31/used_locations":
					fmt.Fprint(w, usage)
				default:
					http.NotFound(w, r)
				}
			})
			r, e := s.ListRubrics(context.Background(), "TEST", false)
			if e != nil || len(r.Rubrics) != 1 || r.Rubrics[0].CriteriaCount != nil || len(r.Limitations) != 1 {
				t.Fatalf("unknown treated as zero: %+v %v", r, e)
			}
			p, e := s.PullRubric(context.Background(), "TEST", 31)
			if e != nil || p.Path == "" || p.UsedCourses != nil || p.UsedAssignments != nil || len(p.Limitations) != 1 {
				t.Fatalf("usage failure blocked definition: %+v %v", p, e)
			}
			if _, e = s.PullRubric(context.Background(), "TEST", 32); e == nil {
				t.Fatal("saved incomplete definition")
			}
		})
	}
}

func TestRubricReadsRejectWrongIdentity(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":99,"title":"Wrong rubric","data":[]}`)
	})
	if _, e := s.PullRubric(context.Background(), "TEST", 31); e == nil {
		t.Fatal("accepted a different rubric")
	}
	if _, e := s.PullRubric(context.Background(), "TEST", 0); e == nil {
		t.Fatal("accepted invalid rubric ID")
	}
}

func TestRubricReadsPullSchoolRubricOnlyFromVerifiedAccount(t *testing.T) {
	for _, wrongCourse := range []bool{false, true} {
		t.Run(fmt.Sprint(wrongCourse), func(t *testing.T) {
			accountReads := 0
			s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Fatal("unexpected write")
				}
				switch r.URL.Path {
				case "/api/v1/courses/1/rubrics/31":
					http.NotFound(w, r)
				case "/api/v1/courses/1":
					id := 1
					if wrongCourse {
						id = 99
					}
					fmt.Fprintf(w, `{"id":%d,"account_id":51}`, id)
				case "/api/v1/accounts/51/rubrics/31":
					accountReads++
					fmt.Fprint(w, readRubricFixture)
				case "/api/v1/accounts/51/rubrics/31/used_locations":
					fmt.Fprint(w, `[]`)
				default:
					t.Errorf("unexpected read %s", r.URL)
					http.NotFound(w, r)
				}
			})
			r, e := s.PullRubric(context.Background(), "TEST", 31)
			if wrongCourse {
				if e == nil || accountReads != 0 {
					t.Fatalf("used unverified account: %+v %v", r, e)
				}
			} else if e != nil || accountReads != 1 || r.UsedAssignments == nil || *r.UsedAssignments != 0 {
				t.Fatalf("school definition unavailable: %+v %v", r, e)
			}
		})
	}
}
