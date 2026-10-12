// Package doctor provides fixed, local-only Install diagnostics shared by the
// CLI and MCP adapter. It does not inspect student work or client settings.
package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/hubcoord"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

type Check struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Explanation string `json:"explanation"`
	Action      string `json:"action,omitempty"`
}

type Report struct {
	InstallRoot string  `json:"install_root"`
	Checks      []Check `json:"checks"`
	Failed      bool    `json:"failed"`
}

func (r *Report) add(name, status, explanation, action string) {
	r.Checks = append(r.Checks, Check{Name: name, Status: status, Explanation: explanation, Action: action})
	if status == "fail" {
		r.Failed = true
	}
}

// Run executes every local check independently. exe and build identify the
// running Engine, not a client registration or a remote service.
func Run(root, exe, build string) Report {
	return RunMode(root, exe, build, false)
}

// RunMode adds bounded live checks unless offline is requested. Offline mode
// still reports those checks, marked not_verified, without calling Canvas or
// probing the hub.
func RunMode(root, exe, build string, offline bool) Report {
	return runMode(root, exe, build, offline, platform.LoadCredential, platform.CredentialSelfTest, authenticatedCheck, findHub)
}

func run(root, exe, build string, loadCredential func() (platform.Credential, error), credentialSelfTest func() error) Report {
	return runMode(root, exe, build, true, loadCredential, credentialSelfTest, authenticatedCheck, findHub)
}

type liveCheckFunc func(string, string) error
type hubCheckFunc func(string) bool

func runMode(root, exe, build string, offline bool, loadCredential func() (platform.Credential, error), credentialSelfTest func() error, liveCheck liveCheckFunc, hubCheck hubCheckFunc) Report {
	r := Report{InstallRoot: root, Checks: make([]Check, 0, 8)}
	if st, err := os.Stat(exe); err != nil || st.IsDir() {
		r.add("engine", "fail", "Running Engine identity is unavailable; executable: "+exe+"; build: "+build, "Run the Engine from the extracted Install and retry.")
	} else {
		r.add("engine", "pass", "Executable: "+exe+"; build: "+build, "")
	}

	if offline {
		r.add("canvas_connection", "not_verified", "Canvas connection check skipped by --offline.", "Run cse doctor without --offline to check the saved connection.")
	} else {
		credential, credErr := loadCredential()
		switch {
		case credErr != nil && strings.Contains(strings.ToLower(credErr.Error()), "no se credential"):
			r.add("canvas_connection", "not_configured", "No Canvas credential is saved on this machine; no live request was made.", "Use cse setup when ready to connect.")
		case credErr != nil:
			r.add("canvas_connection", "not_verified", "The saved Canvas credential could not be read; no live request was made.", "Check local credential storage access, then retry.")
		default:
			err := liveCheck(credential.Origin, credential.Token)
			var ce *canvas.Error
			switch {
			case err == nil:
				r.add("canvas_connection", "pass", "Authenticated read-only Canvas check succeeded.", "")
			case errors.As(err, &ce) && ce.Kind == "authentication":
				r.add("canvas_connection", "fail", "Canvas rejected authentication for the read-only connection check.", "Use cse setup to review the saved connection.")
			case errors.As(err, &ce) && ce.Kind == "forbidden":
				r.add("canvas_connection", "fail", "Canvas denied the authenticated connection check.", "Check that the saved account can access the Canvas API.")
			default:
				r.add("canvas_connection", "not_verified", "Canvas could not be reached or returned an unusable response; this does not establish that the credential is invalid.", "Check the network, DNS, TLS, proxy or Canvas service, then retry.")
			}
		}
	}
	if offline {
		r.add("hub", "not_verified", "Local hub discovery skipped by --offline.", "Run cse doctor without --offline to check for a running hub.")
	} else if hubCheck(root) {
		r.add("hub", "pass", "A healthy hub for this Install answered its non-secret identity check.", "")
	} else {
		r.add("hub", "not_verified", "No healthy same-Install hub was found; discovery may be absent or stale.", "Run cse ui to open the Install hub.")
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		r.add("install", "fail", "Install directory is not readable: "+root, "Choose the extracted Install directory and retry.")
	} else if f, err := os.Open(root); err != nil {
		r.add("install", "fail", "Install directory cannot be read: "+root, "Check local folder access.")
	} else {
		_ = f.Close()
		r.add("install", "pass", "Install directory is readable: "+root, "")
	}

	loc, locErr := workspace.GetRulesLocation(root)
	if locErr != nil {
		r.add("active_rules", "fail", "Active teacher rules are unavailable: "+locErr.Error(), "Restore access to the configured rules directory; doctor does not switch to local rules.")
	} else {
		failed := ""
		for _, logical := range []string{"AGENTS.md", "playbooks/setup.md"} {
			path, err := workspace.ResolveTeacherPath(root, logical)
			if err == nil {
				var b []byte
				b, err = os.ReadFile(path)
				if err == nil && len(b) == 0 {
					err = fmt.Errorf("file is empty")
				}
			}
			if err != nil {
				failed = logical + " cannot be read: " + err.Error()
				break
			}
		}
		if failed != "" {
			r.add("active_rules", "fail", "Active rules at "+loc.RulesRoot+" are not fully readable: "+failed, "Restore the named active instruction or playbook, then retry.")
		} else {
			r.add("active_rules", "pass", "Active AGENTS.md and playbooks/setup.md are readable from "+loc.RulesRoot+" ("+loc.Mode+" mode).", "")
		}
	}

	var config domain.CourseConfig
	_, err := workspace.ReadDocument(filepath.Join(root, "context", "courses.md"), &config)
	if err != nil {
		r.add("course_config", "fail", "context/courses.md could not be parsed: "+err.Error(), "Repair its schema and course entries, then retry.")
	} else if len(config.Courses) == 0 {
		r.add("course_config", "not_configured", "Course configuration is valid, with no saved courses.", "Use cse setup to connect Canvas and select courses when ready.")
	} else {
		r.add("course_config", "pass", fmt.Sprintf("Course configuration is valid (%d saved course(s)).", len(config.Courses)), "")
	}

	if err != nil {
		r.add("saved_origin", "not_verified", "Saved Canvas origin cannot be compared until context/courses.md parses.", "Repair the course configuration, then retry.")
	} else {
		credential, credErr := loadCredential()
		switch {
		case credErr != nil && strings.Contains(strings.ToLower(credErr.Error()), "no se credential"):
			r.add("saved_origin", "not_configured", "No Canvas credential is saved on this machine; origin consistency is not applicable.", "Use cse setup when ready to connect.")
		case credErr != nil && strings.Contains(credErr.Error(), "requires Windows"):
			r.add("saved_origin", "not_verified", "Saved credential origin cannot be checked on this platform.", "Run this check on the Windows Install machine.")
		case credErr != nil:
			r.add("saved_origin", "fail", "Saved Canvas credential could not be read; no credential data was displayed.", "Check Windows Credential Manager access, then retry.")
		case sameOrigin(credential.Origin, config.Instance):
			r.add("saved_origin", "pass", "Saved Canvas origin matches context/courses.md.", "")
		default:
			r.add("saved_origin", "fail", "Saved Canvas origin differs from context/courses.md.", "Use cse setup for the intended Canvas instance or correct context/courses.md.")
		}
	}

	if err := credentialSelfTest(); err != nil {
		if strings.Contains(err.Error(), "requires Windows") {
			r.add("credential_round_trip", "not_verified", "Native dummy credential round trip is supported on Windows only.", "Run this check on the Windows Install machine.")
		} else {
			r.add("credential_round_trip", "fail", "Native dummy credential round trip failed: "+err.Error(), "Check Windows Credential Manager access, then retry.")
		}
	} else {
		r.add("credential_round_trip", "pass", "Native dummy credential byte round trip passed; temporary dummy credential removed.", "")
	}
	return r
}

func authenticatedCheck(origin, token string) error {
	c, err := canvas.New(origin, token)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Decode only into a private throwaway value. The identity is never exposed.
	var identity struct {
		ID int64 `json:"id"`
	}
	if err := c.Get(ctx, "/api/v1/users/self", &identity); err != nil {
		return err
	}
	if identity.ID <= 0 {
		return &canvas.Error{Kind: "validation"}
	}
	return nil
}

func findHub(root string) bool { _, ok := hubcoord.Find(root); return ok }

func sameOrigin(a, b string) bool {
	return strings.EqualFold(strings.TrimRight(strings.TrimSpace(a), "/"), strings.TrimRight(strings.TrimSpace(b), "/"))
}

func (r Report) JSON() ([]byte, error) { return json.MarshalIndent(r, "", "  ") }

func (r Report) Human() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Install: %s\n", r.InstallRoot)
	for _, check := range r.Checks {
		fmt.Fprintf(&b, "[%s] %s: %s\n", strings.ToUpper(check.Status), check.Name, check.Explanation)
		if check.Action != "" {
			fmt.Fprintf(&b, "  Action: %s\n", check.Action)
		}
	}
	return b.String()
}
