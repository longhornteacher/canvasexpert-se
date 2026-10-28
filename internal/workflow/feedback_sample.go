package workflow

import "fmt"

// FeedbackSample deliberately excludes submission content, teacher notes and
// private contacts. Comment is the same text used by posting and Draft PDF.
type FeedbackSample struct {
	StudentRef string `json:"student_ref"`
	Alias      string `json:"alias"`
	Hold       string `json:"hold"`
	Comment    string `json:"comment,omitempty"`
}

func BuildFeedbackSample(root, path, ref string) (FeedbackSample, error) {
	kind, err := ReviewInputKind(path)
	if err != nil {
		return FeedbackSample{}, err
	}
	if kind != "results" {
		return FeedbackSample{}, fmt.Errorf("--show requires a results document, not %q", kind)
	}
	model, err := BuildReview(root, path)
	if err != nil {
		return FeedbackSample{}, err
	}
	for _, card := range model.Cards {
		if card.Ref == ref {
			sample := FeedbackSample{StudentRef: ref, Alias: card.Alias, Hold: card.Hold}
			if sample.Hold == "" {
				sample.Comment = card.Comment
			}
			return sample, nil
		}
	}
	return FeedbackSample{}, fmt.Errorf("student_ref %q is not in this results document", ref)
}
