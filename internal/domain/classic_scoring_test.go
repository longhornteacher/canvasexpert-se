package domain

import "testing"

func TestClassicQuestionContracts(t *testing.T) {
	snapshot := AssignmentSnapshot{Instance: "https://canvas.example", CourseID: 1, AssignmentID: 10, Title: "Quiz", Points: MustDecimal("10"), SubmissionTypes: []string{"online_quiz"}, GradingType: "points"}
	zero := MustDecimal("0")
	for _, tc := range []struct {
		name  string
		q     ClassicQuestionResult
		valid bool
	}{
		{"zero", ClassicQuestionResult{QuestionID: 1, Evidence: "missing", Score: &zero, Comment: "No response."}, true},
		{"omitted", ClassicQuestionResult{QuestionID: 1, Evidence: "missing", Comment: "No response."}, false},
		{"unknown", ClassicQuestionResult{QuestionID: 1, Evidence: "unknown", TeacherNote: "Unreadable upload"}, true},
		{"unknown score", ClassicQuestionResult{QuestionID: 1, Evidence: "unknown", TeacherNote: "Unreadable", Score: &zero}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := ClassicQuizResults{Schema: 1, Kind: "classic-quiz-results", Packet: "packet.md", Assignment: snapshot, QuizID: 20, ScoringNotes: "Use prompt.", Students: []ClassicStudentResult{{StudentRef: "s1", Attempt: AttemptRef{Attempt: 1, SubmittedAt: "2026-10-09T12:00:00Z"}, Questions: []ClassicQuestionResult{tc.q}}}}
			if (d.Validate() == nil) != tc.valid {
				t.Fatal("unexpected validation")
			}
		})
	}
	score := MustDecimal("1.005")
	if NormalizeClassicScore(&score, MustDecimal("1.005")) == nil {
		t.Fatal("rounded above maximum accepted")
	}
	score = MustDecimal("1.234")
	if e := NormalizeClassicScore(&score, MustDecimal("2")); e != nil || score.Cmp(MustDecimal("1.23")) != 0 {
		t.Fatal(score, e)
	}
}
