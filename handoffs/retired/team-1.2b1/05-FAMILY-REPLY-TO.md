# 05: Family emails reply to the actual teacher

Read [00-BRIEF.md](00-BRIEF.md) first. Contract R8 applies here. Requires slice 02 (ownership and the team list on `Service`). Suggested model: Sonnet, or a careful Haiku.

## Result

Family Contact messages and parent grade updates for a student in an **owned course** do two things:
- **Reply-To.** The Outlook Reply-To is set to that teammate's email.
- **Note.** The body carries a plain note, just before the sign-off, asking families to reply to their child's teacher.

The review page shows exactly that text. Emails about the operator's own courses are unchanged, byte for byte.

The parent-update grade guarantees and the `parent-updates.md` playbook are unchanged. This is delivery addressing plus one fixed note, not report content.

## Background (2026-10-22 survey)

**Outlook.**
- `internal/outlook/outlook.go:23-30` defines `Message{To, Cc, Subject, HTMLBody, TextBody, Attachments}`, checked by `validate` (:45-69).
- COM `compose` is at `outlook_windows.go:104-166`. It sets To and CC (around :132), then the body, then attachments, then `Recipients.ResolveAll`.
- `SelfAddress` is at :264-306.
- The non-Windows stub is `outlook_other.go`.
- Nothing sets `ReplyRecipients` today.

**Shared delivery.**
- `deliverFamilyMail` (`internal/workflow/parent_send.go:207-311`) sends one email per student. At :260-264 it calls `task.Render(st)`, then sets To and Cc.
- `familyMailTask` (:201-205) has no course information.

**Family Contact.**
- Rows are `FamilyAudienceRow{Ref, Alias, Course (nickname), Group, Message}` (`family_contact.go:49-55`).
- The renderer is `renderFamily` (`family_render.go:69-285`). The sign-off is appended at :109-111, then everything is sanitized at :168-247.
- The review is `buildFamilyReview` (`family_review.go:29-109`). It renders **one shared preview per message** with placeholders (:54-65), and shows per-student facts only.
- Sending is `sendFamilyContact` (`family_review.go:111-142`), which has `row.Course` at :136-139.

**Parent update.**
- One email per student can cover several courses: `ParentReportStudent.Courses[].Course`, nicknames (`parent_report.go:53-63`).
- The renderer is `ParentRun.render` (`parent_drafts.go:408-584`). The footer (portal link, sign-off, `aiNotice`) is at :572-583.
- The review card renders each student's actual email (`parent_send.go:152`).

## Work

### 1. Outlook Reply-To

- `outlook.Message` gains `ReplyTo []string`.
- `validate` checks each address with the same rule used for To and Cc.
- In `compose` (Windows), after CC is set:
  1. For each address, call `ReplyRecipients.Add(addr)`.
  2. Then call `ReplyRecipients.ResolveAll`, which must return true. Otherwise return an error naming "Reply-To".
  3. Release COM objects as the surrounding code does.
- Mirror the field in `outlook_other.go`. If the stub records messages for tests, record it there too.
- **Test:** `validate` rejects a malformed Reply-To. There is no COM unit test; the live check is in 07.

### 2. Reply routing helper (`internal/workflow/family_reply.go`)

```go
type replyRouting struct {
	ReplyTo []string // empty: operator-only, change nothing
	NoteHTML string  // empty: operator-only
	Hold string      // non-empty: do not send this student's email
}
func (s *Service) replyFor(courseNicknames []string, selfAddress func() (string, error)) replyRouting
```

The note never names the operator, so renderers call `replyFor(courses, nil)` and use only `NoteHTML` and `Hold`. With a nil `selfAddress`, skip the self-address step: `ReplyTo` may then be incomplete, and renderers ignore it. Delivery calls it again with the real `selfAddress` and uses `ReplyTo`.

- Look up each nickname's configured course and its owner (`s.Owned`). Unknown nicknames are treated as the operator's.
- **No owned courses:** return the zero value.
- **Any owner without an email:** `Hold = "<Name> has no email on the Team page, so replies couldn't be routed."`
- **One distinct owner, and no operator courses:**
  - `ReplyTo = [owner.Email]`;
  - note: `Please reply to <Name>, your child's teacher, at <email>. Replies to this email go to <Name>.`
- **Several owners, or a mix with operator courses:**
  - `ReplyTo` = the distinct owner emails, plus the operator's address from `selfAddress()` if any operator course is included. If `selfAddress` fails, set `Hold = "couldn't read your Outlook address to include you in replies."`
  - The note is one line per owned course: `For <course nickname>, please reply to <Name> at <email>.`
  - Operator courses get no line.
- **HTML.** The note is a single `<p>` with `<br>` between lines, with name, email and nickname HTML-escaped. Plain-text bodies get the same lines as text.

### 3. Renderers

**Family Contact (`renderFamily`).**
- It needs the row's course: thread it via the task/row.
- Insert `NoteHTML` immediately **before** the sign-off is appended (:109-111), so it passes through the same sanitizer (:168-247). Check that the sanitizer allows `<p>` and `<br>`; it does for normal content.
- With an empty note, the output is identical to today. Assert this with a test against a pre-change rendering.

**Parent update (`ParentRun.render`).**
- `render` is a `ParentRun` method with no `Service`, so pass the note in precomputed: compute `replyFor(student courses, nil).NoteHTML` where the run is rendered from a `Service` (`SendParentUpdates`, `SendParentTest`, `BuildParentReview`), and hand it to `render` as a parameter or a per-student map on the run.
- Insert it before the sign-off **in the email footer only** (:572-583). The printed student-sheet branch (`forStudent`, :565-567) never gets the note.
- With an empty note, the output is identical.

Both renderers compute the routing through `replyFor`, so preview, digest and send agree. If `team.md` or `courses.md` changes between review and send, the digest changes. The existing digest-mismatch handling then applies, which is correct; do not special-case it.

### 4. Delivery (`deliverFamilyMail`)

`familyMailTask` gains:

```go
Courses func(st) []string
```

It returns the student's course nicknames for this email: one for Family Contact (the row's course), and the student's report courses for a parent update.

At :260-264, compute `replyFor(...)`, passing `selfAddress` backed by the sender's `SelfAddress` (extend the `MailSender` interface if it doesn't expose it; `OutlookSender` already has `outlook.SelfAddress`). Then:
- if `Hold != ""`, record that student's delivery as `skipped` with the message, the same state as a student with no contact on file (:255), and send nothing for them. A later run sends it once the cause is fixed. If a later run doesn't retry `skipped`, stop and report;
- otherwise set `msg.ReplyTo`.

Independent students continue.

**`SendParentTest`** (`parent_send.go:336-366`) builds its own `outlook.Message` addressed to the teacher and does **not** go through `deliverFamilyMail`. Apply the same routing there:
- the rendered body carries the note (via the renderer);
- set `msg.ReplyTo` from `replyFor(courses, sender.SelfAddress)`.

The test email then shows exactly what a family would get, Reply-To included. That is the live check in 07.

### 5. Review pages

**Family Contact (`buildFamilyReview`).** Group each message's rows by reply routing (own, or per owner label / mixed key):
- Render **one shared preview per group** instead of one per message, labeled `Your students` / `Ms. Turner's students`.
- Each student card gets a fact `Replies go to: <ReplyTo joined>` when routed, or `Held: <Hold>`.
- Messages with only own-course rows look exactly as today.

**Parent update.** The card already renders the student's actual email, so the note appears automatically. Add the same `Replies go to` / `Held` fact.

The template (`templates/review.html:10,15-16`) may need a label for grouped previews. Keep the markup change minimal and the copy warm-plain.

### 6. Playbooks

No edit to `parent-updates.md` (AGENTS.md keeps it unchanged). Slice 06 adds one sentence about the automatic note to the team section of the workspace `AGENTS.md` and to `family-contact.md`.

## Tests

- `replyFor`, each case:
  - none;
  - one owner;
  - an owner with no email (hold);
  - two owners;
  - an owner plus an operator course (self address included);
  - `selfAddress` error (hold);
  - HTML escaping of a name containing `&`.
- Family render:
  - an owned row has the note before the sign-off;
  - an own row is byte-identical to the pre-change output.
- Parent render: the same two cases. Plus a student with an own course and a Turner course gets per-course lines.
- `deliverFamilyMail` with the fake sender:
  - `ReplyTo` is set for owned rows and empty for own rows;
  - a held student is not sent (recorded `skipped`) while others are.
- `SendParentTest` for an owned-course student: the fake sender receives the note and `ReplyTo`.
- A printed student report sheet for an owned-course student has no note.
- Family review:
  - grouped previews (own plus Turner) with the right labels and facts;
  - an own-only message renders as before.
- Outlook `validate` with a bad Reply-To.

## Acceptance

All of the above pass, plus full tests, vet and the diff check. CHANGELOG `### Family replies go to the actual teacher`, ending: "Local tests only; Reply-To on a real Outlook draft is pending live check (07). No family email was sent."

## Out of scope

- Sending from a teammate's own Outlook account.
- SentOnBehalfOf.
- Per-teammate signatures.
- Changing who appears in To and Cc.
