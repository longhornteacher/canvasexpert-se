package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/workspace"
)

// apiGet is the read-only escape hatch for Canvas data no command covers yet:
// one GET with this machine's credential, the JSON printed for the agent. It
// returns one page; add per_page and page to the query for more. A non-nil
// hide replaces people's names with saved aliases before anything is printed.
func apiGet(ctx context.Context, c *canvas.Client, path string, out io.Writer, hide *identityRedactor) error {
	if !strings.HasPrefix(path, "/api/v1/") && !strings.HasPrefix(path, "/api/quiz/v1/") {
		return inputError(fmt.Errorf("api PATH must start with /api/v1/ or /api/quiz/v1/"))
	}
	var response json.RawMessage
	if e := c.Get(ctx, path, &response); e != nil {
		var ce *canvas.Error
		if errors.As(e, &ce) && ce.Body != "" {
			return fmt.Errorf("%w (HTTP %d): %s", e, ce.Status, ce.Body)
		}
		return e
	}
	var pretty any
	d := json.NewDecoder(bytes.NewReader(response))
	d.UseNumber()
	if d.Decode(&pretty) == nil {
		if hide != nil {
			hide.walk(pretty, "")
		}
		if b, err := json.MarshalIndent(pretty, "", "  "); err == nil {
			response = b
		}
	} else if hide != nil {
		return errors.New("Canvas returned a response that is not JSON; it was not shown because student names are hidden")
	}
	_, e := fmt.Fprintln(out, string(response))
	return e
}

// identityRedactor keeps real names out of raw API output when the teacher has
// "Hide student names" on. A Canvas user object is recognized by its identity
// fields or by the key that holds it; its name fields become the student's
// saved alias (or a placeholder for anyone not on the private roster) and its
// login, SIS, email and avatar fields are removed. Free text such as messages,
// submissions and file names is evidence and is left as Canvas returned it.
type identityRedactor struct {
	students map[int64]workspace.Student
}

const hiddenName = "(name hidden)"

var (
	userMarkers  = []string{"sortable_name", "short_name", "login_id", "sis_user_id", "avatar_image_url", "full_name", "email", "primary_email"}
	userHolders  = map[string]bool{"user": true, "users": true, "author": true, "edited_by": true, "participants": true}
	userNames    = []string{"name", "sortable_name", "short_name", "display_name", "full_name"}
	userPrivate  = []string{"login_id", "sis_user_id", "sis_import_id", "integration_id", "email", "primary_email", "avatar_url", "avatar_image_url", "pronouns"}
	namePrefixes = []string{"author", "user", "student", "submitter"}
)

// newIdentityRedactor reads the private roster for alias lookup. An unreadable
// roster still hides names; it only loses the aliases.
func newIdentityRedactor(root, instance string) *identityRedactor {
	r := &identityRedactor{students: map[int64]workspace.Student{}}
	if store, e := workspace.OpenStudents(root); e == nil {
		for _, v := range store.Rows {
			if v.Instance == instance {
				r.students[v.UserID] = v
			}
		}
	}
	return r
}

func (r *identityRedactor) walk(v any, key string) {
	switch t := v.(type) {
	case []any:
		for _, child := range t {
			r.walk(child, key)
		}
	case map[string]any:
		for k, child := range t {
			r.walk(child, k)
		}
		if r.isUser(t, key) {
			label, student, found := r.label(t["id"])
			for _, k := range userNames {
				if _, ok := t[k]; ok {
					t[k] = label
				}
			}
			for _, k := range userPrivate {
				delete(t, k)
			}
			if found {
				t["student_ref"] = student.Ref
			}
		}
		for _, prefix := range namePrefixes {
			if _, ok := t[prefix+"_name"]; ok {
				t[prefix+"_name"], _, _ = r.label(t[prefix+"_id"])
			}
		}
	}
}

func (r *identityRedactor) isUser(t map[string]any, key string) bool {
	for _, k := range userMarkers {
		if _, ok := t[k]; ok {
			return true
		}
	}
	_, hasID := t["id"]
	return hasID && userHolders[key]
}

func (r *identityRedactor) label(id any) (string, workspace.Student, bool) {
	var n int64
	switch v := id.(type) {
	case json.Number:
		n, _ = strconv.ParseInt(v.String(), 10, 64)
	case string:
		n, _ = strconv.ParseInt(v, 10, 64)
	}
	if v, ok := r.students[n]; ok && n > 0 {
		return v.Alias, v, true
	}
	return hiddenName, workspace.Student{}, false
}
