package canvas

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{7}, 64)...)

func mediaClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(h)
	t.Cleanup(server.Close)
	previous := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	c, e := New(server.URL, "CANVAS-PAT")
	http.DefaultTransport = previous
	if e != nil {
		t.Fatal(e)
	}
	return c, server
}

func TestUploadQuizMediaSendsNoTokenAndReturnsUnsignedURL(t *testing.T) {
	var base string
	puts := 0
	c, server := mediaClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/quiz/v1/courses/1/quizzes/55/items/media_upload_url":
			if r.Header.Get("Authorization") != "Bearer CANVAS-PAT" {
				t.Error("Canvas request lost its token")
			}
			_, _ = io.WriteString(w, `{"url":"`+base+`/media/abc?X-Amz-Signature=SECRET"}`)
		case r.Method == "PUT" && r.URL.Path == "/media/abc":
			puts++
			b, _ := io.ReadAll(r.Body)
			if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
				t.Error("credentials reached the storage host")
			}
			if r.Header.Get("Content-Type") != "image/png" || !bytes.Equal(b, pngBytes) {
				t.Errorf("wrong upload: %q", r.Header.Get("Content-Type"))
			}
			w.WriteHeader(200)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	})
	base = server.URL
	got, e := c.UploadQuizMedia(t.Context(), 1, 55, pngBytes)
	if e != nil || got != server.URL+"/media/abc" || puts != 1 {
		t.Fatalf("got %q, %v, puts=%d", got, e, puts)
	}
}

func TestUploadQuizMediaS3HostIsolationAndNoRedirect(t *testing.T) {
	const s3 = "bucket.s3.us-east-1.amazonaws.com"
	storageCalls := 0
	var canvasServer, storageServer *httptest.Server
	canvasServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"url":"https://`+s3+`/item_media/x?X-Amz-Signature=SECRET"}`)
	}))
	storageServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storageCalls++
		if r.Header.Get("Authorization") != "" {
			t.Error("PAT sent to S3")
		}
		w.Header().Set("Location", canvasServer.URL+"/steal")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(canvasServer.Close)
	t.Cleanup(storageServer.Close)
	previous := http.DefaultTransport
	http.DefaultTransport = canvasServer.Client().Transport
	c, e := New(canvasServer.URL, "CANVAS-PAT")
	http.DefaultTransport = previous
	if e != nil {
		t.Fatal(e)
	}
	c.http.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() == s3 {
			r = r.Clone(r.Context())
			r.URL.Host = strings.TrimPrefix(storageServer.URL, "https://")
			return storageServer.Client().Transport.RoundTrip(r)
		}
		return canvasServer.Client().Transport.RoundTrip(r)
	})
	// The fixture S3 host has no port, so only the 443 rule needs the rewrite above.
	_, e = c.UploadQuizMedia(t.Context(), 1, 55, pngBytes)
	if e == nil || storageCalls != 1 || strings.Contains(e.Error(), "SECRET") {
		t.Fatalf("a redirecting storage host must fail after one call without leaking the URL: %v calls=%d", e, storageCalls)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestUploadQuizMediaRejectsBadFilesAndDestinations(t *testing.T) {
	var slot string
	calls := 0
	c, _ := mediaClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			t.Error("file was sent to a rejected destination")
		}
		calls++
		_, _ = io.WriteString(w, `{"url":"`+slot+`"}`)
	})
	if _, e := c.UploadQuizMedia(t.Context(), 1, 55, []byte("not an image at all")); e == nil {
		t.Error("text file accepted")
	}
	if _, e := c.UploadQuizMedia(t.Context(), 1, 55, append(append([]byte{}, pngBytes...), make([]byte, MaxQuizMediaBytes)...)); e == nil {
		t.Error("oversize file accepted")
	}
	if calls != 0 {
		t.Errorf("Canvas was contacted before the file was validated: %d", calls)
	}
	for name, u := range map[string]string{
		"http":      "http://bucket.s3.amazonaws.com/x?X-Amz-Signature=SECRET",
		"other":     "https://evil.example.com/x?X-Amz-Signature=SECRET",
		"unsigned":  "https://bucket.s3.amazonaws.com/x",
		"userinfo":  "https://user:pw@bucket.s3.amazonaws.com/x?X-Amz-Signature=SECRET",
		"s3 port":   "https://bucket.s3.amazonaws.com:8443/x?X-Amz-Signature=SECRET",
		"fragment":  "https://bucket.s3.amazonaws.com/x?X-Amz-Signature=SECRET#f",
		"lookalike": "https://s3.amazonaws.com.evil.example/x?X-Amz-Signature=SECRET",
	} {
		slot = u
		_, e := c.UploadQuizMedia(t.Context(), 1, 55, pngBytes)
		if e == nil || strings.Contains(e.Error(), "SECRET") {
			t.Errorf("%s: want a clean rejection, got %v", name, e)
		}
	}
}
