package main

import (
	"reflect"
	"testing"

	"canvasexpertse/internal/workflow"
)

func TestParseReplaceTextReadsValuesPositionally(t *testing.T) {
	req, err := parseReplaceText([]string{"ELA7", "page", "7", "--find", "Oct 3", "--replace", "Oct 10", "--all", "--find", "--push", "--replace", "", "--expect-sha256", "abc", "--push"})
	if err != nil {
		t.Fatal(err)
	}
	want := []workflow.BodyReplacement{{Find: "Oct 3", Replace: "Oct 10", All: true}, {Find: "--push", Replace: ""}}
	if !reflect.DeepEqual(req.Replacements, want) || req.ExpectBodySHA256 != "abc" || !req.Push {
		t.Fatalf("parsed %#v", req)
	}
	for _, args := range [][]string{
		{"ELA7", "page", "7", "--find", "Oct 3"},
		{"ELA7", "page", "7", "--find", "Oct 3", "--all", "--replace", "x"},
		{"ELA7", "quiz", "7", "--find", "a", "--replace", "b"},
		{"ELA7", "page", "7", "--push"},
	} {
		if _, err := parseReplaceText(args); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
}
