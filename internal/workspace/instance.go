package workspace

import (
	"path/filepath"

	"canvasexpertse/internal/domain"
)

// SyncInstance records the saved Canvas origin in settings/courses.md. It does
// so only when no courses are configured yet or the origin already matches;
// otherwise it reports changed=false and leaves the file alone.
func SyncInstance(root, origin string) (changed bool, err error) {
	path := filepath.Join(root, "settings", "courses.md")
	var config domain.CourseConfig
	body, err := ReadDocument(path, &config)
	if err != nil {
		return false, err
	}
	if len(config.Courses) != 0 && config.Instance != origin {
		return false, nil
	}
	config.Instance = origin
	return true, WriteDocument(path, config, body)
}
