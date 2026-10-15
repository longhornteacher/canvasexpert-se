//go:build windows

package platform

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	folderShell        = syscall.NewLazyDLL("shell32.dll")
	folderBrowse       = folderShell.NewProc("SHBrowseForFolderW")
	folderPath         = folderShell.NewProc("SHGetPathFromIDListEx")
	folderOLE          = syscall.NewLazyDLL("ole32.dll")
	folderInitialize   = folderOLE.NewProc("CoInitializeEx")
	folderUninitialize = folderOLE.NewProc("CoUninitialize")
	folderFree         = folderOLE.NewProc("CoTaskMemFree")
	folderUser         = syscall.NewLazyDLL("user32.dll")
	folderForeground   = folderUser.NewProc("SetForegroundWindow")
	folderPosition     = folderUser.NewProc("SetWindowPos")
	folderPost         = folderUser.NewProc("PostMessageW")

	// One teacher can have one modal picker open. A single process-lifetime
	// callback avoids allocating a Windows callback for every Browse click.
	folderPickerMu sync.Mutex
	folderReady    chan uintptr // accessed only on the locked dialog thread
	folderCallback = syscall.NewCallback(func(window, message, unused, data uintptr) uintptr {
		if message == 1 { // BFFM_INITIALIZED
			// Keep the teacher's dialog visible above the browser.
			folderPosition.Call(window, ^uintptr(0), 0, 0, 0, 0, 0x0001|0x0002) // HWND_TOPMOST, no move/size
			folderForeground.Call(window)
			folderReady <- window
		}
		return 0
	})
)

// Native BROWSEINFOW uses pointer-size alignment on Windows x86/x64/ARM64.
type folderBrowseInfo struct {
	Owner, Root uintptr
	DisplayName *uint16
	Title       *uint16
	Flags       uint32
	Callback    uintptr
	Data        uintptr
	Image       int32
}

// PickFolder uses Windows' own Shell dialog. It needs no PowerShell, .NET,
// external executable, PATH entry, or elevation. Cancel returns an empty path.
func PickFolder(ctx context.Context) (string, error) {
	folderPickerMu.Lock()
	defer folderPickerMu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := folderInitialize.Call(0, 2) // COINIT_APARTMENTTHREADED
	if int32(hr) < 0 {
		return "", errors.New("the Windows folder chooser could not initialize; paste the folder path instead")
	}
	defer folderUninitialize.Call()

	shown, done := make(chan uintptr, 1), make(chan struct{})
	workerDone := make(chan struct{})
	folderReady = shown
	defer func() { close(done); <-workerDone; folderReady = nil }()
	go func() {
		defer close(workerDone)
		select {
		case window := <-shown:
			select {
			case <-ctx.Done():
				folderPost.Call(window, 0x0010, 0, 0) // WM_CLOSE
			case <-done:
			}
		case <-done:
		}
	}()

	title, _ := syscall.UTF16PtrFromString("Choose your teacher setup UserSync folder")
	var displayName [260]uint16
	info := folderBrowseInfo{DisplayName: &displayName[0], Title: title,
		Flags: 0x0001 | 0x0010 | 0x0040, Callback: folderCallback} // filesystem only, edit box, resizable/new-folder UI
	id, _, _ := folderBrowse.Call(uintptr(unsafe.Pointer(&info)))
	if id != 0 {
		defer folderFree.Call(id)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if id == 0 {
		return "", nil
	}
	// Ex avoids the legacy MAX_PATH limit and preserves Unicode paths.
	path := make([]uint16, 32768)
	ok, _, _ := folderPath.Call(id, uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)), 0)
	if ok == 0 {
		return "", errors.New("choose a filesystem folder, or paste its path instead")
	}
	return syscall.UTF16ToString(path), nil
}
