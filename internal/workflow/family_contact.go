package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// FamilyContact contains agent-authored content and stable audience references.
// It deliberately contains no private recipient or Canvas user identity.
type FamilyContact struct {
	Title      string          `json:"title"`
	Recipients string          `json:"recipients,omitempty"`
	Messages   []FamilyMessage `json:"messages"`
}

type FamilyMessage struct {
	Key         string         `json:"key"`
	Audience    FamilyAudience `json:"audience"`
	Subject     string         `json:"subject"`
	Body        string         `json:"body"`
	Format      string         `json:"format,omitempty"`
	Attachments []string       `json:"attachments,omitempty"`
}

type FamilyAudience struct {
	Courses  []string            `json:"courses,omitempty"`
	Groups   []FamilyGroupTarget `json:"groups,omitempty"`
	Students []string            `json:"student_refs,omitempty"`
}

type FamilyGroupTarget struct {
	Course   string `json:"course"`
	GroupSet int64  `json:"group_set"`
	ID       int64  `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
}

type FamilyAudienceRow struct {
	Ref     string `json:"student_ref"`
	Alias   string `json:"alias"`
	Course  string `json:"course"`
	Group   string `json:"group,omitempty"`
	Message string `json:"message"`
}

type FamilyAudienceSnapshot struct {
	Schema           int                 `json:"schema"`
	ContactDigest    string              `json:"contact_digest"`
	MembershipDigest string              `json:"membership_digest"`
	Students         []FamilyAudienceRow `json:"students"`
	Counts           map[string]int      `json:"counts"`
	Warnings         []string            `json:"warnings"`
	Blocking         []string            `json:"blocking"`
}

func familyDigest(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// familyRunPath keeps session files in UserInstall, including when an absolute
// path is supplied. Every individual read/write also checks existing symlinks.
func familyRunPath(root, dir, name string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(absRoot, dir)
	}
	rel, err := filepath.Rel(absRoot, dir)
	if err != nil {
		return "", err
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 4 || parts[0] != "courses" || parts[1] == "" || parts[2] != "family-contact" || parts[3] == "" {
		return "", fmt.Errorf("family contact run must be courses/<course>/family-contact/<run> in UserInstall")
	}
	return workspace.ConstrainPath(absRoot, filepath.Join(rel, name))
}

func LoadFamilyContact(root, dir string) (FamilyContact, error) {
	var doc FamilyContact
	path, err := familyRunPath(root, dir, "contact.json")
	if err != nil {
		return doc, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return doc, err
	}
	if err = json.Unmarshal(b, &doc); err != nil {
		return doc, fmt.Errorf("contact.json is invalid JSON")
	}
	if strings.TrimSpace(doc.Title) == "" || len(doc.Messages) == 0 {
		return doc, fmt.Errorf("contact.json needs a title and at least one message")
	}
	if doc.Recipients != "" && doc.Recipients != "parents_only" {
		return doc, fmt.Errorf("recipients must be omitted or parents_only")
	}
	keys := map[string]bool{}
	for _, m := range doc.Messages {
		if strings.TrimSpace(m.Key) == "" || keys[m.Key] || strings.TrimSpace(m.Subject) == "" || strings.TrimSpace(m.Body) == "" {
			return doc, fmt.Errorf("each message needs a unique key, subject and body")
		}
		keys[m.Key] = true
		if m.Format != "" && m.Format != "markdown" && m.Format != "html" {
			return doc, fmt.Errorf("message %q format must be markdown or html", m.Key)
		}
		if len(m.Audience.Courses)+len(m.Audience.Groups)+len(m.Audience.Students) == 0 {
			return doc, fmt.Errorf("message %q needs an audience", m.Key)
		}
		for _, g := range m.Audience.Groups {
			if g.Course == "" || g.GroupSet <= 0 || g.ID < 0 || (g.ID == 0 && normalizedFamilyGroup(g.Name) == "") {
				return doc, fmt.Errorf("message %q group needs course, positive group_set and id or name", m.Key)
			}
		}
	}
	return doc, nil
}

func normalizedFamilyGroup(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), ""))
}

func (s *Service) PrepareFamilyContact(ctx context.Context, dir string) (FamilyAudienceSnapshot, error) {
	doc, err := LoadFamilyContact(s.Root, dir)
	if err != nil {
		return FamilyAudienceSnapshot{}, err
	}
	path, err := familyRunPath(s.Root, dir, "audience.json")
	if err != nil {
		return FamilyAudienceSnapshot{}, err
	}
	warnings, err := ValidateFamilyContent(filepath.Dir(path), doc)
	if err != nil {
		return FamilyAudienceSnapshot{}, err
	}
	snapshot, err := s.resolveFamilyAudience(ctx, doc)
	if err != nil {
		return snapshot, err
	}
	snapshot.Warnings = append(snapshot.Warnings, warnings...)
	sort.Strings(snapshot.Warnings)
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return snapshot, err
	}
	return snapshot, workspace.WriteFile(path, append(b, '\n'), 0600)
}

// LoadFamilyAudience reads the saved audience for private review. Blocking
// problems remain visible here; delivery renews live membership separately.
func LoadFamilyAudience(root, dir string) (FamilyContact, FamilyAudienceSnapshot, error) {
	doc, err := LoadFamilyContact(root, dir)
	if err != nil {
		return doc, FamilyAudienceSnapshot{}, err
	}
	var saved FamilyAudienceSnapshot
	path, err := familyRunPath(root, dir, "audience.json")
	if err != nil {
		return doc, saved, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return doc, saved, fmt.Errorf("prepare family contact before reviewing or sending")
	}
	if json.Unmarshal(b, &saved) != nil || saved.Schema != 1 || saved.ContactDigest != familyDigest(doc) {
		return doc, saved, fmt.Errorf("contact or audience changed; prepare family contact again")
	}
	contactPath, err := familyRunPath(root, dir, "contact.json")
	if err != nil {
		return doc, saved, err
	}
	contactInfo, err := os.Stat(contactPath)
	if err != nil {
		return doc, saved, err
	}
	audienceInfo, err := os.Stat(path)
	if err != nil {
		return doc, saved, err
	}
	if contactInfo.ModTime().After(audienceInfo.ModTime()) {
		return doc, saved, fmt.Errorf("contact.json is newer than the prepared audience; prepare family contact again")
	}
	return doc, saved, nil
}

// CheckFamilyAudience renews live membership immediately before delivery.
// An agent-edited snapshot cannot redirect a message to another student.
func (s *Service) CheckFamilyAudience(ctx context.Context, dir string) (FamilyContact, FamilyAudienceSnapshot, error) {
	doc, saved, err := LoadFamilyAudience(s.Root, dir)
	if err != nil {
		return doc, saved, err
	}
	if len(saved.Blocking) > 0 {
		return doc, saved, fmt.Errorf("family audience has blocking problems: %s", strings.Join(saved.Blocking, "; "))
	}
	fresh, err := s.resolveFamilyAudience(ctx, doc)
	if err != nil {
		return doc, saved, err
	}
	if len(fresh.Blocking) > 0 || fresh.MembershipDigest != saved.MembershipDigest || !reflect.DeepEqual(fresh.Students, saved.Students) || !reflect.DeepEqual(fresh.Counts, saved.Counts) {
		return doc, saved, fmt.Errorf("Canvas audience or membership changed; prepare family contact again")
	}
	return doc, saved, nil
}

func (s *Service) resolveFamilyAudience(ctx context.Context, doc FamilyContact) (FamilyAudienceSnapshot, error) {
	out := FamilyAudienceSnapshot{Schema: 1, ContactDigest: familyDigest(doc), Students: []FamilyAudienceRow{}, Counts: map[string]int{}, Warnings: []string{}, Blocking: []string{}}
	if s.Client == nil || (s.Config.Instance != "" && s.Config.Instance != s.Client.Origin()) {
		return out, fmt.Errorf("family contact requires the configured Canvas instance")
	}
	store, err := workspace.OpenStudents(s.Root)
	if err != nil {
		return out, err
	}
	rosters := map[int64]map[string]workspace.Student{}
	identity := []string{s.Client.Origin()}
	loadCourse := func(nickname string) (domain.Course, map[string]workspace.Student, error) {
		course, e := s.Course(nickname)
		if e != nil {
			return course, nil, e
		}
		if roster, ok := rosters[course.ID]; ok {
			return course, roster, nil
		}
		enrollments, e := StudentEnrollments(ctx, s.Client, course.ID)
		if e != nil {
			return course, nil, fmt.Errorf("could not read student enrollments for %s; check Canvas access and try preparing again", course.Nickname)
		}
		// Group roster reads may have added private mappings since the last
		// course. Reload before saving so those stable references survive.
		store, e = workspace.OpenStudents(s.Root)
		if e != nil {
			return course, nil, e
		}
		roster := map[string]workspace.Student{}
		for _, enrollment := range enrollments {
			if enrollment.Type != "StudentEnrollment" || (enrollment.State != "active" && enrollment.State != "invited") {
				continue
			}
			id := enrollment.UserID
			if id == 0 {
				id = enrollment.User.ID
			}
			st, e := store.Upsert(s.Client.Origin(), id, enrollment.User.Name, enrollment.User.ShortName, enrollment.User.SISID, enrollment.User.Login)
			if e != nil {
				return course, nil, fmt.Errorf("could not map course enrollment to a stable reference")
			}
			roster[st.Ref] = st
		}
		if e = store.Save(); e != nil {
			return course, nil, e
		}
		identity = append(identity, fmt.Sprintf("course:%s:%d", course.Nickname, course.ID))
		for ref, st := range roster {
			identity = append(identity, fmt.Sprintf("enrollment:%d:%s:%d", course.ID, ref, st.UserID))
		}
		rosters[course.ID] = roster
		return course, roster, nil
	}
	groups := map[string]*groupRosterLive{}
	groupCourses := map[string]map[string]workspace.Student{}
	matches := map[string]map[string]FamilyAudienceRow{}
	add := func(st workspace.Student, course, group, key string) {
		if matches[st.Ref] == nil {
			matches[st.Ref] = map[string]FamilyAudienceRow{}
		}
		row := FamilyAudienceRow{Ref: st.Ref, Alias: st.Alias, Course: course, Group: group, Message: key}
		prior, exists := matches[st.Ref][key]
		// Repeated selectors for the same message are one delivery. Prefer a
		// group selector so the group's placeholder remains available.
		if !exists || (prior.Group == "" && group != "") {
			matches[st.Ref][key] = row
		}
	}
	for _, m := range doc.Messages {
		out.Counts[m.Key] = 0
		for _, nickname := range m.Audience.Courses {
			course, roster, e := loadCourse(nickname)
			if e != nil {
				return out, e
			}
			for _, st := range roster {
				add(st, course.Nickname, "", m.Key)
			}
		}
		for _, target := range m.Audience.Groups {
			course, roster, e := loadCourse(target.Course)
			if e != nil {
				return out, e
			}
			setKey := fmt.Sprintf("%d:%d", course.ID, target.GroupSet)
			live, exists := groups[setKey]
			if !exists {
				live, e = s.groupRosterLive(ctx, course.Nickname, target.GroupSet)
				if e != nil {
					return out, fmt.Errorf("could not resolve collaborative groups for %s set %d; verify course/set identity and Canvas access", course.Nickname, target.GroupSet)
				}
				groups[setKey], groupCourses[setKey] = live, roster
				identity = append(identity, "set:"+setKey)
				if !live.Roster.RosterComplete {
					out.Blocking = append(out.Blocking, fmt.Sprintf("%s group set %d has unresolved memberships; refresh the roster", course.Nickname, target.GroupSet))
				}
				for _, g := range live.Roster.Groups {
					identity = append(identity, fmt.Sprintf("group:%s:%d:%s", setKey, g.ID, g.Name))
					eligibleCount := 0
					for _, member := range g.Members {
						identity = append(identity, fmt.Sprintf("member:%s:%d:%s:%s", setKey, g.ID, member.StudentRef, member.State))
						if _, eligible := roster[member.StudentRef]; eligible {
							eligibleCount++
						}
					}
					if eligibleCount == 0 {
						out.Warnings = append(out.Warnings, fmt.Sprintf("%s: group %q is empty", course.Nickname, g.Name))
					}
				}
			}
			var selected []domain.GroupSummary
			for _, g := range live.Roster.Groups {
				if (target.ID > 0 && target.ID == g.ID) || (target.ID == 0 && normalizedFamilyGroup(target.Name) == normalizedFamilyGroup(g.Name)) {
					selected = append(selected, g)
				}
			}
			if len(selected) != 1 {
				return out, fmt.Errorf("message %q group selector in %s set %d matched %d groups; choose an exact group id", m.Key, course.Nickname, target.GroupSet, len(selected))
			}
			g := selected[0]
			if target.ID > 0 && target.Name != "" && normalizedFamilyGroup(target.Name) != normalizedFamilyGroup(g.Name) {
				return out, fmt.Errorf("message %q group id and name do not identify the same group; refresh the group selector", m.Key)
			}
			for _, member := range g.Members {
				if st, eligible := roster[member.StudentRef]; eligible {
					add(st, course.Nickname, g.Name, m.Key)
				}
			}
		}
		for _, ref := range m.Audience.Students {
			found := false
			for _, configured := range s.Config.Courses {
				course, roster, e := loadCourse(configured.Nickname)
				if e != nil {
					return out, e
				}
				if st, ok := roster[ref]; ok {
					add(st, course.Nickname, "", m.Key)
					found = true
				}
			}
			if !found {
				return out, fmt.Errorf("student_ref %q is not currently enrolled in a configured course", ref)
			}
		}
	}
	for ref, byMessage := range matches {
		if len(byMessage) > 1 {
			keys := []string{}
			for key := range byMessage {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			out.Blocking = append(out.Blocking, fmt.Sprintf("student_ref %s matches multiple messages: %s", ref, strings.Join(keys, ", ")))
		}
		for _, row := range byMessage {
			out.Students = append(out.Students, row)
			out.Counts[row.Message]++
		}
	}
	unmatched := map[string]bool{}
	for _, roster := range groupCourses {
		for ref := range roster {
			if len(matches[ref]) == 0 {
				unmatched[ref] = true
			}
		}
	}
	for ref := range unmatched {
		out.Warnings = append(out.Warnings, fmt.Sprintf("student_ref %s matches no message and will be skipped", ref))
	}
	sort.Slice(out.Students, func(i, j int) bool {
		if out.Students[i].Ref == out.Students[j].Ref {
			return out.Students[i].Message < out.Students[j].Message
		}
		return out.Students[i].Ref < out.Students[j].Ref
	})
	sort.Strings(identity)
	sort.Strings(out.Warnings)
	sort.Strings(out.Blocking)
	out.MembershipDigest = familyDigest(identity)
	return out, nil
}
