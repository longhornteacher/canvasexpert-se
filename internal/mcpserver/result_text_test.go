package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestResultTextKeepsEveryPopulatedField(t *testing.T) {
	no, yes := false, true
	if got := resultText(TextOutput{Text: "body"}); got != "body" {
		t.Fatalf("a result with no metadata must be its body alone: %q", got)
	}
	got := resultText(TextOutput{
		Text: "body", State: "incomplete", Outcome: "partial", ReviewID: "r1", SessionID: "s1", URL: "http://127.0.0.1:1/x",
		ResultPath: "courses/C/results.md", RevisionNotes: []ReviewRevision{{Ref: "ref-1", Note: "shorter"}},
		SetupStatusAvailable: true, CredentialSaved: &no, ValidationSucceeded: &yes, ConnectionTested: &no, SetupOutcome: "held",
	})
	for _, want := range []string{"state: incomplete", "outcome: partial", "review_id: r1", "session_id: s1", "url: http://127.0.0.1:1/x", "result_path: courses/C/results.md", "revision_note: ref-1: shorter", "setup_status_available: true", "credential_saved: false", "validation_succeeded: true", "connection_tested: false", "setup_outcome: held"} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("metadata lacks %q:\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, "\n\nbody") {
		t.Fatalf("metadata must come before the unchanged body:\n%s", got)
	}
}

func TestToolsReturnPlainTextWithoutOutputSchemaOrStructuredCopy(t *testing.T) {
	ctx := context.Background()
	s := &Server{root: t.TempDir()}
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	addReadTool(srv, "echo", "Echo.", func(context.Context, *mcp.CallToolRequest, struct{}) (TextOutput, error) {
		return TextOutput{Text: "line one\nline two with \"quotes\" and C:/path", State: "pending"}, nil
	})
	st, ct := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	_ = s
	tools, err := cs.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].OutputSchema != nil {
		t.Fatalf("tool list should carry no output schema: %+v %v", tools, err)
	}
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{}})
	if err != nil || result.IsError || result.StructuredContent != nil || len(result.Content) != 1 {
		t.Fatalf("expected one plain block and no structured copy: %+v %v", result, err)
	}
	if text := result.Content[0].(*mcp.TextContent).Text; text != "state: pending\n\nline one\nline two with \"quotes\" and C:/path" {
		t.Fatalf("text was altered: %q", text)
	}
}
