package workflow

import (
	"strings"
	"testing"
)

const fbGreen = `<p style="color: #2e7d32;"><strong>`
const fbRed = `<p style="color: #c62828;"><strong>`

func fbQuiz(t *testing.T, items, rationales string) (QuizForge, error) {
	t.Helper()
	return ReadQuizForge(writeQF(t, `{"title":"Feedback","items":[`+items+`],"rationales":[`+rationales+`]}`))
}

func feedbackTexts(t *testing.T, p map[string]any) []string {
	t.Helper()
	var out []string
	for _, v := range entryOf(p)["answer_feedback"].(map[string]any) {
		out = append(out, v.(string))
	}
	return out
}

func TestMCFeedbackIsGreenForCorrectAndRedForDistractors(t *testing.T) {
	q, err := fbQuiz(t,
		`{"id":"q","type":"MC","points":1,"prompt":"p","choices":[{"id":"a","text":"A","correct":true},{"id":"b","text":"B"}]}`,
		`{"item_id":"q","rationale":"General note.","choices":[{"id":"a","rationale":"It fits."},{"id":"b","rationale":"It does not fit."}]}`)
	if err != nil {
		t.Fatal(err)
	}
	p := q.itemPayloads()[0]
	got := strings.Join(feedbackTexts(t, p), "\n")
	for _, want := range []string{
		fbGreen + `Why this is the best answer:</strong> It fits.</p>`,
		fbRed + `Why this isn't the best answer:</strong> It does not fit.</p>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	if n := entryOf(p)["feedback"].(map[string]any)["neutral"]; n != "<p>General note.</p>" {
		t.Errorf("MC general rationale must stay uncolored: %v", n)
	}
}

func TestMAFeedbackLabels(t *testing.T) {
	q, err := fbQuiz(t,
		`{"id":"q","type":"MA","points":2,"prompt":"p","choices":[{"id":"a","text":"A","correct":true},{"id":"b","text":"B","correct":true},{"id":"c","text":"C"}]}`,
		`{"item_id":"q","choices":[{"id":"a","rationale":"One."},{"id":"b","rationale":"Two."},{"id":"c","rationale":"Three."}]}`)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(feedbackTexts(t, q.itemPayloads()[0]), "\n")
	for _, want := range []string{
		fbGreen + `Why this choice is correct:</strong> One.</p>`,
		fbGreen + `Why this choice is correct:</strong> Two.</p>`,
		fbRed + `Why this choice isn't correct:</strong> Three.</p>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

func TestMissedChoiceItemsExplainTheCorrectAnswer(t *testing.T) {
	q, err := fbQuiz(t,
		`{"id":"m","type":"MC","points":1,"prompt":"p","choices":[{"id":"a","text":"<em>Simile</em>","correct":true},{"id":"b","text":"B"}]},`+
			`{"id":"x","type":"MA","points":2,"prompt":"p","choices":[{"id":"a","text":"A","correct":true},{"id":"b","text":"B","correct":true},{"id":"c","text":"C"}]}`,
		`{"item_id":"m","choices":[{"id":"a","rationale":"It uses like."},{"id":"b","rationale":"No."}]},`+
			`{"item_id":"x","choices":[{"id":"a","rationale":"One."},{"id":"b","rationale":"Two."},{"id":"c","rationale":"Three."}]}`)
	if err != nil {
		t.Fatal(err)
	}
	p := q.itemPayloads()
	if got, want := entryOf(p[0])["feedback"].(map[string]any)["incorrect"], fbGreen+`Why “Simile” is the best answer:</strong> It uses like.</p>`; got != want {
		t.Errorf("MC incorrect feedback = %v, want %v", got, want)
	}
	want := fbGreen + `Why “A” is a correct choice:</strong> One.</p>` + fbGreen + `Why “B” is a correct choice:</strong> Two.</p>`
	if got := entryOf(p[1])["feedback"].(map[string]any)["incorrect"]; got != want {
		t.Errorf("MA incorrect feedback = %v, want %v", got, want)
	}
}

func TestTFFeedbackIsNeutralGreen(t *testing.T) {
	q, err := fbQuiz(t, `{"id":"q","type":"TF","points":1,"prompt":"p","answer":true}`, `{"item_id":"q","rationale":"It is so."}`)
	if err != nil {
		t.Fatal(err)
	}
	want := fbGreen + `Why this is correct:</strong> It is so.</p>`
	if n := entryOf(q.itemPayloads()[0])["feedback"].(map[string]any)["neutral"]; n != want {
		t.Errorf("got %v", n)
	}
}

func TestRationaleInnerStripsAuthoredFormatting(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`<p style="color: #2e7d32;"><strong>Why this is correct:</strong> X</p>`, "X"},
		{`<span style="color:red">Plain</span>`, "Plain"},
		{`<p>One.</p><p>Two.</p>`, "One.<br>Two."},
		{`<p>Keep <em>this</em>.</p>`, "Keep <em>this</em>."},
		{`Just text`, "Just text"},
	} {
		if got := rationaleInner(tc.in); got != tc.want {
			t.Errorf("rationaleInner(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAutoScoredItemsNeedStudentFeedback(t *testing.T) {
	mc := `{"id":"q","type":"MC","points":1,"prompt":"p","choices":[{"id":"a","text":"A","correct":true},{"id":"b","text":"B"}]}`
	tf := `{"id":"q","type":"TF","points":1,"prompt":"p","answer":true}`
	for _, tc := range []struct{ name, items, rat, want string }{
		{"missing distractor", mc, `{"item_id":"q","choices":[{"id":"a","rationale":"It fits."}]}`, "choice b"},
		{"missing correct choice", mc, `{"item_id":"q","choices":[{"id":"b","rationale":"No."}]}`, "choice a"},
		{"missing TF rationale", tf, ``, "rationale"},
		{"no id", `{"type":"TF","points":1,"prompt":"p","answer":true}`, ``, "needs an id"},
	} {
		if _, err := fbQuiz(t, tc.items, tc.rat); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want mention of %q", tc.name, err, tc.want)
		}
	}
}

func TestWrittenItemsNeedNoRationale(t *testing.T) {
	_, err := fbQuiz(t, `{"id":"e","type":"ESSAY","points":5,"prompt":"Explain."},{"id":"f","type":"FILEUPLOAD","points":3,"prompt":"Upload."}`, ``)
	if err != nil {
		t.Fatal(err)
	}
}
