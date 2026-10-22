package workflow

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Differentiation tags are non-collaborative groups. They are teacher-owned and
// read-only here: counts and IDs only, never members.

type TagInfo struct {
	TagID       int64  `json:"tag_id"`
	Name        string `json:"name"`
	MemberCount int    `json:"member_count"`
}

type TagSetInfo struct {
	TagSetID int64     `json:"tag_set_id"`
	Name     string    `json:"name"`
	Tags     []TagInfo `json:"tags"`
}

type TagCatalog struct {
	Course string       `json:"course"`
	Sets   []TagSetInfo `json:"tag_sets"`
}

// ReadTags lists every differentiation tag set in a course with member counts.
func (s *Service) ReadTags(ctx context.Context, nickname string) (TagCatalog, error) {
	course, err := s.Course(nickname)
	if err != nil {
		return TagCatalog{}, err
	}
	s, err = s.ForCourse(ctx, course)
	if err != nil {
		return TagCatalog{}, err
	}
	return s.readTagCatalog(ctx, course.ID, course.Nickname)
}

func (s *Service) readTagCatalog(ctx context.Context, courseID int64, nickname string) (TagCatalog, error) {
	var cats []canvasGroupCategory
	if err := s.Client.List(ctx, fmt.Sprintf("/api/v1/courses/%d/group_categories?collaboration_state=non_collaborative&per_page=100", courseID), &cats); err != nil {
		return TagCatalog{}, err
	}
	out := TagCatalog{Course: nickname, Sets: []TagSetInfo{}}
	// The category response carries no non_collaborative flag (observed live); the
	// query filter scopes it, and each group below is checked.
	for _, c := range cats {
		if c.ID <= 0 || c.ContextType != "Course" || c.CourseID != courseID {
			continue
		}
		var groups []canvasGroup
		if err := s.Client.List(ctx, fmt.Sprintf("/api/v1/group_categories/%d/groups?per_page=100", c.ID), &groups); err != nil {
			return TagCatalog{}, err
		}
		set := TagSetInfo{TagSetID: c.ID, Name: c.Name, Tags: []TagInfo{}}
		for _, g := range groups {
			if g.ID <= 0 || g.ContextType != "Course" || g.CourseID != courseID || g.GroupCategoryID != c.ID || !nonCollaborative(g.NonCollaborative) {
				continue
			}
			set.Tags = append(set.Tags, TagInfo{TagID: g.ID, Name: g.Name, MemberCount: g.MembersCount})
		}
		out.Sets = append(out.Sets, set)
	}
	sort.Slice(out.Sets, func(i, j int) bool { return out.Sets[i].TagSetID < out.Sets[j].TagSetID })
	return out, nil
}

// tagKey compares names loosely: case, spacing and a trailing plural "s" do not
// matter, so "Book Club" in one course matches "Book Clubs" in another.
func tagKey(name string) string {
	key := strings.ToLower(strings.Join(strings.Fields(name), " "))
	if len(key) > 3 && strings.HasSuffix(key, "s") {
		key = key[:len(key)-1]
	}
	return key
}

type resolvedTag struct {
	TagInfo
	Set string
}

func (t resolvedTag) Label() string { return t.Set + "/" + t.Name }

func (c TagCatalog) all() []resolvedTag {
	var out []resolvedTag
	for _, set := range c.Sets {
		for _, tag := range set.Tags {
			out = append(out, resolvedTag{TagInfo: tag, Set: set.Name})
		}
	}
	return out
}

func (c TagCatalog) byID(id int64) (resolvedTag, bool) {
	for _, t := range c.all() {
		if t.TagID == id {
			return t, true
		}
	}
	return resolvedTag{}, false
}

// Resolve finds one tag from "Set/Tag", a bare tag name unique in the course,
// or an exact tag ID. A missing or ambiguous name is an error, never a guess.
func (c TagCatalog) Resolve(ref string) (resolvedTag, error) {
	ref = strings.TrimSpace(ref)
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		if t, ok := c.byID(id); ok {
			return t, nil
		}
	}
	pick := func(matches []resolvedTag, what string) (resolvedTag, error) {
		switch len(matches) {
		case 0:
			return resolvedTag{}, nil
		case 1:
			return matches[0], nil
		}
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Label()
		}
		return resolvedTag{}, fmt.Errorf("%s %q is ambiguous (%s); name it as Set/Tag or use the tag ID", what, ref, strings.Join(names, ", "))
	}
	if set, tag, ok := strings.Cut(ref, "/"); ok {
		var matches []resolvedTag
		for _, t := range c.all() {
			if tagKey(t.Set) == tagKey(set) && tagKey(t.Name) == tagKey(tag) {
				matches = append(matches, t)
			}
		}
		if t, err := pick(matches, "tag"); err != nil || t.TagID > 0 {
			return t, err
		}
	}
	var matches []resolvedTag
	for _, t := range c.all() {
		if tagKey(t.Name) == tagKey(ref) {
			matches = append(matches, t)
		}
	}
	t, err := pick(matches, "tag")
	if err != nil || t.TagID > 0 {
		return t, err
	}
	return resolvedTag{}, fmt.Errorf("no differentiation tag matches %q in this course", ref)
}

// label names a tag group ID for diffs, falling back to the bare ID.
func (c TagCatalog) label(id int64) string {
	if t, ok := c.byID(id); ok {
		return fmt.Sprintf("%s (%d)", t.Label(), t.MemberCount)
	}
	return fmt.Sprintf("group %d", id)
}
