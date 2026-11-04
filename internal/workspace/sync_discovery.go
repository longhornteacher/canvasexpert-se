package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncMarkerFile names a UserSync folder so another PC can find it. It is plain
// JSON beside the shared setup and is never teacher rules or private data.
const syncMarkerFile = "cse-sync.json"

const syncMarkerBody = `{
  "schema": 1,
  "product": "CanvasExpert-SE",
  "note": "This folder holds CanvasExpert teacher setup shared between PCs. Keep it here; CanvasExpert program files stay on each PC."
}
`

// ensureSyncMarker adds the marker to an active UserSync folder that lacks it.
// A failure leaves sharing unchanged; discovery then relies on the folder's shape.
func ensureSyncMarker(syncRoot string) {
	p := filepath.Join(syncRoot, syncMarkerFile)
	if _, err := os.Lstat(p); err == nil || !os.IsNotExist(err) {
		return
	}
	_ = WriteFile(p, []byte(syncMarkerBody), 0644)
}

// syncSearchRoots lists this PC's OneDrive and Google Drive folders. Tests replace it.
var syncSearchRoots = defaultSyncSearchRoots

func defaultSyncSearchRoots() []string {
	// Test binaries never scan the developer's real cloud folders.
	if testing.Testing() {
		return nil
	}
	var roots []string
	for _, name := range []string{"OneDriveCommercial", "OneDriveConsumer", "OneDrive"} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			roots = append(roots, v)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if matches, err := filepath.Glob(filepath.Join(home, "OneDrive*")); err == nil {
			roots = append(roots, matches...)
		}
		roots = append(roots, filepath.Join(home, "Google Drive"), filepath.Join(home, "My Drive"))
	}
	// Google Drive for desktop mounts a drive letter, usually G:, with "My Drive" inside.
	for letter := 'D'; letter <= 'Z'; letter++ {
		roots = append(roots, string(letter)+`:\My Drive`)
	}
	return roots
}

var discovery struct {
	sync.Mutex
	at    time.Time
	found []string
}

// DiscoverUserSync finds existing UserSync folders on this PC that the UserInstall
// at root is not using yet. It returns nothing once root is bound to UserSync.
// Results are cached briefly because hub pages ask on every render.
func DiscoverUserSync(root string) []string {
	loc, err := GetRulesLocation(root)
	if err != nil || loc.Mode == "sync" {
		return nil
	}
	discovery.Lock()
	defer discovery.Unlock()
	if discovery.found == nil || time.Since(discovery.at) > time.Minute {
		discovery.found, discovery.at = scanUserSync(), time.Now()
	}
	var out []string
	for _, p := range discovery.found {
		if !pathOverlaps(p, loc.InstallRoot) {
			out = append(out, p)
		}
	}
	return out
}

// UnconnectedSyncNote explains, in teacher words, that this PC has a shared
// folder it is not using. It is empty when nothing was found.
func UnconnectedSyncNote(found []string) string {
	switch len(found) {
	case 0:
		return ""
	case 1:
		return "Your CanvasExpert shared folder is at " + found[0] + ", but this PC isn't using it yet, so settings, student notes and parent contacts saved on your other PC aren't here. Connect this PC to that folder."
	}
	return "This PC found more than one CanvasExpert shared folder (" + strings.Join(found, "; ") + ") and isn't using any of them yet, so settings, student notes and parent contacts saved on your other PC aren't here. Connect this PC to the one your other PC uses."
}

func resetSyncDiscovery() {
	discovery.Lock()
	discovery.found = nil
	discovery.Unlock()
}

// scanUserSync looks in each cloud folder and two levels below it. Dot folders
// are skipped; a cap keeps a very large drive from slowing a page.
func scanUserSync() []string {
	found := []string{}
	seen := map[string]bool{}
	visited := 0
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if visited >= 2000 {
			return
		}
		visited++
		if isUserSyncDir(dir) {
			key := strings.ToLower(filepath.Clean(dir))
			if !seen[key] {
				seen[key] = true
				found = append(found, filepath.Clean(dir))
			}
			return
		}
		if depth == 2 {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Type()&os.ModeSymlink == 0 {
				walk(filepath.Join(dir, e.Name()), depth+1)
			}
		}
	}
	roots := map[string]bool{}
	for _, r := range syncSearchRoots() {
		key := strings.ToLower(filepath.Clean(r))
		if roots[key] {
			continue
		}
		roots[key] = true
		if info, err := os.Stat(r); err == nil && info.IsDir() {
			walk(r, 0)
		}
	}
	return found
}

// isUserSyncDir recognizes the marker, or the shape of a UserSync folder made
// before the marker existed: shared setup without the Engine or a git checkout.
func isUserSyncDir(dir string) bool {
	if b, err := os.ReadFile(filepath.Join(dir, syncMarkerFile)); err == nil {
		var m struct {
			Product string `json:"product"`
		}
		return json.Unmarshal(b, &m) == nil && m.Product == "CanvasExpert-SE"
	}
	exists := func(name string) bool { _, err := os.Stat(filepath.Join(dir, name)); return err == nil }
	if !exists(filepath.Join("context", "courses.md")) || !(exists("AGENTS.md") || exists("playbooks")) {
		return false
	}
	return !exists("cse.cmd") && !exists("engine") && !exists(".git")
}
