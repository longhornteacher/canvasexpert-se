# 09 — Shared-artifact submission pulls

Requires: [01](01-groups.md), [08](08-group-assignment-authoring.md) snapshot contracts. Existing assignments are valid targets. Result: read shared work once while retaining each member's own submission, attempt, grade, and feedback identity.

## Surface and data

Extend `pull-work COURSE ASSIGNMENT_ID [--group GROUP_ID]` and `pull_work {course, assignment?, group_id?}`. A group filter requires an explicit assignment. Without the filter, read all in-scope groups for that assignment. Broad unfinished-work pulls can discover supported group work without silently widening an explicitly selected assignment/group.

Save `kind: group-packet` with the assignment snapshot, pull time, and paths to `kind: group-work` files. Each group-work contains:

- origin/course/assignment/set/group identity, current group label and full observed course-enrolled membership;
- a stable artifact key within this packet, original body/attachments and readable text/status, source member submission IDs/attempts, and deterministic content identity;
- per-member `student_ref`/alias, returned submission group identity, submission ID, current/prior `AttemptRef`, effective due date, excused/visibility flags, current grade/rubric and existing comments;
- complete/partial evidence and precise reasons. Grade/comment visibility and authored group membership are observations, not inferred permanent history.

Keep group membership and per-member submissions separate even when their content matches. Deduplicate identical attachment downloads by Canvas file identity/content within this packet; do not erase their source associations. A member can have a different current attempt or historical artifact after a membership change. Preserve that difference rather than arbitrarily choosing the first member's submission.

## Implementation

Use flat assignment submissions with `include[]=group`, history, comments and rubric data, followed by detailed reads needed for exact current identities. Read the representative's assignment-specific `group_members` audience from shared contracts rather than assuming a generic group roster exposes everyone Canvas may affect. The list API can group shared-grading responses, but its behavior differs in individual-grading mode; do not base the storage contract on that optional grouping. Add needed group fields to the Canvas submission DTO, normalized privately through existing student mapping. [Submissions](https://developerdocs.instructure.com/services/canvas/resources/submissions), [controller grouping behavior](https://github.com/instructure/canvas-lms/blob/master/app/controllers/submissions_api_controller.rb).

Add a focused `group_pull.go` operation and domain types, using existing attachment download and exact attempt logic. Route only supported text/file group assignments to it. Remove `unpullableAssignment`'s blanket group rejection only for this new branch. Preserve generic scoring guards until slices 10-11; pulling group evidence alone does not authorize individual `results` writes.

Use deterministic evidence fingerprints from relevant body/file IDs/attempt references, not a permanent content store. A null/unknown group, incomplete membership, or differing current artifacts is reported and held for shared assessment rather than merged into a false consensus. Individually assessable member evidence can remain usable in slice 11. Students outside groups remain listed separately with the specific Canvas state; do not create a club for them or discard their submitted work.

## Code anchors (verified in Dev 2026-10-17)

- `pull_work.go` rejects group assignments at two points: discovery (`:204`, on list data) and after `GetAssignment` (`:257`). The group branch goes before the flat `Submissions` read (`:209`) and needs its own discovery using the same enrollment/`active` map built just above. Do not thread group logic through the per-student packet loop (`:307` onward); `completedHint`, `attemptWork` and the packet/student-work files all assume one student per document.
- The `Submission` DTO has no group fields and `Submissions()` (`canvas_reads.go:204`) does not request `include[]=group`; add both for the group path only.
- `attemptWork` downloads attachments per call. Calling it once per member would download a shared file N times; deduplicate by Canvas file identity inside the group pull, as specified.
- Reusable unchanged: `unsupportedSubmission`, `liveHandled` (in shared mode a propagated grade makes every member read as handled, which is the right discovery behavior), `fetchSubmissions`, `attemptRef`.
- `completedHint` (`:562`) scans only `kind: results` files and only `grade` rows. Individual-mode `group-results` (slice 11) must be recognized there, per student/attempt; shared-mode completion can rely on `liveHandled` plus the group-results outcomes.

## Acceptance

- Three matching member submissions produce one artifact and three distinct member records with their own attempts and grades.
- A former member's historical submission is not reassigned by current membership; differing current artifacts or attempts are held for shared assessment while other groups pull normally.
- An unreadable attachment preserves metadata and independent evidence; unknown evidence never becomes "missing".
- A group-scoped request reads no other group; wrong-course/set targets fail before download; a fresh pull preserves existing unsent results.

Update formats and scoring guidance with a group packet specimen. Do not claim live group submissions have been validated from fakes.
