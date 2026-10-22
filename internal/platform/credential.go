// Package platform provides per-machine Windows integration. Credentials never
// enter workspace files or command-line arguments.
package platform

import (
	"encoding/binary"
	"errors"
	"net"
	"net/url"
	"strconv"
)

const credentialTarget = "CanvasExpert-SE/Canvas/v1"
const teammateTargetPrefix = credentialTarget + "/teammate/"

// ErrNoCredential reports that this machine has no saved Canvas credential.
var ErrNoCredential = errors.New("no SE credential is saved on this machine")

// ErrNoTeammateCredential reports that a teammate's key is not saved on this PC.
// It also satisfies errors.Is(err, ErrNoCredential).
var ErrNoTeammateCredential error = noTeammateCredential{}

type noTeammateCredential struct{}

func (noTeammateCredential) Error() string {
	return "this teammate's Canvas key isn't on this PC"
}
func (noTeammateCredential) Is(target error) bool { return target == ErrNoCredential }

// TeammateTarget names a teammate's Credential Manager entry by Canvas user ID.
// An unusable ID returns "", which callers treat as a failure.
func TeammateTarget(canvasUserID int64) string {
	if canvasUserID <= 0 {
		return ""
	}
	return teammateTargetPrefix + strconv.FormatInt(canvasUserID, 10)
}

type Credential struct{ Origin, Token string }

// A length-prefixed blob preserves token bytes, including non-UTF-8 test data.
func encodeCredential(c Credential) ([]byte, error) {
	if len(c.Origin) == 0 || len(c.Token) == 0 || 8+len(c.Origin)+len(c.Token) > 2560 {
		return nil, errors.New("credential is empty or exceeds Windows storage limits")
	}
	blob := make([]byte, 8+len(c.Origin)+len(c.Token))
	copy(blob, "SE01")
	binary.LittleEndian.PutUint32(blob[4:8], uint32(len(c.Origin)))
	copy(blob[8:], c.Origin)
	copy(blob[8+len(c.Origin):], c.Token)
	return blob, nil
}
func decodeCredential(blob []byte) (Credential, error) {
	if len(blob) < 9 || string(blob[:4]) != "SE01" {
		return Credential{}, errors.New("saved SE credential is invalid; run cse setup")
	}
	n := int(binary.LittleEndian.Uint32(blob[4:8]))
	if n <= 0 || n >= len(blob)-8 {
		return Credential{}, errors.New("saved SE credential is invalid; run cse setup")
	}
	return Credential{Origin: string(blob[8 : 8+n]), Token: string(blob[8+n:])}, nil
}
func validBrowserURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Host == "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	if u.Hostname() == "localhost" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && ip.IsLoopback()
}
