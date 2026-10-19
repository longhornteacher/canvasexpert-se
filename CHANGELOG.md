# Changelog

Notable changes are recorded here. Version names use semantic versioning; **1.0** is `1.0.0` and **1.0 beta** was `1.0.0-beta.1`.

## Development notes

### New Quiz audiences from clubs (2026-10-19)

- A QuizForge file can carry the shared individual club audience in `metadata.canvas.audience`. The restriction targets the quiz's exact associated assignment, so the quiz stays one gradebook column and each student answers independently; a quiz whose assignment is native group work is refused. Push creates the quiz, adds items, applies settings and module placement, applies the audience and publishes last, so a refused or uncertain audience never publishes. The sidecar records accepted stages and a resume repeats only unfinished work; a sidecar bound to another Canvas or course is refused, and older sidecars keep working. Audience-only refresh returns an `assignment-dates` action and sends no quiz, item or settings write, while New Quiz attempts and grades protect removals. Local synthetic evidence only; Canvas Live club-audience quiz and student-view observations remain separate.

### Course Page audiences from clubs (2026-10-19)

- A course Page can carry the shared individual club audience. Push creates it unpublished, applies the complete date-details restriction, finishes requested module placement and publishes last; a refused or uncertain audience never publishes. Unrelated override rows and availability dates survive the collection replacement, `204` is accepted, and an audience-only update sends no stale body. Explicit refresh recomputes the managed student row while preserving manual rows and newer teacher edits. Local synthetic evidence only; Page/quiz student access observations remain separate.

### Explicit assignment audience refresh (2026-10-19)

- Agents can prepare and review a current-club audience refresh through the saved assignment receipt or latest action. The returned assignment-dates path preserves exact owned rows, manual exceptions, date semantics and original create receipts. Current/history work and grades protect access; safe independent changes still proceed with accurate partial results. Pending/uncertain writes are retained without replay. Local synthetic evidence only; Page/quiz authoring integration and student access observations remain separate.


### Individual book-club assignment audiences (2026-10-19)

- Dev can prepare a dated private-ref audience from selected collaborative clubs and create or initially target an individual assignment. Review shows recipients and preserved manual exceptions; empty, incomplete or changed snapshots hold publication. Student-only audiences need no artificial date, accepted/uncertain override writes are retained, and ordinary assignment-date updates preserve all overridden date values. Explicit refresh and Page/quiz audience integration follow in later slices. Local synthetic evidence only; no Canvas Live access check is claimed.


### Parent dates and private extra-time evidence (2026-10-18)

- Parent classification now preserves an explicit null student due date instead of falling back to the general assignment deadline. Completion/classification regressions cover null and empty dates, malformed dates, omitted-date fallback and individualized dates; the current-term/14-day reporting gate is unchanged.
- New individual and group scoring pulls include the saved private Extra time flag as true/false, using stable student identity and the active teacher-setup authority. Older absent fields remain unknown. The scoring/group playbooks resolve only established allowances to an absolute deadline, avoid double extensions already in Canvas and hold only timing when a flagged student has no documented rule. Local preparation checks cover calendar seconds, first-genuine attempts and preservation of Canvas's due-date guard. No duration is implied by the checkbox, and this does not edit Canvas deadlines. Packaging, installed playbook adoption and Canvas Live observation remain separate.

### School calendar and parent-report scope (2026-10-18)

- The school calendar now supports named inclusive grading periods and timezone-based current-period resolution. Late timing can be prepared from a first-genuine attempt decision, with school-time, skipped-date and current-Canvas timing provenance; `late-only` updates timing fields without changing grades or feedback. Local focused tests pass. Canvas Live timing writes and destination-client adoption remain pending.
- Parent reports now require a saved current-period scope and complete due-based inventory for the 14-day retrospective window, with Due Soon limited to the next 7 calendar days and clipped at the period boundary. Student-specific effective dates, explicit undated overrides, visibility evidence and unreleased-grade privacy are handled; all inventory rows appear in email and print. Empty windows are skipped, and legacy reports remain preserved but require a fresh pull before send or print. Focused local parent and calendar tests pass; Canvas Live, delivery and destination-client adoption remain pending.

### Parent-report classifications (1.1.0-beta.1)

- Parent reports resolve stale Canvas missing/no-submission indicators using positive teacher-entered grades before drafting. Current recorded zeros lead under Missing/0, retain grade history and never repeat under Corrections. The Engine computes skip eligibility after classification: every included course must meet the configured cutoff (90 by default), with no Missing/0 items. Playbook and context seeds carry these standing rules. Verification is local; no new Canvas Live or delivery claim.

### Collaborative group sets

- Dev can list collaborative group sets and clubs, pull an exact-ID baseline, and create or edit group-set settings and club names/descriptions. Canvas differentiation tags remain excluded. Local implementation checks do not establish live or student access behavior.
- Dev can read stable-ref membership rosters, pending self-sign-up requests, and unassigned students, save explicit add/move/remove plans, and request synchronous even allocation. Membership changes are per-student and conflict-checked; pending requests hold explicit placements, submitted-work moves/removals require specific authorization, accepted writes are never replayed, and uncertain writes reconcile from a fresh roster without blind retry. Successful allocation reports map refs from Canvas's response without a membership read-back. No CE-SE Canvas membership writes or allocations have been run.
- Dev can create and edit collaborative group Pages, pull exact group Page baselines, and read revision history. Group Pages use exact group routes and identity checks, default to teacher-only editing, and preserve student body edits on conflict. Local operation-path tests cover routing, scoped reads, publication ordering, and unsent body preservation; student access and editor behavior remain unobserved in Canvas Live.
- Dev can create ungraded course, distributed club, and direct group discussion topics, and explicitly requested announcements; `pull-discussion` saves editable topic baselines and returned distributed child IDs. Collaborative set/group identity is checked; announcements require recorded intent and audience. Settings-only changes preserve newer live topic content, while requested title/body replacement is held on conflict. Slice 05a clearly rejects linked graded discussions. Local checks only; no CE-SE Canvas discussion writes or student views were tested.
- Group-set, club and discussion authoring now send Canvas REST fields at the documented top level. Discussion edits hold without writing when a fresh pull shows the audience group set, linked assignment, or announcement identity changed, preserving the unsent draft and recording the live baseline. Strict local fixtures reject wrapped payloads; no CE-SE Canvas Live writes were run.
- Dev can pull a complete/partial discussion packet beside the existing editable `topic.md`, including verified distributed club children, full replies merged by source identity, stable-ref/alias attribution, deleted markers, and attachment status. `discussion-reply` creates an explicitly requested teacher entry/reply after exact topic and parent checks; accepted IDs/outcomes persist without read-back, while uncertain replies are never replayed. Local operation-path and compiled MCP checks only; no Canvas pulls or replies were run.
- Dev can add native course Discussion and titled external URL items to ordered modules. Group links are course-checked, equal-titled destinations remain distinct on resume, and `external-resources` reads paginated collaboration/conference metadata without exposing raw participant IDs or provisioning provider resources. Local checks only; no CE-SE module writes or metadata reads were run against Canvas Live.
- The UserInstall seed now includes a Phase 1 book-club workflow playbook covering only the shipped group sets/memberships, shared Pages, ungraded discussions, written evidence, teacher replies, module links and existing resource metadata. Graded group work and later optional group tools remain clearly out of scope.
- Phase 2 slice 08 adds group assignment authoring with explicit grading mode, group-aware snapshots and resume identity, guarded structural edits, plus exact per-row assignment override creation/edit/removal and effective-date pulls. Student IDs are mapped only through existing private refs; group scoring and shared submission pulls remain later slices. Focused local checks and MCP smoke fixtures cover the paths; no CE-SE Canvas Live writes or student access were tested.
- Assignment-date resume records accepted override-create IDs and visibility outcomes. Uncertain date and visibility writes remain unresolved across resumes and are not replayed; no Canvas Live writes were run.
- Phase 2 slice 09 adds typed shared-artifact submission pulls for group assignments, with per-member attempts and grades, effective dates, comments, rubric data, and deduplicated attachments that retain their source associations. Broad pulls preserve completion filtering; explicit group/assignment pulls remain available for reassessment. Local operation-path checks cover scope, member identity, attachment handling, and unsent-result preservation; Canvas Live pulls and student access remain untested.

## 1.0.0 (2026-10-17)

First stable release.

### Install and update from one link

- An agent can install or update CanvasExpert from the repository link: the README tells it to download the newest Windows ZIP from a permanent release link, extract it to a local folder, and run `cse.cmd playbooks update`.
- The ZIP no longer carries playbooks, `TEACHER.md` or teacher files. The Engine lays them down on first run, so extracting a newer ZIP over an existing folder never overwrites a teacher's settings or choices. With UserSync, update restores the local pointer files.
- `cse playbooks update` (MCP `update-playbooks`) keeps each shipped version under `shipped/`, replaces playbooks a teacher never edited (including older versions recognized by fingerprint), and keeps edited ones for the agent to reconcile.

### Teacher choices

- `Teacher_Override.md` holds a teacher's overrides of shipped defaults, read after the playbooks and never replaced. Standing preferences such as "always skip review when I ask to post" are saved there.
- Shipped defaults no longer include one district's grade-portal and tutoring links; teachers supply their own. The school calendar starter includes a bell schedule.

### Feedback and scoring

- Feedback leads with `Overall: X/Y` (marked `(late)` when late), then one line per item. New Quiz feedback never carries an overall line.
- Class exemplars: two anonymous excerpts per prompt, sent as a comment-only follow-up once the last matching student is scored. `comment-only` now works on ordinary assignments and never changes a grade.
- Parent updates list corrections as soon as work is finished, not only after feedback is posted.
- Rationales are required only for auto-scored quiz items.

### Verification

Comment layout, the lateness write and the comment-only follow-up were verified in Canvas Live on Test-Student-only CS8 assignments; install and update were verified locally and on the developer's own install. A colleague's install from the link has not yet been observed. See the [capability record](docs/engine-capabilities.md).

## 1.0.0-beta.1

This beta brings together the current Engine, teacher playbooks and portable Windows Install. This entry describes the release scope; it does not claim a public release date or complete client validation.

### Teacher-first installation

- Bundled selected-client installation for local Work/Codex or Cowork, without a separate Codex CLI.
- Private WebUI launch independent of destination restart, plus local MCP diagnostics that distinguish saved configuration from destination tool access.
- Configuration preservation, repeat-install behavior and an explicit choice before replacing another Install's connection.
- Thin Cowork extension launcher targeting Install's Engine rather than carrying a second Engine.
- Three-step onboarding, a short teaching-assistant starter and an optional teaching profile consulted by scoring and authoring.

### Teaching workflows

- Assignment creation/editing, rubric attachment, native Pages, course assets and Module assembly.
- QuizForge New Quiz authoring, including essay and file-upload items and differentiated support pages.
- Scoped work pulls, rubric/local-basis assessment, exact score arithmetic, review and ordinary assessment/feedback posting.
- New Quiz written-response review and comment-only feedback; item points remain teacher-entered in SpeedGrader.
- Authoritative editable teacher rules in Install or optional Sync, with preservation of existing preferences and unsent work.

### Beta limits

Fresh-user installation under a standard Windows account, destination-client adoption of the new routes and a teacher-judged lesson/feedback example remain pending. Selected Canvas operations have separate recorded live evidence; local tests do not prove student rendering, client compatibility or every district configuration. See the [capability record](docs/engine-capabilities.md) for current evidence and limitations.
