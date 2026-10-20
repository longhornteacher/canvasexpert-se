package outlook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInlineAttachmentMAPIProperties(t *testing.T) {
	props := attachmentProperties(Attachment{ContentID: "cover@family", ContentType: "image/png"})
	want := map[string]any{"0x3712001F": "cover@family", "0x370E001F": "image/png", "0x7FFE000B": true}
	for _, p := range props {
		key := p.Schema[len("http://schemas.microsoft.com/mapi/proptag/"):]
		if want[key] != p.Value {
			t.Fatalf("unexpected property %#v", p)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing CID properties: %#v", want)
	}
	if len(attachmentProperties(Attachment{})) != 0 {
		t.Fatal("ordinary attachment received inline properties")
	}
}

func TestValidateAttachmentAvailabilityAndCID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cover.png")
	if err := os.WriteFile(path, []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	m := Message{To: []string{"synthetic@example.invalid"}, Subject: "Test", TextBody: "Body", Attachments: []Attachment{{Path: path, ContentID: "cover@family"}}}
	if err := validate(m); err != nil {
		t.Fatal(err)
	}
	m.Attachments = append(m.Attachments, m.Attachments[0])
	if validate(m) == nil {
		t.Fatal("duplicate CID accepted")
	}
	m.Attachments = []Attachment{{Path: filepath.Join(t.TempDir(), "absent")}}
	if validate(m) == nil {
		t.Fatal("missing attachment accepted")
	}
}
