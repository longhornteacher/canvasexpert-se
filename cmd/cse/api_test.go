package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/workspace"
)

func apiFixture(t *testing.T, handler http.HandlerFunc) *canvas.Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	previous := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	c, e := canvas.New(server.URL, "FIXTURE-ONLY-TOKEN")
	http.DefaultTransport = previous
	if e != nil {
		t.Fatal(e)
	}
	return c
}

func TestAPIGetPrintsJSONAndCanvasErrors(t *testing.T) {
	var methods []string
	c := apiFixture(t, func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if strings.Contains(r.URL.Path, "/missing") {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"errors":[{"message":"The specified resource does not exist."}]}`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"7","points_possible":50}]`))
	})
	var out bytes.Buffer
	if e := apiGet(context.Background(), c, "/api/quiz/v1/courses/1/quizzes/2/items", &out, nil); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), `"points_possible": 50`) {
		t.Fatalf("response not printed: %q", out.String())
	}
	e := apiGet(context.Background(), c, "/api/v1/courses/1/missing", &out, nil)
	if e == nil || !strings.Contains(e.Error(), "does not exist") {
		t.Fatalf("Canvas error detail missing: %v", e)
	}
	if e = apiGet(context.Background(), c, "/courses/1", &out, nil); e == nil {
		t.Fatal("non-API path allowed")
	}
	for _, m := range methods {
		if m != http.MethodGet {
			t.Fatalf("api sent %s", m)
		}
	}
}

func TestAPIGetHidesNamesBehindAliases(t *testing.T) {
	c := apiFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/enrollments"):
			_, _ = w.Write([]byte(`[{"id":1,"user_id":41,"user":{"id":41,"name":"Avery Quill","sortable_name":"Quill, Avery","short_name":"Avery","login_id":"aquill","sis_user_id":"S41","email":"aquill@school.example"}},
				{"id":2,"user_id":99,"user":{"id":99,"name":"Blake Rowe","sortable_name":"Rowe, Blake","short_name":"Blake"}}]`))
		case strings.Contains(r.URL.Path, "/discussion_topics"):
			_, _ = w.Write([]byte(`{"participants":[{"id":"41","display_name":"Avery Quill","avatar_image_url":"https://x"}],"view":[{"id":5,"user_id":41,"user_name":"Avery Quill","message":"Avery Quill says hi"}]}`))
		default:
			_, _ = w.Write([]byte(`[{"id":3,"name":"Ghost Book Club","avatar_url":null,"members_count":4,"points_possible":50}]`))
		}
	})
	root := t.TempDir()
	store, e := workspace.OpenStudents(root)
	if e != nil {
		t.Fatal(e)
	}
	st, e := store.Upsert(c.Origin(), 41, "Avery Quill", "Avery", "S41", "aquill")
	if e != nil || store.Save() != nil {
		t.Fatal(e)
	}
	hide := newIdentityRedactor(root, c.Origin())

	var out bytes.Buffer
	if e := apiGet(context.Background(), c, "/api/v1/courses/1/enrollments", &out, hide); e != nil {
		t.Fatal(e)
	}
	got := out.String()
	for _, leaked := range []string{"Avery", "Quill", "aquill", "S41", "Blake", "Rowe"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("%q leaked: %s", leaked, got)
		}
	}
	if !strings.Contains(got, `"name": "`+st.Alias+`"`) || !strings.Contains(got, `"student_ref": "`+st.Ref+`"`) || !strings.Contains(got, hiddenName) {
		t.Fatalf("known student not shown by alias and ref, or unknown user not hidden: %s", got)
	}

	out.Reset()
	if e := apiGet(context.Background(), c, "/api/v1/courses/1/discussion_topics/2/view", &out, hide); e != nil {
		t.Fatal(e)
	}
	got = out.String()
	if strings.Count(got, st.Alias) != 2 || strings.Contains(got, "avatar") || !strings.Contains(got, "Avery Quill says hi") {
		t.Fatalf("discussion names not aliased, or student writing changed: %s", got)
	}

	out.Reset()
	if e := apiGet(context.Background(), c, "/api/v1/courses/1/groups", &out, hide); e != nil {
		t.Fatal(e)
	}
	if got = out.String(); !strings.Contains(got, `"name": "Ghost Book Club"`) || !strings.Contains(got, `"points_possible": 50`) {
		t.Fatalf("non-user data changed: %s", got)
	}
}
