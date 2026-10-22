// cse-probe is a developer-only, narrowly scoped disposable-course probe.
// It is not included in the starter workspace or release ZIP. Root executes
// each selected step; this harness never publishes automatically or touches
// real students. Its explicit publish step needs separate human authorization.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/outlook"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

const allowedOrigin = "https://pearland.instructure.com"
const courseID int64 = 121046
const prefix = "[CSE 0.1b DISPOSABLE]"
const unattendedPrefix = "[CSE UNATTENDED]"
const statePath = ".test-workspaces/live-probe/state.json"

type probeState struct {
	Origin                string                          `json:"origin"`
	CourseID              int64                           `json:"course_id"`
	TestStudentID         int64                           `json:"test_student_id,omitempty"`
	AssignmentID          int64                           `json:"assignment_id,omitempty"`
	RubricID              int64                           `json:"rubric_id,omitempty"`
	AssociationID         int64                           `json:"association_id,omitempty"`
	OverrideID            int64                           `json:"override_id,omitempty"`
	CriterionID           string                          `json:"criterion_id,omitempty"`
	CriterionIDs          []string                        `json:"criterion_ids,omitempty"`
	ModuleID              int64                           `json:"module_id,omitempty"`
	ModuleName            string                          `json:"module_name,omitempty"`
	Published             bool                            `json:"published"`
	Title                 string                          `json:"title,omitempty"`
	StartedAt             string                          `json:"started_at,omitempty"`
	PublicationAuthorized bool                            `json:"publication_authorized,omitempty"`
	CommentIDs            []int64                         `json:"comment_ids,omitempty"`
	Outcomes              map[string]domain.ActionOutcome `json:"outcomes"`
}
type probe struct {
	c     *canvas.Client
	state probeState
	path  string
}

func main() {
	if e := run(context.Background(), os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string) error {
	selectedState := statePath
	if len(args) > 0 && args[0] == "--state" {
		if len(args) < 3 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("--state requires a path followed by a command")
		}
		selectedState, args = args[1], args[2:]
	}
	if len(args) == 0 || args[0] == "help" {
		fmt.Println("Optional leading --state PATH. Unattended helpers: select --assignment ID | configure-fixture [--assignment ID] [--submission-types online_text_entry,online_upload] | create-module --name '[CSE UNATTENDED] ...' | read-module | submit-file --file SYNTHETIC_PATH. Read-only field probe: field-read course | image-files | page ID | new-quiz ID | assignment ID ([CSE FIELD PROBE] objects only). Use a separate state per assignment; configure only unpublished exact named fixtures, then select. select never adopts legacy coursework.")
		fmt.Println("DEV ONLY: cse-probe inspect-test-student | create | restrict | verify-visibility | attach-rubric | read | submit [--body SYNTHETIC_TEXT] [--submitted-at RFC3339] | submit-as [same flags] | grade [--grade POINTS] [--rubric POINTS] [--comment SYNTHETIC_TEXT] | edit-comment --id ID --text SYNTHETIC_TEXT | couple-association [--enabled=false] | late --status none|late | set-due --at RFC3339 | publish | unpublish | pull [--workspace DEV_ROOT] | post --results RESULTS [--workspace DEV_ROOT] | inspect-state\nHardcoded CS8 course 121046; exact disposable assignment; built-in Test Student only. Publish requires separate teacher authorization and an explicit invocation; no automatic publication. Unpublish may be rejected by Canvas after submissions; no policy workaround. No delete/course-policy commands. State: " + statePath)
		return nil
	}
	if len(args) > 0 && args[0] == "outlook-self-test" {
		return outlookSelfTest()
	}
	path, e := filepath.Abs(selectedState)
	if e != nil {
		return e
	}
	p := &probe{path: path, state: probeState{Origin: allowedOrigin, CourseID: courseID, Outcomes: map[string]domain.ActionOutcome{}}}
	if b, e := os.ReadFile(path); e == nil {
		if e = json.Unmarshal(b, &p.state); e != nil {
			return fmt.Errorf("invalid local probe state")
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if p.state.Origin != allowedOrigin || p.state.CourseID != courseID {
		return fmt.Errorf("probe state is outside the authorized origin/course")
	}
	if p.state.Outcomes == nil {
		p.state.Outcomes = map[string]domain.ActionOutcome{}
	}
	if args[0] == "inspect-state" {
		return emit(p.state)
	}
	credential, e := platform.LoadCredential()
	if e != nil {
		return e
	}
	if credential.Origin != allowedOrigin {
		return fmt.Errorf("saved SE credential is not the authorized Canvas origin")
	}
	p.c, e = canvas.New(credential.Origin, credential.Token)
	credential.Token = ""
	if e != nil {
		return e
	}
	start := time.Now()
	defer func() { fmt.Fprintf(os.Stderr, "elapsed=%s requests=%d\n", time.Since(start), p.c.Requests()) }()
	command := args[0]
	args = args[1:]
	switch command {
	case "field-read":
		return p.fieldRead(ctx, args)
	case "select":
		return p.selectFixture(ctx, args)
	case "configure-fixture":
		return p.configureFixture(ctx, args)
	case "create-module":
		return p.createModule(ctx, args)
	case "read-module":
		if len(args) != 0 {
			return fmt.Errorf("read-module takes no arguments")
		}
		return p.readModule(ctx)
	case "inspect-test-student":
		if len(args) != 0 {
			return fmt.Errorf("inspect-test-student takes no arguments")
		}
		u, e := p.builtin(ctx)
		if e != nil {
			return e
		}
		p.state.TestStudentID = u.ID
		if e = p.save(); e != nil {
			return e
		}
		return emit(map[string]any{"course_id": courseID, "test_student_id": u.ID, "built_in_endpoint": true, "note": "Canvas documents this read creates the built-in Test Student if absent"})
	case "create":
		if len(args) != 0 {
			return fmt.Errorf("create takes no arguments")
		}
		return p.create(ctx)
	case "restrict":
		if len(args) != 0 {
			return fmt.Errorf("restrict takes no arguments")
		}
		return p.restrict(ctx)
	case "verify-visibility":
		if len(args) != 0 {
			return fmt.Errorf("verify-visibility takes no arguments")
		}
		if e := p.verifyTestStudentOnly(ctx); e != nil {
			return e
		}
		return emit(map[string]any{"assignment_id": p.state.AssignmentID, "test_student_only": true, "override_id": p.state.OverrideID})
	case "attach-rubric":
		if len(args) != 0 {
			return fmt.Errorf("attach-rubric takes no arguments")
		}
		return p.attach(ctx)
	case "read":
		if len(args) != 0 {
			return fmt.Errorf("read takes no arguments")
		}
		a, e := p.checkedAssignment(ctx)
		if e != nil {
			return e
		}
		shape := assignmentShape(a.Assignment)
		shape["omit_from_final_grade"], shape["post_to_sis"] = a.OmitFromFinalGrade, a.PostToSIS
		if e = emit(shape); e != nil {
			return e
		}
		if p.state.TestStudentID > 0 {
			sub, e := p.submission(ctx)
			if e != nil {
				return e
			}
			return emit(submissionShape(sub))
		}
		return nil
	case "submit", "submit-as":
		return p.submit(ctx, args, command == "submit-as")
	case "submit-file":
		return p.submitFile(ctx, args)
	case "grade":
		return p.grade(ctx, args)
	case "edit-comment":
		return p.edit(ctx, args)
	case "couple-association":
		return p.couple(ctx, args)
	case "late":
		return p.late(ctx, args)
	case "set-due":
		return p.due(ctx, args)
	case "publish", "unpublish":
		if len(args) != 0 {
			return fmt.Errorf("publication step takes no arguments")
		}
		return p.publication(ctx, command == "publish")
	case "pull", "post":
		return p.serviceWorkflow(ctx, command, args)
	default:
		return fmt.Errorf("unknown probe step; run help")
	}
}
func (p *probe) serviceWorkflow(ctx context.Context, command string, args []string) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	rootArg := flags.String("workspace", ".test-workspaces/live-probe/workspace", "developer workspace copied from clean template")
	results := flags.String("results", "", "workspace-relative scoring results file for post")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("unexpected workflow probe argument")
	}
	root, e := workspace.ResolveRoot(*rootArg, "")
	if e != nil {
		return e
	}
	var config domain.CourseConfig
	if _, e = workspace.ReadTeacherDocument(root, "context/courses.md", &config); e != nil {
		return e
	}
	if config.Instance != allowedOrigin {
		return fmt.Errorf("developer workspace targets a different Canvas origin")
	}
	nickname := ""
	for _, c := range config.Courses {
		if c.ID == courseID {
			nickname = c.Nickname
		}
	}
	if nickname == "" {
		return fmt.Errorf("developer workspace must explicitly configure authorized course 121046")
	}
	if _, e = p.assignment(ctx); e != nil {
		return e
	}
	if e = p.target(ctx); e != nil {
		return e
	}
	team, e := workspace.ReadTeam(root)
	if e == nil {
		e = workspace.CheckOwners(config, team)
	}
	if e != nil {
		return e
	}
	// Keys stays nil: the probe uses its own key, but ownership rules still apply.
	service := &workflow.Service{Root: root, Client: p.c, Config: config, Team: team, InstallRoot: root}
	if command == "pull" {
		if *results != "" {
			return fmt.Errorf("pull does not take --results")
		}
		report, e := service.PullBuiltinTestStudent(ctx, nickname, p.state.AssignmentID, p.state.TestStudentID)
		if e != nil {
			return e
		}
		return emit(report)
	}
	if *results == "" {
		return fmt.Errorf("post requires --results")
	}
	rel := *results
	if filepath.IsAbs(rel) {
		rel, e = filepath.Rel(root, rel)
		if e != nil {
			return e
		}
	}
	resultPath, e := workspace.ConstrainPath(root, rel)
	if e != nil {
		return e
	}
	var doc domain.ResultsDocument
	if _, e = workspace.ReadDocument(resultPath, &doc); e != nil {
		return e
	}
	if doc.Assignment.Instance != allowedOrigin || doc.Assignment.CourseID != courseID || doc.Assignment.AssignmentID != p.state.AssignmentID || doc.Assignment.Title != p.state.Title {
		return fmt.Errorf("results do not target exact persisted disposable probe")
	}
	store, e := workspace.OpenStudents(root)
	if e != nil {
		return e
	}
	for _, row := range doc.Students {
		student, e := store.Resolve(allowedOrigin, row.StudentRef)
		if e != nil {
			return e
		}
		if student.UserID != p.state.TestStudentID {
			return fmt.Errorf("developer post refuses a result for any real student")
		}
	}
	report, e := service.Post(ctx, resultPath)
	if emitErr := emit(report); emitErr != nil {
		return emitErr
	}
	if e != nil {
		return e
	}
	for _, row := range report.Students {
		if row.Outcomes.Comment.ID > 0 {
			found := false
			for _, id := range p.state.CommentIDs {
				if id == row.Outcomes.Comment.ID {
					found = true
				}
			}
			if !found {
				p.state.CommentIDs = append(p.state.CommentIDs, row.Outcomes.Comment.ID)
			}
		}
	}
	return p.save()
}
func emit(value any) error {
	b, e := json.MarshalIndent(value, "", "  ")
	if e == nil {
		fmt.Println(string(b))
	}
	return e
}
func (p *probe) save() error {
	b, e := json.MarshalIndent(p.state, "", "  ")
	if e != nil {
		return e
	}
	return workspace.WriteFile(p.path, b, 0600)
}
func (p *probe) builtin(ctx context.Context) (workflow.CanvasUser, error) {
	var u workflow.CanvasUser
	e := p.c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/student_view_student", courseID), &u)
	if e == nil && u.ID <= 0 {
		e = fmt.Errorf("built-in Test Student endpoint lacks a positive ID")
	}
	return u, e
}
func (p *probe) target(ctx context.Context) error {
	u, e := p.builtin(ctx)
	if e != nil {
		return e
	}
	if p.state.TestStudentID <= 0 || p.state.TestStudentID != u.ID {
		return fmt.Errorf("probe target does not match current built-in Test Student; inspect-test-student first")
	}
	return nil
}
func (p *probe) assignment(ctx context.Context) (workflow.Assignment, error) {
	a, e := p.checkedAssignment(ctx)
	return a.Assignment, e
}
func (p *probe) checkedAssignment(ctx context.Context) (fixtureAssignment, error) {
	if p.state.AssignmentID <= 0 {
		return fixtureAssignment{}, fmt.Errorf("no known probe assignment; create or select first")
	}
	a, e := p.getFixture(ctx, p.state.AssignmentID)
	if e != nil {
		return a, e
	}
	if e = validateAssignment(a.Assignment, p.state); e != nil {
		return a, e
	}
	if strings.HasPrefix(a.Name, unattendedPrefix) && !a.safelyExcluded() {
		return a, fmt.Errorf("unattended fixture must omit final grades and disable SIS posting")
	}
	return a, nil
}
func validateAssignment(a workflow.Assignment, s probeState) error {
	if s.Origin != allowedOrigin || s.CourseID != courseID || a.ID != s.AssignmentID || a.CourseID != courseID || (!strings.HasPrefix(a.Name, prefix) && !strings.HasPrefix(a.Name, unattendedPrefix)) || a.Name != s.Title || (a.Published && !s.PublicationAuthorized) || a.GradingType != "points" {
		return fmt.Errorf("refusing write/read: target is not the exact authorized unpublished disposable assignment")
	}
	return nil
}
func (p *probe) publication(ctx context.Context, published bool) error {
	// Root invokes publish only after the human has authorized this explicit
	// temporary experiment. This development field never exists in cse product
	// grading flows and is not permission to publish any other assignment.
	if p.state.AssignmentID <= 0 {
		return fmt.Errorf("no exact probe assignment")
	}
	fixture, e := p.getFixture(ctx, p.state.AssignmentID)
	a := fixture.Assignment
	if e != nil {
		return e
	}
	check := p.state
	if !published {
		check.PublicationAuthorized = true
	}
	if e = validateAssignment(a, check); e != nil {
		return e
	}
	if published {
		if e = p.verifyTestStudentOnly(ctx); e != nil {
			return e
		}
	}
	if strings.HasPrefix(a.Name, unattendedPrefix) && !fixture.safelyExcluded() {
		return fmt.Errorf("unattended publication requires final-grade and SIS exclusion")
	}
	step := "unpublish"
	if published {
		step = "publish"
	}
	if a.Published == published {
		p.state.PublicationAuthorized = published
		p.state.Published = published
		p.state.Outcomes[step] = domain.ActionOutcome{Status: "preserved", ID: a.ID}
		if e = p.save(); e != nil {
			return e
		}
		return emit(assignmentShape(a))
	}
	if published {
		p.state.PublicationAuthorized = true
	}
	if e = p.begin(step); e != nil {
		return e
	}
	var response workflow.Assignment
	e = p.c.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, p.state.AssignmentID), map[string]any{"assignment": map[string]any{"published": published}}, &response)
	if e == nil && (response.ID != p.state.AssignmentID || response.Published != published) {
		e = &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	if e == nil {
		p.state.PublicationAuthorized = published
		p.state.Published = published
	}
	if f := p.finish(step, e, p.state.AssignmentID); f != nil {
		if !published {
			return fmt.Errorf("Canvas did not confirm unpublication; probe may remain published, no deletion or course-policy workaround was applied: %w", f)
		}
		return f
	}
	return emit(assignmentShape(response))
}

// restrict limits the disposable assignment to the built-in Test Student before any publish.
func (p *probe) restrict(ctx context.Context) error {
	a, e := p.assignment(ctx)
	if e != nil {
		return e
	}
	if a.Published {
		return fmt.Errorf("restrict requires an unpublished disposable assignment")
	}
	if e = p.target(ctx); e != nil {
		return e
	}
	if p.state.OverrideID == 0 {
		if e = p.begin("restrict-override"); e != nil {
			return e
		}
		var override struct {
			ID int64 `json:"id"`
		}
		e = p.c.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/assignments/%d/overrides", courseID, p.state.AssignmentID), map[string]any{"assignment_override": map[string]any{"title": prefix + " Test Student only", "student_ids": []int64{p.state.TestStudentID}}}, &override)
		if e == nil && override.ID <= 0 {
			e = &canvas.Error{Kind: "uncertain", Uncertain: true}
		}
		if e == nil {
			p.state.OverrideID = override.ID
		}
		if f := p.finish("restrict-override", e, override.ID); f != nil {
			return f
		}
	}
	if a.OnlyVisibleToOverrides == nil || !*a.OnlyVisibleToOverrides {
		if e = p.begin("restrict-visibility"); e != nil {
			return e
		}
		var response workflow.Assignment
		e = p.c.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, p.state.AssignmentID), map[string]any{"assignment": map[string]any{"only_visible_to_overrides": true}}, &response)
		if e == nil && response.ID != p.state.AssignmentID {
			e = &canvas.Error{Kind: "uncertain", Uncertain: true}
		}
		if f := p.finish("restrict-visibility", e, p.state.AssignmentID); f != nil {
			return f
		}
	}
	if e = p.verifyTestStudentOnly(ctx); e != nil {
		return e
	}
	return emit(map[string]any{"assignment_id": p.state.AssignmentID, "test_student_only": true, "override_id": p.state.OverrideID, "published": false})
}

// verifyTestStudentOnly is the publish guard: only the exact Test Student override may see the assignment.
func (p *probe) verifyTestStudentOnly(ctx context.Context) error {
	if e := p.target(ctx); e != nil {
		return e
	}
	a, e := p.assignment(ctx)
	if e != nil {
		return e
	}
	if a.OnlyVisibleToOverrides == nil || !*a.OnlyVisibleToOverrides || p.state.OverrideID <= 0 {
		return fmt.Errorf("assignment is not limited to overrides; run restrict before publishing")
	}
	var overrides []struct {
		ID              int64   `json:"id"`
		StudentIDs      []int64 `json:"student_ids"`
		CourseSectionID int64   `json:"course_section_id"`
		GroupID         int64   `json:"group_id"`
	}
	if e = p.c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d/overrides?per_page=100", courseID, p.state.AssignmentID), &overrides); e != nil {
		return e
	}
	if len(overrides) != 1 || overrides[0].ID != p.state.OverrideID || len(overrides[0].StudentIDs) != 1 || overrides[0].StudentIDs[0] != p.state.TestStudentID || overrides[0].CourseSectionID != 0 || overrides[0].GroupID != 0 {
		return fmt.Errorf("visibility is not limited to exactly the built-in Test Student; refusing")
	}
	return nil
}
func (p *probe) submission(ctx context.Context) (workflow.Submission, error) {
	if e := p.target(ctx); e != nil {
		return workflow.Submission{}, e
	}
	sub, e := workflow.GetSubmission(ctx, p.c, courseID, p.state.AssignmentID, p.state.TestStudentID)
	if e == nil && (sub.UserID != p.state.TestStudentID || sub.AssignmentID != p.state.AssignmentID) {
		e = fmt.Errorf("returned submission is not the authorized built-in Test Student target")
	}
	return sub, e
}
func (p *probe) begin(step string) error {
	old := p.state.Outcomes[step]
	if old.Status == "pending" || old.Status == "uncertain" {
		return fmt.Errorf("%s previous write is uncertain; inspect live state before another write", step)
	}
	p.state.Outcomes[step] = domain.ActionOutcome{Status: "pending"}
	return p.save()
}
func (p *probe) finish(step string, e error, id int64) error {
	out := domain.ActionOutcome{Status: "succeeded", ID: id}
	if e != nil {
		out.Status = "failed"
		out.Message = e.Error()
		var ce *canvas.Error
		if errors.As(e, &ce) && ce.Uncertain {
			out.Status = "uncertain"
		}
	}
	p.state.Outcomes[step] = out
	if saveErr := p.save(); saveErr != nil {
		return fmt.Errorf("remote outcome %s known, local state save failed: %w", out.Status, saveErr)
	}
	if e != nil {
		shape := map[string]any{"step": step, "status": out.Status, "assignment_id": p.state.AssignmentID, "reason": out.Message}
		var ce *canvas.Error
		if errors.As(e, &ce) {
			shape["http_status"] = ce.Status
			shape["failure_kind"] = ce.Kind
			shape["uncertain"] = ce.Uncertain
		}
		_ = emit(shape)
		return e
	}
	return nil
}
func (p *probe) create(ctx context.Context) error {
	if p.state.AssignmentID > 0 {
		return fmt.Errorf("probe assignment already exists; use read")
	}
	if p.state.Title == "" {
		p.state.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		p.state.Title = prefix + " " + time.Now().UTC().Format("20060102T150405Z")
	}
	if e := p.begin("create"); e != nil {
		return e
	}
	var a workflow.Assignment
	e := p.c.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/assignments", courseID), map[string]any{"assignment": map[string]any{"name": p.state.Title, "description": "<p>Disposable CanvasExpert-SE API test. Built-in Test Student only. Do not assign, publish, or include in final grades.</p>", "points_possible": 20, "grading_type": "points", "submission_types": []string{"online_text_entry"}, "published": false, "omit_from_final_grade": true, "post_to_sis": false}}, &a)
	if e == nil && a.ID > 0 {
		p.state.AssignmentID = a.ID
	} else if e == nil {
		e = &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	if f := p.finish("create", e, p.state.AssignmentID); f != nil {
		return f
	}
	if e = validateAssignment(a, p.state); e != nil {
		return e
	}
	return emit(assignmentShape(a))
}
func (p *probe) attach(ctx context.Context) error {
	a, e := p.assignment(ctx)
	if e != nil {
		return e
	}
	if p.state.RubricID > 0 || len(a.Rubric) > 0 {
		return fmt.Errorf("probe already has a rubric; read before further changes")
	}
	if e = p.begin("attach-rubric"); e != nil {
		return e
	}
	criteria := map[string]any{"0": map[string]any{"description": "Synthetic response evidence", "long_description": "Probe criterion only. Represent explicit fractional points honestly.", "points": 100, "criterion_use_range": false, "ratings": map[string]any{"0": map[string]any{"description": "Complete synthetic evidence", "points": 100}, "1": map[string]any{"description": "Fractional synthetic evidence", "points": 62.5}, "2": map[string]any{"description": "No synthetic evidence", "points": 0}}}}
	var response struct {
		Rubric struct {
			ID   int64                `json:"id"`
			Data []workflow.Criterion `json:"data"`
		} `json:"rubric"`
		Association workflow.RubricAssociation `json:"rubric_association"`
	}
	e = p.c.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/rubrics", courseID), map[string]any{"rubric": map[string]any{"title": prefix + " 100-point rubric", "criteria": criteria, "skip_updating_points_possible": true, "free_form_criterion_comments": false}, "rubric_association": map[string]any{"association_id": p.state.AssignmentID, "association_type": "Assignment", "use_for_grading": false, "purpose": "grading"}}, &response)
	if response.Rubric.ID > 0 {
		p.state.RubricID = response.Rubric.ID
	}
	if response.Association.ID > 0 {
		p.state.AssociationID = response.Association.ID
	}
	if len(response.Rubric.Data) == 1 {
		p.state.CriterionID = response.Rubric.Data[0].ID
	}
	if e == nil && (p.state.RubricID <= 0 || p.state.AssociationID <= 0) {
		e = &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	if f := p.finish("attach-rubric", e, p.state.RubricID); f != nil {
		return f
	}
	a, e = p.assignment(ctx)
	if e != nil {
		return e
	}
	if len(a.Rubric) == 1 {
		p.state.CriterionID = a.Rubric[0].ID
	}
	if e = p.save(); e != nil {
		return e
	}
	return emit(assignmentShape(a))
}
func (p *probe) submit(ctx context.Context, args []string, asUser bool) error {
	flags := flag.NewFlagSet("submit", flag.ContinueOnError)
	body := flags.String("body", "This is synthetic Test Student evidence. It includes a claim and a supporting explanation.", "synthetic body")
	submitted := flags.String("submitted-at", "", "explicit synthetic timestamp")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("unexpected submit argument")
	}
	if submitted != nil && *submitted != "" {
		if _, e := time.Parse(time.RFC3339Nano, *submitted); e != nil {
			return fmt.Errorf("submitted-at must be RFC3339")
		}
	}
	if _, e := p.assignment(ctx); e != nil {
		return e
	}
	if e := p.target(ctx); e != nil {
		return e
	}
	step := "submit"
	if asUser {
		step = "submit-as"
	}
	if e := p.begin(step); e != nil {
		return e
	}
	fields := map[string]any{"submission_type": "online_text_entry", "body": "<p>" + escapeSynthetic(*body) + "</p>"}
	if !asUser {
		fields["user_id"] = p.state.TestStudentID
	}
	if *submitted != "" {
		fields["submitted_at"] = *submitted
	}
	path := fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions", courseID, p.state.AssignmentID)
	if asUser {
		path += "?as_user_id=" + strconv.FormatInt(p.state.TestStudentID, 10)
	}
	var sub workflow.Submission
	e := p.c.Write(ctx, "POST", path, map[string]any{"submission": fields}, &sub)
	if e == nil && (sub.UserID != p.state.TestStudentID || sub.AssignmentID != p.state.AssignmentID) {
		e = &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	if f := p.finish(step, e, sub.ID); f != nil {
		return fmt.Errorf("submission on unpublished disposable assignment: %w; no publication or course-policy workaround was applied", f)
	}
	return emit(submissionShape(sub))
}
func escapeSynthetic(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;").Replace(s)
}
func (p *probe) grade(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("grade", flag.ContinueOnError)
	grade := flags.String("grade", "", "entered assignment points")
	rubric := flags.String("rubric", "", "rubric points out of100")
	comment := flags.String("comment", "", "synthetic comment")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if len(flags.Args()) != 0 || (*grade == "" && *rubric == "" && *comment == "") {
		return fmt.Errorf("grade requires selected grade/rubric/comment fields")
	}
	a, e := p.assignment(ctx)
	if e != nil {
		return e
	}
	live, e := p.submission(ctx)
	if e != nil {
		return e
	}
	payload := map[string]any{}
	if *grade != "" {
		d, e := boundedDecimal(*grade, "20")
		if e != nil {
			return e
		}
		payload["submission"] = map[string]any{"posted_grade": d.String()}
	}
	if *rubric != "" {
		d, e := boundedDecimal(*rubric, "100")
		if e != nil {
			return e
		}
		if len(a.Rubric) != 1 || a.Rubric[0].ID == "" || p.state.RubricID != a.RubricSettings.ID {
			return fmt.Errorf("rubric does not match exact probe rubric")
		}
		payload["rubric_assessment"] = map[string]any{a.Rubric[0].ID: map[string]any{"points": d, "comments": "Synthetic disposable probe assessment."}}
	}
	if *comment != "" {
		if live.Attempt < 1 {
			return fmt.Errorf("actual Test Student attempt is absent; comment attempt probe held")
		}
		payload["comment"] = map[string]any{"text_comment": *comment, "attempt": live.Attempt}
	}
	if e = p.begin("grade"); e != nil {
		return e
	}
	var response workflow.Submission
	e = p.c.Write(ctx, "PUT", p.submissionPath(), payload, &response)
	if e == nil && (response.UserID != p.state.TestStudentID || response.AssignmentID != p.state.AssignmentID) {
		e = &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	before := map[int64]bool{}
	for _, c := range live.Comments {
		before[c.ID] = true
	}
	for _, c := range response.Comments {
		if !before[c.ID] && c.ID > 0 && c.Comment == *comment {
			p.state.CommentIDs = append(p.state.CommentIDs, c.ID)
		}
	}
	if f := p.finish("grade", e, response.ID); f != nil {
		return f
	}
	return emit(submissionShape(response))
}
func boundedDecimal(raw, max string) (domain.Decimal, error) {
	d, e := domain.ParseDecimal(raw)
	if e != nil || d.Sign() < 0 || d.Cmp(domain.MustDecimal(max)) > 0 {
		return d, fmt.Errorf("probe points must be finite and between 0 and %s", max)
	}
	return d, nil
}
func (p *probe) submissionPath() string {
	return fmt.Sprintf("/api/v1/courses/%d/assignments/%d/submissions/%d", courseID, p.state.AssignmentID, p.state.TestStudentID)
}
func (p *probe) edit(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("edit-comment", flag.ContinueOnError)
	id := flags.Int64("id", 0, "exact returned probe comment ID")
	text := flags.String("text", "", "synthetic replacement")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if len(flags.Args()) != 0 || *id <= 0 || *text == "" {
		return fmt.Errorf("edit-comment requires --id and --text")
	}
	known := false
	for _, v := range p.state.CommentIDs {
		if v == *id {
			known = true
		}
	}
	if !known {
		return fmt.Errorf("comment ID was not returned by this disposable probe")
	}
	if _, e := p.assignment(ctx); e != nil {
		return e
	}
	live, e := p.submission(ctx)
	if e != nil {
		return e
	}
	found := false
	for _, c := range live.Comments {
		if c.ID == *id {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("known comment no longer exists")
	}
	if e = p.begin("edit-comment"); e != nil {
		return e
	}
	var response workflow.SubmissionComment
	e = p.c.Write(ctx, "PUT", p.submissionPath()+fmt.Sprintf("/comments/%d", *id), map[string]any{"comment": *text}, &response)
	if e == nil && (response.ID != *id || response.Comment != *text) {
		e = &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	if f := p.finish("edit-comment", e, response.ID); f != nil {
		return f
	}
	return emit(map[string]any{"comment_id": response.ID, "text_matches": response.Comment == *text, "attempt": response.Attempt})
}
func (p *probe) couple(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("couple-association", flag.ContinueOnError)
	enabled := flags.Bool("enabled", true, "automatic grade calculation setting")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("unexpected coupling argument")
	}
	a, e := p.assignment(ctx)
	if e != nil {
		return e
	}
	if a.RubricSettings.ID != p.state.RubricID || p.state.RubricID <= 0 || p.state.AssociationID <= 0 || a.RubricAssociation == nil || a.RubricAssociation.ID != p.state.AssociationID {
		return fmt.Errorf("probe rubric identity mismatch")
	}
	before, e := workflow.Snapshot(a, p.c.Origin())
	if e != nil || before.Rubric == nil || len(before.Rubric.Criteria) == 0 {
		return fmt.Errorf("probe lacks a complete live rubric definition")
	}
	if e = p.begin("couple-association"); e != nil {
		return e
	}
	var response json.RawMessage
	e = p.c.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/rubrics/%d", courseID, p.state.RubricID), couplingFields(*before.Rubric, p.state.AssignmentID, *enabled), &response)
	if e == nil {
		a, e = p.assignment(ctx)
		if e == nil {
			var after domain.AssignmentSnapshot
			after, e = workflow.Snapshot(a, p.c.Origin())
			before.UpdatedAt, after.UpdatedAt = "", ""
			before.Rubric.UseForGrading, before.Rubric.UseForGradingKnown = *enabled, true
			if e == nil && !workflow.SameSnapshot(before, after) {
				e = fmt.Errorf("coupling changed the fixture definition, points, or unrelated settings")
			}
		}
		if e != nil {
			e = &canvas.Error{Kind: "uncertain", Uncertain: true}
		}
	}
	if f := p.finish("couple-association", e, p.state.AssociationID); f != nil {
		return f
	}
	return emit(assignmentShape(a))
}

// The rubric update endpoint needs a complete definition to change coupling.
// Omitting criteria empties the rubric, and its skip-points flag is top-level.
// Keep all fixture criterion/rating IDs so existing assessments remain attached.
func couplingFields(r domain.Rubric, assignmentID int64, enabled bool) map[string]any {
	criteria := map[string]any{}
	for i, c := range r.Criteria {
		ratings := map[string]any{}
		for j, rating := range c.Ratings {
			ratings[strconv.Itoa(j)] = map[string]any{"id": rating.ID, "points": rating.Points, "description": rating.Description, "long_description": rating.LongDescription}
		}
		criteria[strconv.Itoa(i)] = map[string]any{"id": c.ID, "points": c.Points, "description": c.Description, "long_description": c.LongDescription, "criterion_use_range": c.UseRange, "ratings": ratings}
	}
	freeForm := "0"
	if r.FreeFormCriterionComments {
		freeForm = "1"
	}
	return map[string]any{"rubric_association_id": r.AssociationID, "skip_updating_points_possible": "true", "rubric": map[string]any{"title": r.Title, "free_form_criterion_comments": freeForm, "criteria": criteria}, "rubric_association": map[string]any{"association_id": assignmentID, "association_type": "Assignment", "use_for_grading": enabled, "purpose": "grading"}}
}
func (p *probe) late(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("late", flag.ContinueOnError)
	status := flags.String("status", "none", "none or late on Test Student only")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if len(flags.Args()) != 0 || (*status != "none" && *status != "late") {
		return fmt.Errorf("late status must be none or late")
	}
	if _, e := p.assignment(ctx); e != nil {
		return e
	}
	if _, e := p.submission(ctx); e != nil {
		return e
	}
	if e := p.begin("late"); e != nil {
		return e
	}
	var response workflow.Submission
	e := p.c.Write(ctx, "PUT", p.submissionPath(), map[string]any{"submission": map[string]any{"late_policy_status": *status}}, &response)
	if f := p.finish("late", e, response.ID); f != nil {
		return f
	}
	return emit(submissionShape(response))
}
func (p *probe) due(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("set-due", flag.ContinueOnError)
	at := flags.String("at", "", "synthetic applicable due date")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("unexpected due argument")
	}
	if _, e := time.Parse(time.RFC3339Nano, *at); e != nil {
		return fmt.Errorf("due date must be RFC3339")
	}
	if _, e := p.assignment(ctx); e != nil {
		return e
	}
	if e := p.begin("set-due"); e != nil {
		return e
	}
	var a workflow.Assignment
	e := p.c.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, p.state.AssignmentID), map[string]any{"assignment": map[string]any{"due_at": *at}}, &a)
	if f := p.finish("set-due", e, a.ID); f != nil {
		return f
	}
	return emit(assignmentShape(a))
}
func assignmentShape(a workflow.Assignment) map[string]any {
	criteria := []map[string]any{}
	for _, c := range a.Rubric {
		ratings := []map[string]any{}
		for _, r := range c.Ratings {
			ratings = append(ratings, map[string]any{"rating_id": r.ID, "points": r.Points})
		}
		criteria = append(criteria, map[string]any{"criterion_id": c.ID, "points": c.Points, "ratings": ratings})
	}
	return map[string]any{"assignment_id": a.ID, "course_id": a.CourseID, "title": a.Name, "description": a.Description, "published": a.Published, "unpublished": !a.Published, "grading_type": a.GradingType, "submission_types": a.SubmissionTypes, "points_possible": a.Points, "due_at": a.DueAt, "unlock_at": a.UnlockAt, "lock_at": a.LockAt, "rubric_id": a.RubricSettings.ID, "rubric_total": a.RubricSettings.Points, "association": a.RubricAssociation, "criteria": criteria}
}
func submissionShape(s workflow.Submission) map[string]any {
	history := []map[string]any{}
	for _, h := range s.History {
		history = append(history, map[string]any{"attempt": h.Attempt, "submitted_at": h.SubmittedAt, "body_present": h.Body != "", "attachment_count": len(h.Attachments), "submission_type": h.Type, "cached_due_date": h.CachedDueDate, "comments": commentShapes(h.Comments)})
	}
	return map[string]any{"assignment_id": s.AssignmentID, "test_student_id": s.UserID, "submission_id": s.ID, "attempt": s.Attempt, "body_present": s.Body != "", "attachment_count": len(s.Attachments), "submission_type": s.Type, "submitted_at": s.SubmittedAt, "score": s.Score, "entered_score": s.EnteredScore, "points_deducted": s.PointsDeducted, "state": s.State, "grade_matches_current_submission": s.GradeMatchesCurrentSubmission, "late": s.Late, "late_policy_status": s.LatePolicyStatus, "seconds_late": s.SecondsLate, "cached_due_date": s.CachedDueDate, "rubric_assessment": s.RubricAssessment, "comments": commentShapes(s.Comments), "history": history}
}
func commentShapes(comments []workflow.SubmissionComment) []map[string]any {
	result := []map[string]any{}
	for _, c := range comments {
		result = append(result, map[string]any{"id": c.ID, "attempt": c.Attempt, "text_length": len(c.Comment), "created_at": c.CreatedAt})
	}
	return result
}

// outlookSelfTest sends one message to the Outlook profile's own address. It
// never accepts another recipient.
func outlookSelfTest() error {
	if ok, reason := outlook.Status(); !ok {
		return fmt.Errorf("Outlook is not available: %s", reason)
	}
	self, e := outlook.SelfAddress()
	if e != nil {
		return e
	}
	out, e := outlook.Send(outlook.Message{
		To:       []string{self},
		Subject:  "CE-SE Outlook self-test",
		HTMLBody: "<p>CE-SE sent this test message from Go through classic Outlook.</p>",
		TextBody: "CE-SE sent this test message from Go through classic Outlook.",
	})
	fmt.Println("outcome:", out)
	return e
}
