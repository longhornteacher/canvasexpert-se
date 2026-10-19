# Synthetic book-club unit: one request, three individual resources

Synthetic example only. Replace every ID, title and club with teacher-selected
values from the configured course. Ghost members are A and B; Alone members are
C and D; one enrolled student belongs to no club. The shared module already
exists and already holds an unrelated entry.

The teacher asks: "Create the Ghost calendar Page, individual reading response,
and New Quiz for Book Club -> Ghost, in the existing Book Club module."

The agent writes three drafts, prepares each audience, reviews, and pushes. The
Engine resolves Ghost's current members into a recorded individual audience for
each resource. Each student works independently; the club is only the audience.

## 1. Course Page (Ghost calendar)

```yaml
schema: 1
kind: page
operation: create
course: ELA7
title: Ghost calendar
body_format: markdown
published: true
module_id: 401
audience:
  mode: individual
  policy: snapshot
  source:
    group_category_id: 501
    group_ids: [701]
---
# Ghost reading calendar

| Week | Reading | Response due |
|---|---|---|
| 1 | Chapters 1-3 | Friday |
| 2 | Chapters 4-6 | Friday |

Submit each response independently in the assignment.
```

`prepare-audience ELA7 <page file>` then `push <page file>`. The Page is created
unpublished, restricted to Ghost, placed in module 401, and published last.

## 2. Individual reading response (Ghost)

```yaml
schema: 1
kind: assignment
operation: create
course: ELA7
title: Ghost individual reading response
points: 20
submission_types: [online_upload]
published: true
module_id: 401
audience:
  mode: individual
  policy: snapshot
  source:
    group_category_id: 501
    group_ids: [701]
---
Write your own response, using evidence from your reading. Each student submits
independently.
```

`prepare-audience ELA7 <response file>` then `push <response file>`. The
assignment is created unpublished, restricted to Ghost, placed in module 401,
and published last.

## 3. New Quiz (Ghost reading check)

```text
<QUIZFORGE_JSON>
{
  "quizforge_version": "3.0-json",
  "title": "Ghost reading check",
  "instructions": "<p>Answer on your own. Your club reads the same book, but this check is individual work.</p>",
  "metadata": {"canvas": {"published": true, "module_id": 401, "audience": {"mode": "individual", "policy": "snapshot", "source": {"group_category_id": 501, "group_ids": [701]}}}},
  "items": [
    {"id": "q1", "type": "MC", "points": 2,
     "prompt": "<p>Which detail describes a setting?</p>",
     "choices": [
       {"id": "a", "text": "The quiet library at sunset", "correct": true},
       {"id": "b", "text": "The character decides to apologize", "correct": false}
     ]},
    {"id": "q2", "type": "ESSAY", "points": 5,
     "prompt": "<p>Describe how a setting can influence a character's decision.</p>",
     "word_limit_max": 150,
     "rubric_hint": "Assess a clear explanation and a relevant example."}
  ]
}
</QUIZFORGE_JSON>
```

`prepare-audience ELA7 <quiz file>` then `push-quiz ELA7 <quiz file>`. The quiz
is created, its questions are added, settings and module placement are applied,
the audience is applied, and publication happens last.

## What the result looks like

- Module 401 keeps its existing entry and gains three native entries in order:
  the Page, the response assignment, and the quiz's associated assignment.
- All three resources are restricted to the same Ghost students. Alone members
  and the unassigned student have no access.
- No resource sets native `group_category_id`, uses a collaborative group
  override, or changes club membership.

## Staged failure and resume

If the quiz publication is refused, the quiz stays unpublished with its exact ID
saved. Rerun the same file: the Engine repeats only the unfinished step, so the
questions, settings, module entry and audience row are not duplicated. The same
holds for a refused Page or assignment audience: that resource stays unpublished
and resumable while the other two continue independently.

## Membership refresh with a protected removal

When Ghost's membership changes, run `prepare-audience ELA7 <saved file or latest
action> --refresh` for each resource and push the returned action path.

- A student with a real submission, attempt or grade keeps access by default,
  and the result reports the actual applied audience separately from current
  club membership.
- Safe removals for other students still proceed in the same row.
- The refresh changes only who can see the resource. It sends no Page body, quiz
  question, quiz setting or attempt change.
- Pages have no submission evidence route, so a Page refresh removes a departing
  student even if they viewed the Page. Say so plainly rather than implying work
  protection.

Do not recreate a resource or change club membership to refresh access.
