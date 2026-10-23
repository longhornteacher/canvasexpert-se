//go:build windows

package platform

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

// Drawn by scripts/make-shortcut-icon.ps1.
//
//go:embed assets/CanvasExpert.ico
var shortcutIcon []byte

// shortcutIconFile writes the icon beside the Engine, which git and the ZIP leave
// to the Engine. A missing icon only costs the shortcut its picture.
func shortcutIconFile(root string) (string, bool) {
	path := filepath.Join(root, "engine", "CanvasExpert.ico")
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, shortcutIcon) {
		return path, true
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", false
	}
	if err := os.WriteFile(path, shortcutIcon, 0o644); err != nil {
		return "", false
	}
	return path, true
}

// CreateDesktopShortcut uses the current user's actual Desktop, including a
// redirected Desktop. Windows handles the link without an external script.
func CreateDesktopShortcut(root string) (string, error) {
	if err := shortcutLauncher(root); err != nil {
		return "", err
	}
	desktop, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, 0)
	if err != nil {
		return "", fmt.Errorf("find your Windows Desktop: %w", err)
	}
	return createDesktopShortcutAt(root, desktop)
}

func shortcutLauncher(root string) error {
	path := filepath.Join(root, "webui.cmd")
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("find %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s must be a file", path)
	}
	return nil
}

func createDesktopShortcutAt(root, desktop string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if err := shortcutLauncher(root); err != nil {
		return "", err
	}
	path := filepath.Join(desktop, "CanvasExpert WebUI.lnk")
	target := filepath.Join(root, "webui.cmd")
	info, err := os.Lstat(path)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if exists && !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s already exists and is not a regular shortcut; it was left unchanged", path)
	}
	err = withShellShortcut(path, exists, func(link *shellLink, file *persistFile) error {
		if exists {
			previous, err := link.text(3, true) // IShellLinkW.GetPath
			if err != nil {
				return fmt.Errorf("read shortcut target: %w", err)
			}
			previousInfo, err := os.Stat(previous)
			if err != nil {
				return fmt.Errorf("inspect existing shortcut target %s: %w; %s was left unchanged", previous, err, path)
			}
			targetInfo, err := os.Stat(target)
			if err != nil {
				return fmt.Errorf("inspect intended shortcut target %s: %w; %s was left unchanged", target, err, path)
			}
			if !os.SameFile(previousInfo, targetInfo) {
				return fmt.Errorf("%s already points elsewhere (%s); it was left unchanged", path, previous)
			}
		}
		for _, property := range []struct {
			method      int
			name, value string
		}{
			{20, "TargetPath", target}, {9, "WorkingDirectory", root}, {11, "Arguments", ""},
			{7, "Description", "Open CanvasExpert WebUI"},
		} {
			if err := link.setText(property.method, property.value); err != nil {
				return fmt.Errorf("set shortcut %s: %w", property.name, err)
			}
		}
		if icon, ok := shortcutIconFile(root); ok {
			if err := link.setIcon(icon); err != nil {
				return fmt.Errorf("set shortcut icon: %w", err)
			}
		}
		if err := file.save(path); err != nil {
			return fmt.Errorf("save desktop shortcut: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// COM objects and their apartment must remain on one OS thread.
func withShellShortcut(path string, load bool, use func(*shellLink, *persistFile) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		// S_FALSE means COM was already initialized on this thread.
		if oleErr, ok := err.(*ole.OleError); !ok || oleErr.Code() != 1 {
			return err
		}
	}
	defer ole.CoUninitialize()
	unknown, err := ole.CreateInstance(ole.NewGUID("{00021401-0000-0000-C000-000000000046}"), ole.IID_IUnknown)
	if err != nil {
		return err
	}
	defer unknown.Release()
	var link *shellLink
	if err := unknown.PutQueryInterface(ole.NewGUID("{000214F9-0000-0000-C000-000000000046}"), &link); err != nil {
		return err
	}
	defer link.Release()
	var file *persistFile
	if err := unknown.PutQueryInterface(ole.NewGUID("{0000010B-0000-0000-C000-000000000046}"), &file); err != nil {
		return err
	}
	defer file.Release()
	if load {
		name, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		if err := file.call(5, uintptr(unsafe.Pointer(name)), 0); err != nil {
			return fmt.Errorf("open existing desktop shortcut: %w", err)
		}
		runtime.KeepAlive(name)
	}
	return use(link, file)
}

// IShellLinkW supplies Unicode paths; WScript.Shell converts some paths through
// the machine's legacy code page and cannot represent every teacher's folder.
type shellLink struct{ ole.IUnknown }
type persistFile struct{ ole.IUnknown }

func comCall(object *ole.IUnknown, method int, args ...uintptr) error {
	vtable := (*[21]uintptr)(unsafe.Pointer(object.RawVTable))
	hr, _, _ := syscall.SyscallN(vtable[method], append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)...)
	if int32(hr) < 0 {
		return ole.NewError(hr)
	}
	return nil
}

func (link *shellLink) setText(method int, text string) error {
	value, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return err
	}
	err = comCall(&link.IUnknown, method, uintptr(unsafe.Pointer(value)))
	runtime.KeepAlive(value)
	return err
}

func (link *shellLink) setIcon(path string) error { // IShellLinkW.SetIconLocation
	value, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	err = comCall(&link.IUnknown, 17, uintptr(unsafe.Pointer(value)), 0)
	runtime.KeepAlive(value)
	return err
}

func (link *shellLink) text(method int, path bool) (string, error) {
	buffer := make([]uint16, 32768)
	args := []uintptr{uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer))}
	if path {
		args = append(args, 0, 4)
	} // SLGP_RAWPATH; never resolve/launch the target.
	if err := comCall(&link.IUnknown, method, args...); err != nil {
		return "", err
	}
	return syscall.UTF16ToString(buffer), nil
}

func (file *persistFile) call(method int, args ...uintptr) error {
	return comCall(&file.IUnknown, method, args...)
}

func (file *persistFile) save(path string) error {
	value, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	err = file.call(6, uintptr(unsafe.Pointer(value)), 1)
	runtime.KeepAlive(value)
	return err
}
