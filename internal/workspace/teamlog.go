package workspace

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TeamLogEntry is one write made with a teammate's key. It never carries
// student names, comment text or grades.
type TeamLogEntry struct {
	Time      string `json:"time"`
	Teammate  string `json:"teammate"`
	Course    string `json:"course"`
	CourseID  int64  `json:"course_id"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	Uncertain bool   `json:"uncertain"`
}

func teamLogFile(installRoot string) string {
	return filepath.Join(installRoot, ".cse", "team-log.jsonl")
}

// RedactTeamLogPath drops the query and replaces a numeric segment after
// submissions, users or students with :student.
func RedactTeamLogPath(path string) string {
	path, _, _ = strings.Cut(path, "?")
	parts := strings.Split(path, "/")
	for i := 1; i < len(parts); i++ {
		switch parts[i-1] {
		case "submissions", "users", "students":
			if parts[i] != "" && strings.Trim(parts[i], "0123456789") == "" {
				parts[i] = ":student"
			}
		}
	}
	return strings.Join(parts, "/")
}

// AppendTeamLog appends one line under the UserInstall root. The log is per
// machine, so it never follows a UserSync authority.
func AppendTeamLog(installRoot string, e TeamLogEntry) error {
	if e.Time == "" {
		e.Time = time.Now().UTC().Format(time.RFC3339)
	}
	e.Path = RedactTeamLogPath(e.Path)
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	p := teamLogFile(installRoot)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// ReadTeamLog returns the newest entries first, for one teammate label or for
// everyone when label is empty. A missing log is empty.
func ReadTeamLog(installRoot, label string, limit int) ([]TeamLogEntry, error) {
	f, err := os.Open(teamLogFile(installRoot))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var all []TeamLogEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var e TeamLogEntry
		if json.Unmarshal(sc.Bytes(), &e) == nil && (label == "" || e.Teammate == label) {
			all = append(all, e)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	var out []TeamLogEntry
	for i := len(all) - 1; i >= 0 && (limit <= 0 || len(out) < limit); i-- {
		out = append(out, all[i])
	}
	return out, nil
}
