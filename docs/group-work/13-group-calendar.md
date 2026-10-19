# 13 — Club meeting calendars

**Tier C: built only when the teacher requests this capability; no other slice depends on it.** Requires: [01](01-groups.md). Result: create and edit club meeting/checkpoint events without changing assignment deadlines or setting up a background scheduler.

## Surface and document

Add `kind: calendar-event`, `operation: create|update`, course/group identity, title, body format/description, RFC3339 start/end, optional location and exact observed event baseline. Use validate/push/push_document. Add `calendar COURSE --group ID --from DATE --to DATE` / `list_group_events {course, group_id, start_date, end_date}` and `pull-event COURSE EVENT_ID --group ID` / `pull_group_event` for an editable baseline.

```yaml
schema: 1
kind: calendar-event
operation: create
course: ELA7
group_id: 701
title: Club meeting 1
body_format: markdown
start_at: '2026-11-03T10:00:00-06:00'
end_at: '2026-11-03T10:25:00-06:00'
location_name: Classroom
```

The teacher/agent resolves actual meeting dates and daylight-saving offsets using the course timezone. Repeated meetings can be separate keyed files generated from the teacher's schedule; no new RRULE parser or unattended automation is needed. Explicit all-day support may use a date with `all_day: true`; do not infer an all-day event from missing times.

## Implementation

Add domain/workflow calendar feature files. Creation uses the verified group's `context_code`; GET/PUT use the exact event ID and verify its returned context. Store event ID, URL, original requested fields and outcomes. Do not permit a plan to move an event to another context or edit an assignment masquerading as a calendar event. Listing has a bounded teacher-selected date range and follows pagination. Calendar context and event parameters are documented in [Calendar Events](https://developerdocs.instructure.com/services/canvas/resources/calendar_events).

Update only requested title, description, time or location fields from a pulled baseline. A conflict preserves the unsent draft and gives a fresh baseline. An event change does not modify an assignment's due date. An accepted create/edit is recorded once; timeout recovery uses known IDs or bounded matching with the full context/time/title identity, not title alone.

Explain group-calendar visibility as a membership-based Canvas surface; do not promise push notifications or calendar subscription on every student's device. A schedule or content description containing private teacher notes is not publishable student content.

## Acceptance

- Two club events go to their own group calendars; a wrong-course/group or mismatched event context causes no write.
- A rescheduled live event holds a stale time replacement while a location-only edit preserves the new time; explicit offsets survive a daylight-saving transition.
- Calendar edits never write assignment deadlines or grades, and no timer/background job is installed.

Add teacher guidance linking meeting events to the unit plan while keeping graded deadlines in assignments.
