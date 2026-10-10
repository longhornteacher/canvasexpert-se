package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"canvasexpertse/internal/canvas"
)

// apiGet is the read-only escape hatch for Canvas data no command covers yet:
// one GET with this machine's credential, the JSON printed for the agent. It
// returns one page; add per_page and page to the query for more.
func apiGet(ctx context.Context, c *canvas.Client, path string, out io.Writer) error {
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
	if json.Unmarshal(response, &pretty) == nil {
		if b, err := json.MarshalIndent(pretty, "", "  "); err == nil {
			response = b
		}
	}
	_, e := fmt.Fprintln(out, string(response))
	return e
}
