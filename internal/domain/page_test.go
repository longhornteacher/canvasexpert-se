package domain

import "testing"

func TestGroupPageValidationRejectsCourseModuleAndScopeMismatch(t *testing.T) {
	roles := []string{"members"}
	create := PageDocument{Schema: 1, Kind: "page", Operation: "create", Course: "ELA7", GroupID: 701, Title: "Notebook", BodyFormat: "html", EditingRoles: &roles, ModuleID: 8}
	if err := create.ValidateBody("<p>Notebook</p>"); err == nil {
		t.Fatal("group Page accepted course module placement")
	}
	create.ModuleID = 0
	create.GroupID = 0
	if err := create.ValidateBody("<p>Notebook</p>"); err == nil {
		t.Fatal("course Page accepted members editing without group scope")
	}
	baseline := PageSnapshot{Instance: "https://canvas.example", CourseID: 1, GroupID: 701, PageID: 3, Slug: "notebook", Title: "Notebook"}
	update := PageDocument{Schema: 1, Kind: "page", Operation: "update", Course: "ELA7", Instance: baseline.Instance, CourseID: baseline.CourseID, GroupID: 702, PageID: baseline.PageID, Title: baseline.Title, BodyFormat: "html", Observed: &baseline}
	if err := update.ValidateBody("<p>Notebook</p>"); err == nil {
		t.Fatal("group Page update accepted a different group than its baseline")
	}
}
