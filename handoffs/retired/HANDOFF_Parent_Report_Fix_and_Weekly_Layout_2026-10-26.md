> **Retired 2026-10-28 — implemented in Dev.** Historical instructions follow; do not execute them again. Commit e413931 fixed the inventory and added the weekly ELA layout. A read-only Dev Engine pull returned nonempty inventories for all 51 students and included all expected assignment IDs. The independent Missing count, Outlook draft inspection, delivery and UserInstall adoption remain pending; the dated operator steps were not performed as part of this retirement.

# HANDOFF: Fix empty parent report and add the weekly ELA update layout

Date: 2026-10-26 (Monday). From: Cowork session (ELA 7 project). To: Codex working in the CE-SE repo.
Teacher: the teacher, Pearland ISD. Teaches ELA 7, ELA 7 PAP and CS 8.

This document stands alone. You don't need earlier chats, Claude memory, or any other handoff. Where I name source files or functions I haven't seen, I mark them as guesses to verify.

---

## 0. TL;DR

1. **Bug (blocking):** `pull_parent_report` (CLI `parent-report`) returns `work_due: []` for **all 51 students** in ELA7 + ELA7 PAP. Every student gets `skip_eligible: true`. Canvas actually has 11 published assignments (ELA7) and 12 (PAP) due in the Term 2 window, with real missing and low scores. Two pulls gave the same result. The parent-update flow is unusable until this is fixed.
2. **Feature:** Coach approved a specific weekly email layout (Section 4). The Engine's fixed parent-update render can't produce it. It always adds the full "Work due in this period" table, has no "Failing (below 70)" list, and has no place for teacher announcements. Add a layout mode that renders exactly Section 4.
3. **Goal:** Once 1 and 2 are done, an agent can run the whole flow with no hand-filling. Pull, draft, `start_parent_review`, then Coach clicks **Fill Outlook Drafts only**. He has said hand-filling names or data "defeats the purpose".

---

## 1. Environment

| Item | Value |
|---|---|
| Engine install root | `D:\Development Projects\CE-SE-fresh-test` |
| Teacher rules root (sync mode) | `D:\Documents\OneDrive - Pearland ISD\Teaching Assistant Project` |
| Engine version at time of bug | `1.5.0-dev.97ef417f6f80.dirty.20261025T234840Z` (`engine_info update-status` = current, no newer build waiting) |
| Canvas instance | `https://pearland.instructure.com` |
| Outlook | `engine_info outlook-status` → `outlook_available: true` (classic Outlook) |
| Courses (configured nicknames) | `ELA7`, `ELA7 PAP`, `CS8` (IDs in UserSync `dev-reference/course-ids.md`) |
| Course timezone | America/Chicago (DST ends Sun 2026-11-01) |
| Canvas term | 248 "YEAR-2026/2027", `grading_period_group_id: null` |
| Canvas course grading periods | **none** (`/api/v1/courses/<ELA7>/grading_periods` → `[]`; course `grading_periods: null`) |
| Engine term source | `context/school-calendar.md`. Term 2 = 2026-10-13 to 2026-12-18 |
| Late policy (ELA7, verified) | 10% per day, floor 30%, missing deduction disabled |
| Relevant playbooks | `playbooks/parent-updates.md` (sha256 `adcb1d22…6839`), `playbooks/family-contact.md` |
| Parent-update settings | `context/parent-updates.md`. `bobcat_hour_url: https://flex.securly.com/login` is set. `sign_off` is **blank**. `skip_score: 90` |

---

## 2. Bug: empty `work_due` for every student

### Reproduce

```
.\cse.cmd parent-report ELA7 "ELA7 PAP"
```
(MCP: `pull_parent_report {courses: ["ELA7","ELA7 PAP"]}`)

Runs from 2026-10-26 (both identical in shape):
- `courses/ELA7/parent-updates/20261026T214549Z/report.json`
- `courses/ELA7/parent-updates/20261026T215735Z/report.json`

Report scope (correct):
```json
"scope": {
  "timezone": "America/Chicago",
  "period_name": "Term 2",
  "period_start_date": "2026-10-13",
  "period_end_date": "2026-12-18",
  "recent_start_at": "2026-10-13T00:00:00-05:00",
  "recent_end_at": "2026-10-26T16:57:35-05:00",
  "due_soon_end_at": "2026-11-02T16:57:35-06:00"
}
```
Every student object looks like this (no `missing`, `corrections`, `due_soon`, `history` or `flags` keys at all):
```json
{ "student_ref": "s-…", "alias": "…",
  "courses": [{ "course": "ELA7", "course_score": "60.12",
                "course_score_source": "canvas_enrollment_current_score",
                "work_due": [] }],
  "skip_eligible": true }
```
Course scores run from 60.12 to 98.51. Several students are below 70 and still `skip_eligible: true`. That alone contradicts the playbook's skip rule (skip only when every course ≥ `skip_score` and no Missing/0, **or** no required work was due).

### Ground truth from Canvas (read-only GETs, 2026-10-26 ~21:45Z)

Published assignments with `due_at` inside the Term 2 window up to `as_of`:

**ELA7**, all in group 258105 (Daily):

| id | name | due_at (UTC) | submission type |
|---|---|---|---|
| 3698911 | Is Brain Rot Real? - Informational Text Quiz | 2026-10-14T04:59:59Z | external_tool (New Quiz) |
| 3697325 | Benchmark 1 Reflection | 2026-10-15T04:59:59Z | online_text_entry |
| 3701686 | Fall Benchmark 1 | 2026-10-15T04:59:59Z | none |
| 3701426 | Book Club Launch Discussion | 2026-10-16T04:59:59Z | discussion_topic |
| 3698912 | Typing - Hotline Miami | 2026-10-17T04:59:59Z | none |
| 3705185 | First Impressions | 2026-10-20T21:00:00Z | discussion_topic |
| 3705769 | The Terror - Anticipation Writing | 2026-10-21T04:59:59Z | external_tool |
| 3706334 | Claim Check: Fun Facts Edition | 2026-10-22T21:00:00Z | **on_paper** |
| 3705777 | The Terror - Moment of Truth: Reading and Response | 2026-10-22T21:00:00Z | external_tool |
| 3705188 | Typing - Stardew Valley | 2026-10-23T21:00:00Z | none |
| 3707044 | Make it Sound True | 2026-10-24T04:59:59Z | online_text_entry |

**ELA7 PAP**, all in group 258205:

| id | name | due_at (UTC) | submission type |
|---|---|---|---|
| 3697309 | Benchmark 1 Reflection | 2026-10-14T04:59:59Z | online_text_entry |
| 3698630 | Is Brain Rot Real? - Informational Text Quiz | 2026-10-14T04:59:59Z | external_tool |
| 3697804 | Intro to Informational - The HuggingFace Attack | 2026-10-14T21:00:00Z | online_text_entry |
| 3701755 | Fall Benchmark 1 | 2026-10-15T04:59:59Z | none |
| 3701427 | Book Club Launch Discussion | 2026-10-16T04:59:59Z | discussion_topic |
| 3698606 | Typing - Hotline Miami | 2026-10-17T04:59:59Z | none |
| 3705186 | First Impressions | 2026-10-20T21:00:00Z | discussion_topic |
| 3705775 | The Terror - Anticipation Writing | 2026-10-20T21:00:00Z | external_tool |
| 3705779 | The Terror - Moment of Truth: Reading and Response | 2026-10-21T21:00:00Z | external_tool |
| 3706345 | Claim Check: Fun Facts Edition | 2026-10-22T21:00:00Z | **on_paper** |
| 3703862 | Typing - Stardew Valley | 2026-10-23T21:00:00Z | none |
| 3707007 | Make it Sound True | 2026-10-24T04:59:59Z | online_text_entry |

Submission evidence from the first page (100 rows) of `GET /api/v1/courses/<ELA7>/students/submissions?student_ids[]=all&workflow_state=unsubmitted&include[]=assignment&per_page=100`:
- 17 rows have `missing: true`. In-window examples: First Impressions (5 students), Moment of Truth (2), Brain Rot quiz (1), Anticipation Writing (1), Book Club Launch (1), Benchmark 1 Reflection (1).
- **Every row has `grading_period_id: null`.** That's expected, since Canvas has no grading periods here.

So `work_due` must be nonempty for every enrolled student, and at least 10 ELA7 students have real Missing/0 items.

### Leading hypotheses (verify in this order)

1. **One unsupported submission type kills the whole inventory.** Both courses have an `on_paper` assignment in the window (3706334, 3706345). The Engine is known to reject `on_paper` elsewhere: `pull_assignment` fails with `unsupported submission mode: on_paper`. If the parent-report assignment loop hits that error and the caller swallows it, returning an empty inventory per course, you'd see exactly this symptom. Check how the report builder handles a per-assignment classification error.
2. **Filtering by Canvas `grading_period_id`.** The Engine names the period "Term 2" from `context/school-calendar.md`, but Canvas has no grading periods, so every submission has `grading_period_id: null`. If the inventory filter matches on `grading_period_id` (or calls `/students/submissions?grading_period_id=…`) instead of the student-effective due date, everything drops out.
3. **Window comparison bug.** `recent_start_at` is `-05:00` and `due_soon_end_at` is `-06:00` (it crosses the 11/01 DST change). That's correct as wall-clock time. Still, confirm the inventory compares instants (UTC), not strings or local dates with mixed offsets.
4. **Zeros counted as "turned in".** Coach marks missing work 0 right away (see Section 3). A teacher-entered 0 makes Canvas report `workflow_state: graded`, `missing: false`. If the inventory or Missing logic keys off `missing`/`workflow_state`/`submitted_at`, every zeroed item reads as completed work. That alone wouldn't empty `work_due` (graded rows still belong in the inventory), but it would wrongly empty Missing/0 and make low students `skip_eligible`. Check this together with hypotheses 1–2.
5. **Pagination or enrollment join.** Submissions fetched per page but joined on the wrong key (for example `student_ref` hashed before the join) would also produce empty lists. Less likely given that course scores came through.

### Required fix

- `work_due` holds every published, graded assignment whose **student-effective due date** (overrides included) falls in `[later of period start or as_of − correction_days, as_of]`, as the playbook already says.
- Support `on_paper`, `none`, `external_tool` (New Quizzes), `discussion_topic` and `online_text_entry` in classification. For `on_paper` and `none`, "missing" means no score and Canvas `missing: true`, or a recorded 0.
- **Never fail silent.** If one assignment can't be classified, keep the rest and add a `flags` entry on the affected students/course (`"assignment 3706334 could not be classified: <reason>"`). Any student with a flag must not be `skip_eligible`.
- Treat the period window as **dates from `school-calendar.md`**, never Canvas `grading_period_id`, when the course has no Canvas grading periods.
- Sanity guard: if a course has ≥1 published, graded, in-window assignment but every student's `work_due` is empty, abort the run with a clear error instead of writing a report.

---

## 3. What Coach wants from the update (decisions already made)

- **Audience:** families of every active student in `ELA7` and `ELA7 PAP` (one email per student, students' own course only). Use the existing Outlook path with parents as recipients. If the engine supports `recipients: "parents_only"`, use it for this layout. Otherwise keep the current grade-update default and document which it is.
- **Every student gets an email.** No skip by score. Students with nothing to report get the empty-state line in Section 4. (This differs from the default `skip_score` behaviour. Make it a setting of the new layout.)
- **No student name anywhere in the email body or subject.** The parent knows who their child is. No `{{student_name}}`.
- **Term 2 only.** Missing and failing items come from the Term 2 window (2026-10-13 onward, through `as_of`).
- **Missing = 0. They are the same thing (Coach's rule, non-negotiable).** Coach enters a **0 the moment work is missing**, because students ignore a blank. Entering that grade makes Canvas set `workflow_state: graded` and `missing: false`, and the submission can look "turned in" or "graded". The Engine must **not** use `missing`, `workflow_state`, `submitted_at` or `submission_type` to decide this. The rule:
  - Current posted score **= 0** → **Missing**, always, whether or not there is a submission, a grade, or a late flag.
  - No score, past due, and Canvas `missing: true` → **Missing** (the case where he hasn't entered the 0 yet).
  - Current score **> 0** → never missing, even if Canvas still says `missing: true` or `late: true`.
  - Excused (`excused: true`) → never listed.
  - Don't list future or not-yet-due work.
  - In the email, a Missing item shows **only** `– due {M/D}`. Never show "0", never say "recorded zero", and never add "(late points deducted)" to a missing item. Use one wording for every missing item.
  - Zeros never appear under Failing. Failing is only scores > 0 and < 70.
- **Failing** = scored, posted, in-window work with current score **< 70 out of 100** that isn't already in Missing. All assignments are scored out of 100; if `points_possible ≠ 100`, show the percentage rounded to whole numbers.
- **Late deduction marker:** when Canvas `points_deducted > 0` on the current submission, append the literal text ` (late points deducted)` after the score. The score shown is Canvas's current score **after** deduction. Don't show the deduction amount.
- **Unposted grades** (`posted_at: null`) are never shown as failing.
- **Every assignment title is a link** to its Canvas `html_url` in the student's own course.
- **No inventory table, no Corrections section, no Due Soon section** in this layout.
- **Sign-off:** the teacher's usual sign-off (in UserSync). Set `sign_off: "<teacher sign-off from UserSync>"` in `context/parent-updates.md` (it is currently blank).
- **Tone:** direct, imperative. Never ask parents to reply. Don't use "Please encourage".

---

## 4. Approved email (render exactly this)

Subject:
```
ELA 7 Update – Week of Oct. 26
```
(General form: `ELA 7 Update – Week of {Mon. D}` from the run's week start. Use an en dash. Abbreviate the month AP-style: Oct., Nov., Dec., Jan., Feb., Aug., Sept.; March, April, May, June, July spelled out.)

Body (Markdown → HTML email):

```markdown
Hello,

Here's a quick ELA update.

**Work that needs attention (Term 2)**

Missing:

- [{title}]({html_url}) – due {M/D}
- …

Failing (below 70):

- [{title}]({html_url}) – {score}
- [{title}]({html_url}) – {score} (late points deducted)
- …

All missing work should be done as soon as possible.

**Major grades this week**

{announcements block, per course – see 4.1}

**Extra help**
Sign up for Bobcat Hour tutorials at [flex.securly.com](https://flex.securly.com/login).

Thank you,
<teacher sign-off>
```

Rendering rules:
- Leave out the "Missing:" sub-list when it's empty, and the "Failing (below 70):" sub-list when it's empty.
- Show "All missing work should be done as soon as possible." only when Missing is nonempty.
- When both are empty, replace the whole content under **Work that needs attention (Term 2)** with exactly:
  `No missing or failing work in Term 2 right now.`
- `due {M/D}` is the student-effective due date in America/Chicago, no leading zeros (10/20).
- Order items by due date ascending, then title.
- Use `{score}` as an integer when whole, otherwise one decimal place.
- The Bobcat Hour link text is `flex.securly.com`. The href comes from `bobcat_hour_url`.

### 4.1 Announcements block (teacher-authored, per course)

The agent supplies this in the drafts file per course. The Engine inserts it verbatim (Markdown) into each student's email for that course. It must never contain grades or names. For this week:

**ELA7:**
```markdown
- **Thursday, 10/29 – Essay (ECR):** [The Last Human Light: Features ECR](https://pearland.instructure.com/courses/<ELA7>/assignments/3708512). Plan the essay in class Monday through Wednesday with [Plan Your Features ECR](https://pearland.instructure.com/courses/<ELA7>/assignments/3708515) (due Wednesday, 10/28). Type the essay in class Thursday. Major-grade writing done outside of school will not be accepted.
- **Friday, 10/30 – Book Club Reading Quiz 1:** Prepare with the [Book Club Quiz 1: Build Your Study Guide](https://pearland.instructure.com/courses/<ELA7>/pages/book-club-quiz-1-build-your-study-guide) page.
```

**ELA7 PAP:**
```markdown
- **Thursday, 10/29 – Essay (ECR):** [The Last Human Light: Features ECR](https://pearland.instructure.com/courses/<ELA7-PAP>/assignments/3708513). Plan the essay in class Monday through Wednesday with [Plan Your Features ECR](https://pearland.instructure.com/courses/<ELA7-PAP>/assignments/3708517) (due Wednesday, 10/28). Type the essay in class Thursday. Major-grade writing done outside of school will not be accepted.
- **Friday, 10/30 – Book Club Reading Quiz 1:** Prepare with the [Book Club Quiz 1: Build Your Study Guide](https://pearland.instructure.com/courses/<ELA7-PAP>/pages/book-club-quiz-1-build-your-study-guide) page.
```

Facts behind the block (verified 2026-10-26, Canvas is the source of truth per Coach):
- Features ECR: ELA7 3708512 is **unpublished** (unlocks 10/29 13:00Z), Major Grade group 258107. PAP 3708513 is published, unlocks 10/29 13:00Z. Both are due 2026-10-29T21:00:00Z (Thu 4:00 PM CDT).
- Plan Your Features ECR: ELA7 3708515 and PAP 3708517, both due 2026-10-28T21:00:00Z (Wed 4:00 PM CDT).
- Book Club Quiz 1 is Friday 10/30 per the unit schedule. The per-book quiz assignments in Canvas are unpublished and undated. Families get the shared study-guide page, not a quiz link.
- Announcement links must point at the student's own course. Links work for parents only when they are signed in as observers. That's fine.

### 4.2 Suggested drafts.json extension (guess; design as you see fit)

```json
{
  "schema": 2,
  "kind": "parent-drafts",
  "run_id": "…",
  "layout": "weekly_update",
  "layout_options": {
    "subject": "ELA 7 Update – Week of Oct. 26",
    "skip_by_score": false,
    "failing_below": 70,
    "late_marker": " (late points deducted)",
    "include_inventory": false,
    "include_corrections": false,
    "include_due_soon": false
  },
  "announcements": {
    "ELA7": "…markdown…",
    "ELA7 PAP": "…markdown…"
  },
  "students": [
    { "student_ref": "s-…", "decision": "send" }
  ]
}
```
Missing and failing lists should be **computed by the Engine** from the report, not typed by the agent, so values can't be misstated. Validation should reject any announcement text that contains a student alias, a `student_ref`, or an email address.

The `report.json` needs a per-course `failing` list (or enough fields in `work_due`: `score`, `points_possible`, `posted`, `points_deducted`, `html_url`, `due_at`) for the renderer to build it.

---

## 5. Acceptance tests

Use synthetic fixtures for unit tests (no real names). For live checks, compare only aggregate counts and assignment IDs. Never paste student identities into logs or fixtures.

1. **Live inventory:** `parent-report ELA7 "ELA7 PAP"` run after the fix. Every student has nonempty `work_due`. The ELA7 inventory covers the 11 IDs in Section 2, and PAP covers the 12. `on_paper` items 3706334 and 3706345 are present.
2. **Missing count cross-check:** the number of ELA7 (student, assignment) pairs in Missing for in-window assignments equals a direct Canvas count across all pages: (current score = 0, any workflow state) **plus** (no score, past due, `missing: true`), excluding excused. Expect this to be well above the 17 `missing: true` rows seen on page 1, because zeroed items don't carry the missing flag. Report both numbers in the PR.
3. **Failing:** a synthetic student with scores 55, 69.5, 70, and 62 with `points_deducted: 8` renders as "55", "69.5", omits 70, and "62 (late points deducted)". An unposted 40 isn't shown.
3b. **Zero = Missing:** synthetic rows render as follows.
   - A graded 0 with `workflow_state: graded`, `missing: false` and a `submitted_at` → **Missing**.
   - A graded 0 with no submission → **Missing**.
   - A blank past-due row with `missing: true` → **Missing**.
   - A 0 with `points_deducted > 0` → **Missing**, with no late marker.
   - A 45 with `missing: true` still set → **Failing**, not Missing.
   - An excused row → nothing.
   None of the Missing lines shows a score.
4. **Empty state:** a synthetic student with no Missing/0 and nothing below 70 renders the single line `No missing or failing work in Term 2 right now.` and is still `send`.
5. **No name leak:** the rendered subject and body for every student contain no student name, alias or `student_ref`.
6. **Per-course links:** a PAP student's email links only to `/courses/<ELA7-PAP>/…`. An ELA7 student's email links only to `/courses/<ELA7>/…`.
7. **Silent-failure guard:** inject one unclassifiable assignment. The run keeps the others, adds a `flags` entry, and the affected students are not skip-eligible. Inject "all inventories empty while in-window assignments exist" and the run aborts with an error.
8. **Outlook:** `start_parent_review` on the new run shows previews matching Section 4. "Fill Outlook Drafts only" creates one draft per student in classic Outlook Drafts and sends nothing. `review_status` reports counts by `student_ref` only.

---

## 6. After the fix: operator steps (for whichever agent runs it)

1. `engine_info version` and confirm the fixed build.
2. Set `sign_off: "<teacher sign-off from UserSync>"` in `context/parent-updates.md` (use `write_workspace_file` with the current sha256).
3. `pull_parent_report {courses: ["ELA7","ELA7 PAP"]}`. Spot-check that `work_due` is nonempty.
4. If the run date is still the week of 10/26, use the Section 4.1 announcements as written. Otherwise ask Coach for that week's announcements.
5. `write_draft` `drafts.json` beside `report.json` with `layout: "weekly_update"` and every student `send`.
6. `start_parent_review {folder}`. Give Coach the link and tell him to choose **Fill Outlook Drafts only**.
7. `review_status`, then report in one line: drafts created / held / failed (aliases only).

---

## 7. Constraints from Coach

- Student PII never goes to outside models or websites. The district OneDrive / M365 tenant is the exception. The agent must never see parent addresses. The Engine keeps them private, and that design must not change.
- All scoring is out of 100.
- The Engine never sends without Coach's click on the review page. "Fill Outlook Drafts only" is the path for this job.
- Anything created in Canvas stays **unpublished** unless he says otherwise. (This job makes no Canvas writes.)
- Record any gap you decide not to close in `context/engine-notes.md`, and update `help fields` and the capability records when evidence changes.
- Also add the `on_paper` support from the 2026-10-25 handoff if it turns out to be the root cause here. It fixes `pull_assignment` too.

## 8. IDs for reference

| Thing | ELA7 | ELA7 PAP |
|---|---|---|
| Features ECR (Major Grade) | 3708512 | 3708513 |
| Plan Your Features ECR | 3708515 | 3708517 |
| Quiz 1 study-guide page slug | book-club-quiz-1-build-your-study-guide | same |
| on_paper in-window assignment | 3706334 | 3706345 |
| Assignment groups | Daily 258105, Major Grade 258107, Imported 275632 | Daily/default 258205, Major 258206 |
| Report runs showing the bug | `courses/ELA7/parent-updates/20261026T214549Z`, `…/20261026T215735Z` | (same runs cover both courses) |
