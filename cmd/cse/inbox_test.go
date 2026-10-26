package main

import (
	"reflect"
	"testing"

	"canvasexpertse/internal/workflow"
)

func TestParseInboxListOptions(t *testing.T) {
	options, err := parseInboxListOptions([]string{"--unread", "--course", "ELA7", "--scope", "archived"})
	if err != nil || options != (workflow.InboxListOptions{Scope: "archived", Unread: true, Course: "ELA7"}) {
		t.Fatalf("unexpected Inbox options: %+v err=%v", options, err)
	}
	for _, args := range [][]string{
		{"--scope", "all"},
		{"--course"},
		{"--unread", "--unread"},
		{"--other"},
	} {
		if _, err := parseInboxListOptions(args); err == nil {
			t.Fatalf("accepted invalid Inbox list options %q", args)
		}
	}
}

func TestParseInboxSetState(t *testing.T) {
	refs, state, err := parseInboxSetState([]string{"conversation-a", "conversation-b", "--state", "archived"})
	if err != nil || state != "archived" || !reflect.DeepEqual(refs, []string{"conversation-a", "conversation-b"}) {
		t.Fatalf("unexpected Inbox state request: refs=%q state=%q err=%v", refs, state, err)
	}
	for _, args := range [][]string{
		{"conversation-a", "--state", "deleted"},
		{"--state", "read"},
		{"conversation-a", "--state"},
	} {
		if _, _, err := parseInboxSetState(args); err == nil {
			t.Fatalf("accepted invalid Inbox state request %q", args)
		}
	}
}
