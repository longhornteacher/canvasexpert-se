# 17 — Integrated book-club workflow and summaries

Requires: grows with the phases. **Playbook v1** is written after Phase 1 (slices 01, 02, 03, 05a, 06, 16) and covers only those; each later slice adds its own section when it lands (shared product after 10/11, discussion assessment after 07, peer critique/calendar/messaging only if built). The text below describes the finished playbook. Result: a teacher can request a complete unit and ongoing help in classroom language without becoming the workflow coordinator.

## Seed and ownership

Create `workspace-template/playbooks/group-work.md`; update teacher AGENTS/CLAUDE routing and relevant authoring/scoring/native-content references to point to it. Read `TEACHER.md`, `Teacher_Override.md`, teaching profile, grading preferences, current courses and task context before asking questions. Reusable group-work preferences belong in the current teacher-setup authority; unit plans, drafts, evidence and outcomes stay local in UserInstall.

Use the actual supported tools/commands shipped by earlier slices, not the proposed names before implementation. The playbook must distinguish supported paths, specific held actions, and external provider steps. Pending student-view/live evidence does not require a disposable-course trial before routine supported teaching.

## Unit planning and assembly

The agent records an ordinary local `unit.md` with the teacher's learning targets, books, chosen group set, club IDs/labels, meeting/read schedule, role rotation, shared/individual deliverables, grading mode, peer-review plan, and links to the actual authoring/result files and returned Canvas IDs. This is a useful task document, not a new Engine `kind`, dependency executor, synchronization manifest, or universal workflow framework.

Ask only missing consequential choices: course/books, grouping or self-selection, club constraints, schedule, whether a shared product uses shared or individual grades, and whether communications should be sent. Infer routine style from saved choices. Reuse existing clubs only after checking identity and teacher intent. Multiple clubs can read the same book; a book title is not a unique group identifier.

Build reusable student content: course unit hub, club reading plan, rotating-role directions, member-editable evidence notebook, weekly discussions, shared product with rubric, individual preparation/reflections, and optional peer critique. Use the existing classroom standards (learning target, clear directions, proficient-grade rigor and exemplars) without imposing a new points policy. Existing defaults apply unless the teacher changes them.

Roles such as facilitator, evidence finder, connector, and summarizer are instructional content with teacher-chosen rotation. They do not automatically grant Canvas group leadership/moderator permissions. Never publish private tier tags, accommodations, student notes, or balancing rationales in rosters or role sheets. Shared group resources and private differentiation supports can coexist.

Resume using saved returned IDs/outcomes. If a notebook is already populated, preserve it; if one group's page fails, complete independent groups and report that page specifically. Publishing the course module does not publish all its children or prove student group access. Do not send announcements, Inbox reminders, or teacher replies merely to announce setup completion.

## Ongoing teacher support

Pull fresh discussion/group-assignment/peer-review evidence only for requested clubs/time/assignments. Produce a teacher-only summary with:

- source links and as-of time, including incomplete reads;
- actual written participation, substantive claims/evidence and questions worth revisiting;
- missing preparation/review obligations only when complete relevant evidence supports the statement;
- next instructional moves tied to observed needs, clearly presented as suggestions.

Do not grade from post counts, claim to hear in-person discussion, infer Google document contribution from timestamps, or describe an inaccessible student as disengaged. Keep teacher-only concerns separate from any proposed student communication. Summary generation never posts grades/messages by itself.

Scoring follows the appropriate earlier slice: individual discussion, shared group product, individual group product, or ordinary individual reflection. Preserve requested audience and default score review. If a teacher selects one student on a shared-grade assignment, explain the propagated scope and hold that write rather than expanding it. Continue independent valid assessment.

## Integrated acceptance

Use a focused fake-Canvas scenario with two clubs/books, multiple members, an unassigned student, group notebooks, one distributed graded discussion, one shared product and individual reflections. Verify actual output documents and routed requests, not the presence of phrases in the playbook.

- Setup stops only for a necessary missing choice or private action; saved course/style/grouping choices are reused.
- Partial page/topic setup resumes without duplicates and preserves student-added notebook text.
- One club's incomplete thread read does not block another club's summary or assessment.
- Shared grading, individual grading, private feedback and an explicitly requested club message keep their distinct audiences.
- Individual reflections and New Quiz reading checks keep current Engine behavior; unavailable optional calendar/conference features are reported without blocking the rest.

Inspect review rendering for full group audiences and private-comment separation using existing synthetic browser methods when those views change. Authorized live observations should separately cover actual group access, shared/individual grade and rubric propagation, comment visibility, discussion placement, and any requested bookings/messages. Record what was observed and leave other evidence pending; do not require an exhaustive certification or introduce real student data into Dev fixtures.

Finish by updating the capability evidence with the actual supported scope and giving the teacher the relevant Canvas links and any specific remaining action. Do not report completion of a feature merely because a provider metadata record exists or a local test passed.
