package localui

import (
	"html/template"
	"net/http"
)

type capabilityStatus string

const (
	capabilityGreen  capabilityStatus = "green"
	capabilityYellow capabilityStatus = "yellow"
	capabilityRed    capabilityStatus = "red"
)

func (s capabilityStatus) Label() string {
	switch s {
	case capabilityGreen:
		return "Green"
	case capabilityYellow:
		return "Yellow"
	default:
		return "Red"
	}
}

type capability struct {
	Title, Detail string
}

type capabilityGroup struct {
	Title, Summary string
	Status         capabilityStatus
	Items          []capability
}

// Keep this teacher-facing snapshot aligned with docs/engine-capabilities.md.
// Recorded live evidence and local checks are different kinds of verification.
const capabilitiesReviewed = "2026-10-09"

var capabilityGroups = []capabilityGroup{
	{
		Title: "Working with recorded evidence", Status: capabilityGreen,
		Summary: "The core assignment and grading tools have recorded checks. Each feature below names the scope of that evidence.",
		Items: []capability{
			{"Discover teacher courses and read modules", "Verified in Canvas Live."},
			{"Create assignments with directions, points, dates, and submission settings", "Creation and edits verified in Canvas Live."},
			{"Publish assignments when requested", "Verified in Canvas Live. New assignments wait for requested rubric and module pieces to succeed before publication."},
			{"Edit existing assignments while preserving unrelated content", "Verified in Canvas Live, including preservation of unsent edits when conflicts occur."},
			{"Create and attach assignment rubrics", "Verified in Canvas Live. Criterion maxima are preserved when ratings are omitted."},
			{"Place newly created assignments in existing modules", "Verified in Canvas Live, including repeat handling without duplicates."},
			{"Pull text submissions and prior attempts", "Verified in Canvas Live with the built-in Test Student."},
			{"Download file submissions and expose readable text", "A synthetic text-file submission was verified in Canvas Live."},
			{"Calculate component scores, unequal weights, and rubric-to-assignment scaling", "Exact arithmetic verified, including different rubric and assignment scales."},
			{"Grade assignments without a Canvas rubric", "An explicit local scoring basis works. Posts the grade and feedback without creating a rubric."},
			{"Post honest grades, rubric assessments, and feedback", "Verified in Canvas Live, including replacing an existing higher grade with the computed assessment."},
			{"Post correctly when Canvas automatically grades from the rubric", "Verified in Canvas Live: the honest rubric assessment is posted first, then the computed grade."},
			{"Revise a selected feedback comment without changing the grade", "Exact comment revision verified in Canvas Live, without a duplicate comment."},
			{"Apply supplied lateness timing and let Canvas calculate deductions", "Verified in Canvas Live. A changed due date holds the timing update while grading continues."},
			{"Create New Quizzes from QuizForge files", "Verified in Canvas Live (CS8, 2026-10-09): the original QuizForge all-types sampler (multiple choice, multiple answer, true/false, matching, fill-in-the-blank, numeric, ordering, categorization, with a stimulus passage and rationales) created one unpublished New Quiz through the New Quizzes API; reading it back showed every item type, authored points, feedback, and the student-visible result settings. A separate quiz verified time limit, attempts, due date, publication, and module placement. A resumed push completed without duplicating. Code blocks render in a dark Monokai box with Python colors and TEKS codes appear as a small label, both confirmed in the stored quiz. Essays, file uploads, and images are not handled."},
			{"Create Classic Quizzes from QuizForge files", "Verified in Canvas Live (CS8, 2026-10-09): a sampler with 11 questions (multiple choice, multiple answer, true/false, short answer, multi-blank, dropdown, numeric exact and range, matching, essay, file upload), a stimulus, rationales, code highlighting, and TEKS labels read back with the right types, answer weights, comments, and a 28-point total on both the quiz and its assignment. Resume without duplicates, publication, due date, access code, and module placement were also verified. Items Classic cannot hold (ordering, categorization, word-bank blanks) are refused before any write, checked locally only. Scoring a Classic quiz's writing is not part of the Engine."},
			{"Differentiated hub: restricted tier pages linked from an assignment or Classic quiz", "Verified in Canvas Live (CS8, 2026-10-09), reading state back from Canvas: each tier page is created unpublished, restricted to overrides, assigned to the matching non-collaborative tag, and only then published; a missing tag leaves the page restricted to nobody with a teacher action; a title collision stops the push before any write; an interrupted push resumes without duplicate pages; links in the hub description point at the exact page slugs. Hub is the only style offered (the Bridge was declined). Tiers can be added only when creating. Not yet checked: what a student in each tier sees."},
			{"Recover interrupted work, avoid duplicate writes, and continue independent students", "Supported by live repeat and recovery checks plus focused local tests. Independent-student continuation was checked locally."},
		},
	},
	{
		Title: "Partial or awaiting meaningful validation", Status: capabilityYellow,
		Summary: "Some pieces work, but the limits and remaining checks matter before relying on the whole workflow.",
		Items: []capability{
			{"Read PDF, Word, image, and other non-text attachment contents", "Original files are preserved. Built-in readable extraction currently handles text formats; other formats need additional reading tools."},
			{"Determine lateness from genuine attempts, school calendar, and accommodations", "The scoring playbook applies documented extra time when judging lateness, without adding an extension twice. Reliable teacher-session use still needs validation."},
			{"Discover unfinished work, skip completed attempts, and find resubmissions", "Local tests pass. An ordinary enrolled student's completion and resubmission workflow remains unverified in Canvas Live."},
			{"Student experience of quizzes and hub pages", "Not yet checked. Canvas cannot open a New Quiz as the built-in Test Student, so New Quizzes need the editor's Preview or a real enrolled account; Classic quizzes and hub pages are not checked from a student's account either. Read-back shows stored items, settings, and page restrictions, not what a student sees or how Canvas auto-scores an attempt."},
			{"Produce useful educational judgments and feedback in the teacher's voice", "Playbooks exist. Teaching quality and student-facing usability still need a human validation check; synthetic tests do not establish them."},
		},
	},
	{
		Title: "Future capabilities", Status: capabilityRed,
		Summary: "These are things we want CE-SE to do. The requested Canvas actions are not implemented yet.",
		Items: []capability{
			{"Add hub tiers to an existing assignment or quiz", "Hub pages are created together with a new assignment or Classic quiz. Adding them to existing content, and the Bridge style (declined by design), are not available."},
			{"Apply individual extensions or accommodations to Canvas deadlines", "The scoring playbook accounts for documented extra time when judging lateness. Writing individual Canvas deadlines is not implemented; local extra-time settings do not change those dates."},
			{"Score or edit a Classic quiz's writing", "Authoring is implemented. Scoring each essay with a score and comment per question is future work; the original CanvasExpert verified the Canvas path."},
			{"Import a QuizForge QTI ZIP", "Tried 2026-10-09 through Canvas content migration: it produced an empty New Quiz with no error, so it is not offered. QuizForge content would need conversion to the quiz file format."},
			{"Create general standalone Canvas Pages", "Tier support pages are created as part of hub delivery. General pages for other purposes are future work."},
			{"Manage standalone/shared rubrics or edit existing rubric definitions", "Current support creates and attaches new assignment rubrics."},
			{"Create or reorganize modules, or move existing assignments between them", "Current support reads modules and places new assignments into existing modules."},
			{"Apply grading curves", "Future capability."},
		},
	},
}

func (h *Hub) capabilitiesPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.render(w, http.StatusOK, capabilitiesTemplate, struct {
		Reviewed string
		Groups   []capabilityGroup
	}{capabilitiesReviewed, capabilityGroups})
}

var capabilitiesTemplate = template.Must(template.New("capabilities").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CanvasExpert-SE capabilities</title><style>` + funkyCSS + `
.cap-page{transform:none}
.cap-legend{display:flex;gap:.7rem 1rem;flex-wrap:wrap;margin:1rem 0}.cap-status{display:inline-flex;align-items:center;gap:.4rem;white-space:nowrap;color:var(--ink);background:#fff}.cap-status .light{width:.9rem;height:.9rem;border:2px solid var(--ink);border-radius:50%;background:var(--light);flex:none}.cap-status.green{--light:#25b85b}.cap-status.yellow{--light:var(--sun)}.cap-status.red{--light:#ef354b}
.cap-group{margin:2rem 0}.cap-group h2{font-size:1.25rem;text-transform:uppercase;letter-spacing:.06em;margin:0 0 .6rem}.cap-group h2 .pill{vertical-align:middle}.cap-list{list-style:none;padding:0;margin:1.1rem 0;display:grid;gap:.9rem}.cap-item{display:grid;grid-template-columns:minmax(15rem,1fr) minmax(0,1.6fr);gap:.6rem 1.5rem;align-items:start}.cap-item:nth-child(n){transform:none}.cap-item h3{font-size:1rem;margin:0 0 .5rem;line-height:1.35}.cap-item p{margin:0;font-size:.95rem;font-weight:400}.cap-next{margin-top:2rem}
@media(max-width:44rem){.cap-item{grid-template-columns:1fr}.cap-group h2{line-height:1.6}}
</style><main class="wide cap-page"><div class="badge">CanvasExpert: SE</div>` + hubNav + `<h1>What can CE-SE do?</h1>
<p class="lead">Your Canvas Live capability map: what's working, what still needs a meaningful check, and what's on the wish list.</p>
<p class="reassure"><b>Reviewed <time datetime="{{.Reviewed}}">{{.Reviewed}}</time>.</b> These lights reflect recorded evidence, not real-time Canvas health. Live checks used disposable assignments and synthetic built-in Test Student work. Local tests and remaining human checks are called out below.</p>
<div class="cap-legend" aria-label="Status meanings"><span><span class="pill cap-status green"><span class="light" aria-hidden="true"></span>Green</span> Working with recorded evidence</span><span><span class="pill cap-status yellow"><span class="light" aria-hidden="true"></span>Yellow</span> Partial or awaiting validation</span><span><span class="pill cap-status red"><span class="light" aria-hidden="true"></span>Red</span> Not implemented</span></div>
{{range .Groups}}<section class="cap-group"><h2>{{.Title}} <span class="pill">{{len .Items}} features</span></h2><p>{{.Summary}}</p><ul class="cap-list">{{$status := .Status}}{{range .Items}}<li class="card cap-item"><div><h3>{{.Title}}</h3><span class="pill cap-status {{$status}}"><span class="light" aria-hidden="true"></span>{{$status.Label}}</span></div><p>{{.Detail}}</p></li>{{end}}</ul></section>{{end}}
<aside class="card cap-next"><h2>The next useful milestone</h2><p>An ordinary student submits work, receives a score and feedback, and the completed attempt disappears from discovery. After a resubmission, the new attempt should appear with its earlier history. A human check of the student's experience and feedback closes the largest remaining gap in the core workflow.</p></aside>
</main></html>`))
