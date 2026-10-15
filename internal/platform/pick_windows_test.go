//go:build windows

package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestPickFolderAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if path, err := PickFolder(ctx); path != "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("%q %v", path, err)
	}
}

// Opt in because these checks briefly open real native dialogs. They select
// only a temporary synthetic directory and never touch a teacher's Hub or UserSync.
func TestNativeFolderPickerWithoutPATH(t *testing.T) {
	if os.Getenv("CSE_TEST_NATIVE_PICKER") != "1" {
		t.Skip("set CSE_TEST_NATIVE_PICKER=1 for the native dialog smoke check")
	}
	t.Setenv("PATH", "")
	want := filepath.Join(t.TempDir(), "Synthetic UserSync ü space")
	if err := os.Mkdir(want, 0700); err != nil {
		t.Fatal(err)
	}
	selection, _ := syscall.UTF16PtrFromString(want)
	original := folderCallback
	defer func() { folderCallback = original }()
	send := folderUser.NewProc("SendMessageW")
	for _, cancelDialog := range []bool{false, true} {
		folderCallback = syscall.NewCallback(func(window, message, unused, data uintptr) uintptr {
			if message == 1 {
				folderReady <- window
				command := uintptr(2) // IDCANCEL
				if !cancelDialog {
					send.Call(window, 0x0467, 1, uintptr(unsafe.Pointer(selection))) // BFFM_SETSELECTIONW
					command = 1                                                      // IDOK
				}
				folderPost.Call(window, 0x0111, command, 0) // WM_COMMAND
			}
			return 0
		})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		got, err := PickFolder(ctx)
		cancel()
		runtime.KeepAlive(selection)
		if err != nil {
			t.Fatal(err)
		}
		if cancelDialog {
			if got != "" {
				t.Fatal("cancel returned a path")
			}
		} else if !strings.EqualFold(got, want) {
			t.Fatalf("selected %q, want %q", got, want)
		}
	}
	// A disconnected browser/request closes its still-open native dialog.
	folderCallback = original
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if _, err := PickFolder(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("context close: %v", err)
	}
}
