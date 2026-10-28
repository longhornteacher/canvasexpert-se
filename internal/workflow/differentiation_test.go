package workflow

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func TestDifferentiationReadsOnlyTagsAcrossPagesWithBulkMemberships(t *testing.T) {
	requests := 0
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			t.Fatal("tag display wrote to Canvas")
		}
		switch r.URL.Path {
		case "/api/v1/courses/1/groups":
			if r.URL.Query().Get("collaboration_state") != "non_collaborative" {
				t.Error("missing differentiation filter")
			}
			if r.URL.Query().Get("page") == "2" {
				_, _ = io.WriteString(w, `[{"id":102,"name":"Blue","non_collaborative":true}]`)
			} else {
				w.Header().Set("Link", `</api/v1/courses/1/groups?collaboration_state=non_collaborative&page=2>; rel="next"`)
				_, _ = io.WriteString(w, `[{"id":101,"name":"Silver","non_collaborative":true},{"id":999,"name":"Project group","non_collaborative":false}]`)
			}
		case "/api/v1/courses/1/enrollments":
			io.WriteString(w, `[{"user_id":1,"type":"StudentEnrollment"},{"user_id":2,"type":"StudentEnrollment"},{"user_id":3,"type":"StudentEnrollment"}]`)
		case "/api/v1/courses/1/bulk_user_tags":
			if len(r.URL.Query()["user_ids[]"]) != 3 {
				t.Error("missing roster identities in bulk tag read")
			}
			io.WriteString(w, `{"1":[101,102,999],"2":[101],"3":[]}`)
		default:
			t.Errorf("unexpected tag API call: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	tags, err := DifferentiationTags(context.Background(), s.Client, 1)
	want := map[int64][]string{1: {"Blue", "Silver"}, 2: {"Silver"}}
	if err != nil || !reflect.DeepEqual(tags, want) || requests != 4 {
		t.Fatalf("tag read: %v %v (%d requests)", tags, err, requests)
	}
}

func TestDifferentiationPermissionFailureIsReported(t *testing.T) {
	s, _ := workflowFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })
	if tags, err := DifferentiationTags(context.Background(), s.Client, 1); err == nil || tags != nil {
		t.Fatal("permission failure treated as no tags")
	}
}
