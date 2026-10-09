// Package canvas owns authenticated HTTP transport for one Canvas origin.
package canvas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const maxJSON = 32 << 20

// Error deliberately excludes URLs, tokens, underlying transport messages, and
// response bodies. Uncertain means a write might have reached Canvas.
type Error struct {
	Kind      string
	Status    int
	Uncertain bool
}

func (e *Error) Error() string {
	switch e.Kind {
	case "authentication":
		return "Canvas authentication failed; run cse setup"
	case "forbidden":
		return "Canvas denied this operation"
	case "unsupported":
		return "Canvas does not support this operation"
	case "not_found":
		return "Canvas item was not found"
	case "validation":
		return "invalid Canvas request or response"
	case "throttled":
		return "Canvas rate limit reached; try again later"
	case "uncertain":
		return "Canvas write completion is uncertain; check live state before retrying"
	case "canceled":
		return "Canvas request canceled"
	default:
		return "Canvas connection failed"
	}
}

type Client struct {
	origin        *url.URL
	token         string
	http          *http.Client
	requests      atomic.Int64
	mu            sync.Mutex
	active, limit int
	changed       chan struct{}
	nextAllowed   time.Time
	backoff       time.Duration
}

type countingTransport struct {
	base  http.RoundTripper
	count *atomic.Int64
}

func (t countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.count.Add(1)
	return t.base.RoundTrip(r)
}

// New accepts an HTTPS origin, without credentials, a path, or query parameters.
// The default transport uses normal platform TLS trust and environment proxies.
func New(origin, token string) (*Client, error) {
	u, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" || strings.ContainsAny(token, "\r\n") || strings.TrimSpace(token) == "" {
		return nil, &Error{Kind: "validation"}
	}
	u.Path = ""
	u.Host = strings.ToLower(u.Host)
	c := &Client{origin: u, token: token, limit: 4, changed: make(chan struct{}), backoff: 250 * time.Millisecond}
	c.http = &http.Client{Timeout: 45 * time.Second, Transport: countingTransport{http.DefaultTransport, &c.requests}}
	c.http.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || req.URL.Scheme != "https" || req.URL.User != nil {
			return &Error{Kind: "validation"}
		}
		// Check every hop: net/http otherwise forwards Authorization to subdomains.
		req.Header.Del("Authorization")
		if !sameOrigin(c.origin, req.URL) {
			if via[0].Header.Get("X-CSE-Download") != "1" {
				return &Error{Kind: "validation"}
			}
		} else if via[0].Header.Get("Authorization") != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		if via[0].Method != http.MethodGet {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return c, nil
}

func (c *Client) Origin() string  { return c.origin.String() }
func (c *Client) Requests() int64 { return c.requests.Load() }

func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}
func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Hostname(), b.Hostname()) && effectivePort(a) == effectivePort(b)
}

func (c *Client) apiURL(path string) (*url.URL, error) {
	u, err := url.Parse(path)
	if err != nil || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return nil, &Error{Kind: "validation"}
	}
	u = c.origin.ResolveReference(u)
	if !sameOrigin(c.origin, u) || !(strings.HasPrefix(u.Path, "/api/v1/") || strings.HasPrefix(u.Path, "/api/quiz/v1/")) {
		return nil, &Error{Kind: "validation"}
	}
	return u, nil
}

func (c *Client) signalLocked() { close(c.changed); c.changed = make(chan struct{}) }
func (c *Client) acquire(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return &Error{Kind: "canceled"}
		}
		c.mu.Lock()
		delay := time.Until(c.nextAllowed)
		if c.active < c.limit && delay <= 0 {
			c.active++
			c.mu.Unlock()
			return nil
		}
		ch := c.changed
		c.mu.Unlock()
		if delay > 0 {
			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return &Error{Kind: "canceled"}
			case <-ch:
				t.Stop()
			case <-t.C:
			}
		} else {
			select {
			case <-ctx.Done():
				return &Error{Kind: "canceled"}
			case <-ch:
			}
		}
	}
}
func (c *Client) release() { c.mu.Lock(); c.active--; c.signalLocked(); c.mu.Unlock() }
func (c *Client) throttle(delay time.Duration) {
	c.mu.Lock()
	c.limit = 1
	if until := time.Now().Add(delay); until.After(c.nextAllowed) {
		c.nextAllowed = until
	}
	c.signalLocked()
	c.mu.Unlock()
}
func pause(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return &Error{Kind: "canceled"}
	case <-t.C:
		return nil
	}
}
func retryDelay(h http.Header, fallback time.Duration) time.Duration {
	d := fallback
	if seconds, err := strconv.Atoi(h.Get("Retry-After")); err == nil && seconds >= 0 {
		// Clamp before duration multiplication, which can otherwise overflow
		// and turn a long server-requested delay into an immediate retry.
		if seconds > 30 {
			seconds = 30
		}
		d = time.Duration(seconds) * time.Second
	} else if date, err := http.ParseTime(h.Get("Retry-After")); err == nil {
		d = time.Until(date)
	}
	if d < 0 {
		d = 0
	}
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}
func statusError(status int, body []byte, write bool) *Error {
	kind := "transport"
	switch status {
	case 401:
		kind = "authentication"
	case 403:
		kind = "forbidden"
		if strings.Contains(strings.ToLower(string(body)), "rate limit exceeded") {
			kind = "throttled"
		}
	case 404:
		kind = "not_found"
	case 400, 409, 422:
		kind = "validation"
	case 405, 501:
		kind = "unsupported"
	case 429:
		kind = "throttled"
	}
	uncertain := write && (status == 408 || status >= 500 || (status >= 300 && status < 400))
	if uncertain {
		kind = "uncertain"
	}
	return &Error{Kind: kind, Status: status, Uncertain: uncertain}
}

// request retries safe reads only. The slot remains occupied through body use.
func (c *Client) request(ctx context.Context, method string, u *url.URL, body []byte, download bool, consume func(*http.Response) error) error {
	write := method != http.MethodGet
	attempts := 1
	if !write {
		attempts = 3
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if err := c.acquire(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
		if err != nil {
			c.release()
			return &Error{Kind: "validation"}
		}
		if sameOrigin(c.origin, u) {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		req.Header.Set("Accept", "application/json")
		if write {
			req.Header.Set("Content-Type", "application/json")
		}
		if download {
			req.Header.Set("X-CSE-Download", "1")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			c.release()
			if write {
				return &Error{Kind: "uncertain", Uncertain: true}
			}
			var ce *Error
			if errors.As(err, &ce) {
				return ce
			}
			if ctx.Err() != nil {
				return &Error{Kind: "canceled"}
			}
			if attempt+1 < attempts {
				if err = pause(ctx, c.backoff<<attempt); err != nil {
					return err
				}
				continue
			}
			return &Error{Kind: "transport"}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			c.release()
			ce := statusError(resp.StatusCode, data, write)
			delay := retryDelay(resp.Header, c.backoff<<attempt)
			if ce.Kind == "throttled" {
				c.throttle(delay)
			}
			retry := ce.Kind == "throttled" || resp.StatusCode == 500 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504
			if !write && retry && attempt+1 < attempts {
				if err = pause(ctx, delay); err != nil {
					return err
				}
				continue
			}
			return ce
		}
		err = consume(resp)
		resp.Body.Close()
		c.release()
		if err != nil && write {
			return &Error{Kind: "uncertain", Uncertain: true}
		}
		var ce *Error
		if !download && errors.As(err, &ce) && ce.Kind == "transport" && attempt+1 < attempts {
			if err = pause(ctx, c.backoff<<attempt); err != nil {
				return err
			}
			continue
		}
		return err
	}
	return &Error{Kind: "transport"}
}

func readJSON(resp *http.Response, out any) error {
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxJSON+1))
	if err != nil {
		return &Error{Kind: "transport"}
	}
	if len(data) > maxJSON {
		return &Error{Kind: "validation"}
	}
	if out == nil || (resp.StatusCode == http.StatusNoContent && len(bytes.TrimSpace(data)) == 0) {
		return nil
	}
	if err = json.Unmarshal(data, out); err != nil {
		return &Error{Kind: "validation"}
	}
	return nil
}
func (c *Client) Get(ctx context.Context, path string, out any) error {
	u, err := c.apiURL(path)
	if err != nil {
		return err
	}
	return c.request(ctx, http.MethodGet, u, nil, false, func(resp *http.Response) error { return readJSON(resp, out) })
}

// List collects every page atomically; a failed page leaves out unchanged.
func (c *Client) List(ctx context.Context, path string, out any) error {
	v := reflect.ValueOf(out)
	if v.Kind() != reflect.Ptr || v.IsNil() || v.Elem().Kind() != reflect.Slice {
		return &Error{Kind: "validation"}
	}
	u, err := c.apiURL(path)
	if err != nil {
		return err
	}
	all := make([]json.RawMessage, 0)
	seen := map[string]bool{}
	for page := 0; u != nil; page++ {
		if page >= 1000 || seen[u.String()] {
			return &Error{Kind: "validation"}
		}
		seen[u.String()] = true
		var items []json.RawMessage
		var link string
		pageURL := u
		err = c.request(ctx, http.MethodGet, u, nil, false, func(resp *http.Response) error {
			link = resp.Header.Get("Link")
			// Relative pagination references belong to the URL that produced
			// this response, including a permitted same-origin redirect.
			pageURL = resp.Request.URL
			return readJSON(resp, &items)
		})
		if err != nil {
			return err
		}
		all = append(all, items...)
		next, err := nextLink(link)
		if err != nil {
			return err
		}
		if next == "" {
			u = nil
		} else {
			rel, parseErr := url.Parse(next)
			if parseErr != nil {
				return &Error{Kind: "validation"}
			}
			u, err = c.apiURL(pageURL.ResolveReference(rel).String())
			if err != nil {
				return err
			}
		}
	}
	data, _ := json.Marshal(all)
	decoded := reflect.New(v.Elem().Type())
	if err = json.Unmarshal(data, decoded.Interface()); err != nil {
		return &Error{Kind: "validation"}
	}
	v.Elem().Set(decoded.Elem())
	return nil
}

func nextLink(header string) (string, error) {
	// Split outside angle brackets and quoted parameters (commas can occur in URLs).
	start := 0
	angle, quoted := false, false
	var parts []string
	for i, r := range header {
		switch r {
		case '<':
			if !quoted {
				angle = true
			}
		case '>':
			if !quoted {
				angle = false
			}
		case '"':
			if !angle {
				quoted = !quoted
			}
		case ',':
			if !angle && !quoted {
				parts = append(parts, header[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, header[start:])
	next := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		end := strings.Index(part, ">")
		if !strings.HasPrefix(part, "<") || end < 1 {
			return "", &Error{Kind: "validation"}
		}
		for _, param := range strings.Split(part[end+1:], ";") {
			kv := strings.SplitN(strings.TrimSpace(param), "=", 2)
			if len(kv) == 2 && strings.EqualFold(kv[0], "rel") {
				for _, rel := range strings.Fields(strings.Trim(kv[1], "\"")) {
					if rel == "next" {
						if next != "" {
							return "", &Error{Kind: "validation"}
						}
						next = part[1:end]
					}
				}
			}
		}
	}
	return next, nil
}

func (c *Client) Write(ctx context.Context, method, path string, body any, out any) error {
	method = strings.ToUpper(method)
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch && method != http.MethodDelete {
		return &Error{Kind: "validation"}
	}
	u, err := c.apiURL(path)
	if err != nil {
		return err
	}
	data, err := json.Marshal(body)
	if err != nil {
		return &Error{Kind: "validation"}
	}
	return c.request(ctx, method, u, data, false, func(resp *http.Response) error { return readJSON(resp, out) })
}

// Download preserves bytes and does not replay a partially delivered download.
// The caller should remove an incomplete destination file on failure.
func (c *Client) Download(ctx context.Context, rawURL string, dst io.Writer) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return &Error{Kind: "validation"}
	}
	if dst == nil {
		return &Error{Kind: "validation"}
	}
	return c.request(ctx, http.MethodGet, u, nil, true, func(resp *http.Response) error {
		_, err := io.Copy(dst, resp.Body)
		if err != nil {
			return &Error{Kind: "transport"}
		}
		return nil
	})
}
