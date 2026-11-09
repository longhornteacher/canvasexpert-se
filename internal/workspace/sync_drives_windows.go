//go:build windows

package workspace

import "golang.org/x/sys/windows"

// localDriveRoots lists the drive letters worth looking in for Google Drive's
// "My Drive". Network drives and unassigned letters are left out: a mapped
// school drive that is unreachable (a laptop at home) can take many seconds to
// answer, and Google Drive for desktop is not a network drive. GetDriveType
// answers from the drive table without contacting the network.
func localDriveRoots() []string {
	var roots []string
	for letter := 'D'; letter <= 'Z'; letter++ {
		root := string(letter) + `:\`
		p, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if t := windows.GetDriveType(p); t == windows.DRIVE_REMOTE || t == windows.DRIVE_NO_ROOT_DIR {
			continue
		}
		roots = append(roots, root)
	}
	return roots
}
