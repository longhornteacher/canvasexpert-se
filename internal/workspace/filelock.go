package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LockLocalFile holds a short exclusive guard for one destination file across
// local Engine processes, so a compare-and-replace cannot interleave with
// another adapter's. The guard file lives in the install's private .cse folder,
// never beside the destination (which may be in a provider-synced folder), and
// the operating system releases it if the process exits. It is a same-PC
// commit guard only; it does not coordinate other PCs.
func LockLocalFile(installRoot, target string) (func(), error) {
	dir := filepath.Join(installRoot, ".cse", "locks")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("could not prepare the local write guard: %w", err)
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(abs))))
	f, err := os.OpenFile(filepath.Join(dir, hex.EncodeToString(sum[:16])+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("could not open the local write guard: %w", err)
	}
	if err = lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("could not take the local write guard: %w", err)
	}
	return func() {
		_ = unlockFile(f)
		_ = f.Close()
	}, nil
}
