//go:build !windows

package platform

import "errors"

func LoadCredential() (Credential, error) {
	return Credential{}, errors.New("SE credentials require Windows; run cse setup on the Windows machine")
}
func SaveCredential(Credential) error { return errors.New("SE credential storage requires Windows") }
func DeleteCredential() error         { return errors.New("SE credential storage requires Windows") }
func CredentialSelfTest() error       { return errors.New("SE credential self-test requires Windows") }
func OpenBrowser(string) error {
	return errors.New("SE default-browser launch requires Windows; open the displayed local URL")
}
func OpenDefaultFile(string) error {
	return errors.New("default file handler requires Windows; open the prepared extension manually")
}
