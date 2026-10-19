# "Once this hits for real…" checks

The 2026-10-17 CS8 run (see [README](README.md#live-results-2026-10-17-cs8)) proved the Engine paths with real Canvas objects, but a Test Student behaves differently from real students and several student-facing behaviors cannot be seen from a teacher token. These are the checks to make the first time each feature is used for real. None is a gate for using the feature. Each one names what to look at, what a good result looks like, and what to tell the agent if it does not.

Do them with the first real club or assignment, and note the result in [engine-capabilities.md](../engine-capabilities.md). Skip any you do not use.

## Clubs and memberships

- **Once students are in clubs:** open Canvas as one club member (or ask a student) and confirm they see their own club, its Page and its discussion, and not another club's. Good: only their own. Not good: another club's Page or topic is visible; stop and tell the agent.
- **Once a move or removal has happened:** confirm the moved student no longer sees the old club's Page edit button and sees the new club's. Student notifications for joining or leaving a group may arrive; that is Canvas, not CE-SE.
- **Once self sign-up is turned on:** confirm a student can join a club themselves and that a full club (with a limit) refuses the next student. Even allocation moves every unassigned student, so check the unassigned list first.

## Pages and discussions

- **Once a club Page has `editing_roles: [members]`:** have a member edit it, then run the Page pull and confirm the agent sees the student's text and does not overwrite it.
- **Once a distributed discussion is published:** confirm each club sees only its own conversation and that a student post in one club does not appear in another.
- **Once students have posted:** run the discussion pull. Good: every student post is attributed to the right alias, replies are nested, nothing is marked unknown. Not good: posts attributed to nobody; the pull is partial and the agent must say so instead of drawing conclusions.
- **Teacher replies:** after the first reply, confirm it appears once, under the intended post, as the teacher. A teacher's own post may be labelled "unknown" in the pull; that is expected.

## Group assignments and shared grading

- **Once a group assignment is published:** confirm only the intended clubs see it and that the club due date shows for members. Check `pull-assignment-dates` agrees with what a student sees.
- **Once a club submits:** run `pull-work COURSE ASSIGNMENT_ID`. Good: one artifact, every member listed with their own attempt, `shared_assessment_status: ready`. If a member did not appear, or the group says held, read the reason before assessing.
- **Before the first shared grade:** confirm in SpeedGrader that "assign grades to each student individually" is off. After posting, open two members in SpeedGrader: both should show the same score and rubric. Good: same grade, rubric, and one shared comment each. Not good: only one member graded; tell the agent, and do not repeat the post.
- **Private comments:** after posting a private comment, confirm only that student's submission shows it.
- **A late club:** after a late shared grade, look at one member's score. Canvas applies the course late policy, so the score can be lower than the entered grade. That is expected; the Engine never calculates a penalty.
- **A club with a changed roster after submitting:** the Engine should hold the shared grade and say why. Reassess after pulling again.
- **An excused or absent member:** confirm the pull lists them and that the shared grade is held until you decide what should reach them.

## Individual grades on a shared product

- **Once a "grade each student individually" assignment has a club submission:** after posting, open each member in SpeedGrader. Good: different grades and rubrics per student, each with their own comment, and no comment copied to clubmates.
- **After a mid-run settings change:** if the grading mode is flipped in Canvas, remaining students should be held, not graded.

## Graded discussions

- **Once a graded discussion is created:** confirm in Canvas that the gradebook column is individual and that the rubric is attached. A distributed topic should show a separate conversation per club under one column.
- **Once students post:** run `pull-work` on the discussion's assignment. Good: each student's own posts and a digest; the whole thread as context. Check one student by hand against what they wrote.
- **Edited posts:** have a student edit a post after the pull and confirm the Engine holds only that student on posting.
- **Checkpointed discussions:** if a discussion uses Canvas checkpoints, expect the grade to be held and graded in SpeedGrader. If that surprises you, tell the agent.
- **The review page:** it opens in your browser with every card approved by default. Revise or deselect any card you do not want before clicking Post. If it ever posts without a click from you, close it and tell the agent.

## Navigation and external links

- **Once a unit module is published:** confirm the module shows the discussion and the club links, and that a student outside a club who opens a club link is refused. A module link never grants access.

## If something looks wrong

Stop posting, keep the results file, and tell the agent what you saw in Canvas. The Engine keeps accepted outcomes in the results file so it will not repeat a write; the file is the record of what was sent.
