//go:build windows

package platform

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	advapi       = syscall.NewLazyDLL("advapi32.dll")
	credRead     = advapi.NewProc("CredReadW")
	credWrite    = advapi.NewProc("CredWriteW")
	credDelete   = advapi.NewProc("CredDeleteW")
	credFree     = advapi.NewProc("CredFree")
	shellExecute = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")
)

// Go's native pointer alignment matches CREDENTIALW on both 32 and 64 bit Windows.
type nativeCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

func windowsError(action string, err error) error {
	if code, ok := err.(syscall.Errno); ok {
		return fmt.Errorf("Windows Credential Manager %s failed (code %d)", action, uint32(code))
	}
	return fmt.Errorf("Windows Credential Manager %s failed", action)
}
func saveAt(target string, c Credential) error {
	blob, err := encodeCredential(c)
	if err != nil {
		return err
	}
	defer func() {
		for i := range blob {
			blob[i] = 0
		}
	}()
	name, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return errors.New("invalid SE credential target")
	}
	user, _ := syscall.UTF16PtrFromString("CanvasExpert-SE")
	native := nativeCredential{Type: 1, TargetName: name, CredentialBlobSize: uint32(len(blob)), CredentialBlob: &blob[0], Persist: 2, UserName: user}
	ok, _, callErr := credWrite.Call(uintptr(unsafe.Pointer(&native)), 0)
	runtime.KeepAlive(blob)
	runtime.KeepAlive(name)
	runtime.KeepAlive(user)
	if ok == 0 {
		return windowsError("write", callErr)
	}
	return nil
}
func loadAt(target string) (Credential, error) {
	name, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return Credential{}, errors.New("invalid SE credential target")
	}
	var native *nativeCredential
	ok, _, callErr := credRead.Call(uintptr(unsafe.Pointer(name)), 1, 0, uintptr(unsafe.Pointer(&native)))
	runtime.KeepAlive(name)
	if ok == 0 {
		if callErr == syscall.Errno(1168) {
			return Credential{}, fmt.Errorf("%w; run cse setup", ErrNoCredential)
		}
		return Credential{}, windowsError("read", callErr)
	}
	defer credFree.Call(uintptr(unsafe.Pointer(native)))
	if native == nil || native.CredentialBlob == nil || native.CredentialBlobSize > 2560 {
		return Credential{}, errors.New("saved SE credential is invalid; run cse setup")
	}
	blob := unsafe.Slice(native.CredentialBlob, int(native.CredentialBlobSize))
	return decodeCredential(blob)
}
func deleteAt(target string) error {
	name, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return errors.New("invalid SE credential target")
	}
	ok, _, callErr := credDelete.Call(uintptr(unsafe.Pointer(name)), 1, 0)
	runtime.KeepAlive(name)
	if ok == 0 && callErr != syscall.Errno(1168) {
		return windowsError("delete", callErr)
	}
	return nil
}
func LoadCredential() (Credential, error) { return loadAt(credentialTarget) }
func SaveCredential(c Credential) error   { return saveAt(credentialTarget, c) }

// DeleteCredential forgets this machine's saved Canvas token; absent is fine.
func DeleteCredential() error { return deleteAt(credentialTarget) }

// CredentialSelfTest only touches a randomly named dummy SE credential.
// It never reads, overwrites, or deletes the configured teacher credential.
func CredentialSelfTest() (result error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return errors.New("cannot create credential self-test identifier")
	}
	target := "CanvasExpert-SE/SelfTest/" + hex.EncodeToString(nonce[:])
	want := Credential{Origin: "https://unicode-ñ-学.example", Token: "dummy-🔐-\x00\xff\x80"}
	if err := saveAt(target, want); err != nil {
		return err
	}
	defer func() {
		if err := deleteAt(target); err != nil {
			result = err
		}
	}()
	got, err := loadAt(target)
	if err != nil {
		return err
	}
	if got.Origin != want.Origin || !bytes.Equal([]byte(got.Token), []byte(want.Token)) {
		return errors.New("Windows Credential Manager byte round trip failed")
	}
	return nil
}

func OpenBrowser(rawURL string) error {
	if !validBrowserURL(rawURL) {
		return errors.New("browser URL must be HTTPS or loopback HTTP")
	}
	verb, _ := syscall.UTF16PtrFromString("open")
	urlPtr, err := syscall.UTF16PtrFromString(rawURL)
	if err != nil {
		return errors.New("invalid browser URL")
	}
	result, _, _ := shellExecute.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(urlPtr)), 0, 0, 1)
	runtime.KeepAlive(verb)
	runtime.KeepAlive(urlPtr)
	if result <= 32 {
		return errors.New("could not open the default browser; open the displayed local URL")
	}
	return nil
}

// OpenDefaultFile asks Windows to open a prepared file with its registered handler.
func OpenDefaultFile(path string) error {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return errors.New("invalid file path")
	}
	result, _, _ := shellExecute.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
	runtime.KeepAlive(verb)
	runtime.KeepAlive(file)
	if result <= 32 {
		return errors.New("could not open prepared file with its default handler")
	}
	return nil
}
