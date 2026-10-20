package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"canvasexpertse/internal/outlook"
	"canvasexpertse/internal/workspace"
)

func familyReviewContacts(t *testing.T, f *familyAudienceFixture) {
	t.Helper()
	store, err := workspace.OpenStudents(f.service.Root)
	if err != nil {
		t.Fatal(err)
	}
	contacts, err := workspace.OpenContacts(f.service.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, student := range store.Rows {
		contacts.Students[workspace.ContactKey(student)] = workspace.StudentContacts{
			StudentEmail: fmt.Sprintf("student-%d@example.invalid", student.UserID),
			Parents:      []workspace.ParentContact{{Email: fmt.Sprintf("parent-%d@example.invalid", student.UserID), EmailEnabled: true}, {Email: "disabled@example.invalid", EmailEnabled: false}},
		}
	}
	if err = contacts.Save(); err != nil {
		t.Fatal(err)
	}
}

func prepareFamilyReview(t *testing.T, f *familyAudienceFixture) {
	t.Helper()
	if _, err := f.service.PrepareFamilyContact(context.Background(), f.dir); err != nil {
		t.Fatal(err)
	}
}

func familyReviewFacts(card ReviewCard) map[string]string {
	facts := map[string]string{}
	for _, fact := range card.Facts {
		facts[fact.Label] = fact.Value
	}
	return facts
}

func familyReviewSkipExcept(f *familyAudienceFixture, keep string) map[string]bool {
	skip := map[string]bool{}
	for _, ref := range f.refs {
		skip[ref] = ref != keep
	}
	return skip
}

func TestFamilyReviewSharedPreviewsPrivateRecipientsAndMissingParentHold(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	familyReviewContacts(t, f)
	store, err := workspace.OpenStudents(f.service.Root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := store.Resolve(f.service.Client.Origin(), f.refs[100])
	if err != nil {
		t.Fatal(err)
	}
	contacts, err := workspace.OpenContacts(f.service.Root)
	if err != nil {
		t.Fatal(err)
	}
	contact := contacts.Students[workspace.ContactKey(student)]
	contact.Parents[0].EmailEnabled = false
	contacts.Students[workspace.ContactKey(student)] = contact
	if err = contacts.Save(); err != nil {
		t.Fatal(err)
	}
	prepareFamilyReview(t, f)
	// Viewing the saved review does not depend on another Canvas round trip.
	f.privateError = true
	model, err := f.service.BuildParentReview(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Frames) != 5 || len(model.Cards) != 11 || !model.Email || model.Test {
		t.Fatalf("wrong shared review shape: %+v", model)
	}
	if ParentReviewSource(f.dir) != filepath.Join(f.dir, "contact.json") {
		t.Fatal("family review source did not select contact.json")
	}
	for _, frame := range model.Frames {
		if frame.HTML == "" || !strings.Contains(frame.Label, "book-") || !strings.Contains(frame.Label, "students") {
			t.Fatalf("incomplete message preview: %+v", frame)
		}
	}
	seen := map[string]bool{}
	for _, card := range model.Cards {
		if seen[card.Ref] || len(card.Frames) != 0 {
			t.Fatalf("student duplicated/shared preview repeated on card: %+v", card)
		}
		seen[card.Ref] = true
		facts := familyReviewFacts(card)
		if card.Ref == f.refs[100] {
			if !strings.Contains(card.Hold, "No enabled parent") {
				t.Fatalf("missing parent was not held: %+v", card)
			}
		} else if facts["To"] == "" || facts["Cc"] == "" || card.Hold != "" {
			t.Fatalf("private recipient lines missing or unexpectedly held: %+v", card)
		}
		if strings.Contains(facts["Cc"], "disabled@example.invalid") {
			t.Fatal("disabled parent shown as recipient")
		}
	}
}

func TestFamilyReviewDraftMediaSkipIdempotencyAndAcceptedSend(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	familyReviewContacts(t, f)
	source := familyRenderAssets(t)
	for _, name := range []string{"cover.png", "schedule.txt"} {
		data, err := os.ReadFile(filepath.Join(source, "assets", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = workspace.WriteFile(filepath.Join(f.dir, "assets", name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for i := range f.doc.Messages {
		f.doc.Messages[i].Body += "\n\n![Book cover](assets/cover.png)"
		f.doc.Messages[i].Attachments = []string{"assets/schedule.txt"}
	}
	f.save(t)
	prepareFamilyReview(t, f)
	model, err := f.service.BuildParentReview(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range model.Frames {
		if !strings.Contains(frame.HTML, "data:image/png;base64,") {
			t.Fatal("shared preview does not show inline image")
		}
		if len(frame.Files) != 1 || frame.Files[0] != "schedule.txt" {
			t.Fatalf("attached file is not visible for review: %+v", frame.Files)
		}
	}
	sender := &fakeSender{}
	status, summary, err := f.service.SendParentUpdates(context.Background(), f.dir, map[string]bool{f.refs[100]: true}, true, sender)
	if err != nil || status != "completed" || len(sender.drafts) != 10 || len(sender.sent) != 0 {
		t.Fatalf("draft dispatch: status=%s drafts=%d sent=%d err=%v", status, len(sender.drafts), len(sender.sent), err)
	}
	if strings.Contains(summary, "@") {
		t.Fatalf("summary leaked an address: %s", summary)
	}
	seen := map[string]bool{}
	bookNames := []string{"Ghost", "MexiKid", "Book Three", "Book Four", "Book Five"}
	for _, message := range sender.drafts {
		if len(message.To) != 1 || len(message.Cc) != 1 || seen[message.To[0]] {
			t.Fatalf("duplicate/malformed recipients: %+v", message)
		}
		seen[message.To[0]] = true
		if len(message.Attachments) != 2 {
			t.Fatalf("inline image/file missing: %+v", message.Attachments)
		}
		var inline, regular int
		for _, attachment := range message.Attachments {
			if attachment.ContentID != "" {
				inline++
				if !strings.Contains(message.HTMLBody, `src="cid:`+attachment.ContentID+`"`) {
					t.Fatal("outgoing image CID does not match attachment")
				}
			} else {
				regular++
			}
		}
		if inline != 1 || regular != 1 {
			t.Fatalf("wrong media kinds: %+v", message.Attachments)
		}
		id, parseErr := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(message.To[0], "student-"), "@example.invalid"), 10, 64)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		index := id % 100
		if id == 300 {
			index = 0
		}
		if !strings.Contains(message.HTMLBody, bookNames[index]) || strings.Contains(message.HTMLBody, "{{student_name}}") || strings.Contains(message.HTMLBody, "{{group_name}}") {
			t.Fatalf("wrong per-student message substitution: %s", message.HTMLBody)
		}
	}
	out, err := LoadParentOutcomes(f.dir)
	if err != nil || out.Students[f.refs[100]].State != "skipped" {
		t.Fatalf("skip not persisted: %+v %v", out, err)
	}
	if _, _, err = f.service.SendParentUpdates(context.Background(), f.dir, map[string]bool{f.refs[100]: true}, true, sender); err != nil || len(sender.drafts) != 10 {
		t.Fatalf("accepted drafts repeated: %d %v", len(sender.drafts), err)
	}
	// A teacher can later include the skipped student; accepted drafts stay put.
	if _, _, err = f.service.SendParentUpdates(context.Background(), f.dir, nil, true, sender); err != nil || len(sender.drafts) != 11 {
		t.Fatalf("skipped student did not get exactly one draft: %d %v", len(sender.drafts), err)
	}
	if _, _, err = f.service.SendParentUpdates(context.Background(), f.dir, nil, false, sender); err != nil || len(sender.sent) != 11 {
		t.Fatalf("send did not create one fresh mail per student: %d %v", len(sender.sent), err)
	}
	if _, _, err = f.service.SendParentUpdates(context.Background(), f.dir, nil, false, sender); err != nil || len(sender.sent) != 11 {
		t.Fatalf("accepted send repeated: %d %v", len(sender.sent), err)
	}
	model, err = f.service.BuildParentReview(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range model.Cards {
		if !card.Done {
			t.Fatalf("accepted send not marked done: %+v", card)
		}
	}
	raw, err := os.ReadFile(filepath.Join(f.dir, "outcomes.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "@") || strings.Contains(string(raw), "Private") {
		t.Fatalf("outcomes leaked private content: %s", raw)
	}
}

func TestFamilyReviewUncertainSendHeldAndMembershipDriftStopsAllMail(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	familyReviewContacts(t, f)
	prepareFamilyReview(t, f)
	sender := &fakeSender{next: outlook.Uncertain, err: errors.New("transport lost after send")}
	skip := familyReviewSkipExcept(f, f.refs[100])
	status, _, err := f.service.SendParentUpdates(context.Background(), f.dir, skip, false, sender)
	if err != nil || status != "partial" || len(sender.sent) != 1 {
		t.Fatalf("uncertain dispatch: %s %d %v", status, len(sender.sent), err)
	}
	if _, _, err = f.service.SendParentUpdates(context.Background(), f.dir, skip, false, sender); err != nil || len(sender.sent) != 1 {
		t.Fatalf("uncertain send repeated: %d %v", len(sender.sent), err)
	}
	model, err := f.service.BuildParentReview(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range model.Cards {
		if card.Ref == f.refs[100] && !strings.Contains(card.Hold, "Sent Items") {
			t.Fatalf("uncertain review hold missing: %+v", card)
		}
	}
	// Another student's membership changes after preparation: no remaining mail.
	f.members[61] = nil
	newSender := &fakeSender{}
	status, _, err = f.service.SendParentUpdates(context.Background(), f.dir, nil, false, newSender)
	if err == nil || status != "held" || len(newSender.sent) != 0 || len(newSender.drafts) != 0 {
		t.Fatalf("drift did not hold all delivery: %s %d %v", status, len(newSender.sent), err)
	}
}

func TestFamilyReviewParentsOnlyKeepsParentWhoseAddressEqualsStudent(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	familyReviewContacts(t, f)
	f.doc.Recipients = "parents_only"
	f.save(t)
	store, err := workspace.OpenStudents(f.service.Root)
	if err != nil {
		t.Fatal(err)
	}
	contacts, err := workspace.OpenContacts(f.service.Root)
	if err != nil {
		t.Fatal(err)
	}
	student, err := store.Resolve(f.service.Client.Origin(), f.refs[100])
	if err != nil {
		t.Fatal(err)
	}
	key := workspace.ContactKey(student)
	contact := contacts.Students[key]
	contact.Parents[0].Email = contact.StudentEmail
	contacts.Students[key] = contact
	if err = contacts.Save(); err != nil {
		t.Fatal(err)
	}
	prepareFamilyReview(t, f)
	sender := &fakeSender{}
	if _, _, err = f.service.SendParentUpdates(context.Background(), f.dir, nil, true, sender); err != nil {
		t.Fatal(err)
	}
	if len(sender.drafts) != 11 {
		t.Fatalf("parents-only count: %d", len(sender.drafts))
	}
	seenSame := false
	for _, message := range sender.drafts {
		if len(message.Cc) != 0 || len(message.To) != 1 {
			t.Fatalf("parents-only recipient lines: %+v", message)
		}
		if message.To[0] == "student-100@example.invalid" {
			seenSame = true
			continue
		}
		if strings.Contains(message.To[0], "student-") || !strings.Contains(message.To[0], "parent-") {
			t.Fatalf("student retained in parents-only delivery: %+v", message.To)
		}
	}
	if !seenSame {
		t.Fatal("parent sharing the student's address was silently excluded")
	}
}

type familyOutcomeSaveFailureSender struct {
	fakeSender
	dir         string
	backup      string
	mutationErr error
}

func (sender *familyOutcomeSaveFailureSender) Send(message outlook.Message) (outlook.Outcome, error) {
	sender.sent = append(sender.sent, message)
	source := filepath.Join(sender.dir, "outcomes.json")
	sender.backup = filepath.Join(sender.dir, "pending-outcomes.json")
	// Preserve the real pre-delivery uncertain checkpoint, then make only the
	// accepted-outcome replacement fail. Restore that checkpoint after return.
	if sender.mutationErr = os.Rename(source, sender.backup); sender.mutationErr == nil {
		sender.mutationErr = os.Mkdir(source, 0700)
	}
	return outlook.Sent, nil
}

func TestFamilyReviewAcceptedSendOutcomeSaveFailureCannotReplay(t *testing.T) {
	f := newFamilyAudienceFixture(t)
	familyReviewContacts(t, f)
	prepareFamilyReview(t, f)
	sender := &familyOutcomeSaveFailureSender{dir: f.dir}
	status, _, err := f.service.SendParentUpdates(context.Background(), f.dir, nil, false, sender)
	if sender.mutationErr != nil {
		t.Fatal(sender.mutationErr)
	}
	if err == nil || status != "partial" || len(sender.sent) != 1 {
		t.Fatalf("outcome persistence interruption: %s %d %v", status, len(sender.sent), err)
	}
	if err = os.Remove(filepath.Join(f.dir, "outcomes.json")); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(sender.backup, filepath.Join(f.dir, "outcomes.json")); err != nil {
		t.Fatal(err)
	}
	out, err := LoadParentOutcomes(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	var uncertainRef string
	for ref, delivery := range out.Students {
		if delivery.State == "uncertain" {
			uncertainRef = ref
		}
	}
	if uncertainRef == "" {
		t.Fatal("no pre-send uncertain checkpoint survived")
	}
	retry := &fakeSender{}
	if _, _, err = f.service.SendParentUpdates(context.Background(), f.dir, familyReviewSkipExcept(f, uncertainRef), false, retry); err != nil || len(retry.sent) != 0 {
		t.Fatalf("accepted mail replayed after save failure: %d %v", len(retry.sent), err)
	}
}
