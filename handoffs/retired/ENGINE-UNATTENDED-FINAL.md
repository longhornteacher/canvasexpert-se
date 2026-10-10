> **Retired 2026-10-10.** Closed without the human finish: the ordinary-student completion/resubmission check stays pending human validation on the Capabilities page ("The next useful milestone"); the steps below remain a usable script for it. The fixture inventory moved to docs/cs8-test-objects.md.

# Engine unattended finish

2026-10-09. The authorized unattended run used the isolated `.test-workspaces/engine-unattended-20261009/` workspace and synthetic Canvas built-in Test Student only. See [capability record](../../docs/engine-capabilities.md) for verified behavior, build identity, evidence and limits.

Production fixes: normalize Canvas HTML entities when resuming creates; preserve rubric maxima when ratings are omitted; show computed assessments during validation; recognize completed result files throughout the workspace; post coupled rubric assessments and computed grades separately. The narrow developer probe now selects exact fixtures, verifies grade/SIS exclusions, creates/reads a dedicated module, and uploads synthetic files. Its coupling update was repaired and its affected test fixture restored from the original snapshot. No existing coursework or ordinary students' records were written.

Final Engine: [dist/cse.exe](../../dist/cse.exe), revision `2d49715d2c0b+dirty`, built `2026-10-09T16:31:36Z` with Go 1.27.1. SHA-256 `443E4FC96411DEF0AD8F30DAA3396C1819A01DE83607F244558137BFC900F713`. `go test ./...`, `go vet ./...` and the pinned Windows build passed. The final coupled live check stored truthful rubric points 2.5 + 4 and grade 13/20 with coupling enabled; repeat made no HTTP requests.

## Human finish

Use any available student; no particular participant was selected. Have the teacher coordinate these steps and identify the participant's existing alias/student_ref for the Engine session.

1. Open the [text fixture](https://pearland.instructure.com/courses/121046/assignments/3698077). Submit two brief responses: A explains why measured evidence supports a claim; B gives an example and explains the connection. These are clearly labeled disposable tests; the teacher can direct the selected participant through the synthetic instructions. The fixture has a test due date; leave teacher decisions about timing to the scoring session.
2. Pull this exact assignment through ordinary `cse pull-work CS8 3698077` in the isolated development workspace. Verify that the participant's actual current attempt appears with the right alias/student_ref. In a teacher-authorized scoring session, assess/post and record completion; then confirm ordinary current-course discovery does not requeue that completed attempt.
3. Have the same participant improve the response and resubmit once. Confirm ordinary discovery now finds the new attempt and its earlier work. Review the visible assignment directions and score/rubric/feedback presentation for clarity and voice.
4. Optional presentation check: upload a small text file through the [file fixture](https://pearland.instructure.com/courses/121046/assignments/3698079). Live built-in Test Student upload/download already succeeded, so file mechanics do not depend on this human step.

The remaining evidence gap is normal enrollment plus an ordinary student's completion/resubmission and actual student-facing usability. Controlled HTTP tests cover discovery/completion mechanics; Test Student intake alone cannot establish that final human path.

## Fixture inventory for later cleanup

All assignments have verified `omit_from_final_grade: true` and `post_to_sis: false`.

| Canvas resource | State left by run | Reference |
|---|---|---|
| Plain 3698077 | Published; Test Student attempts 1–4 | [Assignment](https://pearland.instructure.com/courses/121046/assignments/3698077) |
| Rubric 3698078 / rubric 71830 / association 130470 | Published; Test Student attempts; coupling experiment enabled | [Assignment](https://pearland.instructure.com/courses/121046/assignments/3698078) |
| File 3698079 / rubric 71829 / association 130469 / file 19221778 | Published; Test Student file attempt | [Assignment](https://pearland.instructure.com/courses/121046/assignments/3698079) |
| Ratingless diagnostic 3698080 / rubric 71831 | Unpublished; zero-maxima diagnostic retained | [Assignment](https://pearland.instructure.com/courses/121046/assignments/3698080) |
| Numeric anchors 3698081 / rubric 71832 | Unpublished; rubric maxima fixed; module item 9025926 | [Assignment](https://pearland.instructure.com/courses/121046/assignments/3698081) |
| Attachment anchors 3698082 / rubric 71833 | Unpublished; rubric attachment verified; module item 9025927 | [Assignment](https://pearland.instructure.com/courses/121046/assignments/3698082) |
| Module 636143 | Unpublished; contains 3698081 and 3698082 | [Course modules](https://pearland.instructure.com/courses/121046/modules) |

Published assignments can be visible to enrolled students. No resources were deleted or hidden by changing course policy. Canvas may reject unpublishing assignments with submissions. IDs are cleanup references; read current live state before acting.

Existing checkout changes, including the format-reference edits that preceded this run, were preserved. Raw synthetic evidence and executables are in the ignored development workspace; the one-off fixture restoration utility has already been executed and is not a product command.
