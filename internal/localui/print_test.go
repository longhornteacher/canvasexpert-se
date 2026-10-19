package localui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/workflow"
)

func TestStudentReportDocumentPutsEachStudentOnItsOwnPage(t *testing.T) {
	doc, err := studentReportHTML([]workflow.StudentSheet{{Name: "A", HTML: "<p>one</p>"}, {Name: "B", HTML: "<p>two</p>"}})
	if err != nil {
		t.Fatal(err)
	}
	page := string(doc)
	if strings.Count(page, `<section class="sheet">`) != 2 || !strings.Contains(page, "page-break-after:always") || !strings.Contains(page, "<p>two</p>") {
		t.Fatal(page)
	}
}

func TestStudentReportNeverOverwritesAnEarlierFile(t *testing.T) {
	dir := t.TempDir()
	first := uniquePath(dir, "Student reports CS8", ".pdf")
	if err := os.WriteFile(first, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := uniquePath(dir, "Student reports CS8", ".pdf"); got != filepath.Join(dir, "Student reports CS8 (2).pdf") {
		t.Fatal(got)
	}
}
