//go:build !windows

package platform

import "errors"

func RevealFile(path string) error {
	return errors.New("file reveal requires Windows; open the prepared file's folder manually")
}
