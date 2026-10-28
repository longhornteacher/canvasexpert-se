package localui

import (
	"bytes"
	"context"
	"html/template"
	"time"

	"canvasexpertse/internal/workflow"
)

// The print page inlines draft HTML so it paginates, so the policy blocks
// scripts and every network load for standalone draft rendering.
var draftTemplate = template.Must(template.New("draft").Parse(`<!doctype html>
<html lang="en"><meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:">
<title>{{.Title}}</title>
<style>
@page{size:letter;margin:0.6in}
body{margin:0;color:#111;font:11pt/1.45 "Segoe UI",Arial,sans-serif}
h1{font-size:18pt;margin:0 0 6pt}
h2{font-size:13pt;margin:12pt 0 4pt}
.banner,.notice,.sub{font-size:9pt;color:#555}
.banner{border-bottom:1px solid #999;padding-bottom:4pt;margin-bottom:10pt}
.card{margin-bottom:14pt}
.page{break-before:page;page-break-before:always}
.label{font-size:8.5pt;font-weight:600;color:#444;margin-top:8pt;text-transform:uppercase;letter-spacing:.04em}
table{border-collapse:collapse}
td,th{border:1px solid #aaa;padding:2pt 6pt;vertical-align:top;text-align:left}
.facts{font-size:9.5pt;margin:4pt 0}
.hold{border:1.5pt solid #b00;padding:4pt 6pt;color:#900}
.done{color:#555;font-style:italic}
pre{white-space:pre-wrap;font:inherit;margin:2pt 0}
.private{border-left:3pt solid #888;padding-left:6pt}
img{max-width:100%}
</style>
<body>
{{if not .Handout}}<div class="banner">Draft · {{.Kind}} · not yet in Canvas · {{.Date}}</div>{{end}}
<h1>{{.Title}}</h1>
{{if and .Notice (not .Handout)}}<p class="notice">{{.Notice}}</p>{{end}}
{{range .Frames}}<div class="label">{{.Label}}</div><div>{{.HTML}}</div>{{end}}
{{range .Cards}}<section class="card{{if .NewPage}} page{{end}}">
{{if .Sub}}<div class="sub">{{.Sub}}</div>{{end}}
{{if .Heading}}<h2>{{.Alias}}{{if .Total}} ({{.Total}}){{end}}</h2>{{else if .Total}}<p><b>{{.Total}}</b></p>{{end}}
{{if not $.Handout}}{{if .Hold}}<p class="hold">Will not post: {{.Hold}}</p>{{end}}{{if .Done}}<p class="done">Already posted</p>{{end}}{{if .Late}}<p>Late: {{.Late}}</p>{{end}}
{{if .Facts}}<table class="facts">{{range .Facts}}<tr><td>{{.Label}}</td><td>{{.Value}}</td></tr>{{end}}</table>{{end}}{{end}}
{{range .Shown}}{{if not $.Handout}}<div class="label">{{.Label}}</div>{{end}}<div>{{.HTML}}</div>{{end}}
{{if .Comment}}{{if not $.Handout}}<div class="label">Feedback to be posted</div>{{end}}<pre>{{.Comment}}</pre>{{end}}
{{if .RubricComments}}<div class="label">Rubric comments</div>{{range .RubricComments}}<p><b>{{.Label}}:</b> {{.Value}}</p>{{end}}{{end}}
{{if not $.Handout}}{{if .Key}}<div class="label">Answer key (teacher only)</div><pre class="private">{{.Key}}</pre>{{end}}
{{if .TeacherNote}}<div class="label">Teacher note</div><pre class="private">{{.TeacherNote}}</pre>{{end}}
{{if .LateBasis}}<div class="label">Late timing basis</div><pre class="private">{{.LateBasis}}</pre>{{end}}
{{if .Submission}}<div class="label">Student submission</div><pre>{{.Submission}}</pre>{{end}}{{end}}
</section>
{{end}}</body></html>`))

type draftFrame struct {
	Label string
	HTML  template.HTML
}

type draftCard struct {
	workflow.ReviewCard
	Sub              string
	Heading, NewPage bool
	Shown            []draftFrame
}

type draftView struct {
	Title, Kind, Notice, Date string
	Handout                   bool
	Frames                    []draftFrame
	Cards                     []draftCard
}

func printFrames(frames []workflow.ReviewFrame) []draftFrame {
	var out []draftFrame
	for _, f := range frames {
		h := f.HTML
		if f.PrintHTML != "" {
			h = f.PrintHTML
		}
		// Frame HTML comes from the shared draft renderer: the Engine's
		// own rendering of the draft. The page policy above keeps it inert.
		out = append(out, draftFrame{Label: f.Label, HTML: template.HTML(h)})
	}
	return out
}

// draftHTML lays out a review model for paper. The teacher copy shows
// the full draft rendering; the handout keeps only what students see:
// directions, questions and posted feedback, never keys, settings or private notes.
func draftHTML(m workflow.ReviewModel, handout bool, now time.Time) ([]byte, error) {
	v := draftView{Title: m.Title, Kind: m.Kind, Notice: m.Notice, Date: now.Format("Jan 2, 2006"), Handout: handout}
	if !handout {
		v.Frames = printFrames(m.Frames)
	}
	// Scores and feedback are per student, so each student starts a page.
	perStudent := !m.AllOrNothing
	for _, c := range m.Cards {
		if handout && (c.Hold != "" || c.Done) {
			continue
		}
		d := draftCard{ReviewCard: c, Heading: c.Alias != m.Title, Shown: printFrames(c.Frames)}
		if handout && len(d.Shown) > 1 {
			d.Shown = d.Shown[:1] // the student-facing content; later frames are rationales
		}
		if perStudent || c.Operation == "hub-tier-page" {
			d.NewPage = len(v.Cards) > 0
			if perStudent && d.NewPage {
				d.Sub = m.Title
			}
		}
		v.Cards = append(v.Cards, d)
	}
	var b bytes.Buffer
	err := draftTemplate.Execute(&b, v)
	return b.Bytes(), err
}

// SaveDraftPDF prints a draft (assignment, Page, quiz, or scores) to a PDF in
// the teacher's Downloads folder and opens it. Scores can carry student names
// and work, so the file stays outside the workspace the agent reads.
func SaveDraftPDF(ctx context.Context, m workflow.ReviewModel, handout bool) (string, error) {
	doc, err := draftHTML(m, handout, time.Now())
	if err != nil {
		return "", err
	}
	name := "Draft - " + m.Title
	if handout {
		name = "Handout - " + m.Title
	}
	return savePrintable(ctx, doc, name)
}
