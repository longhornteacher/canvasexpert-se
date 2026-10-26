package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/workflow"
)

// Safety fields stay in this developer helper, not the production contract.
type fixtureAssignment struct {
	workflow.Assignment
	OmitFromFinalGrade *bool `json:"omit_from_final_grade"`
	PostToSIS          *bool `json:"post_to_sis"`
}

func (a fixtureAssignment) safelyExcluded() bool {
	return a.OmitFromFinalGrade != nil && *a.OmitFromFinalGrade && a.PostToSIS != nil && !*a.PostToSIS
}

func (p *probe) getFixture(ctx context.Context, id int64) (fixtureAssignment, error) {
	var a fixtureAssignment
	e := p.c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/assignments/%d?include[]=rubric_assessment", courseID, id), &a)
	if e == nil && len(a.Rubric) > 0 && a.RubricSettings.ID > 0 && (a.RubricAssociation == nil || a.RubricAssociation.UseForGrading == nil) {
		var rubric struct {
			Associations []workflow.RubricAssociation `json:"associations"`
		}
		e = p.c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/rubrics/%d?include[]=assignment_associations", courseID, a.RubricSettings.ID), &rubric)
		matches := []workflow.RubricAssociation{}
		for _, association := range rubric.Associations {
			if association.AssociationType == "Assignment" && association.AssociationID == id {
				matches = append(matches, association)
			}
		}
		if len(matches) == 1 {
			a.RubricAssociation = &matches[0]
		}
	}
	return a, e
}

func (p *probe) adopt(a fixtureAssignment) error {
	p.state.AssignmentID, p.state.Title = a.ID, a.Name
	p.state.Published, p.state.PublicationAuthorized = a.Published, a.Published
	p.state.RubricID, p.state.AssociationID = a.RubricSettings.ID, 0
	p.state.CriterionID, p.state.CriterionIDs = "", nil
	for _, c := range a.Rubric {
		p.state.CriterionIDs = append(p.state.CriterionIDs, c.ID)
	}
	if len(a.Rubric) == 1 {
		p.state.CriterionID = a.Rubric[0].ID
	}
	if a.RubricAssociation != nil {
		p.state.AssociationID = a.RubricAssociation.ID
	}
	return p.save()
}

func (p *probe) namedFixture(a fixtureAssignment, id int64) error {
	if id <= 0 || a.ID != id || a.CourseID != courseID || !strings.HasPrefix(a.Name, unattendedPrefix+" ") || a.GradingType != "points" {
		return fmt.Errorf("target must be an exact [CSE UNATTENDED] points assignment in the authorized probe course")
	}
	if p.state.AssignmentID > 0 && (id != p.state.AssignmentID || a.Name != p.state.Title) {
		return fmt.Errorf("state already owns another assignment; use a separate --state path")
	}
	return nil
}

func (p *probe) selectFixture(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("select", flag.ContinueOnError)
	id := flags.Int64("assignment", 0, "exact production-created unattended fixture ID")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if len(flags.Args()) != 0 || *id <= 0 {
		return fmt.Errorf("select requires --assignment ID")
	}
	a, e := p.getFixture(ctx, *id)
	if e != nil {
		return e
	}
	if e = p.namedFixture(a, *id); e != nil {
		return e
	}
	if !a.safelyExcluded() {
		return fmt.Errorf("fixture must omit final grades and disable SIS posting; configure-fixture before publication")
	}
	if e = p.adopt(a); e != nil {
		return e
	}
	shape := assignmentShape(a.Assignment)
	shape["omit_from_final_grade"], shape["post_to_sis"] = a.OmitFromFinalGrade, a.PostToSIS
	return emit(shape)
}

func (p *probe) configureFixture(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("configure-fixture", flag.ContinueOnError)
	id := flags.Int64("assignment", p.state.AssignmentID, "exact unpublished unattended fixture ID")
	types := flags.String("submission-types", "", "optional comma-separated online_text_entry,online_upload")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if len(flags.Args()) != 0 || *id <= 0 {
		return fmt.Errorf("configure-fixture requires an assignment ID")
	}
	fields := map[string]any{"omit_from_final_grade": true, "post_to_sis": false}
	if *types != "" {
		values := strings.Split(*types, ",")
		for _, value := range values {
			if value != "online_text_entry" && value != "online_upload" {
				return fmt.Errorf("only synthetic text/file submission types are supported")
			}
		}
		fields["submission_types"] = values
	}
	a, e := p.getFixture(ctx, *id)
	if e != nil {
		return e
	}
	if e = p.namedFixture(a, *id); e != nil {
		return e
	}
	if a.Published {
		return fmt.Errorf("configure-fixture requires an unpublished fixture")
	}
	// Record identity before writing so uncertainty cannot lose the target.
	if e = p.adopt(a); e != nil {
		return e
	}
	if e = p.begin("configure-fixture"); e != nil {
		return e
	}
	var response fixtureAssignment
	e = p.c.Write(ctx, "PUT", fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, *id), map[string]any{"assignment": fields}, &response)
	if e == nil && (response.ID != *id || response.Name != a.Name || response.Published || !response.safelyExcluded()) {
		e = &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	if f := p.finish("configure-fixture", e, *id); f != nil {
		return f
	}
	fresh, e := p.checkedAssignment(ctx)
	if e != nil {
		return e
	}
	shape := assignmentShape(fresh.Assignment)
	shape["omit_from_final_grade"], shape["post_to_sis"] = fresh.OmitFromFinalGrade, fresh.PostToSIS
	return emit(shape)
}

func (p *probe) createModule(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("create-module", flag.ContinueOnError)
	name := flags.String("name", "", "dedicated unattended module name")
	if e := flags.Parse(args); e != nil {
		return e
	}
	if len(flags.Args()) != 0 || !strings.HasPrefix(*name, unattendedPrefix+" ") || strings.TrimSpace(strings.TrimPrefix(*name, unattendedPrefix)) == "" {
		return fmt.Errorf("create-module requires --name '[CSE UNATTENDED] ...'")
	}
	if p.state.ModuleID > 0 {
		return fmt.Errorf("module already exists; use read-module")
	}
	if p.state.ModuleName != "" && p.state.ModuleName != *name {
		return fmt.Errorf("state already records a different module name")
	}
	p.state.ModuleName = *name
	if e := p.begin("create-module"); e != nil {
		return e
	}
	var module workflow.Module
	e := p.c.Write(ctx, "POST", fmt.Sprintf("/api/v1/courses/%d/modules", courseID), map[string]any{"module": map[string]any{"name": *name}}, &module)
	if module.ID > 0 {
		p.state.ModuleID = module.ID
	}
	if e == nil && (module.ID <= 0 || module.Name != *name) {
		e = &canvas.Error{Kind: "uncertain", Uncertain: true}
	}
	if f := p.finish("create-module", e, module.ID); f != nil {
		return f
	}
	return p.readModule(ctx)
}

func (p *probe) readModule(ctx context.Context) error {
	if p.state.ModuleID <= 0 || !strings.HasPrefix(p.state.ModuleName, unattendedPrefix+" ") {
		return fmt.Errorf("no exact unattended module in state")
	}
	var module struct {
		workflow.Module
		Published  bool `json:"published"`
		ItemsCount int  `json:"items_count"`
	}
	e := p.c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d", courseID, p.state.ModuleID), &module)
	if e != nil {
		return e
	}
	if module.ID != p.state.ModuleID || module.Name != p.state.ModuleName {
		return fmt.Errorf("module no longer matches exact persisted fixture")
	}
	// List independently so Canvas's optional embedded items limit cannot hide
	// assignment membership. Keep titles, URLs, and other content out of output.
	var items []struct {
		ID        int64  `json:"id"`
		ModuleID  int64  `json:"module_id"`
		Type      string `json:"type"`
		ContentID int64  `json:"content_id"`
	}
	if e = p.c.List(ctx, fmt.Sprintf("/api/v1/courses/%d/modules/%d/items?per_page=100", courseID, p.state.ModuleID), &items); e != nil {
		return e
	}
	shapes := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item.ID <= 0 || item.ModuleID != p.state.ModuleID {
			return fmt.Errorf("module item no longer matches exact persisted fixture")
		}
		shapes = append(shapes, map[string]any{"id": item.ID, "type": item.Type, "content_id": item.ContentID})
	}
	return emit(map[string]any{"course_id": courseID, "module_id": module.ID, "name": module.Name, "published": module.Published, "items_count": module.ItemsCount, "items": shapes})
}
