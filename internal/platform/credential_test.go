package platform

import (
	"bytes"
	"testing"
)

func TestCredentialEncodingPreservesUnicodeAndRawBytes(t *testing.T) {
	want := Credential{Origin: "https://ñ-学.example", Token: string([]byte{0, 255, 128, 65})}
	blob, err := encodeCredential(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeCredential(blob)
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin != want.Origin || !bytes.Equal([]byte(got.Token), []byte(want.Token)) {
		t.Fatal("credential bytes changed")
	}
}
func TestCredentialRejectsMalformedBlob(t *testing.T) {
	for _, blob := range [][]byte{nil, []byte("SE01"), []byte("SE01\xff\xff\xff\xffx")} {
		if _, err := decodeCredential(blob); err == nil {
			t.Fatal("accepted malformed blob")
		}
	}
}
func TestBrowserURLRestrictions(t *testing.T) {
	for _, raw := range []string{"https://example.com", "http://127.0.0.1:45123/setup", "http://[::1]:123/"} {
		if !validBrowserURL(raw) {
			t.Fatal("rejected safe browser URL")
		}
	}
	for _, raw := range []string{"file:///x", "http://example.com", "https://user:password@example.com", "javascript:alert(1)"} {
		if validBrowserURL(raw) {
			t.Fatal("accepted unsafe browser URL")
		}
	}
}
