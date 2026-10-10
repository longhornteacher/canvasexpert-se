package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"canvasexpertse/internal/canvas"
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
	if e := apiGet(context.Background(), c, "/api/quiz/v1/courses/1/quizzes/2/items", &out); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), `"points_possible": 50`) {
		t.Fatalf("response not printed: %q", out.String())
	}
	e := apiGet(context.Background(), c, "/api/v1/courses/1/missing", &out)
	if e == nil || !strings.Contains(e.Error(), "does not exist") {
		t.Fatalf("Canvas error detail missing: %v", e)
	}
	if e = apiGet(context.Background(), c, "/courses/1", &out); e == nil {
		t.Fatal("non-API path allowed")
	}
	for _, m := range methods {
		if m != http.MethodGet {
			t.Fatalf("api sent %s", m)
		}
	}
}
