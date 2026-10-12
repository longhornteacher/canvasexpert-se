// Package canvasexpertse carries the files embedded into the executable.
package canvasexpertse

import (
	"embed"
	"strings"
)

//go:embed VERSION
var releaseVersion string

// ReleaseVersion is the sole product version, also used by the release scripts.
func ReleaseVersion() string { return strings.TrimSpace(releaseVersion) }

// WorkspaceTemplate is the starter workspace the hub creates in a chosen
// folder. all: keeps dot-folders such as .claude.
//
//go:embed all:workspace-template
var WorkspaceTemplate embed.FS

// EngineDocs is the format reference and synthetic examples, served by
// `cse help formats` and `cse help examples` so they always match this Engine.
//
//go:embed engine-docs
var EngineDocs embed.FS
