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
const capabilitiesReviewed = "2026-10-10"

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
			{"Create New Quizzes from QuizForge files", "Verified in Canvas Live (CS8, 2026-10-09): the original QuizForge all-types sampler (multiple choice, multiple answer, true/false, matching, fill-in-the-blank, numeric, ordering, categorization, with a stimulus passage and rationales) created one unpublished New Quiz through the New Quizzes API; reading it back showed every item type, authored points, feedback, and the student-visible result settings. A separate quiz verified time limit, attempts, due date, publication, and module placement. A resumed push completed without duplicating. Code blocks render in a dark Monokai box with Python colors and TEKS codes appear as a small label, both confirmed in the stored quiz. Images are not handled. Written items: see the New Quiz written items entry below."},
									{"Differentiated hub: restricted tier pages linked from an assignment or quiz", "Verified in Canvas Live (CS8, 2026-10-09) for assignment hub pages, reading state back from Canvas: each tier page is created unpublished, restricted to overrides, assigned to the matching non-collaborative tag, and only then published; a title collision stops the push before any write; an interrupted push resumes without duplicate pages; links in the hub description point at the exact page slugs. Hub is the only style offered (the Bridge was declined). Tiers can be added only when creating. Hub also works on New Quizzes (2026-10-10): pages are delivered unpublished, links are bound into the New Quiz instructions, and pages publish only with the quiz. Local tests cover the New Quiz path and a missing tag (page restricted to nobody, with a teacher action); no Canvas Live run of the New Quiz path or the missing-tag case is recorded. Not yet checked: what a student in each tier sees."},
			{"Recover interrupted work, avoid duplicate writes, and continue independent students", "Supported by live repeat and recovery checks plus focused local tests. Independent-student continuation was checked locally."},
		},
	},
	{
		Title: "Partial or awaiting meaningful validation", Status: capabilityYellow,
		Summary: "Some pieces work, but the limits and remaining checks matter before relying on the whole workflow.",
		Items: []capability{
			{"New Quiz written items (essay, file upload) in push-quiz", "Added 2026-10-10: push-quiz creates ESSAY and FILEUPLOAD as New Quiz items, with optional word limits and accepted file types; teacher notes are never sent. Verified in Canvas Live API storage (CS8, 2026-10-10): items, points, word limits and accepted types read back as sent, and a rerun adds nothing; whether students see the limits enforced is pending a human Build, then Preview. CE-SE cannot score these items yet, so the teacher grades them in SpeedGrader."},
			{"Attach a rubric to a New Quiz (ECR/SCR)", "Added 2026-10-10: pull-item now accepts a New Quiz assignment so attach-rubric can add a new independent rubric to it; any other edit of a New Quiz is refused. Verified in Canvas Live through cse (CS8, 2026-10-10): pull-item and push attached a two-criterion rubric to the teacher's published New Quiz 3698192, and the Engine's read-back confirmed the exact rubric, independent grading, and unchanged points and settings. Before that, a direct REST attach also kept a New Quiz at its points. Writing a rubric assessment on a New Quiz submission is unchecked and waits for real student work."},
			{"Review scores on a local page before posting (cse review)", "Built 2026-10-10 for assignment scores; verified by local tests and one Canvas Live run (CS8 disposable assignment, built-in Test Student: Revise, re-review, Post, grade and comment read back, page loaded in the Claude browser pane). Each student shows the exact comment text and computed score, held students are called out, and Post writes only approved students. Revision notes are saved beside the results file. The scoring Playbook makes it the default step before posting; the exit-on-click command's notification reached the agent in a live session. Authoring preview (new assignment, hub tier pages, QuizForge quiz) was added later and is verified by local tests plus a look in the Claude pane only; it does not simulate Canvas's student view. Assignment edit review (live now/after diff, conflict flagged, Apply edit) was verified in Canvas Live on the disposable assignment for a directions edit; the conflict flag is local-test only. Score cards also show the agent's lateness reasoning as teacher-only text (added 2026-10-10, local tests only). Still pending: a normal Chromium browser, the ChatGPT pane, and a multi-student class."},
			{"Read PDF, Word, image, and other non-text attachment contents", "Original files are preserved. Built-in readable extraction currently handles text formats; other formats need additional reading tools."},
			{"Determine lateness from genuine attempts, school calendar, and accommodations", "The scoring playbook applies documented extra time when judging lateness, without adding an extension twice. Reliable teacher-session use still needs validation."},
			{"Discover unfinished work, skip completed attempts, and find resubmissions", "Local tests pass. An ordinary enrolled student's completion and resubmission workflow remains unverified in Canvas Live."},
			{"What Canvas keeps and shows in each field", "One reference, served to agents by cse help fields, records each field with its evidence level. Canvas Live API read-back (CS8, 2026-10-10, unpublished probe New Quiz 3698178 and assignment 3698180): the New Quiz kept realistic quiz HTML (headings, emphasis, lists, a table with colspan, inline styles, the code box, LaTeX text, the equation image, links) and removed only script and onclick; New Quizzes returned course images as expiring signed links; choices and feedback were stored exactly; matching text was stored raw, while ordering text had & and < escaped; assignment directions kept every heading level, bold, italic, code and both link kinds. Teacher views the same day: in New Quiz Preview the image, inline style, LaTeX (rendered as math), equation image, code box and choice formatting all displayed; matching text showed tags literally while ordering text rendered them, with & and < shown normally in both and line breaks collapsed to spaces; the assignment page showed every heading, bold, italic and the code block. Earlier views: in New Quiz teacher Preview the code box was dark and colored, matching/ordering text plain, and feedback appeared after submitting; a submission comment kept its line breaks in Student View. SpeedGrader display of submission-comment paragraphs is still unchecked. A 2026-10-10 page check went through the Canvas editor, so it is not Engine-path evidence."},
			{"Student experience of quizzes and hub pages", "Partially checked. Canvas cannot open a New Quiz as the built-in Test Student, so New Quizzes need the editor's Preview or a real enrolled account. Ordinary enrolled-student behavior and hub tier pages remain unverified from a student account. Read-back of authoring and restrictions does not establish those broader experiences."},
			{"Produce useful educational judgments and feedback in the teacher's voice", "Playbooks exist. Teaching quality and student-facing usability still need a human validation check; synthetic tests do not establish them."},
		},
	},
	{
		Title: "Future capabilities", Status: capabilityRed,
		Summary: "These are things we want CE-SE to do. The requested Canvas actions are not implemented yet.",
		Items: []capability{
			{"Add hub tiers to an existing assignment or quiz", "Hub pages are created together with a new assignment or New Quiz. Adding them to existing content, and the Bridge style (declined by design), are not available."},
			{"Apply individual extensions or accommodations to Canvas deadlines", "The scoring playbook accounts for documented extra time when judging lateness. Writing individual Canvas deadlines is not implemented; local extra-time settings do not change those dates."},
			{"Import a QuizForge QTI ZIP", "Tried 2026-10-09 through Canvas content migration: it produced an empty New Quiz with no error, so it is not offered. QuizForge content would need conversion to the quiz file format."},
			{"Create general standalone Canvas Pages", "Tier support pages are created as part of hub delivery. General pages for other purposes are future work."},
			{"Manage standalone/shared rubrics or edit existing rubric definitions", "Current support creates and attaches new rubrics to assignments and New Quizzes."},
			{"Create or reorganize modules, or move existing assignments between them", "Current support reads modules and places new assignments into existing modules."},
			{"Score New Quiz written work (ECR/SCR)", "Planned (teacher decision 2026-10-10): the agent reads written responses, scores them against the quiz rubric (or the authored rubric hints), and posts a feedback comment with those rubric scores visible to the student, after review. The teacher enters item points in SpeedGrader. Not built: reading New Quiz responses is unchecked and waits for real student work on a teacher quiz in CS8."},
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
