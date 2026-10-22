package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/outlook"
	"canvasexpertse/internal/workspace"
)

func replyService(courses ...domain.Course) *Service {
	return &Service{
		Config: domain.CourseConfig{Courses: append([]domain.Course{{Nickname: "mine", ID: 1}}, courses...)},
		Team: workspace.TeamConfig{Schema: 1, Teammates: []workspace.Teammate{
			{Label: "turner", Name: "Ms. Turner", Email: "turner@school.org"},
			{Label: "lee", Name: "Mr. Lee & Co", Email: "lee@school.org"},
			{Label: "noemail", Name: "Ms. Quiet"},
		}},
	}
}

var (
	turnerCourse  = domain.Course{Nickname: "Turner P2", ID: 2, Owner: "turner"}
	leeCourse     = domain.Course{Nickname: "Lee P1", ID: 3, Owner: "lee"}
	noEmailCourse = domain.Course{Nickname: "Quiet P4", ID: 4, Owner: "noemail"}
)

func TestReplyForCases(t *testing.T) {
	s := replyService(turnerCourse, leeCourse, noEmailCourse)
	self := func() (string, error) { return "me@school.org", nil }

	if r := s.replyFor([]string{"mine", "unknown"}, self); len(r.ReplyTo) != 0 || r.NoteHTML != "" || r.Hold != "" {
		t.Fatalf("own courses must change nothing: %+v", r)
	}
	r := s.replyFor([]string{"Turner P2"}, nil)
	if strings.Join(r.ReplyTo, ",") != "turner@school.org" || r.NoteHTML != "<p>Please reply to Ms. Turner, your child&#39;s teacher, at turner@school.org. Replies to this email go to Ms. Turner.</p>" {
		t.Fatalf("one owner: %+v", r)
	}
	if r := s.replyFor([]string{"Quiet P4"}, nil); r.Hold != "Ms. Quiet has no email on the Team page, so replies couldn't be routed." || r.NoteHTML != "" {
		t.Fatalf("no email: %+v", r)
	}
	r = s.replyFor([]string{"Turner P2", "Lee P1"}, nil)
	if strings.Join(r.ReplyTo, ",") != "turner@school.org,lee@school.org" || r.NoteHTML != "<p>For Turner P2, please reply to Ms. Turner at turner@school.org.<br>For Lee P1, please reply to Mr. Lee &amp; Co at lee@school.org.</p>" {
		t.Fatalf("two owners: %+v", r)
	}
	r = s.replyFor([]string{"mine", "Turner P2"}, self)
	if strings.Join(r.ReplyTo, ",") != "turner@school.org,me@school.org" || r.NoteHTML != "<p>For Turner P2, please reply to Ms. Turner at turner@school.org.</p>" {
		t.Fatalf("owner plus own course: %+v", r)
	}
	if r := s.replyFor([]string{"mine", "Turner P2"}, func() (string, error) { return "", errors.New("no outlook") }); r.Hold != "couldn't read your Outlook address to include you in replies." {
		t.Fatalf("self address error: %+v", r)
	}
	if r := s.replyFor([]string{"Lee P1"}, nil); !strings.Contains(r.NoteHTML, "Mr. Lee &amp; Co") {
		t.Fatalf("name not escaped: %s", r.NoteHTML)
	}
}

func TestFamilyRenderNoteBeforeSignOffAndOwnRowUnchanged(t *testing.T) {
	dir := familyRenderAssets(t)
	s := replyService(turnerCourse)
	msg := FamilyMessage{Body: "Hello there"}
	before, err := RenderFamilyMessage(dir, msg, "Ada", "", "Ms. Teacher")
	if err != nil {
		t.Fatal(err)
	}
	own, err := RenderFamilyMessageNote(dir, msg, "Ada", "", "Ms. Teacher", s.replyFor([]string{"mine"}, nil).NoteHTML)
	if err != nil {
		t.Fatal(err)
	}
	if own.HTML != before.HTML || own.PreviewHTML != before.PreviewHTML || own.Digest != before.Digest {
		t.Fatal("an own-course email changed")
	}
	owned, err := RenderFamilyMessageNote(dir, msg, "Ada", "", "Ms. Teacher", s.replyFor([]string{"Turner P2"}, nil).NoteHTML)
	if err != nil {
		t.Fatal(err)
	}
	note, sign := strings.Index(owned.HTML, "Please reply to Ms. Turner"), strings.Index(owned.HTML, "Ms. Teacher")
	if note < 0 || sign < 0 || note > sign {
		t.Fatalf("note must come before the sign-off: %s", owned.HTML)
	}
}

func TestParentRenderNoteInEmailOnlyAndOwnUnchanged(t *testing.T) {
	svc, dir, a, _ := parentFixtureRun(t)
	run, err := LoadParentRun(svc.Root, dir)
	if err != nil {
		t.Fatal(err)
	}
	student, sd := workspace.Student{Name: "Ada Lovelace", FirstName: "Ada"}, run.Drafts.Students[0]
	before := run.Render(sd, student)

	svc.Config.Courses = []domain.Course{{Nickname: "cs8", ID: 1}}
	svc.Team = workspace.TeamConfig{Schema: 1}
	svc.applyReplyNotes(run)
	if run.Render(sd, student).HTML != before.HTML {
		t.Fatal("an own-course email changed")
	}

	svc.Config.Courses = []domain.Course{{Nickname: "cs8", ID: 1, Owner: "turner"}}
	svc.Team = workspace.TeamConfig{Schema: 1, Teammates: []workspace.Teammate{{Label: "turner", Name: "Ms. Turner", Email: "turner@school.org"}}}
	svc.applyReplyNotes(run)
	email, sheet := run.Render(sd, student).HTML, run.RenderStudent(sd, student).HTML
	i := strings.Index(email, "Please reply to Ms. Turner")
	if i < 0 || strings.Index(email, "prepared with AI tools") < i {
		t.Fatalf("email note misplaced: %s", email)
	}
	if strings.Contains(sheet, "reply to") {
		t.Fatalf("printed sheet must not carry the note: %s", sheet)
	}

	// A student with an own course and a Turner course gets per-course lines.
	rs := run.students[a.Ref]
	rs.Courses = append(rs.Courses, rs.Courses[0])
	rs.Courses[1].Course = "mine"
	run.students[a.Ref] = rs
	svc.Config.Courses = append(svc.Config.Courses, domain.Course{Nickname: "mine", ID: 9})
	svc.applyReplyNotes(run)
	mixed := run.Render(sd, student).HTML
	if !strings.Contains(mixed, "For cs8, please reply to Ms. Turner at turner@school.org.") || strings.Contains(mixed, "For mine") {
		t.Fatalf("mixed note: %s", mixed)
	}
}

func TestDeliverFamilyMailReplyToAndHeldStudentRetried(t *testing.T) {
	svc, dir, a, b := parentFixtureRun(t)
	contacts, _ := workspace.OpenContacts(svc.Root)
	contacts.Students[workspace.ContactKey(b)] = workspace.StudentContacts{StudentEmail: "bo@student.example", Parents: []workspace.ParentContact{{Email: "bp@example.org", EmailEnabled: true}}}
	if err := contacts.Save(); err != nil {
		t.Fatal(err)
	}
	svc.Config.Courses = []domain.Course{{Nickname: "mine", ID: 1}, {Nickname: "Quiet P4", ID: 4, Owner: "noemail"}}
	svc.Team = workspace.TeamConfig{Schema: 1, Teammates: []workspace.Teammate{{Label: "noemail", Name: "Ms. Quiet"}}}
	render := func(workspace.Student) (outlook.Message, error) {
		return outlook.Message{Subject: "S", HTMLBody: "<p>x</p>"}, nil
	}
	tasks := []familyMailTask{
		{Ref: a.Ref, Send: true, Render: render, Courses: func(workspace.Student) []string { return []string{"mine"} }},
		{Ref: b.Ref, Send: true, Render: render, Courses: func(workspace.Student) []string { return []string{"Quiet P4"} }},
	}
	f := &fakeSender{}
	status, summary, err := svc.deliverFamilyMail(context.Background(), dir, tasks, false, nil, false, f)
	if err != nil || status != "partial" || len(f.sent) != 1 || len(f.sent[0].ReplyTo) != 0 {
		t.Fatalf("status=%s sent=%d err=%v: own row sends with no Reply-To, held student does not", status, len(f.sent), err)
	}
	if !strings.Contains(summary, "Ms. Quiet has no email on the Team page") {
		t.Fatalf("summary: %s", summary)
	}
	out, _ := LoadParentOutcomes(dir)
	if d := out.Students[b.Ref]; d.State != "skipped" || d.Reason == "" {
		t.Fatalf("held student must be recorded skipped: %+v", d)
	}

	// Once the email is added, a later run sends the skipped student with Reply-To.
	svc.Team.Teammates[0].Email = "quiet@school.org"
	if _, _, err = svc.deliverFamilyMail(context.Background(), dir, tasks, false, nil, false, f); err != nil {
		t.Fatal(err)
	}
	if len(f.sent) != 2 || strings.Join(f.sent[1].ReplyTo, ",") != "quiet@school.org" {
		t.Fatalf("skipped student was not retried with Reply-To: %+v", f.sent)
	}
}

func TestSendParentTestCarriesNoteAndReplyTo(t *testing.T) {
	svc, dir, _, _ := parentFixtureRun(t)
	svc.Config.Courses = []domain.Course{{Nickname: "cs8", ID: 1, Owner: "turner"}}
	svc.Team = workspace.TeamConfig{Schema: 1, Teammates: []workspace.Teammate{{Label: "turner", Name: "Ms. Turner", Email: "turner@school.org"}}}
	f := &fakeSender{}
	if _, err := svc.SendParentTest(dir, f); err != nil {
		t.Fatal(err)
	}
	m := f.sent[0]
	if strings.Join(m.To, ",") != "teacher@example.org" || strings.Join(m.ReplyTo, ",") != "turner@school.org" || !strings.Contains(m.HTMLBody, "Please reply to Ms. Turner") {
		t.Fatalf("test email must show what a family gets: %+v", m)
	}
}

func TestFamilyReviewGroupsPreviewsByReplyRouting(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	familyReviewContacts(t, f)
	prepareFamilyReview(t, f)
	own, err := f.service.BuildParentReview(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, fr := range own.Frames {
		if strings.Contains(fr.Label, "Your students") {
			t.Fatal("own-only message must be labeled as before")
		}
	}
	f.service.Config.Courses[1].Owner = "turner"
	f.service.Team = workspace.TeamConfig{Schema: 1, Teammates: []workspace.Teammate{{Label: "turner", Name: "Ms. Turner", Email: "turner@school.org"}}}
	model, err := f.service.BuildParentReview(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Frames) <= len(own.Frames) {
		t.Fatalf("expected a preview per routing: %d vs %d", len(model.Frames), len(own.Frames))
	}
	var yours, turners int
	for _, fr := range model.Frames {
		switch {
		case strings.Contains(fr.Label, "Your students") && !strings.Contains(fr.HTML, "reply to"):
			yours++
		case strings.Contains(fr.Label, "Ms. Turner's students") && strings.Contains(fr.HTML, "Please reply to Ms. Turner"):
			turners++
		}
	}
	if yours == 0 || turners == 0 {
		t.Fatalf("labels/notes wrong: %+v", model.Frames)
	}
	var routed, plain int
	for _, c := range model.Cards {
		switch familyReviewFacts(c)["Replies go to"] {
		case "turner@school.org":
			routed++
		case "":
			plain++
		}
	}
	if routed == 0 || plain == 0 {
		t.Fatalf("cards: routed=%d plain=%d", routed, plain)
	}
}
