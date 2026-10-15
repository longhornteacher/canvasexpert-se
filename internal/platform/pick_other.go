//go:build !windows

package platform

import (
	"context"
	"errors"
)

func PickFolder(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", errors.New("the folder chooser is available on Windows; paste the folder path instead")
}
