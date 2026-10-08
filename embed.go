// Package canvasexpertse carries the files embedded into the executable.
package canvasexpertse

import "embed"

// WorkspaceTemplate is the starter workspace the hub creates in a chosen
// folder. all: keeps dot-folders such as .claude.
//
//go:embed all:workspace-template
var WorkspaceTemplate embed.FS
