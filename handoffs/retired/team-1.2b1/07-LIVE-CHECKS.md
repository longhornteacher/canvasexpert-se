# 07: Live checks

Read [00-BRIEF.md](00-BRIEF.md) first. The **developer** drives these checks, with an agent assisting. They run after 01–06 are built and committed. Each check names its Canvas writes, and the developer approves each one in chat before it runs. Results go into the Status table in [docs/team/README.md](../../../docs/team/README.md), into `capabilities.go` / `docs/engine-capabilities.md`, and into the CHANGELOG notes. Record dates, course nicknames, counts and Canvas IDs only, never student names.

All checks use the **Test Student** (every course has one) and **disposable** objects:
- Title prefix `[CSE DISPOSABLE]`, plus `(CE)` where needed.
- Unpublished where possible.
- The developer deletes them in Canvas afterward.

Run from a Dev-built Engine against a scratch UserInstall, never the teacher's real UserInstall. Copy the real `context/courses.md` instance and course entries into the scratch install, and add only what each check says.

## Part A: the developer alone (CS8 and Collab, no teammate key)

**The trick.** A course that is owned but where the operator is an active teacher routes to the **operator's** key (R2), while consent and hiding still apply (R3, R4: they follow ownership). So everything in slices 03 and 05 can be tested on CS8 without a colleague's key:
1. In the scratch install, add a team entry by hand to `context/team.md`:
   - `label: devcheck`, `name: Dev Check`;
   - the developer's own Canvas user ID **+ 1** (any positive ID that isn't the developer's);
   - for `email`, a **second** address the developer controls, not the one Outlook sends from, so a Reply-To is actually distinguishable;
   - a `key_expires` date 10 days out.
2. Set `owner: devcheck` on CS8.

No key is saved; none is needed, because routing picks the operator key.

**Test Student pulls.** The Test Student can only be pulled through `cse-probe` (`PullBuiltinTestStudent`), which is locked to CS8 (`cse-probe/main.go:28`). It also requires a title starting with `[CSE UNATTENDED] ` and a fresh probe fixture state per title, because renaming breaks `namedFixture`. After slice 02, cse-probe loads `team.md`, so consent and hiding apply to it. So every Test Student assignment below is created **with its final title** and never renamed.

**A1. Routing and expiry display.**
- Run `cse pull-assignment-dates CS8 <any>`.
- Expect: no "key isn't on this PC" error. That is the live proof that `GET /api/v1/courses/:id` returns the caller's active teacher enrollment, so the operator key was routed.
- No expiry note is printed, because Dev Check's key wasn't used.
- The Hub Team page and home show "Expires soon" for Dev Check.
- Writes: none.

**A2. Tag refusal.**
- Create `[CSE DISPOSABLE] Consent check` in CS8: unpublished, no `(CE)`.
- Run `cse pull-work CS8 "[CSE DISPOSABLE] Consent check"`. No submission is needed.
- Expect the R3 refusal, before any submission read.
- Writes: the assignment (developer, by hand).

**A3. Hidden grade.**
1. Create `[CSE UNATTENDED] Hidden check (CE)` in CS8: unpublished, points 10, online text entry. The Test Student submits (Student View).
2. Pull it with the cse-probe Test Student pull. Score it with a comment. Post the results with `cse post` (team-aware) if it accepts the probe's packet; otherwise use the probe's post path. Review is skipped for this check, and the developer explicitly approves this post in chat.
3. Expect:
   - the policy mutation succeeds: **this is the first live proof that GraphQL accepts the legacy ID**;
   - the grade and comment are written;
   - the summary line names "Dev Check".
4. Check the gradebook column: is it set to manual posting?
5. Check Student View: are **both** the grade and the comment hidden?
6. Then click **Post grades** in the gradebook and check Student View again: are both visible?
- Writes: the post policy, one grade and one comment.

**A4. Already-visible hold.**
- With A3's grade posted, the Test Student resubmits. Pull again and post a new score.
- Expect the "already visible" hold, and nothing written.
- Then, by hand in SpeedGrader, change the grade on the posted submission and check Student View. Does the change show immediately? If it **doesn't**, record that; the hold rule could be relaxed later, but don't change code in this check.
- Writes: the Test Student resubmission; one manual grade change by the developer.

**A5. New Quiz comment.**
- Create `[CSE UNATTENDED] Quiz check (CE)`, a New Quiz with one essay item, **left on automatic posting**. The Test Student takes it.
- Before pulling, read the submission (read-only `cse api`): is `posted_at` already set at submit?
  - If it is: expect CE-SE to hold the comment-only post as "already visible". This confirms the playbook advice to set manual posting first.
  - Then repeat with a second quiz (`… Quiz check 2 (CE)`) set to manual posting **before** the Test Student starts. Expect the comment to post and stay hidden in Student View until posted.
- If the Test Student pull doesn't support New Quizzes, record that and skip A5. It then waits for first real use.
- Writes: quiz, attempt, policy, comment.

**A6. Family reply-to (no family email).**
- Use only the parent-update **test send** (`SendParentTest`), which goes to the developer's own Outlook address, for any one CS8 student. It never reaches a family.
- Expect:
  - the note naming Dev Check and the second address appears before the sign-off;
  - in the received message, **Reply** addresses Dev Check's (second) address, not the sending address. This proves COM `ReplyRecipients`.
- Don't create Family Contact drafts for real students: a draft would sit in Outlook addressed to real families. Family Contact uses the same Outlook compose code, and its routing is covered by local tests.
- Writes: none to Canvas; one test email to self.

**A7. Copy once, Collab to CS8.**
1. In Collab, create an unpublished module `[CSE DISPOSABLE] Copy check` containing:
   - a Page with an embedded course file;
   - an assignment;
   - a New Quiz with one item.
2. **Remove `owner` from CS8 first.** This is now a copy into the developer's own course.
3. Run `copy-module Collab "[CSE DISPOSABLE] Copy check" CS8`.
4. Record:
   - status;
   - whether the module and **all three items** arrived (this proves the property shape);
   - whether the file came along;
   - where the assignment landed among assignment groups;
   - whether the New Quiz appeared, and after how long;
   - the migration issues;
   - the time taken.
5. Run the same command again. Expect a refusal citing the first copy.
6. Separately, put `(CE)` on the Collab assignment and run with a new destination. Expect a refusal before any copy.
- Writes: the Collab disposables, and one content migration into CS8. Delete the copied module and items in CS8 afterward.

## Part B: with one willing teammate

Do this only after Part A passes, and only with the teammate present and agreeing.

**B1. Add the teammate.**
- The teammate sits at the team PC and follows the Add page: private window, new token, expiry at the last day of school.
- Expect: the card shows "Key saved on this PC"; **Choose courses** lists their courses; pick one.
- Check that the token appears in no file under the scratch install. Grep for its first 8 characters, then clear the terminal.

**B2. Test key.**
- Expect "Ms. ___'s key works."

**B3. Routing in their course.**
- `list_courses`: the course shows `owner`, and `team` shows `key_on_this_pc: true`.
- Run a read (`modules`) in their course. It succeeds with their key, and the action log shows nothing (reads aren't logged).

**B4. Copy into their course.**
- Repeat A7's module (a new disposable title) into their course.
- Expect: their key is used, and the action-log lines appear on the Team page with redacted paths.
- The teammate deletes the copied module afterward, or keeps it if they want.

**B5. Consent in their course.**
- The teammate creates a disposable, unpublished assignment **without** `(CE)`. A pull by title is refused with the R3 message naming the teammate.
- The Test Student path can't run in a teammate's course (cse-probe is CS8-only), so the first hidden post there is the **first real use**:
  - a real `(CE)` assignment the teammate chose;
  - the teammate present;
  - review not skipped.
- Record: the post-policy mutation succeeded with the teammate's key; the summary line; and the teammate seeing hidden grades in SpeedGrader and posting them.

**B6. Remove.**
- Remove the teammate.
- Expect: the key is gone (the Test button says it isn't on this PC); `courses.md` has no rows for them; notes and log are kept.
- The teammate deletes the token in Canvas.

## Part C: when co-teacher enrollment arrives

**C1.** In a course the developer now co-teaches, with `owner` set and the teammate's key **removed** from the PC:
- Commands still work with the developer's key.
- `(CE)` and hiding still apply.
- Writes show the developer as grader in Canvas.

## After each part

- Update the README Status table and the capability items. Move an item to the green group only when its Canvas behavior was observed above.
- Note anything that contradicts a contract in 00-BRIEF, for Opus to decide. Do not change Engine behavior during a live check.
