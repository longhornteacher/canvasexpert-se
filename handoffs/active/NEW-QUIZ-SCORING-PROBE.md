# New Quiz written scoring probe (plan)

2026-10-10. Plan only; nothing here is authorized yet. Answers the one open question in [CLASSIC-QUIZ-RETIREMENT.md](CLASSIC-QUIZ-RETIREMENT.md): **what can CE-SE honestly do with New Quiz written responses?** Probe first, then design. Engine work only if a path survives. Playbook text follows the result.

## What we already know

- Written items exist in `push-quiz` and are stored correctly (CS8 quiz 3698183). The teacher grades them in SpeedGrader today.
- The original CanvasExpert concluded there is **no public API for New Quiz item scores or per-item feedback**. It never wrote them (`docs/reference/classic-quiz-design.md`, `mutation-reconciliation-map.md`).
- OG live-probed **draft submission comments** (GraphQL `createSubmissionComment(draftComment: true, attempt: N)`): teacher-only, visible in SpeedGrader, submitted by the teacher. Not observed: second attempts, and file-upload responses.
- OG reads responses through the Student Analysis report API (`POST /api/quiz/v1/courses/:c/quizzes/:a/reports`, JSON), and saw 502s and no rows for Test Student.
- New Quizzes cannot open as the built-in Test Student (LTI). **Every submission-dependent step needs a real participant.**

## Questions, in order (each has a stop condition)

**Q1. Can CE-SE read written responses?** Needs a participant. Try, cheapest first: (a) REST submissions with `include[]=submission_history` on the quiz assignment; (b) the Student Analysis report, JSON; (c) GraphQL on the submission. Record per source: essay text present, file upload URL present, attempt number, student identity, latency and errors. Stop if none returns essay text: then CE-SE can only prepare a scoring sheet from teacher-pasted responses, and Q2-Q4 matter less.

**Q2. Can CE-SE write an item score?** No participant needed to start. Teacher opens SpeedGrader on a submitted quiz with the Browser pane's network tool watching (or DevTools) while scoring one essay. Record the request the UI makes and its auth (LTI JWT versus the teacher API token). If it only accepts the LTI session, **item scores are not writable by CE-SE**; stop, record, do not try to forge the session.

**Q3. Is a rubric the carrier?** Candidate path, untested anywhere: attach a rubric (one criterion per written item) to the New Quiz assignment, then write the rubric assessment and grade through the Engine's existing rubric path. Check on the unpublished quiz 3698183 first (no participant): does Canvas accept a rubric association on a New Quiz assignment? With a participant: does the written rubric assessment coexist with the quiz's own auto-scored points, or does one overwrite the other, and what does the student see?

**Q4. Is an overall grade write honest and stable?** With a participant who has auto-scored MC points plus written items: write the assignment grade through REST. Does New Quizzes overwrite it later (open the attempt, regrade, a second attempt, or auto-regrade)? Does SpeedGrader show a quiz total that disagrees with the item breakdown? **Do not leave a grade that misstates item scores**; the probe grade is on the participant's synthetic attempt only and is reset afterwards.

**Q5. Drafts on a real attempt.** Re-check GraphQL draft comments with attempt 1 and attempt 2 on a real New Quiz attempt, including a file-upload response, and what the student sees after posting.

## Fixture

- New unpublished `[CSE FIELD PROBE]` quiz in CS8 pushed through `cse push-quiz`: one MC (1 pt), one essay (5), one file upload (3), **two attempts allowed**, result viewing set. Existing probe quiz 3698183 can serve Q3 pre-check only.
- To take it, the quiz must be published and visible to one participant only (teacher-created override or due-date visibility). **Teacher decides and does the publish and participant steps**; the Engine never publishes without explicit authorization. Participant is disposable, identified by alias, and the teacher reads nothing private.
- Participant submits twice: attempt 1 (short essay, small text file), attempt 2 (revised essay). Teacher scores one essay by hand in SpeedGrader between attempts, so Q2 has something to observe.

## Rules

CS8 only. Own `[CSE FIELD PROBE]` objects only. Probes go through `cse` and `cse-probe`, not the OG `canvas-expert` MCP. A temporary diagnostic program may print status and body only, never the token, and is deleted afterwards. No delete or course-policy writes. Record each result with its evidence level (Canvas Live API storage, rendering by human view, local tests) in `docs/engine-capabilities.md` and `internal/localui/capabilities.go` together.

## Decision after the probe

| Result | CE-SE does |
|---|---|
| Q1 readable, Q2 or Q3 writable and Q4 stable | Pull responses, assess, write item scores or rubric assessment, post the honest computed grade through the review page |
| Q1 readable, nothing safely writable | Pull and assess; post teacher-only **draft comments** (Q5) plus a scoring sheet; the teacher finalizes in SpeedGrader |
| Q1 not readable | Scoring sheet from teacher-supplied responses only; keep written items for authoring |

Whichever row wins, the review-before-post default and the rule never to post a grade that misstates item scores still apply.

## Teacher input needed

1. A disposable participant (or the teacher's own second account) able to take a New Quiz in CS8.
2. Permission for the temporary publish limited to that participant, and who does it.
