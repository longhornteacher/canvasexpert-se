package workspace

import (
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
	if !placeholder && len(config.Courses) != 0 && configuredOrigin != strings.TrimRight(origin, "/") {
		return false, nil
	}
	config.Instance = origin
	return true, WriteDocument(path, config, body)
}
