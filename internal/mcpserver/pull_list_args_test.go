package mcpserver

import (
	"reflect"
	"testing"
)

func TestPullWorkListArguments(t *testing.T) {
	group := int64(3)
	for _, in := range []PullWorkInput{
		{Course: "CS8", List: true},
		{Course: "CS8", Assignment: "Quiz", Session: true, List: true},
		{Course: "CS8", Assignment: "Quiz", GroupID: &group, Session: true, List: true},
	} {
		if args, err := pullWorkArgs(in); err == nil {
			t.Errorf("%+v was accepted as %v", in, args)
		}
	}
	args, err := pullWorkArgs(PullWorkInput{Course: "CS8", Session: true, List: true})
	if err != nil || !reflect.DeepEqual(args, []string{"pull-work", "CS8", "--session", "--list"}) {
		t.Fatalf("list arguments: %v %v", args, err)
	}
}
