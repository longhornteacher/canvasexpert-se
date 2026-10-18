---
schema: 1
kind: results
packet: courses/CS8/work/20261018T120000Z/packet.yaml
assignment:
  instance: https://school.instructure.com
  course_id: 101
  assignment_id: 7401
  title: Evidence Paragraph
  points: 10
  submission_types: [online_text_entry]
  grading_type: points
  published: true
  due_at: "2026-10-14T18:00:00-05:00"
students:
  - student_ref: s-ada
    operation: late-only
    late_only:
      attempt:
        attempt: 1
        submitted_at: "2026-10-16T09:00:00-05:00"
      late_timing:
        status: late
        seconds_late: 54000
        due_at: "2026-10-14T18:00:00-05:00"
        basis: "15 school hours late after the documented one-school-day accommodation."
        school_time:
          attempt:
            attempt: 1
            submitted_at: "2026-10-16T09:00:00-05:00"
          submitted_at: "2026-10-16T09:00:00-05:00"
          deadline: "2026-10-15T18:00:00-05:00"
          status: late
          seconds_late: 54000
        observed:
          status_known: true
          late_policy_status: null
          late_known: true
          late: false
          seconds_late_known: true
          seconds_late: 0
          custom_status_known: true
          has_custom_status: false
---

This synthetic row shows the reviewed result shape after `cse prepare-late-timing` resolved the decision against the referenced packet. Canvas due is Wednesday at 6 p.m.; the documented accommodated deadline is Thursday at 6 p.m.; first genuine work arrives Friday at 9 a.m. The Engine counts Thursday 6 hours plus Friday 9 hours, or 54,000 seconds. The `attempt`, `school_time`, and `observed` values come from the packet and preparation; the agent reviews them before using the row.

The example does not set a grade, comments or rubric. `late_policy_status` and `seconds_late_override` are the only Canvas fields this operation changes. Group-mode shape uses `late_only.late_timing`, complete `observed_members` and (when school-time provenance is present) `member_school_time` keyed by each member's stable ref.
