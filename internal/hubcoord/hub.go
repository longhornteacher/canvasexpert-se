package hubcoord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const recordName = "hub.json"

type Location struct{ URL string }
type record struct {
	URL string `json:"url"`
}

func CanonicalRoot(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	abs = filepath.Clean(abs)
	if runtime.GOOS == "windows" {
		abs = strings.ToLower(abs)
	}
	return abs
}

func Find(root string) (Location, bool) {
	var loc Location
	root = CanonicalRoot(root)
	b, err := os.ReadFile(filepath.Join(root, ".cse", recordName))
	if err != nil {
		return loc, false
	}
	var rec record
	if json.Unmarshal(b, &rec) != nil {
		return loc, false
	}
	u, err := validURL(rec.URL)
	if err != nil {
		return loc, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rec.URL+"/_cse/hub-identity", nil)
	if err != nil {
		return loc, false
	}
	c := &http.Client{Timeout: 700 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Do(req)
	if err != nil {
		return loc, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return loc, false
	}
	var identity struct {
		InstallRoot string `json:"install_root"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&identity) != nil || CanonicalRoot(identity.InstallRoot) != root {
		return loc, false
	}
	loc.URL = u.String()
	return loc, true
}

func validURL(raw string) (*url.URL, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid local hub URL")
	}
	return u, nil
}

func Acquire(root string, timeout time.Duration) (func(), error) {
	dir := filepath.Join(root, ".cse")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("could not prepare private hub state: %w", err)
	}
	lock := filepath.Join(dir, "hub-start.lock")
	deadline := time.Now().Add(timeout)
	for {
		err := os.Mkdir(lock, 0700)
		if err == nil {
			return func() { _ = os.Remove(lock) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("could not coordinate hub startup: %w", err)
		}
		if time.Now().After(deadline) {
			return nil, errors.New("another hub start is still being coordinated; retry in a few seconds")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func Write(root, origin string) error {
	path := filepath.Join(root, ".cse", recordName)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("could not prepare private hub state: %w", err)
	}
	b, _ := json.Marshal(record{URL: origin})
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return fmt.Errorf("could not save private hub location: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("could not save private hub location: %w", err)
	}
	return nil
}
func Remove(root, origin string) {
	path := filepath.Join(root, ".cse", recordName)
	b, e := os.ReadFile(path)
	if e != nil {
		return
	}
	var r record
	if json.Unmarshal(b, &r) == nil && r.URL == origin {
		_ = os.Remove(path)
	}
}
