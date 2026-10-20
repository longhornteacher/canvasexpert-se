//go:build windows

package outlook

import (
	"strings"
	"testing"

	ole "github.com/go-ole/go-ole"
)

func TestAttachmentFailureReportsStageAndCodeWithoutPrivateCOMText(t *testing.T) {
	err := ole.NewErrorWithDescription(0x80020009, `C:\private\student-cover.png for parent@example.invalid`)
	message := attachmentFailure("set attachment property 0x3712001F", err).Error()
	if !strings.Contains(message, "0x3712001F") || !strings.Contains(message, "0x80020009") {
		t.Fatalf("diagnostic stage/code missing: %s", message)
	}
	if strings.Contains(message, "private") || strings.Contains(message, "parent@") {
		t.Fatalf("private COM text exposed: %s", message)
	}
}
