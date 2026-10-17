# 14 — Group conference sign-up

**Tier C: built only when the teacher requests this capability; no other slice depends on it.** Before coding, confirm Scheduler is available on the intended Canvas; published appointment groups cannot be unpublished. Requires: [01](01-groups.md). Result: offer teacher conference slots that a club reserves as one group and inspect which clubs have booked.

## Contract and surface

Add `kind: appointments`, `operation: create|update`, exact course/group-set identity, title/description, location, `published`, visibility, booking limits and keyed start/end slot pairs. `published` defaults false. Use validate/push/push_document; optional ordinary content review can reuse the page without introducing an authorization token.

```yaml
schema: 1
kind: appointments
operation: create
course: ELA7
group_category_id: 501
title: Book club conferences
published: false
participant_visibility: private
participants_per_appointment: 1
max_appointments_per_participant: 1
slots:
  - key: tuesday-1
    start_at: '2026-11-03T10:00:00-06:00'
    end_at: '2026-11-03T10:10:00-06:00'
```

The participant is a club, not a count of individual students. Verify the group's course and pass the course context plus single group-category subcontext so Canvas uses group sign-up. Persist the appointment-group ID and actual child-event IDs corresponding to each slot. Existing group signups are read with `appointments COURSE APPOINTMENT_GROUP_ID` / `list_group_appointments {course, appointment_group_id}`; `pull-appointments` / `pull_group_appointments` supplies an update baseline.

## Behavior

Create/update through `/api/v1/appointment_groups`; read appointments/reservations through the documented returned child events. Publication is a consequential one-way Canvas setting: published appointment groups cannot be unpublished. Validate a requested false transition locally and explain the real limit rather than promising rollback. [Appointment Groups](https://developerdocs.instructure.com/services/canvas/resources/appointment_groups).

Existing update documents may edit requested metadata, publish a draft, or append explicit new keyed slots. Omission never deletes a slot. This slice does not delete/reschedule reserved slots or cancel student bookings; the teacher handles those changes in Canvas. List the affected slot/reservation and preserve it. This limitation does not prevent creating additional available slots.

Students normally reserve their own slot. If the teacher explicitly requests booking for a particular club, support `kind: appointment-reservation`, `operation: reserve`, exact appointment-group/child-event/group identity and full observed member audience. Use `POST /api/v1/calendar_events/:event_id/reservations/:participant_id` with the verified group ID as participant for a Group appointment, and explicit `cancel_existing=false`. Do not fall back to the current teacher/user or cancel another booking to make room. Persist the accepted reservation outcome. No automatic booking or cancellation as a side effect of unit setup. [Reservation route](https://developerdocs.instructure.com/services/canvas/resources/calendar_events).

## Implementation and acceptance

Add feature-specific domain/workflow files and normalized read output using stable refs/group labels. Missing Scheduler permission/availability is reported specifically; other group-work features continue.

- Draft creation saves group and slot IDs; resuming never appends the same slots again; visibility and booking limits pass exactly.
- Published-to-unpublished changes and edits that would delete or recreate a booked slot are rejected with the plan preserved.
- A wrong-course/set/child-event or incompatible participant is refused before any reservation write; an explicit reservation is not retried after an ambiguous timeout.
- Unavailable reservation details are reported as unknown, not as empty slots.

Document which actions CE-SE supports and the exact Canvas steps for a held reserved-slot change. Do not treat unverified district availability as a global group-work prohibition.
