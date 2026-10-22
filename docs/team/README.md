# Team teaching (1.2.0-beta.1 plan)

Date: 2026-10-22. Status: **slices 01–06 built in Dev, local tests only; slice 07 live checks pending.** Execution handoffs are in [handoffs/active/team-1.2b1/](../../handoffs/active/team-1.2b1/00-BRIEF.md). This plan does not authorize collecting real colleague keys, Canvas Live writes in a colleague's course, release, or UserInstall/UserSync adoption.

Read [AGENTS.md](../../AGENTS.md), [ARCHITECTURE.md](../ARCHITECTURE.md), [SCORING-RULES.md](../SCORING-RULES.md) and [PRIVACY.md](../PRIVACY.md) first.

## Result

A grade-level team (first case: ELA7) plans together. One teacher, **you** (the operator), runs CE-SE on a **team PC**, a laptop that can be passed around. **Teammates** never install CE-SE. Each teammate saves their Canvas key on the team PC once. From then on, conversation with the agent drives everything:

- **Post to Collab, then copy.** "Post this to Collab, then copy it into my classes and Turner's." CE-SE builds the module in the Collab course with your key. It then copies that module once into each named teacher's courses, using each teacher's own key. Jenkins and Postlewhaite are not named, so their courses are not touched.
- **Score with consent.** "Score Turner's Essay 3." This works only if Turner has put `(CE)` in that assignment's name. Scores and comments arrive **hidden**; Turner reviews them in SpeedGrader and clicks **Post grades**.
- **Everything else works too.** Every other feature (dates, pages, groups, rosters, contacts, parent reports, family messages) works in a teammate's course just as it does in yours. A teacher can sit at the team laptop and enter their own families' contact information. A single family-contact session can then cover the whole team.
- **Family replies go to the right teacher.** Family emails go out from the team PC's Outlook. When a student belongs to a teammate, the email's Reply-To is set to that teammate. It also carries a note asking families to reply to their child's actual teacher.

Teammates who prefer to work on their own keep doing so. You post to Collab, and they import from it themselves in Canvas. The automatic copy is a convenience on top of that.

## Settled decisions

Opus made these on 2026-10-22 from the developer's answers; they are final for 1.2b1. The [execution brief](../../handoffs/active/team-1.2b1/00-BRIEF.md) restates them with exact contracts.

| # | Decision |
|---|---|
| 1 | **Keys.** A teammate's Canvas key is saved in Windows Credential Manager on the team PC only, as `CanvasExpert-SE/Canvas/v1/teammate/<canvas_user_id>`. Your own key's entry is unchanged. Keys are never written to files, logs, page responses or agent context. |
| 2 | **One team PC.** On another PC, a teammate's courses say "Ms. Turner's Canvas key isn't on this PC." There is no key syncing. |
| 3 | **The team list is agent-readable.** `context/team.md` holds labels, display names, emails and key expiry dates, but no secrets. It follows the teacher-setup authority, so UserSync if configured. `context/team/<label>.md` holds each teammate's preferences in prose. |
| 4 | **Course owner.** A course entry in `context/courses.md` gets an optional `owner: <label>`. No owner means it's your course. |
| 5 | **Key choice.** For an owned course, CE-SE uses your key if you are an active teacher there (co-teaching), and otherwise the owner's key. One check per course per command. Co-teacher enrollment, if it arrives, takes over with no configuration change. |
| 6 | **Canvas guards against the wrong key.** A key whose owner isn't a teacher in a course is refused by Canvas. CE-SE adds no extra enrollment check before writes. It translates the refusal into a plain message naming the teammate and course. |
| 7 | **The teammate opts in to scoring.** In a course with an owner, CE-SE pulls work, scores and posts only on assignments whose name contains `(CE)` (any case). The tag is checked at pull and again right before the first write. This applies even when your own key is used. Your own courses never need the tag. |
| 8 | **Grades arrive hidden.** Before the first grade, comment, rubric or late-timing write on an assignment in an owned course, CE-SE sets the assignment to manual posting. A student whose submission is already visible is held, not written over. The teammate posts the grades in Canvas. |
| 9 | **Your review page stays the default** for scoring in teammates' courses. You check quality; the teammate decides what students see. |
| 10 | **Content needs only your request.** Copies, dates, pages, groups and edits in a teammate's course need only your request naming them; only scoring needs the tag. |
| 11 | **Copying from Collab.** The unit copied is one Collab module, matched by exact title, copied with Canvas's selective course copy using the destination's routed key. **Each module is copied at most once per course; CE-SE never re-copies.** The record is kept on the team PC, keyed by Canvas IDs, with one copy run at a time. A failed or uncertain copy is finished by hand in Canvas (Import Course Content). Dates and published state stay as they were in Collab. A Collab module containing `(CE)` in any item name is refused: consent can't arrive by copy. |
| 11a | **After-copy check (Engine).** Once a copy succeeds, the Engine reads the copied items and reports, without changing anything, which are published, which are assigned to no one and which still link to Collab. The team playbook turns that report into per-teacher follow-up. |
| 12 | **Fixing a copy.** Later fixes edit each course's copy through the ordinary edit paths, which already protect live edits. |
| 13 | **Family emails** for a teammate's student are sent from the team PC's Outlook. Reply-To is set to that teammate. A fixed note asks families to reply to their child's teacher. Messages for students in your own courses are unchanged. |
| 14 | **Action log.** Every Canvas write made with a teammate's key is logged to `.cse/team-log.jsonl` in UserInstall: time, teammate, course, method and redacted path. The log holds no student names or IDs. The Team page shows it. |
| 15 | **Expiry.** The Add-a-teammate page asks for the expiry date the teammate chose, defaulting to the last day of school. CE-SE warns from 14 days before that date. A key that stops working fails cleanly with a plain message. |
| 16 | **Scoring sessions.** "All current courses" means your own; a teammate's courses are pulled only when named. |
| 17 | **Version.** This work is 1.2.0-beta.1. |

## Status

| Slice | Handoff | Status | Local evidence | Canvas Live / human evidence |
|---|---|---|---|---|
| 01 Team keys, list and Team page | [01](../../handoffs/active/team-1.2b1/01-TEAM-KEYS.md) | Implemented in Dev | Team list validation, notes, action-log redaction, Team page add/courses/test/remove/expiry via seams; full tests and vet | — |
| 02 Per-course key routing | [02](../../handoffs/active/team-1.2b1/02-KEY-ROUTING.md) | Implemented in Dev | Token-aware fixture: owned-course commands reach Canvas only with the teammate key; post success path with redacted log; co-teaching, uncached transient check, missing key, unknown owner; full tests and vet | — |
| 03 Scoring consent and hidden grades | [03](../../handoffs/active/team-1.2b1/03-SCORING-CONSENT.md) | Implemented in Dev | Tag rule at pull/post; manual-policy mutation before any write, once, with one retry on uncertain; holds for untagged, unconfirmed policy and visible grades across grade, comment, late and group paths; probe-style and co-teaching variants; full tests and vet | — |
| 04 Copy a Collab module | [04](../../handoffs/active/team-1.2b1/04-COLLAB-COPY.md) | Implemented in Dev | Pre-flight refusals incl. (CE) items; copy-once for every prior status and after nickname rename; pending saved before POST, no retries; lock; per-destination keys and failures; compiled MCP smoke with 42 tools; full tests and vet | — |
| 05 Family reply-to | [05](../../handoffs/active/team-1.2b1/05-FAMILY-REPLY-TO.md) | Implemented in Dev | Reply routing cases; own-course emails byte-identical; note only in emails; grouped family previews; skipped hold retried by a later run; test send carries note and Reply-To; full tests and vet | — |
| 06 Playbooks, docs, capabilities | [06](../../handoffs/active/team-1.2b1/06-DOCS-AND-PLAYBOOKS.md) | Implemented in Dev | Seed `team.md` and routing sentences; developer docs, privacy and security notes; Team capabilities group; seed and capability tests, full tests and vet | — |
| 07 Live checks | [07](../../handoffs/active/team-1.2b1/07-LIVE-CHECKS.md) | Part A partly run (2026-10-22): A1, A2, A7 done; A3–A6 pending | — | A1: with `owner` set on CS8 to a team entry (`devcheck`, user ID +1) and the operator being the active teacher, `pull-assignment-dates` ran with no key error, so the operator key was routed. A2: pull of an untagged assignment was refused by the R3 message naming Dev Check before any submission read. A7 (Collab to CS8, 2026-10-22): the copy succeeded with no issues in about 210 s; the module and all three items arrived (Page, assignment, New Quiz) unpublished; the hidden file came along as a new hidden file and the Page's file link was rewritten to the CS8 copy; the assignment and the New Quiz both landed in a new assignment group "Imported Assignments"; a second copy to CS8 was refused citing the first; a (CE)-tagged Collab item was refused before any copy (module-item titles update with the assignment's title). Not run: A3 hidden grade, A4 already-visible hold, A5 New Quiz comment, A6 Reply-To; Parts B and C. A3-A5 need a quiet window because CS8 has 28 real students. |

Order: 01 → 02 → (03, 04 and 05 in any order, one at a time) → 06 → 07. Slice 06 lands its share with each slice. The live checks in 07 need the developer and the Test Student, and some need a willing teammate.

## Pending Canvas facts (checked in slice 07)

- Under manual posting, are both grades **and** comments hidden from Test Student? Does a re-grade of an already-posted submission show at once? This confirms or relaxes the hold in decision 8.
- Is a New Quiz attempt already posted at submit under automatic posting? (If so, teammates set manual posting before students start.) Does a comment-only write on an unposted attempt stay hidden?
- Does `GET /api/v1/courses/:id` return the caller's own active teacher enrollment, which decision 5 relies on?
- Can a teammate's key run a selective copy from Collab into their course? Which selective-data properties copy a module **with** its items?
- After a copy, where do assignments land among assignment groups and weights? Do New Quizzes and linked files arrive?
- Does Outlook's Reply-To appear on a saved draft?

## Not in scope for 1.2b1

- Syncing keys between PCs.
- CE-SE messaging teammates.
- Re-copying from Collab, or keeping copies in step with Collab.
- Blueprint sync.
- Canvas OAuth apps.
- Any role for teammates inside CE-SE beyond the Add-a-teammate page and the pages they already use on the team PC (students and contacts).
