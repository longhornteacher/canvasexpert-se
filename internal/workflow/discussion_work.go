package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/workspace"
)

// discussionAssignmentEvidence is one assignment pull's read of the linked
// graded discussion. Individual students get references into its packet.
type discussionAssignmentEvidence struct {
	packetRel string
	topicID   int64
	packet    domain.DiscussionPacket
	failure   string
}

// readAssignmentDiscussion verifies the topic/assignment association with
// Canvas (never by title) and pulls the whole conversation once.
func (s *Service) readAssignmentDiscussion(ctx context.Context, course domain.Course, a Assignment, snapshot domain.AssignmentSnapshot, relDir string) *discussionAssignmentEvidence {
	if snapshot.DiscussionTopicID == nil {
		return nil
	}
	ev := &discussionAssignmentEvidence{topicID: *snapshot.DiscussionTopicID}
	var topic NativeDiscussion
	if err := s.Client.Get(ctx, discussionPath(course.ID, 0, ev.topicID), &topic); err != nil {
		ev.failure = "the discussion topic could not be read: " + err.Error()
		return ev
	}
	if topic.ID != ev.topicID || (topic.AssignmentID != 0 && topic.AssignmentID != a.ID) {
		ev.failure = "the discussion topic is not linked to this assignment in Canvas"
		return ev
	}
	setID := int64(0)
	if topic.GroupCategoryID != nil {
		setID = *topic.GroupCategoryID
		if setID > 0 {
			if _, err := s.groupCategory(ctx, course.ID, setID); err != nil {
				ev.failure = err.Error()
				return ev
			}
		}
	}
	folder, err := workspace.ConstrainPath(s.Root, filepath.Join(relDir, "discussion"))
	if err == nil {
		err = os.MkdirAll(folder, 0755)
	}
	if err != nil {
		ev.failure = err.Error()
		return ev
	}
	packet, err := s.PullDiscussionEvidence(ctx, course.Nickname, topic, course, setID, 0, folder)
	if err != nil {
		ev.failure = "the conversation could not be read: " + err.Error()
		return ev
	}
	path, err := writeDiscussionPacket(folder, packet)
	if err != nil {
		ev.failure = err.Error()
		return ev
	}
	rel, err := filepath.Rel(s.Root, path)
	if err != nil {
		ev.failure = err.Error()
		return ev
	}
	ev.packetRel, ev.packet = filepath.ToSlash(rel), packet
	return ev
}

// refFor selects one student's own attributed entries. Another person's posts
// are context, never this student's evidence.
func (e *discussionAssignmentEvidence) refFor(studentRef string) *domain.DiscussionEvidenceRef {
	ref := &domain.DiscussionEvidenceRef{Packet: e.packetRel, TopicID: e.topicID}
	if e.failure != "" {
		ref.Status, ref.Reasons = "unavailable", []string{e.failure}
		return ref
	}
	own := []domain.DiscussionEntry{}
	for _, entry := range e.packet.Entries {
		if entry.StudentRef == studentRef {
			own = append(own, entry)
			ref.EntryRefs = append(ref.EntryRefs, domain.DiscussionEntryRef(entry))
		}
	}
	ref.Status, ref.Reasons = discussionCoverage(e.packet, discussionRelevantSources(e.packet, studentRef, nil))
	sort.Strings(ref.EntryRefs)
	ref.Digest = domain.DiscussionEvidenceDigest(own)
	return ref
}

func discussionSourceKey(kind string, contextID, topicID int64) string {
	return fmt.Sprintf("%s:%d:%d", kind, contextID, topicID)
}

// A student's attributed posts identify the conversation(s) they assessed.
// Without that attribution, all observed sources remain relevant: absence must
// never be inferred by choosing an arbitrary club. Cited context adds its source.
func discussionRelevantSources(p domain.DiscussionPacket, studentRef string, contextRefs []string) map[string]bool {
	keys := map[string]bool{}
	hasOwn := false
	cited := map[string]bool{}
	for _, ref := range contextRefs {
		cited[ref] = true
	}
	for _, entry := range p.Entries {
		if entry.StudentRef == studentRef {
			hasOwn = true
		}
		if entry.StudentRef == studentRef || cited[domain.DiscussionEntryRef(entry)] {
			keys[discussionSourceKey(entry.ContextType, entry.ContextID, entry.TopicID)] = true
		}
	}
	if !hasOwn {
		for _, source := range p.Sources {
			keys[discussionSourceKey(source.ContextType, source.ContextID, source.TopicID)] = true
		}
	}
	return keys
}

func discussionCoverage(p domain.DiscussionPacket, keys map[string]bool) (string, []string) {
	status, reasons := "complete", []string{}
	if len(keys) == 0 {
		return "partial", []string{"conversation source coverage could not be established"}
	}
	seen := map[string]bool{}
	for _, source := range p.Sources {
		key := discussionSourceKey(source.ContextType, source.ContextID, source.TopicID)
		if !keys[key] {
			continue
		}
		seen[key] = true
		if source.Status != "complete" {
			status = "partial"
			reasons = append(reasons, fmt.Sprintf("%s %d topic %d: %s", source.ContextType, source.ContextID, source.TopicID, source.Reason))
		}
	}
	for key := range keys {
		if !seen[key] {
			status = "partial"
			reasons = append(reasons, "conversation source is unavailable: "+key)
		}
	}
	for _, entry := range p.Entries {
		if keys[discussionSourceKey(entry.ContextType, entry.ContextID, entry.TopicID)] && entry.AuthorKind == "unknown" {
			status = "partial"
			reasons = append(reasons, "some relevant entries have authors that could not be matched to a student")
			break
		}
	}
	return status, reasons
}

// discussionGuard holds the pre-write checks for a graded discussion: the
// topic/assignment association, checkpoint status, and whether each student's
// assessed posts (and cited context) still read the way they did at pull time.
type discussionGuard struct {
	topicHold  string
	checkpoint bool
	liveErr    string
	liveOwn    map[string][]domain.DiscussionEntry
	liveByRef  map[string]domain.DiscussionEntry
	livePacket domain.DiscussionPacket
	stored     map[string]domain.DiscussionPacket
}

func hasCheckpoints(a domain.AssignmentSnapshot) bool {
	return (a.HasSubAssignments != nil && *a.HasSubAssignments) || (a.Checkpoints != nil && len(*a.Checkpoints) > 0)
}

// newDiscussionGuard returns nil for an assignment that is not a graded discussion.
func (s *Service) newDiscussionGuard(ctx context.Context, doc domain.ResultsDocument, live domain.AssignmentSnapshot) *discussionGuard {
	if doc.Assignment.DiscussionTopicID == nil {
		return nil
	}
	g := &discussionGuard{checkpoint: hasCheckpoints(doc.Assignment) || hasCheckpoints(live), stored: map[string]domain.DiscussionPacket{}}
	topicID := *doc.Assignment.DiscussionTopicID
	if live.DiscussionTopicID != nil && *live.DiscussionTopicID != topicID {
		g.topicHold = "the assignment is now linked to a different discussion topic; pull-work again"
		return g
	}
	var course domain.Course
	for _, c := range s.Config.Courses {
		if c.ID == doc.Assignment.CourseID {
			course = c
		}
	}
	var topic NativeDiscussion
	if err := s.Client.Get(ctx, discussionPath(course.ID, 0, topicID), &topic); err != nil {
		g.liveErr = "the discussion topic could not be re-read: " + err.Error()
		return g
	}
	if topic.ID != topicID || (topic.AssignmentID != 0 && topic.AssignmentID != doc.Assignment.AssignmentID) {
		g.topicHold = "the discussion topic is no longer linked to this assignment"
		return g
	}
	pending := false
	for _, r := range doc.Students {
		if (r.Grade != nil && !gradeComplete(r.Grade)) || (r.CommentOnly != nil && !definite(r.CommentOnly.Outcome)) {
			pending = true
		}
	}
	if !pending {
		return g
	}
	setID := int64(0)
	if topic.GroupCategoryID != nil {
		setID = *topic.GroupCategoryID
	}
	folder, err := writePullFolder(s.Root, course.Nickname, topicID, "recheck")
	if err != nil {
		g.liveErr = err.Error()
		return g
	}
	defer os.RemoveAll(folder)
	packet, err := s.PullDiscussionEvidence(ctx, course.Nickname, topic, course, setID, 0, folder)
	if err != nil {
		g.liveErr = "the conversation could not be re-read: " + err.Error()
		return g
	}
	g.livePacket = packet
	g.liveOwn, g.liveByRef = map[string][]domain.DiscussionEntry{}, map[string]domain.DiscussionEntry{}
	for _, entry := range packet.Entries {
		g.liveByRef[domain.DiscussionEntryRef(entry)] = entry
		if entry.StudentRef != "" {
			g.liveOwn[entry.StudentRef] = append(g.liveOwn[entry.StudentRef], entry)
		}
	}
	return g
}

// hold returns why this student's grade or comment cannot post yet, or "".
func (g *discussionGuard) hold(s *Service, r domain.StudentResult, src postSource) string {
	if g.topicHold != "" {
		return g.topicHold
	}
	var ev *domain.EvidenceSelection
	var components []domain.ComponentScore
	switch {
	case r.Grade != nil:
		ev, components = r.Grade.Evidence, r.Grade.Components
		if g.checkpoint {
			return "checkpointed discussion: its grade is entered in SpeedGrader, and no aggregate grade is posted here"
		}
	case r.CommentOnly != nil:
		ev, components = r.CommentOnly.Evidence, r.CommentOnly.Components
	default:
		return ""
	}
	work := src.work.DiscussionEvidence
	if work == nil || work.Status == "unavailable" {
		return "the discussion evidence was unavailable at pull; pull-work again before scoring"
	}
	if ev == nil || ev.Digest != work.Digest {
		return "the result's evidence does not match the pulled discussion work; pull-work again"
	}
	if g.liveErr != "" {
		return g.liveErr
	}
	for _, c := range components {
		if c.Evidence != "missing" {
			continue
		}
		packet, err := g.storedPacket(s, work.Packet)
		if err != nil {
			return "the pulled discussion packet could not be read: " + err.Error()
		}
		keys := discussionRelevantSources(packet, r.StudentRef, ev.ContextRefs)
		pulled, _ := discussionCoverage(packet, keys)
		fresh, _ := discussionCoverage(g.livePacket, keys)
		if pulled != "complete" || fresh != "complete" {
			return "discussion coverage was partial at pull or post, so missing work cannot be established; score unknown evidence as unknown"
		}
	}
	if domain.DiscussionEvidenceDigest(g.liveOwn[r.StudentRef]) != work.Digest {
		return "this student's discussion posts changed since the pull; pull-work again"
	}
	if len(ev.ContextRefs) > 0 {
		packet, err := g.storedPacket(s, work.Packet)
		if err != nil {
			return "the pulled discussion packet could not be read: " + err.Error()
		}
		byRef := map[string]domain.DiscussionEntry{}
		for _, entry := range packet.Entries {
			byRef[domain.DiscussionEntryRef(entry)] = entry
		}
		for _, ref := range ev.ContextRefs {
			old, ok := byRef[ref]
			cur, live := g.liveByRef[ref]
			if !ok {
				return "cited context entry " + ref + " is not in the pulled discussion"
			}
			if !live || domain.DiscussionEntrySignature(old) != domain.DiscussionEntrySignature(cur) {
				return "cited context entry " + ref + " changed since the pull; pull-work again"
			}
		}
	}
	return ""
}

func (g *discussionGuard) storedPacket(s *Service, rel string) (domain.DiscussionPacket, error) {
	if p, ok := g.stored[rel]; ok {
		return p, nil
	}
	path, err := workspace.ConstrainPath(s.Root, rel)
	if err != nil {
		return domain.DiscussionPacket{}, err
	}
	var p domain.DiscussionPacket
	if _, err = workspace.ReadDocument(path, &p); err != nil {
		return p, err
	}
	g.stored[rel] = p
	return p, nil
}

// discussionEvidenceText renders the student's attributed posts, then any cited
// context, for the review page so feedback is judged against the conversation.
func discussionEvidenceText(root string, work domain.StudentWorkDocument, ev *domain.EvidenceSelection) string {
	ref := work.DiscussionEvidence
	if ref == nil || ref.Packet == "" {
		return ""
	}
	path, err := workspace.ConstrainPath(root, ref.Packet)
	if err != nil {
		return ""
	}
	var packet domain.DiscussionPacket
	if _, err = workspace.ReadDocument(path, &packet); err != nil {
		return ""
	}
	byRef := map[string]domain.DiscussionEntry{}
	for _, entry := range packet.Entries {
		byRef[domain.DiscussionEntryRef(entry)] = entry
	}
	text := fmt.Sprintf("[discussion coverage: %s]", ref.Status)
	for _, reason := range ref.Reasons {
		text += "\n[limit: " + reason + "]"
	}
	for _, r := range ref.EntryRefs {
		if e, ok := byRef[r]; ok {
			text += "\n\n[their post " + e.CreatedAt + "]\n" + e.ReadableText
		}
	}
	if ev != nil {
		for _, r := range ev.ContextRefs {
			if e, ok := byRef[r]; ok {
				text += "\n\n[context from another participant]\n" + e.ReadableText
			}
		}
	}
	return text
}
