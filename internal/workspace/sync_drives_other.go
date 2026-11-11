//go:build !windows

package workspace

// localDriveRoots is empty where there are no drive letters.
func localDriveRoots() []string { return nil }
