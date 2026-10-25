package domain

import (
	"fmt"
	"net/url"
	"strings"
)

// ModuleDocument describes an ordered set of native Canvas module items.
// ModuleID selects an existing module; otherwise the workflow creates one.
type ModuleDocument struct {
	Schema    int            `yaml:"schema"`
	Kind      string         `yaml:"kind"`
	Course    string         `yaml:"course"`
	Instance  string         `yaml:"instance,omitempty"`
	CourseID  int64          `yaml:"course_id,omitempty"`
	Title     string         `yaml:"title"`
	ModuleID  int64          `yaml:"module_id,omitempty"`
	Position  *int           `yaml:"position,omitempty"`
	Published *bool          `yaml:"published,omitempty"`
	Items     []ModuleItem   `yaml:"items"`
	Created   *ModuleCreated `yaml:"created,omitempty"`
}

type ModuleItem struct {
	Type        string `yaml:"type"`
	ContentID   int64  `yaml:"content_id,omitempty"`
	ExternalURL string `yaml:"external_url,omitempty"`
	NewTab      *bool  `yaml:"new_tab,omitempty"`
	Title       string `yaml:"title,omitempty"`
	Position    int    `yaml:"position,omitempty"`
}

type ModuleCreated struct {
	ModuleID    int64               `yaml:"module_id,omitempty"`
	StartedAt   string              `yaml:"started_at,omitempty"`
	Module      ActionOutcome       `yaml:"module,omitempty"`
	Items       []ModuleItemOutcome `yaml:"items,omitempty"`
	Publication ActionOutcome       `yaml:"publication,omitempty"`
	Position    *int                `yaml:"position,omitempty"`
}

type ModuleItemOutcome struct {
	Index        int           `yaml:"index"`
	Type         string        `yaml:"type"`
	ContentID    int64         `yaml:"content_id,omitempty"`
	ExternalURL  string        `yaml:"external_url,omitempty"`
	NewTab       *bool         `yaml:"new_tab,omitempty"`
	Title        string        `yaml:"title,omitempty"`
	Position     int           `yaml:"position,omitempty"`
	ModuleItemID int64         `yaml:"module_item_id,omitempty"`
	Access       string        `yaml:"access,omitempty"`
	Outcome      ActionOutcome `yaml:"outcome,omitempty"`
}

func (d ModuleDocument) Validate() error {
	if d.Schema != 1 || d.Kind != "module" {
		return fmt.Errorf("module document requires schema: 1 and kind: module")
	}
	if strings.TrimSpace(d.Course) == "" || strings.TrimSpace(d.Title) == "" {
		return fmt.Errorf("course and title are required")
	}
	if d.ModuleID < 0 || d.CourseID < 0 {
		return fmt.Errorf("module and course IDs cannot be negative")
	}
	if d.Position != nil && *d.Position < 1 {
		return fmt.Errorf("module position must be a positive one-based integer")
	}
	if len(d.Items) == 0 && (d.ModuleID == 0 || d.Position == nil) {
		return fmt.Errorf("at least one module item is required, except for an existing module's position-only update")
	}
	for i, item := range d.Items {
		if item.Position < 0 {
			return fmt.Errorf("items[%d].position cannot be negative", i)
		}
		switch item.Type {
		case "Page", "File", "Assignment", "Discussion":
			if item.ContentID <= 0 || item.ExternalURL != "" || item.NewTab != nil || strings.TrimSpace(item.Title) != "" {
				return fmt.Errorf("items[%d] %s requires a positive content_id and does not accept title", i, item.Type)
			}
		case "ExternalUrl":
			if item.ContentID != 0 || strings.TrimSpace(item.Title) == "" {
				return fmt.Errorf("items[%d] ExternalUrl requires title and does not accept content_id", i)
			}
			if err := ValidateModuleExternalURL(item.ExternalURL); err != nil {
				return fmt.Errorf("items[%d].external_url: %w", i, err)
			}
		case "SubHeader":
			if item.ContentID != 0 || item.ExternalURL != "" || item.NewTab != nil || strings.TrimSpace(item.Title) == "" {
				return fmt.Errorf("items[%d] SubHeader requires a title and rejects content_id, external_url, and new_tab", i)
			}
		default:
			return fmt.Errorf("items[%d].type must be Page, File, Assignment, Discussion, ExternalUrl, or SubHeader", i)
		}
	}
	if d.Created != nil {
		if d.Created.ModuleID < 0 {
			return fmt.Errorf("created.module_id cannot be negative")
		}
		if len(d.Created.Items) > len(d.Items) {
			return fmt.Errorf("created.items has more entries than items")
		}
		for _, saved := range d.Created.Items {
			if saved.Index < 0 || saved.Index >= len(d.Items) {
				return fmt.Errorf("created item index %d is outside items", saved.Index)
			}
		}
	}
	return nil
}

// ValidateModuleExternalURL accepts stable teacher-selected HTTP(S) links and
// rejects embedded credentials and common temporary/signed URL parameters.
func ValidateModuleExternalURL(raw string) error {
	if raw != strings.TrimSpace(raw) || strings.ContainsAny(raw, "\r\n\t") {
		return fmt.Errorf("must be an absolute HTTP(S) URL without embedded credentials")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return fmt.Errorf("must be an absolute HTTP(S) URL without embedded credentials")
	}
	for key := range u.Query() {
		k := strings.ToLower(key)
		if k == "token" || k == "access_token" || k == "auth" || k == "signature" || k == "sig" || k == "expires" || k == "key" || k == "api_key" || k == "access_key" || k == "client_secret" || k == "credential" || k == "x-amz-signature" || k == "x-amz-credential" || k == "x-amz-security-token" || k == "awsaccesskeyid" || strings.HasPrefix(k, "x-goog-") || strings.HasPrefix(k, "x-amz-") {
			return fmt.Errorf("must not contain credentials or a signed temporary URL")
		}
	}
	return nil
}
