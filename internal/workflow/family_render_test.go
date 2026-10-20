package workflow

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func familyRenderAssets(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aRXsAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "cover.png"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "schedule.txt"), []byte("Read chapter one by Friday."), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFamilyRenderInlineImageAttachmentAndNameSafety(t *testing.T) {
	dir := familyRenderAssets(t)
	msg := FamilyMessage{Key: "book", Subject: "Hello {{ student_name }}", Body: "Dear {{student_name}}, your group is **{{group_name}}**.\n\n![Cover](assets/cover.png)\n\n[Schedule](https://school.example/schedule)", Attachments: []string{"assets/schedule.txt", "assets/cover.png"}}
	r, err := RenderFamilyMessage(dir, msg, "A *B* <C>", "Book & Club", "Teacher")
	if err != nil {
		t.Fatal(err)
	}
	if r.Subject != "Hello A *B* <C>" || !strings.Contains(r.HTML, "A *B* &lt;C&gt;") || !strings.Contains(r.HTML, "<strong>Book &amp; Club</strong>") {
		t.Fatalf("unsafe or missing substitution: %#v", r)
	}
	if len(r.Attachments) != 2 || r.Attachments[0].ContentID == "" || r.Attachments[1].ContentID != "" || r.Attachments[0].ContentType != "image/png" {
		t.Fatalf("attachments: %#v", r.Attachments)
	}
	if !strings.Contains(r.HTML, `src="cid:`+r.Attachments[0].ContentID+`"`) || !strings.Contains(r.PreviewHTML, `src="data:image/png;base64,`) || strings.Contains(r.PreviewHTML, "cid:") {
		t.Fatal("preview and outgoing CID mismatch")
	}
	if !strings.Contains(r.HTML, `href="https://school.example/schedule"`) || !strings.Contains(r.HTML, "Teacher") {
		t.Fatal("link/sign-off lost")
	}
}

func TestFamilyRenderNeutralizesExecutableAndRemoteLoads(t *testing.T) {
	dir := familyRenderAssets(t)
	msg := FamilyMessage{Key: "safety", Format: "html", Subject: "{{student_name}} {{unknown}}", Body: `<script>alert(1)</script><iframe src="https://tracker.example"></iframe><style>@import "https://tracker.example";</style><p style="color:red;background-image:url(https://tracker.example);font-size:18px" onclick="bad()">Hi {{student_name}}</p><img src="https://tracker.example/open" alt="Cover"><img src="assets/cover.png" onerror="bad()"><a href="javascript:alert(1)">bad</a><a href="https://school.example">good</a>`}
	r, err := RenderFamilyMessage(dir, msg, "<script>name</script>", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"<script", "<iframe", "<style", "onclick", "onerror", "tracker.example", "javascript:", "background-image"} {
		if strings.Contains(r.HTML, unsafe) {
			t.Fatalf("unsafe %s in %s", unsafe, r.HTML)
		}
	}
	if !strings.Contains(r.HTML, "&lt;script&gt;name&lt;/script&gt;") || !strings.Contains(r.HTML, "color:red;font-size:18px") || !strings.Contains(r.HTML, "https://school.example") {
		t.Fatal("safe content missing")
	}
	warnings, err := ValidateFamilyContent(dir, FamilyContact{Messages: []FamilyMessage{msg}})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(warnings, " ")
	if !strings.Contains(joined, "unknown placeholder") || !strings.Contains(joined, "remote image") {
		t.Fatalf("warnings: %#v", warnings)
	}
}

func TestFamilyRenderMissingAndOutsideAssetsOmitted(t *testing.T) {
	dir := familyRenderAssets(t)
	secret := filepath.Join(dir, "private.txt")
	if err := os.WriteFile(secret, []byte("private-data"), 0600); err != nil {
		t.Fatal(err)
	}
	msg := FamilyMessage{Key: "assets", Body: "![Missing](assets/missing.png) ![Outside](assets/../private.txt)", Attachments: []string{"assets/../../private.txt", secret}}
	r, err := RenderFamilyMessage(dir, msg, "Student", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Attachments) != 0 || strings.Contains(r.HTML, "private-data") || !strings.Contains(r.HTML, "Missing") {
		t.Fatalf("unsafe/missing handling %#v", r)
	}
	warnings, err := ValidateFamilyContent(dir, FamilyContact{Messages: []FamilyMessage{msg}})
	if err != nil || len(warnings) != 4 {
		t.Fatalf("warnings %#v err %v", warnings, err)
	}
	for _, note := range warnings {
		if strings.Contains(note, dir) || strings.Contains(note, "private.txt") {
			t.Fatalf("path leaked: %s", note)
		}
	}
}

func TestFamilyRenderSymlinkEscapeOmitted(t *testing.T) {
	dir := familyRenderAssets(t)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "assets", "escape.txt")); err != nil {
		t.Skip("symlink creation unavailable")
	}
	r, err := RenderFamilyMessage(dir, FamilyMessage{Body: "Hello", Attachments: []string{"assets/escape.txt"}}, "Student", "", "")
	if err != nil || len(r.Attachments) != 0 {
		t.Fatalf("symlink escape: %#v %v", r, err)
	}
}

func TestFamilyRenderDirectoryJunctionEscapeOmitted(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction check")
	}
	for _, wholeAssets := range []bool{false, true} {
		t.Run(fmt.Sprintf("whole_assets_%v", wholeAssets), func(t *testing.T) {
			dir := t.TempDir()
			outside := t.TempDir()
			if err := os.WriteFile(filepath.Join(outside, "private.txt"), []byte("private-data"), 0600); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(dir, "assets")
			ref := "assets/private.txt"
			if !wholeAssets {
				if err := os.Mkdir(link, 0700); err != nil {
					t.Fatal(err)
				}
				link = filepath.Join(link, "escape")
				ref = "assets/escape/private.txt"
			}
			command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; New-Item -ItemType Junction -Path $env:CSE_TEST_LINK -Target $env:CSE_TEST_TARGET | Out-Null`)
			command.Env = append(os.Environ(), "CSE_TEST_LINK="+link, "CSE_TEST_TARGET="+outside)
			if output, err := command.CombinedOutput(); err != nil {
				t.Skipf("junction unavailable: %v; %s", err, output)
			}
			r, err := RenderFamilyMessage(dir, FamilyMessage{Body: "Hello", Attachments: []string{ref}}, "Student", "", "")
			if err != nil || len(r.Attachments) != 0 {
				t.Fatalf("junction escape: %#v %v", r, err)
			}
		})
	}
}

func TestFamilyRenderDigestIncludesOrdinaryAttachmentBytes(t *testing.T) {
	dir := familyRenderAssets(t)
	msg := FamilyMessage{Body: "Hello", Attachments: []string{"assets/schedule.txt"}}
	before, err := RenderFamilyMessage(dir, msg, "Student", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "schedule.txt"), []byte("Changed reading schedule."), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := RenderFamilyMessage(dir, msg, "Student", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if before.Digest == "" || before.Digest == after.Digest {
		t.Fatal("ordinary attachment edit left reviewed content digest unchanged")
	}
}

func TestFamilyRenderSizeCapIncludesBase64Expansion(t *testing.T) {
	dir := familyRenderAssets(t)
	file, err := os.Create(filepath.Join(dir, "assets", "large.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(16 * 1024 * 1024); err != nil {
		t.Fatal(err)
	}
	file.Close()
	_, err = RenderFamilyMessage(dir, FamilyMessage{Body: "Hello", Attachments: []string{"assets/large.bin"}}, "Student", "", "")
	if err == nil || !strings.Contains(err.Error(), "20 MiB") {
		t.Fatalf("size expansion not enforced: %v", err)
	}
}

func TestFamilyRenderExistingClosingNotDuplicated(t *testing.T) {
	dir := familyRenderAssets(t)
	for _, msg := range []FamilyMessage{{Body: "Hello\n\n**Ms. Teacher**"}, {Format: "html", Body: "<p>Hello</p><p>Ms. Teacher</p>"}} {
		r, err := RenderFamilyMessage(dir, msg, "Student", "", "Ms. Teacher")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(r.HTML, "Ms. Teacher") != 1 {
			t.Fatal(r.HTML)
		}
	}
}
