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
	addReadTool(server, "list_workspace_files", "Discover allowed workspace files and directories. Hidden, private and Engine paths are excluded; results are workspace-relative.", func(_ context.Context, _ *mcp.CallToolRequest, in ListFilesInput) (TextOutput, error) {
		return s.listWorkspaceFiles(in)
	})
	addActionTool(server, "write_workspace_file", "Save teacher instructions, context or playbooks with SHA-256 conflict protection. Never writes to Canvas.", true, func(_ context.Context, _ *mcp.CallToolRequest, in WorkspaceWriteInput) (TextOutput, error) {
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
	rel, _ := filepath.Rel(root, path)
	return filepath.ToSlash(rel)
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
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	prefix := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		prefix = filepath.Join(prefix, part)
		info, err := os.Lstat(prefix)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			// Windows junctions are ModeIrregular. Other reparse points such as
			// OneDrive placeholders are allowed when Readlink reports no link.
			_, linkErr := os.Readlink(prefix)
			if linkErr == nil || info.Mode()&os.ModeSymlink != 0 {
				return "", errors.New("linked workspace paths are not accessible")
			}
			if !os.IsNotExist(linkErr) {
				return "", linkErr
			}
		}
	}
	return path, nil
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
	if _, err := s.safePath(s.relativePath(path)); err != nil {
		return TextOutput{}, err
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
	return TextOutput{Text: fmt.Sprintf("File saved: %s\nsha256: %x", s.relativePath(path), h)}, nil
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
	var roots []string
	if in.Path == "" {
		rootEntries, err := os.ReadDir(s.root)
		if err != nil {
			return TextOutput{}, err
		}
		for _, entry := range rootEntries {
			if entry.IsDir() {
				continue
			}
			if _, err := s.readablePath(entry.Name()); err == nil {
				roots = append(roots, entry.Name())
			}
		}
		roots = append(roots, "context", "playbooks", "courses")
	} else {
		roots = []string{in.Path}
	}
	type entry struct {
		Path      string `json:"path"`
		Directory bool   `json:"directory"`
		Bytes     int64  `json:"bytes,omitempty"`
	}
	entries := []entry{}
	truncated := false
	for _, relative := range roots {
		p, err := s.readablePath(relative)
		if err != nil {
			return TextOutput{}, err
		}
		if _, err = os.Stat(p); os.IsNotExist(err) && in.Path == "" {
			continue
		} else if err != nil {
			return TextOutput{}, err
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel := s.relativePath(path)
			if _, err := s.readablePath(rel); err != nil {
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
			entries = append(entries, entry{rel, d.IsDir(), info.Size()})
			if d.IsDir() && path != p && !in.Recursive {
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return TextOutput{}, err
		}
		if truncated {
			break
		}
	}
	out, err := json.Marshal(struct {
		Entries   []entry `json:"entries"`
		Truncated bool    `json:"truncated"`
	}{entries, truncated})
	return TextOutput{Text: string(out)}, err
}
