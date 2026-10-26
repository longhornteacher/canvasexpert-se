package localui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
)

var printTemplate = template.Must(template.New("print").Parse(`<!doctype html>
<html lang="en"><meta charset="utf-8"><title>Student reports</title>
<style>
@page{size:letter;margin:0.5in}
body{margin:0;color:#1f2933;font-family:Segoe UI,Arial,sans-serif}
.sheet{break-after:page;page-break-after:always}
.sheet:last-child{break-after:auto;page-break-after:auto}
</style>
{{range .}}<section class="sheet">{{.}}</section>
{{end}}</html>`))

// studentReportHTML is one document with each student's sheet on its own page.
func studentReportHTML(sheets []workflow.StudentSheet) ([]byte, error) {
	var parts []template.HTML
	for _, s := range sheets {
		// Sheet HTML is built by the Engine with every value escaped.
		parts = append(parts, template.HTML(s.HTML))
	}
	var b bytes.Buffer
	err := printTemplate.Execute(&b, parts)
	return b.Bytes(), err
}

// edgePath finds Microsoft Edge, which ships with Windows 11.
func edgePath() (string, error) {
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LocalAppData"} {
		base := os.Getenv(env)
		if base == "" {
			continue
		}
		p := filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("Microsoft Edge was not found")
}

// SaveStudentReports writes every sheet into one PDF in the teacher's Downloads
// folder (one student per page) and opens it. When Edge cannot make the PDF it
// saves a printable HTML file instead. The path holds student names, so it is
// outside the workspace the agent can read.
func SaveStudentReports(ctx context.Context, sheets []workflow.StudentSheet, label string) (string, error) {
	if len(sheets) == 0 {
		return "", errors.New("this run has no students to print")
	}
	doc, err := studentReportHTML(sheets)
	if err != nil {
		return "", err
	}
	return savePrintable(ctx, doc, "Student reports "+label)
}

// savePrintable saves doc as a PDF in the teacher's Downloads folder and opens
// it, or saves a printable HTML page when Edge cannot make the PDF.
func savePrintable(ctx context.Context, doc []byte, name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Downloads")
	if info, e := os.Stat(dir); e != nil || !info.IsDir() {
		dir = home
	}
	base := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '-'
		}
		return r
	}, name)
	pdf := uniquePath(dir, base, ".pdf")
	pdfErr := printPDF(ctx, doc, pdf)
	if pdfErr == nil {
		_ = platform.OpenDefaultFile(pdf)
		return pdf, nil
	}
	page := uniquePath(dir, base, ".html")
	if err := os.WriteFile(page, doc, 0600); err != nil {
		return "", fmt.Errorf("could not make the PDF (%v) or save a printable page: %w", pdfErr, err)
	}
	_ = platform.OpenDefaultFile(page)
	return page, fmt.Errorf("could not make the PDF (%v); saved a printable page instead: open it and print with Ctrl+P", pdfErr)
}

func uniquePath(dir, base, ext string) string {
	p := filepath.Join(dir, base+ext)
	for i := 2; ; i++ {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
		p = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
	}
}

// printPDF renders the document with headless Edge in a throwaway profile, so
// the teacher's own Edge windows and settings are untouched.
func printPDF(ctx context.Context, doc []byte, out string) error {
	edge, err := edgePath()
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "cse-print-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	page := filepath.Join(tmp, "print.html")
	if err = os.WriteFile(page, doc, 0600); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, edge, "--headless=new", "--disable-gpu", "--no-first-run", "--no-pdf-header-footer",
		"--user-data-dir="+filepath.Join(tmp, "profile"), "--print-to-pdf="+out, "file:///"+filepath.ToSlash(page))
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Edge could not print: %v %s", err, strings.TrimSpace(string(b)))
	}
	if info, err := os.Stat(out); err != nil || info.Size() == 0 {
		return errors.New("Edge did not write the PDF")
	}
	return nil
}
