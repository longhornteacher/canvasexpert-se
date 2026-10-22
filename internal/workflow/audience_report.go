package workflow

import (
	"context"
	"fmt"
	"strings"
)

type AudienceItem struct {
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	Own       string   `json:"own_audience,omitempty"`
	Effective string   `json:"effective_audience"`
	AlsoIn    []string `json:"also_in_modules,omitempty"`
	Flag      string   `json:"flag,omitempty"`
}

type AudienceModule struct {
	ID       int64          `json:"module_id"`
	Name     string         `json:"module"`
	Audience string         `json:"audience"`
	Items    []AudienceItem `json:"items"`
}

type AudienceReport struct {
	Course  string           `json:"course"`
	Modules []AudienceModule `json:"modules"`
	Flags   []string         `json:"flags"`
}

// ReportAudience shows who can see each module and item, and flags items that
// sit in more than one module (visible if either module grants access).
// Counts and names only; no students are listed.
func (s *Service) ReportAudience(ctx context.Context, nickname, module string) (AudienceReport, error) {
	ss, err := s.newAssignSession(ctx, nickname)
	if err != nil {
		return AudienceReport{}, err
	}
	if err = ss.loadModules(ctx); err != nil {
		return AudienceReport{}, err
	}
	report := AudienceReport{Course: ss.course.Nickname, Modules: []AudienceModule{}, Flags: []string{}}
	member := map[string][]liveModule{}
	for _, m := range ss.modules {
		for _, it := range m.Items {
			if it.Type != "SubHeader" {
				member[it.key()] = append(member[it.key()], m)
			}
		}
	}
	ownCache := map[string]liveAud{}
	hasOwn := map[string]bool{}
	for _, m := range ss.modules {
		if module != "" && !strings.EqualFold(strings.TrimSpace(m.Name), strings.TrimSpace(module)) && fmt.Sprint(m.ID) != strings.TrimSpace(module) {
			continue
		}
		maud, err := ss.moduleAudience(ctx, m)
		if err != nil {
			return report, err
		}
		am := AudienceModule{ID: m.ID, Name: m.Name, Audience: maud.describe(ss.cat), Items: []AudienceItem{}}
		for _, it := range m.Items {
			row := AudienceItem{Title: it.Title, Type: it.Type}
			switch it.Type {
			case "File":
				row.Effective = "follows the file's own access (files cannot be assigned)"
			case "SubHeader", "ExternalUrl", "ExternalTool":
				row.Effective = "follows the module"
			default:
				key := it.key()
				if _, seen := ownCache[key]; !seen {
					if surf, ok, err := ss.itemSurface(ctx, it); err == nil && ok {
						if own, err := surf.Read(ctx); err == nil {
							ownCache[key] = own
							hasOwn[key] = own.hasOwn()
						}
					}
				}
				if own, ok := ownCache[key]; ok && hasOwn[key] {
					row.Own = own.describe(ss.cat)
					row.Effective = row.Own
				}
			}
			if row.Effective == "" {
				row.Effective = "via module"
				var via []string
				broad := false
				for _, other := range member[it.key()] {
					oa, err := ss.moduleAudience(ctx, other)
					if err != nil {
						continue
					}
					via = append(via, fmt.Sprintf("%s: %s", other.Name, oa.describe(ss.cat)))
					if oa.Everyone {
						broad = true
					}
					if other.ID != m.ID {
						row.AlsoIn = append(row.AlsoIn, other.Name)
					}
				}
				row.Effective = strings.Join(via, " | ")
				if len(member[it.key()]) > 1 && !hasOwn[it.key()] {
					row.Flag = "in " + fmt.Sprint(len(member[it.key()])) + " modules; visible if either grants access"
					if broad {
						row.Flag += " (at least one module is Everyone, so this item is not restricted)"
					}
					report.Flags = append(report.Flags, fmt.Sprintf("%q (%s): %s", it.Title, m.Name, row.Flag))
				}
			}
			am.Items = append(am.Items, row)
		}
		report.Modules = append(report.Modules, am)
	}
	return report, nil
}
