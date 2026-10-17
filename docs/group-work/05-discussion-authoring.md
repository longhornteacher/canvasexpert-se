# 05 — Discussion and announcement authoring

Requires: [01](01-groups.md). **Delivered in two stages.** **05a** (Phase 1): ungraded course discussions, distributed club discussions, direct group topics, announcements, and their edit baselines. **05b** (with Phase 2): the `assignment` subdocument, i.e. linked graded discussions with rubric attachment and module placement. 05a rejects a document carrying `assignment` with a clear "not yet supported" hold; 05b removes that hold. Result: author/edit ordinary course discussions, a common prompt with separate club conversations, or a specific club's discussion/announcement. Graded discussion creation uses the existing assignment review rule.

## Surface and document

Add `kind: discussion`, `operation: create|update` to validate/push/review. Use `pull-discussion` to obtain the editable baseline as well as evidence (slice 06 supplies full evidence). MCP uses `push_document`, `pull_discussion`, and `start_review`.

```yaml
schema: 1
kind: discussion
operation: create
course: ELA7
group_category_id: 501
title: Week 1 - A difficult choice
body_format: html
discussion_type: threaded
require_initial_post: false
published: false
assignment:
  points: 100
  grading_type: points
  due_at: '2026-11-06T23:59:00-06:00'
  rubric: # same authorable rubric shape as ordinary assignments
    title: Literary discussion
    use_for_grading: false
    criteria:
      - id: evidence
        description: Interpretation supported with textual evidence
        points: 100
```

Exactly one scope: plain course; course + collaborative `group_category_id` for distributed club discussions; or course + `group_id` for a topic directly in a club. A direct group topic has no linked graded assignment in this contract. Optional settings: `delayed_post_at`, `lock_at`, `pinned`, and `is_announcement`. Use presence-aware update fields. Announcements are a separate author intent; they cannot carry a grading subdocument. Ordinary rubric validators apply; the short specimen intentionally uses one criterion.

## Implementation

Add feature-specific domain/workflow files. Use course/group discussion POST/PUT routes. A distributed discussion sets the group set on the topic, not on its linked assignment. Save topic ID, linked assignment ID, child topic/group IDs when returned, rubric association and optional course module-item outcome. If the children are created lazily, report them as not yet materialized rather than inventing IDs or marking parent creation failed.

Create unpublished, attach the requested independent rubric to the actual linked course assignment, finish any requested course module placement, then publish/schedule as requested. An attachment or placement failure leaves known resources saved and unpublished. Resuming uses those IDs. Topic title/body edit baselines protect student and teacher content; never replace entries or an unrelated linked assignment.

Review for a graded topic displays topic scope, individual grading, points/rubric, dates and publication. Ordinary ungraded authoring can proceed under the request without a new mandatory gate. Announcements need explicit communication scope; a request to create discussion activities does not imply announcements.

Routes and topic fields: [Discussion Topics](https://developerdocs.instructure.com/services/canvas/resources/discussion_topics). Do not assume every district's discussion UI offers identical settings. This slice does not author checkpoint subassignments.

## Acceptance

- A distributed create posts the group set only on the topic and records the returned linked assignment identity; a direct group topic never creates a gradebook column; mixed scopes are rejected before writes.
- A rubric or placement failure followed by resume reuses the saved topic/assignment and completes only the missing stages.
- A changed topic body holds a replacing edit and never touches student posts; unrelated post-count changes do not block a settings-only edit.
- An announcement and a discussion are not confused during uncertain-create lookup; ordinary assignment review is unchanged.
