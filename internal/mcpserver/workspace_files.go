package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"canvasexpertse/internal/workspace"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxWorkspaceAsset = 20 << 20

type ListFilesInput struct {
	Path      string `json:"path,omitempty" jsonschema:"Workspace-relative directory; omit to discover allowed workspace roots"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"Include descendants, bounded to 2000 entries"`
}
type WorkspaceWriteInput struct {
	Path           string `json:"path" jsonschema:"Workspace-relative root .md/.txt reference, context/ or playbooks/ file"`
	Content        string `json:"content" jsonschema:"Complete UTF-8 file contents"`
	ExpectedSHA256 string `json:"expected_sha256,omitempty" jsonschema:"Required current hash to replace an existing file"`
}
type AssetWriteInput struct {
	Path           string `json:"path" jsonschema:"Workspace-relative file under courses/"`
	Base64         string `json:"base64" jsonschema:"Complete base64-encoded asset, at most 20 MiB decoded"`
	ExpectedSHA256 string `json:"expected_sha256,omitempty" jsonschema:"Required current hash to replace an existing file"`
}

func (s *Server) registerWorkspaceTools(server *mcp.Server) {
	addReadTool(server, "list_workspace_files", "Discover allowed workspace files and directories. Hidden, private and Engine paths are excluded; results use logical workspace-relative paths and include UserInstall/rules authority metadata.", func(_ context.Context, _ *mcp.CallToolRequest, in ListFilesInput) (TextOutput, error) {
		return s.listWorkspaceFiles(in)
	})
	addActionTool(server, "write_workspace_file", "Save teacher instructions, context or playbooks through the active authority with SHA-256 conflict protection. Returns authority metadata. Never writes to Canvas.", true, func(_ context.Context, _ *mcp.CallToolRequest, in WorkspaceWriteInput) (TextOutput, error) {
		p, err := s.readablePath(in.Path)
		if err != nil {
			return TextOutput{}, err
		}
		if strings.EqualFold(firstComponent(in.Path), "courses") {
			return TextOutput{}, errors.New("use write_draft or write_asset for courses files")
		}
		if len(in.Content) > maxWorkspaceFile {
			return TextOutput{}, fmt.Errorf("file exceeds the %d-byte write limit", maxWorkspaceFile)
		}
		return s.writeWorkspaceBytes(p, []byte(in.Content), in.ExpectedSHA256)
	})
	addActionTool(server, "write_asset", "Save a base64 course attachment or authoring asset (20 MiB maximum) with SHA-256 conflict protection. Never uploads to Canvas.", true, func(_ context.Context, _ *mcp.CallToolRequest, in AssetWriteInput) (TextOutput, error) {
		p, err := s.coursePath(in.Path)
		if err != nil {
			return TextOutput{}, err
		}
		if len(in.Base64) > base64.StdEncoding.EncodedLen(maxWorkspaceAsset) {
			return TextOutput{}, errors.New("asset exceeds the 20 MiB write limit")
		}
		b, err := base64.StdEncoding.DecodeString(in.Base64)
		if err != nil {
			return TextOutput{}, fmt.Errorf("invalid base64: %w", err)
		}
		if len(b) > maxWorkspaceAsset {
			return TextOutput{}, errors.New("asset exceeds the 20 MiB write limit")
		}
		return s.writeWorkspaceBytes(p, b, in.ExpectedSHA256)
	})
	addReadTool(server, "read_workspace_asset", "Read a course attachment as base64 with its SHA-256 (20 MiB maximum).", func(_ context.Context, _ *mcp.CallToolRequest, in ReadFileInput) (TextOutput, error) {
		p, err := s.coursePath(in.Path)
		if err != nil {
			return TextOutput{}, err
		}
		b, err := readLimited(p, maxWorkspaceAsset)
		if err != nil {
			return TextOutput{}, err
		}
		h := sha256.Sum256(b)
		data, err := json.Marshal(struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
			Base64 string `json:"base64"`
		}{s.relativePath(p), hex.EncodeToString(h[:]), base64.StdEncoding.EncodeToString(b)})
		return TextOutput{Text: string(data)}, err
	})
}

func firstComponent(relative string) string {
	return strings.Split(strings.ReplaceAll(relative, "\\", "/"), "/")[0]
}
func (s *Server) relativePath(path string) string {
	root, _ := filepath.Abs(s.root)
	if loc, err := workspace.GetRulesLocation(s.root); err == nil {
		rulesRoot, _ := filepath.Abs(loc.RulesRoot)
		if rel, relErr := filepath.Rel(rulesRoot, path); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return filepath.ToSlash(rel)
		}
	}
	rel, _ := filepath.Rel(root, path)
	return filepath.ToSlash(rel)
}

func (s *Server) rulesMetadata(relative string) (workspace.RulesLocation, string, error) {
	location, err := workspace.GetRulesLocation(s.root)
	if err != nil {
		return location, "", err
	}
	authority := location.InstallRoot
	if workspace.IsSharedRulePath(filepath.ToSlash(strings.ReplaceAll(relative, "\\", "/"))) {
		authority = location.RulesRoot
	}
	return location, authority, nil
}

func (s *Server) safePath(relative string) (string, error) {
	// Validate the original path: cleaning first would erase a courses/../ escape.
	for _, part := range strings.FieldsFunc(relative, func(r rune) bool { return r == '/' || r == '\\' }) {
		if strings.HasPrefix(part, ".") {
			return "", errors.New("hidden paths and dot path components are not accessible")
		}
		if strings.EqualFold(part, "engine") {
			return "", errors.New("Engine files are not accessible through workspace file tools")
		}
	}
	path, err := workspace.ConstrainPath(s.root, relative)
	if workspace.IsSharedRulePath(filepath.ToSlash(strings.ReplaceAll(relative, "\\", "/"))) {
		path, err = workspace.ResolveTeacherPath(s.root, relative)
	}
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", err
	}
	if workspace.IsSharedRulePath(filepath.ToSlash(strings.ReplaceAll(relative, "\\", "/"))) {
		if location, locationErr := workspace.GetRulesLocation(s.root); locationErr == nil {
			root, err = filepath.Abs(location.RulesRoot)
			if err != nil {
				return "", err
			}
		}
	}
	if err := checkNoLinkedPaths(root, path); err != nil {
		return "", err
	}
	return path, nil
}

func checkNoLinkedPaths(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("path escapes its selected authority")
	}
	prefix := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		prefix = filepath.Join(prefix, part)
		info, err := os.Lstat(prefix)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return err
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			// Windows junctions are ModeIrregular. Other reparse points such as
			// OneDrive placeholders are allowed when Readlink reports no link.
			_, linkErr := os.Readlink(prefix)
			if linkErr == nil || info.Mode()&os.ModeSymlink != 0 {
				return errors.New("linked workspace paths are not accessible")
			}
			if !os.IsNotExist(linkErr) {
				return linkErr
			}
		}
	}
	return nil
}
func (s *Server) readablePath(relative string) (string, error) {
	p, err := s.safePath(relative)
	if err != nil {
		return "", err
	}
	first := firstComponent(relative)
	if !strings.EqualFold(first, "context") && !strings.EqualFold(first, "playbooks") && !strings.EqualFold(first, "courses") {
		ext := strings.ToLower(filepath.Ext(relative))
		if strings.ContainsAny(relative, "/\\") || (ext != ".md" && ext != ".txt") {
			return "", errors.New("access is limited to root .md/.txt references, context/, playbooks/, and courses/")
		}
		if info, err := os.Stat(p); err == nil && !info.Mode().IsRegular() {
			return "", errors.New("root reference must be a regular text file")
		} else if err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return p, nil
}
func (s *Server) coursePath(relative string) (string, error) {
	p, err := s.safePath(relative)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(firstComponent(relative), "courses") || !strings.ContainsAny(relative, "/\\") {
		return "", errors.New("document path must be under courses/")
	}
	return p, nil
}
func (s *Server) writeDraft(in WriteFileInput) (TextOutput, error) {
	p, err := s.coursePath(in.Path)
	if err != nil {
		return TextOutput{}, err
	}
	if len(in.Content) > maxWorkspaceFile {
		return TextOutput{}, fmt.Errorf("draft exceeds the %d-byte write limit", maxWorkspaceFile)
	}
	ext := strings.ToLower(filepath.Ext(p))
	if ext != ".md" && ext != ".yaml" && ext != ".yml" && ext != ".json" && ext != ".txt" {
		return TextOutput{}, errors.New("drafts must use .md, .yaml, .yml, .json, or .txt files")
	}
	return s.writeWorkspaceBytes(p, []byte(in.Content), in.ExpectedSHA256)
}
func (s *Server) writeWorkspaceBytes(path string, b []byte, expected string) (TextOutput, error) {
	// One adapter serializes compare-and-replace so concurrent calls cannot
	// both overwrite the same version of unsent work.
	s.mu.Lock()
	defer s.mu.Unlock()
	routed, err := s.safePath(s.relativePath(path))
	if err != nil {
		return TextOutput{}, err
	}
	if filepath.Clean(routed) != filepath.Clean(path) {
		return TextOutput{}, errors.New("teacher-rules location changed during this write; reread the file and retry")
	}
	if existing, err := os.ReadFile(path); err == nil {
		if expected == "" {
			return TextOutput{}, errors.New("file already exists; read it and provide its sha256 to replace it")
		}
		h := sha256.Sum256(existing)
		if !strings.EqualFold(expected, hex.EncodeToString(h[:])) {
			return TextOutput{}, errors.New("file changed since it was read; reread it before editing")
		}
	} else if !os.IsNotExist(err) {
		return TextOutput{}, err
	} else if expected != "" {
		return TextOutput{}, errors.New("file no longer exists; omit expected_sha256 to create it again")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return TextOutput{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cse-draft-*")
	if err != nil {
		return TextOutput{}, err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(b)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return TextOutput{}, err
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return TextOutput{}, err
	}
	h := sha256.Sum256(b)
	relative := s.relativePath(path)
	location, authority, locationErr := s.rulesMetadata(relative)
	if locationErr != nil {
		return TextOutput{}, fmt.Errorf("teacher rules location unavailable after save: %w", locationErr)
	}
	return TextOutput{Text: fmt.Sprintf("File saved: %s\nmode: %s\ninstall_root: %s\nrules_root: %s\nauthority_root: %s\nsha256: %x", relative, location.Mode, location.InstallRoot, location.RulesRoot, authority, h)}, nil
}
func readLimited(path string, limit int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("expected a regular file")
	}
	if info.Size() > int64(limit) {
		return nil, fmt.Errorf("file exceeds the %d-byte read limit", limit)
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > limit {
		return nil, fmt.Errorf("file exceeds the %d-byte read limit", limit)
	}
	return b, nil
}
func (s *Server) listWorkspaceFiles(in ListFilesInput) (TextOutput, error) {
	type candidate struct{ logical, physical string }
	var candidates []candidate
	location, locationErr := workspace.GetRulesLocation(s.root)
	syncMode := locationErr == nil && location.Mode == "sync"
	listingError := ""
	requestedRoot := strings.Trim(strings.ReplaceAll(in.Path, "\\", "/"), "/")
	isPlaybooksRoot := strings.EqualFold(requestedRoot, "playbooks")
	if in.Path != "" && locationErr != nil && (isPlaybooksRoot || workspace.IsSharedRulePath(requestedRoot)) {
		return TextOutput{}, fmt.Errorf("teacher rules location unavailable: %w", locationErr)
	}
	addRulesDirectory := func(name string) error {
		p := filepath.Join(location.RulesRoot, name)
		if err := checkNoLinkedPaths(location.RulesRoot, p); err != nil {
			if in.Path != "" {
				return err
			}
			listingError = err.Error()
			return nil
		}
		candidates = append(candidates, candidate{name, p})
		return nil
	}
	if in.Path == "" {
		rootEntries, err := os.ReadDir(s.root)
		if err != nil {
			return TextOutput{}, err
		}
		for _, entry := range rootEntries {
			if entry.IsDir() || syncMode && workspace.IsSharedRulePath(entry.Name()) {
				continue
			}
			if _, err := s.readablePath(entry.Name()); err == nil {
				candidates = append(candidates, candidate{entry.Name(), filepath.Join(s.root, entry.Name())})
			}
		}
		if syncMode {
			if entries, readErr := os.ReadDir(location.RulesRoot); readErr == nil {
				for _, entry := range entries {
					if entry.IsDir() || !workspace.IsSharedRulePath(entry.Name()) {
						continue
					}
					if _, pathErr := s.readablePath(entry.Name()); pathErr == nil {
						candidates = append(candidates, candidate{entry.Name(), filepath.Join(location.RulesRoot, entry.Name())})
					}
				}
			} else {
				listingError = fmt.Sprintf("read UserSync rules directory %s: %v", location.RulesRoot, readErr)
			}
		}
		for _, name := range []string{"context", "playbooks", "courses"} {
			p, pathErr := s.safePath(name)
			if pathErr == nil {
				candidates = append(candidates, candidate{name, p})
			}
			if syncMode && (name == "context" || name == "playbooks") {
				if err := addRulesDirectory(name); err != nil {
					return TextOutput{}, err
				}
			}
		}
	} else {
		if !(syncMode && isPlaybooksRoot) {
			p, err := s.readablePath(in.Path)
			if err != nil {
				return TextOutput{}, err
			}
			candidates = append(candidates, candidate{requestedRoot, p})
		} else {
			// Preserve the original caller path through confinement before using
			// the normalized logical directory name for UserSync routing.
			if _, err := s.safePath(in.Path); err != nil {
				return TextOutput{}, err
			}
			if _, err := workspace.ConstrainPath(location.RulesRoot, in.Path); err != nil {
				return TextOutput{}, err
			}
		}
		if syncMode && (strings.EqualFold(requestedRoot, "context") || isPlaybooksRoot) {
			name := strings.ToLower(requestedRoot)
			if err := addRulesDirectory(name); err != nil {
				return TextOutput{}, err
			}
		}
	}
	type entry struct {
		Path      string `json:"path"`
		Directory bool   `json:"directory"`
		Bytes     int64  `json:"bytes,omitempty"`
	}
	entries := []entry{}
	seen := map[string]bool{}
	truncated := false
	for _, candidate := range candidates {
		relative, p := candidate.logical, candidate.physical
		if _, statErr := os.Stat(p); os.IsNotExist(statErr) && in.Path == "" {
			continue
		} else if statErr != nil {
			return TextOutput{}, statErr
		}
		walkErr := filepath.WalkDir(p, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			suffix, relErr := filepath.Rel(p, path)
			if relErr != nil {
				return relErr
			}
			rel := relative
			if suffix != "." {
				rel = filepath.ToSlash(filepath.Join(relative, suffix))
			}
			effective, pathErr := s.readablePath(rel)
			if pathErr != nil || filepath.Clean(effective) != filepath.Clean(path) {
				// Mixed roots such as context/ have local operational files and
				// shared references. Permit traversing each candidate root while
				// filtering every child against its actual authority below.
				if d.IsDir() && suffix == "." {
					info, infoErr := d.Info()
					if infoErr != nil {
						return infoErr
					}
					key := strings.ToLower(filepath.ToSlash(rel))
					if !seen[key] {
						entries = append(entries, entry{rel, true, info.Size()})
						seen[key] = true
					}
					return nil
				}
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if len(entries) >= 2000 {
				truncated = true
				return fs.SkipAll
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			key := strings.ToLower(filepath.ToSlash(rel))
			if !seen[key] {
				entries = append(entries, entry{rel, d.IsDir(), info.Size()})
				seen[key] = true
			}
			if d.IsDir() && path != p && !in.Recursive {
				return filepath.SkipDir
			}
			return nil
		})
		if walkErr != nil {
			return TextOutput{}, walkErr
		}
		if truncated {
			break
		}
	}
	metadata := struct {
		Entries           []entry `json:"entries"`
		Truncated         bool    `json:"truncated"`
		InstallRoot       string  `json:"install_root"`
		RulesRoot         string  `json:"rules_root"`
		Mode              string  `json:"mode"`
		AvailabilityError string  `json:"availability_error,omitempty"`
	}{Entries: entries, Truncated: truncated, InstallRoot: location.InstallRoot, RulesRoot: location.RulesRoot, Mode: location.Mode}
	if locationErr != nil {
		metadata.AvailabilityError = locationErr.Error()
	} else if listingError != "" {
		metadata.AvailabilityError = listingError
	}
	out, err := json.Marshal(metadata)
	return TextOutput{Text: string(out)}, err
}
