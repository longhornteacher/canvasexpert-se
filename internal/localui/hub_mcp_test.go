package localui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/mcpserver"
	"canvasexpertse/internal/workspace"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCourseSaveVisibleToConnectedMCPWithoutRestart(t *testing.T) {
	h, root := studentsHub(t)
	h.savedOriginFn = func() string { return "" }
	config := domain.CourseConfig{Schema: 1, Instance: "https://school.example", Courses: []domain.Course{{Nickname: "CS8", ID: 11, Current: true, Timezone: "America/Chicago"}, {Nickname: "OLD", ID: 22}}}
	const body = "Starter file: no courses configured. Preserved teacher notes."
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), config, body); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	server, close := mcpserver.NewServer(ctx, root)
	defer close()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "course-save-test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	read := func() []domain.Course {
		t.Helper()
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_courses", Arguments: map[string]any{}})
		if err != nil || result.IsError {
			t.Fatalf("configured courses: %v %+v", err, result)
		}
		var output mcpserver.TextOutput
		b, _ := json.Marshal(result.StructuredContent)
		if err := json.Unmarshal(b, &output); err != nil {
			t.Fatal(err)
		}
		var saved struct {
			Source, Root, Instance string
			Courses                []domain.Course
		}
		if err := json.Unmarshal([]byte(output.Text), &saved); err != nil {
			t.Fatal(err)
		}
		if saved.Source != "configured" || saved.Root != root || saved.Instance != config.Instance {
			t.Fatalf("wrong configuration metadata: %+v", saved)
		}
		return saved.Courses
	}
	if saved := read(); len(saved) != 2 || !saved[0].Current || saved[1].Current || saved[0].Timezone != "America/Chicago" {
		t.Fatalf("initial selection: %+v", saved)
	}
	form := url.Values{"request_secret": {"nonce"}, "id": {"11", "22"}, "nick_11": {"CS8"}, "nick_22": {"OLD"}, "active_22": {"on"}}
	if w := hubPost(h, "/courses", form, h.origin); w.Code != http.StatusSeeOther || h.flashErr != "" {
		t.Fatalf("course save: %d %s", w.Code, h.flashErr)
	}
	if saved := read(); len(saved) != 2 || saved[0].Current || !saved[1].Current {
		t.Fatalf("MCP did not see saved active change: %+v", saved)
	}
	if b, err := os.ReadFile(filepath.Join(root, "context", "courses.md")); err != nil || !strings.Contains(string(b), body) {
		t.Fatal("course save lost teacher notes")
	}

	// This MCP client remains connected as the hub enables UserSync and saves the
	// grading voice. Its next logical read must use the active shared authority.
	syncRoot := t.TempDir()
	if w := hubPost(h, "/rules-location", url.Values{"request_secret": {"nonce"}, "sync_root": {syncRoot}}, h.origin); w.Code != http.StatusSeeOther {
		t.Fatalf("select UserSync rules: %d", w.Code)
	}
	voiceForm := url.Values{"request_secret": {"nonce"}, "voice": {"Shared hub voice."}, "feedback_guidance": {"Shared grading preference."}}
	if w := hubPost(h, "/grading", voiceForm, h.origin); w.Code != http.StatusSeeOther {
		t.Fatalf("save shared grading voice: %d", w.Code)
	}
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "read_workspace_file", Arguments: map[string]any{"path": "context/grading.md"}})
	if err != nil || result.IsError {
		t.Fatalf("MCP read of shared grading: %v %+v", err, result)
	}
	var grading mcpserver.TextOutput
	b, _ := json.Marshal(result.StructuredContent)
	if err := json.Unmarshal(b, &grading); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(grading.Text, "mode: sync") || !strings.Contains(grading.Text, "authority_root: "+syncRoot) || !strings.Contains(grading.Text, "Shared hub voice.") || !strings.Contains(grading.Text, "Shared grading preference.") {
		t.Fatalf("connected MCP did not read the hub's active UserSync file: %s", grading.Text)
	}
	if saved := read(); len(saved) != 2 || saved[0].Current || !saved[1].Current {
		t.Fatalf("UserSync changed local course configuration: %+v", saved)
	}
}
