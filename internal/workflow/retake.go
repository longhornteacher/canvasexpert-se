package workflow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// A tutorial retake is a New Quiz whose instructions carry a hidden marker
// naming the original assignment's Canvas ID. Canvas is the record: any PC
// finds the pair by reading the course, and a renamed original keeps its retake.

const retakeTitlePrefix = "TUTORIAL-ONLY RETAKE - "

var retakeMarkerPattern = regexp.MustCompile(`data-cse="retake-for"\s+data-original="(\d+)"`)

// retakeMarker is the hidden instructions marker for the original's ID.
func retakeMarker(originalID int64) string {
	return fmt.Sprintf(`<span data-cse="retake-for" data-original="%d" style="display:none"></span>`, originalID)
}

func retakeTitle(originalName string) string { return retakeTitlePrefix + originalName }

// retakeOriginalID is the original named by a retake's instructions, or 0.
func retakeOriginalID(description string) int64 {
	m := retakeMarkerPattern.FindStringSubmatch(description)
	if m == nil {
		return 0
	}
	id, _ := strconv.ParseInt(m[1], 10, 64)
	return id
}

// isRetake reports whether the assignment is a tutorial retake.
func isRetake(a Assignment) bool {
	return retakeOriginalID(a.Description) > 0 || strings.HasPrefix(a.Name, retakeTitlePrefix)
}

// findRetakes returns every retake of the original in the list. The marker is
// the pairing. A retake without one (New Quiz instructions that did not keep
// it) pairs by title, and only when exactly one non-retake assignment carries
// the original's name, so two originals with one name never share a retake.
// The pairing rule wants exactly one; callers hold on any other count.
func findRetakes(all []Assignment, original Assignment) []Assignment {
	var out []Assignment
	sameName := 0
	for _, a := range all {
		if !isRetake(a) && a.Name == original.Name {
			sameName++
		}
	}
	for _, a := range all {
		switch id := retakeOriginalID(a.Description); {
		case id > 0:
			if id == original.ID {
				out = append(out, a)
			}
		case a.Name == retakeTitle(original.Name) && sameName == 1:
			out = append(out, a)
		}
	}
	return out
}

// retakeFor resolves the one retake for an original, or explains why not.
func retakeFor(all []Assignment, original Assignment) (Assignment, error) {
	found := findRetakes(all, original)
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return Assignment{}, fmt.Errorf("no tutorial retake is paired with %q (assignment %d)", original.Name, original.ID)
	}
	var ids []string
	for _, a := range found {
		ids = append(ids, strconv.FormatInt(a.ID, 10))
	}
	return Assignment{}, fmt.Errorf("%d tutorial retakes are paired with %q (%s); delete or fix all but one", len(found), original.Name, strings.Join(ids, ", "))
}

// retakenOriginals is the set of assignment IDs that have a retake in the list.
func retakenOriginals(all []Assignment) map[int64]bool {
	out := map[int64]bool{}
	for _, a := range all {
		if isRetake(a) {
			continue
		}
		if len(findRetakes(all, a)) > 0 {
			out[a.ID] = true
		}
	}
	return out
}
