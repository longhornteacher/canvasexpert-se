package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/render"
	"canvasexpertse/internal/workflow"
	"canvasexpertse/internal/workspace"
)

func TestValidateShowExactFeedbackOfflineAndPrivate(t *testing.T) {
	root := t.TempDir()
	if err := workspace.WriteDocument(filepath.Join(root, "context", "courses.md"), domain.CourseConfig{Schema: 1, Instance: "https://canvas.example.edu"}, ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "engine-docs", "examples", "synthetic-results-two.md"))
	if err != nil {
		t.Fatal(err)
	}
	var doc domain.ResultsDocument
	path := filepath.Join(root, "results.md")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = workspace.ReadDocument(path, &doc); err != nil {
		t.Fatal(err)
	}
	doc.Students[0].Grade.TeacherNote = "PRIVATE-TEACHER-NOTE"
	workPath := "work.md"
	packet := domain.PacketDocument{Schema: 1, Kind: "packet", Course: "DEMO", Assignment: doc.Assignment, Students: []string{workPath}}
	if err = workspace.WriteDocument(filepath.Join(root, doc.Packet), packet, ""); err != nil {
		t.Fatal(err)
	}
	if err = workspace.WriteDocument(filepath.Join(root, workPath), domain.StudentWorkDocument{Schema: 1, Kind: "student-work", StudentRef: doc.Students[0].StudentRef, Alias: "Robin", AssignmentID: doc.Assignment.AssignmentID, Current: domain.AttemptWork{Ref: doc.Students[0].Grade.Attempt}}, "PRIVATE-SUBMISSION"); err != nil {
		t.Fatal(err)
	}
	for _, held := range []bool{false, true} {
		if held {
			doc.Students[0].Grade.Components[0].Evidence = "unknown"
		}
		if err = workspace.WriteDocument(path, doc, "unsent work"); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(path)
		var out, progress bytes.Buffer
		code := run(context.Background(), []string{"--workspace", root, "validate", "results.md", "--show", doc.Students[0].StudentRef}, &out, &progress)
		if code != 0 {
			t.Fatalf("%d: %s", code, progress.String())
		}
		var sample workflow.FeedbackSample
		if err = json.Unmarshal(out.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		if sample.Alias != "Robin" || sample.StudentRef != doc.Students[0].StudentRef || strings.Contains(out.String(), "PRIVATE-") {
			t.Fatalf("wrong or private sample: %s", out.String())
		}
		if held {
			if sample.Hold == "" || sample.Comment != "" {
				t.Fatalf("held feedback presented as final: %+v", sample)
			}
		} else {
			rows, err := domain.ValidateResults(&doc)
			if err != nil {
				t.Fatal(err)
			}
			g := doc.Students[0].Grade
			want, err := render.RenderFeedback(render.FeedbackInput{Rubric: *doc.Basis(), Interpretation: rows[0].Interpretation, Components: g.Components, Assessment: g.Assessment, AssignmentPoints: doc.Assignment.Points, Feedback: g.Feedback, Entered: domain.EnteredGrade(g.Assessment, doc.Assignment.Points, nil, g.HonestEffort), Header: g.Header})
			if err != nil || sample.Comment != want || sample.Hold != "" {
				t.Fatalf("sample differs from post renderer: %q vs %q (%v)", sample.Comment, want, err)
			}
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("sample changed unsent results")
		}
	}
	var out, progress bytes.Buffer
	if code := run(context.Background(), []string{"--workspace", root, "validate", "results.md", "--show", "missing-ref"}, &out, &progress); code == 0 || !strings.Contains(progress.String(), "not in this results document") || out.Len() != 0 {
		t.Fatalf("unknown selection: %d %s %s", code, out.String(), progress.String())
	}
}
