package main

import (
	"path/filepath"
	"strings"

	"canvasexpertse/internal/workspace"
)

// installInfo says how this folder is updated and whether the running Engine is the
// one its playbooks were released with. It is empty outside an install with a manifest.
type installInfo struct {
	InstallKind           string `json:"install_kind,omitempty"`
	ManifestIdentity      string `json:"manifest_identity,omitempty"`
	EngineMatchesManifest *bool  `json:"engine_matches_manifest,omitempty"`
}

func installInfoFor(workspaceOpt string) installInfo {
	if strings.TrimSpace(workspaceOpt) == "" {
		return installInfo{}
	}
	root, err := filepath.Abs(workspaceOpt)
	if err != nil {
		return installInfo{}
	}
	manifest, ok, err := workspace.ReadEngineManifest(root)
	if err != nil || !ok {
		return installInfo{}
	}
	matches := workspace.ArtifactIdentity(version, buildKind, revision, builtAt) == manifest.Identity
	return installInfo{InstallKind: workspace.InstallKind(root), ManifestIdentity: manifest.Identity, EngineMatchesManifest: &matches}
}
