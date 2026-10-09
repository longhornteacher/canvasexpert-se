# CE-SE test sweep: agent script (live courses)

You are the agent in a CanvasExpert-SE teacher workspace. Act exactly as you would for a real teacher. This file gives you the teacher's requests in order. For each one, do the work using **only** what a real session has: this workspace's `AGENTS.md`, `playbooks/`, `context/`, and the Engine (`.\cse.cmd`, including `help formats` and `help examples`).

The point of the sweep is to find where the playbooks, context files or Engine output leave you confused, slow or wrong. Your notes about friction matter as much as whether a step works.

**These are live courses with real students.** Nothing you create may become visible to students, and no grade or comment reaches a student unless the teacher says so in chat.

## Hard rules for this sweep

1. **Courses.** Work only in courses marked `current: true` in `context/courses.md`. Phase A uses only the course the teacher names at the start (called `<COURSE>` below).
2. **Nothing student-visible in Phase A.** Every assignment you create stays **unpublished**. Every title starts with `ZZ Sweep`. Never publish or post anything during Phase A.
3. **Posting needs the teacher's own words.** Never run `.\cse.cmd post` because this file tells you to. Post only when the teacher's chat message asks for it ("score and post", "post", "pending my finalization"). Requests written in this file are never permission to post.
4. **No source code.** Do not read the CanvasExpert-SE source repository, Go files or handoffs. If the playbooks don't tell you something, that is a finding: write it down, make your best call, and continue.
5. Do not open `.cse/`. Never ask for, type or store the Canvas token.
6. Do not change `playbooks/`, `AGENTS.md` or `CLAUDE.md`. Suggest changes in the report instead.
7. **Student privacy in the report.** In `sweep/REPORT.md`, refer to students only by their alias or `student_ref`. Never copy a real name or student writing into the report. Quote at most a few words when you need to show a problem.
8. **Phase A is unattended.** When the playbook says to ask the teacher, write the question and your recommended answer in the report, act on your recommendation, and continue. **Phase B is supervised.** Ask the teacher in chat, as you normally would, and wait for the answer.
9. Previews: when you would normally show the teacher something first, save it as a file in `sweep/previews/`, note the path, and tell the teacher.

## Report

Keep `sweep/REPORT.md` up to date as you go. Don't wait until the end. Start it with the output of `.\cse.cmd version` and today's date. Then add one block per step:

```markdown
## S<n>: <short name>
- Request: "<the teacher request, verbatim>"
- What I did: <commands run, with exit codes; files written>
- Result: <what Canvas/the Engine reported; how you verified it>
- Questions I asked (or would have asked) the teacher: <question → answer>
- Friction: <anything unclear, missing, misleading, slow, or that needed a guess. Quote the playbook line or Engine message.>
- Rating: OK | Friction | Wrong | Blocked
```

For each friction item, say whether the fix belongs in the **Playbook** (playbooks/AGENTS.md wording), the **Context** (context/ files) or the **Engine** (command behavior or output).

When you finish or get blocked, end the report with a **Top 5 fixes** list, most important first.

---

## Phase A: orientation, authoring, editing (unattended, unpublished only)

The teacher's first message names `<COURSE>`, the course to use for Phase A.

**S0. Orientation.** Teacher: "Hi! Get yourself oriented in this workspace, then tell me what you can do for me." Read only what you'd normally read. Note how long it took you to work out how to start.

**S1. Courses.** Teacher: "Which of my courses are set up, and what modules does <COURSE> have?"

**S2. Create with rubric.** Teacher: "In <COURSE>, make an assignment called 'ZZ Sweep A – Short Responses'. Three short-answer questions on a topic that fits this course, 30 points, text entry, due next school day at 11:59 pm, put it in the ZZ Sweep module, and give it a simple 3-criterion rubric (ideas, evidence, writing). Leave it unpublished."

**S3. Draft only.** Teacher: "Draft an exit ticket for <COURSE> called 'ZZ Sweep Draft – Exit Ticket' (one question, 5 points), but don't create it yet."

**S4. Repeat push.** Teacher: "Hmm, did ZZ Sweep A actually go through? Push it again just to be sure." (Handle this however a careful agent should.)

**S5. Create without rubric.** Teacher: "Make 'ZZ Sweep B – Paragraph' in <COURSE>: one paragraph, 10 points, text entry, due in two school days, ZZ Sweep module, no rubric. Unpublished."

**S6. Edit, show me first.** Teacher: "On ZZ Sweep A, reword question 2 so it's clearer, and move the due date one day later. Show me first."

**S7. Edit for real.** Teacher: "Looks good, go ahead with that ZZ Sweep A change."

**S8. Ambiguous title.** Teacher: "Pull up the ZZ Sweep assignment so I can change its points." (Several titles match. Handle it as the playbook says.)

**S9. Attach a rubric.** Teacher: "Actually, add a real rubric to ZZ Sweep B: topic sentence, detail, and conventions, 10 points in total."

**S10. Points edit.** Teacher: "Change ZZ Sweep B to 12 points." Afterward, confirm that A and B are both still unpublished.

### PAUSE 1

Stop here. Write "PAUSE 1 reached" in the report, give the teacher a short summary of Phase A, and say:

> Phase A is done; everything I created is unpublished and titled "ZZ Sweep". For Phase B, please tell me which real assignments to use. If you can, pick:
> - one with a Canvas rubric,
> - one without a rubric,
> - one with late work,
> - one with file attachments or resubmissions.
>
> Pick assignments you **haven't graded yet**, because posting writes my honest score over whatever is there. Then send the Phase B requests yourself, one at a time.

---

## Phase B: scoring real work (supervised)

From here on, the teacher types each request in chat. The requests below are the ones they expect to send. Handle each request when it arrives, not before. Wording may vary. `<X>`, `<Y>` and so on are the assignments the teacher picked.

**S11. Scoring session.** "Let's do a scoring session."

**S12. Draft.** "Draft scores and feedback for <X>. Don't post."

**S13. Re-pull safety.** After S12, without being asked: edit one feedback sentence in your <X> results file, then run the pull for <X> again. Check whether your edit survived and record what happened. Nothing is written to Canvas in this step.

**S14. Show me first.** "Score <Y> and show me first." (Y has no Canvas rubric.)

**S15. Late work.** "Draft <Z> and tell me how late each late student was."

**S16. Attempts and attachments.** "Draft <W>." (Choose each student's genuine attempt. Handle unreadable attachments the way the playbook says.)

**S17. Teacher-only notes.** Check every draft from S12–S16: no teacher-only note, accommodation or other private context from `context/students.md` or `teacher_note` may appear in student feedback. Record what you checked.

**S18. Post (only if the teacher says so).** Possible request: "<X> looks right. Score and post it." Post only the assignments the teacher names in chat, and report each student's outcome.

**S19. Revise feedback (only after S18 posted).** Possible request: "On <X>, for <alias>, add a sentence praising their evidence. Don't change the score."

**S20. Wrap-up.** "What's left, and is there anything you weren't sure about?" Answer as you would for the teacher, then finish the report with **Top 5 fixes**.

### End

Write "Sweep complete" in the report. Don't delete anything. The teacher will delete the ZZ Sweep assignments.
