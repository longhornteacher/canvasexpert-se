# 15 — Explicit club conversation messages

**Tier C: built only when the teacher requests this capability; no other slice depends on it.** This slice is about creating new club conversations, a distinct path from the teacher's existing Inbox read/reply/state operations. Sending to students is high consequence; the teacher can message in Canvas, so build this only for an explicit need. Requires: [01](01-groups.md), [02](02-memberships.md). Result: send a teacher-requested club message with a clear distinction between private individual messages and a shared club thread.

## Contract

Add `kind: conversation`, `operation: create|reply`, course/group identity, explicit `mode: private_each|club_thread`, observed recipient refs, subject for create, and plain text body. A reply also requires exact conversation ID and observed participants. Use validate/push/push_document; optional review can show the concrete body/audience through existing `review`/`start_review`.

```yaml
schema: 1
kind: conversation
operation: create
course: ELA7
group_id: 701
mode: club_thread
subject: Prepare for our next club meeting
recipient_refs: [student-ref-a, student-ref-b]
```

Body text is the document body. Teacher-only notes, parent contacts, and grading accommodations are never merged into it. Recipient membership must match the explicitly selected snapshot; a subset message must name that subset, and its mode still controls who sees replies. Course-wide/group-address shorthand is not used in payloads where it could expand the reviewed recipients at send time.

## Sending and reads

Use `/api/v1/conversations` with privately resolved user IDs. `private_each` sends one request per recipient with `group_conversation=false` and per-recipient outcomes; `club_thread` sends the exact cohort with true and one audience outcome. This avoids treating a partially completed bulk send as wholly unsent. Replies use the existing conversation's message endpoint with explicit recipients after verifying current participants. Do not automatically add new club members to old conversations. [Conversations API](https://developerdocs.instructure.com/services/canvas/resources/conversations).

A request to draft, plan a unit, create a group, or summarize work is not authorization to send. Explicit teacher sending scope is required; do not add a second confirmation step when already supplied. The future playbook must distinguish “draft this message” from “send this message to these clubs.” This specification itself sends nothing.

Add `club-messages COURSE --group ID` / `list_club_messages` for matching conversations actually visible to the teacher account, and `pull-conversation COURSE CONVERSATION_ID` / `pull_club_conversation` for a selected visible thread. A course/group filter is not proof the teacher can inspect students' private conversations; never claim all club DMs are visible. Normalize identities through existing student refs and retain exact conversation/message IDs.

## Recovery and acceptance

Save conversation/message IDs and accepted outcomes. A repeated complete send creates no new messages. An ambiguous send cannot be resolved merely by finding any conversation with the same subject; use exact actor/participants/message evidence or hold for teacher inspection. Do not resend everything because the final member failed. No new polling scheduler, automatic reminders, message deletion, attachment transport, or parent-email integration in this slice.

- Private mode never creates a common thread; club mode reaches only the selected members; a participant change holds the pending send instead of widening it.
- Wrong-course refs and a reply to an unrelated conversation are rejected; draft/review calls send nothing.
- An accepted first recipient and failed second recipient resume independently without duplication.
- Reads show only teacher-visible conversations and label incomplete access.

Record local adapter/audience checks separately from actual Canvas delivery and student notification evidence.
