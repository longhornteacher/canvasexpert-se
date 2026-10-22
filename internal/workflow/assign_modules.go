package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// AssignModulesOptions maps modules to tags in one call: either an explicit
// module-title to tag-name map, or a title pattern containing {tag}.
type AssignModulesOptions struct {
	TagSet  string            // optional: restrict tag names to this set
	Pattern string            // e.g. "Book Club: {tag}"
	Map     map[string]string // module title -> tag name or Set/Tag
	Preview bool
}

// AssignModulesByTag builds an assign-to document under the course's drafts
// folder and applies it (or previews it), so the teacher has the same record
// as a hand-written document.
func (s *Service) AssignModulesByTag(ctx context.Context, nickname string, opts AssignModulesOptions) (AssignToReport, error) {
	if (opts.Pattern == "") == (len(opts.Map) == 0) {
		return AssignToReport{}, fmt.Errorf("give either module_pattern (with {tag}) or a module-to-tag map")
	}
	if opts.Pattern != "" && strings.Count(opts.Pattern, "{tag}") != 1 {
		return AssignToReport{}, fmt.Errorf("module_pattern must contain {tag} exactly once")
	}
	ss, err := s.newAssignSession(ctx, nickname)
	if err != nil {
		return AssignToReport{}, err
	}
	if err = ss.loadModules(ctx); err != nil {
		return AssignToReport{}, err
	}
	qualify := func(tag string) string {
		if opts.TagSet != "" && !strings.Contains(tag, "/") {
			return opts.TagSet + "/" + tag
		}
		return tag
	}
	doc := domain.AssignToDocument{Schema: 1, Kind: "assign-to", Course: ss.course.Nickname, Mode: "replace"}
	if opts.Pattern != "" {
		prefix, suffix, _ := strings.Cut(opts.Pattern, "{tag}")
		for _, m := range ss.modules {
			name := strings.TrimSpace(m.Name)
			lower := strings.ToLower(name)
			if !strings.HasPrefix(lower, strings.ToLower(prefix)) || !strings.HasSuffix(lower, strings.ToLower(suffix)) || len(name) <= len(prefix)+len(suffix) {
				continue
			}
			tag := strings.TrimSpace(name[len(prefix) : len(name)-len(suffix)])
			doc.Targets = append(doc.Targets, domain.AssignToTarget{Module: domain.Ref{ID: m.ID}, Tags: []string{qualify(tag)}})
		}
		if len(doc.Targets) == 0 {
			return AssignToReport{}, fmt.Errorf("no module title matches the pattern %q", opts.Pattern)
		}
	} else {
		names := make([]string, 0, len(opts.Map))
		for module := range opts.Map {
			names = append(names, module)
		}
		sort.Strings(names)
		for _, module := range names {
			doc.Targets = append(doc.Targets, domain.AssignToTarget{Module: domain.Ref{Text: module}, Tags: []string{qualify(opts.Map[module])}})
		}
	}
	if err = doc.Validate(); err != nil {
		return AssignToReport{}, err
	}
	dir, err := workspace.ConstrainPath(s.Root, filepath.Join("courses", ss.course.Nickname, "drafts"))
	if err != nil {
		return AssignToReport{}, err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return AssignToReport{}, err
	}
	path := filepath.Join(dir, "assign-modules-"+time.Now().UTC().Format("20060102T150405")+".md")
	if err = workspace.WriteDocument(path, &doc, ""); err != nil {
		return AssignToReport{}, err
	}
	return s.runAssignTo(ctx, path, opts.Preview)
}
