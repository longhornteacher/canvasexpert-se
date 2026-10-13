package workspace

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"canvasexpertse/internal/domain"
)

// SyncInstance records the saved Canvas origin in context/courses.md when the
// origin is unset, is still the starter placeholder, there are no saved courses,
// or the origin already matches. A populated different-origin configuration is
// left alone and reports changed=false.
func SyncInstance(root, origin string) (changed bool, err error) {
	path := filepath.Join(root, "context", "courses.md")
	var config domain.CourseConfig
	body, err := ReadDocument(path, &config)
	if err != nil {
		return false, err
	}
	configuredOrigin := strings.TrimRight(strings.TrimSpace(config.Instance), "/")
	placeholder := configuredOrigin == "" || strings.Contains(configuredOrigin, "canvas.example.edu")
	if !placeholder && len(config.Courses) != 0 && !strings.EqualFold(configuredOrigin, strings.TrimRight(strings.TrimSpace(origin), "/")) {
		return false, nil
	}
	config.Instance = origin
	return true, WriteDocument(path, config, body)
}

// SafeOrigin reduces a Canvas address to scheme and host for display. Userinfo,
// path, query and fragment are dropped; an unusable value is not echoed.
func SafeOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "(not set)"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Hostname() == "" {
		return "(not a valid Canvas address)"
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
}

// OriginMismatchMessage explains a saved-credential versus courses.md origin
// difference with both sanitized origins and the two possible corrections.
func OriginMismatchMessage(configured, saved string) string {
	return fmt.Sprintf("context/courses.md uses %s but this PC's saved Canvas credential is for %s. If %s is correct, run cse setup and enter that address; if %s is correct, change the instance line in context/courses.md to %s.",
		SafeOrigin(configured), SafeOrigin(saved), SafeOrigin(configured), SafeOrigin(saved), SafeOrigin(saved))
}

// ConfiguredInstance returns the instance recorded in context/courses.md, or ""
// when it cannot be read.
func ConfiguredInstance(root string) string {
	var config domain.CourseConfig
	if _, err := ReadDocument(filepath.Join(root, "context", "courses.md"), &config); err != nil {
		return ""
	}
	return config.Instance
}
