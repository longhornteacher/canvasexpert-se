//go:build !windows

package outlook

import "fmt"

var errNotWindows = fmt.Errorf("%w: requires Windows with classic Outlook", ErrUnavailable)

func Status() (bool, string)        { return false, "Outlook delivery requires Windows with classic Outlook" }
func Send(Message) (Outcome, error) { return Failed, errNotWindows }
func SaveDraft(Message) error       { return errNotWindows }
func SelfAddress() (string, error)  { return "", errNotWindows }
