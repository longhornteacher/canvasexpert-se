package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CourseRoster is a read-only Canvas snapshot for the student editor. Membership
// and tags are scoped by instance and course ID, independent of editable nicknames.
type CourseRoster struct {
	Instance   string              `json:"instance"`
	CourseID   int64               `json:"course_id"`
	Members    map[string][]string `json:"members"`
	UpdatedAt  string              `json:"updated_at"`
	TagMessage string              `json:"tag_message,omitempty"`
}

func ReadRosters(root string) ([]CourseRoster, error) {
	path, err := ResolveRulesPath(root, filepath.Join(".cse", "rosters.json"))
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rosters []CourseRoster
	err = json.Unmarshal(b, &rosters)
	return rosters, err
}

func SaveRosters(root string, rosters []CourseRoster) error {
	path, err := ResolveRulesPath(root, filepath.Join(".cse", "rosters.json"))
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(rosters, "", "  ")
	if err != nil {
		return err
	}
	return WriteFile(path, append(b, '\n'), 0o600)
}
