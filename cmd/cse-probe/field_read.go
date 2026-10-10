package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// fieldProbePrefix marks the only objects field-read will return in full.
const fieldProbePrefix = "[CSE FIELD PROBE]"

// fieldRead is a read-only helper for the Canvas field-limits probe: it fetches
// stored HTML fields of named probe objects in CS8 in a fresh request.
func (p *probe) fieldRead(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("field-read course | image-files | page ID | new-quiz ID | assignment ID")
	}
	kind, args := args[0], args[1:]
	if kind == "course" {
		var c struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
			Code string `json:"course_code"`
		}
		if e := p.c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d", courseID), &c); e != nil {
			return e
		}
		return emit(c)
	}
	if kind == "image-files" {
		var files []struct {
			ID          int64  `json:"id"`
			DisplayName string `json:"display_name"`
			ContentType string `json:"content-type"`
			Locked      bool   `json:"locked"`
			Hidden      bool   `json:"hidden"`
		}
		if e := p.c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/files?content_types[]=image&per_page=10", courseID), &files); e != nil {
			return e
		}
		return emit(files)
	}
	if len(args) != 1 {
		return fmt.Errorf("field-read %s requires one ID", kind)
	}
	id, e := strconv.ParseInt(args[0], 10, 64)
	if e != nil || id <= 0 {
		return fmt.Errorf("invalid ID")
	}
	if kind == "page" {
		var pg struct {
			PageID int64  `json:"page_id"`
			URL    string `json:"url"`
			Title  string `json:"title"`
		}
		if e = p.c.Get(ctx, fmt.Sprintf("/api/v1/courses/%d/pages/%d", courseID, id), &pg); e != nil {
			return e
		}
		return emit(pg)
	}
	var object, children map[string]any
	var list []map[string]any
	var objectPath, childPath, titleKey string
	switch kind {
	case "new-quiz":
		objectPath, childPath, titleKey = fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d", courseID, id), fmt.Sprintf("/api/quiz/v1/courses/%d/quizzes/%d/items", courseID, id), "title"
	case "assignment":
		objectPath, titleKey = fmt.Sprintf("/api/v1/courses/%d/assignments/%d", courseID, id), "name"
	default:
		return fmt.Errorf("unknown field-read kind")
	}
	if e = p.c.Get(ctx, objectPath, &object); e != nil {
		return e
	}
	if title, _ := object[titleKey].(string); !strings.HasPrefix(title, fieldProbePrefix) {
		return fmt.Errorf("object %d is not a %s object; refusing to read it", id, fieldProbePrefix)
	}
	if childPath != "" {
		if e = p.c.List(ctx, childPath, &list); e != nil {
			return e
		}
	}
	children = map[string]any{"object": object}
	if list != nil {
		children["children"] = list
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(children)
}
