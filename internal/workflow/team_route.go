package workflow

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"canvasexpertse/internal/canvas"
	"canvasexpertse/internal/domain"
	"canvasexpertse/internal/platform"
	"canvasexpertse/internal/workspace"
)

// KeySource supplies Canvas keys. Production reads Credential Manager; tests
// use a map.
type KeySource interface {
	Operator() (platform.Credential, error)
	Teammate(canvasUserID int64) (platform.Credential, error)
}

type platformKeys struct{}

func (platformKeys) Operator() (platform.Credential, error) { return platform.LoadCredential() }
func (platformKeys) Teammate(id int64) (platform.Credential, error) {
	return platform.LoadTeammateCredential(id)
}

// PlatformKeys returns the Credential Manager key source.
func PlatformKeys() KeySource { return platformKeys{} }

// routeState is shared by every ForCourse copy of one Service.
type routeState struct {
	mu       sync.Mutex
	operator map[int64]bool // course ID -> your own key may be used
	clients  map[int64]*canvas.Client
	noted    map[string]bool // expiry note printed, per teammate label
	logOnce  sync.Once
}

var routeInit sync.Mutex

func (s *Service) routing() *routeState {
	routeInit.Lock()
	defer routeInit.Unlock()
	if s.route == nil {
		s.route = &routeState{operator: map[int64]bool{}, clients: map[int64]*canvas.Client{}, noted: map[string]bool{}}
	}
	return s.route
}

// Owned reports whether course belongs to a teammate. Ownership comes only from
// the course's owner and the team list, whichever key is routed. An owner who is
// not on the team list is an input error, never "unowned".
func (s *Service) Owned(course domain.Course) (workspace.Teammate, bool, error) {
	if course.Owner == "" {
		return workspace.Teammate{}, false, nil
	}
	owner, ok := s.Team.Find(course.Owner)
	if !ok {
		return workspace.Teammate{}, false, fmt.Errorf("course %q has owner %q, who isn't on the team list", course.Nickname, course.Owner)
	}
	return owner, true, nil
}

// teammate is the teammate whose key this Service copy uses, or nil for yours.
func (s *Service) teammate() *workspace.Teammate { return s.team }

// requestCount is the HTTP request count for progress reports: a teammate-scoped
// Service counts its own client; otherwise your client plus teammate clients.
func (s *Service) requestCount() int64 {
	if s.team != nil {
		return s.Client.Requests()
	}
	return s.Client.Requests() + s.RoutedRequests()
}

// RoutedRequests counts HTTP requests made with teammate keys, so a command's
// reported total stays honest.
func (s *Service) RoutedRequests() int64 {
	if s.route == nil {
		return 0
	}
	s.route.mu.Lock()
	defer s.route.mu.Unlock()
	var n int64
	for _, c := range s.route.clients {
		n += c.Requests()
	}
	return n
}

// ForCourse returns the Service to use for one course: the receiver for your own
// courses, or a copy whose Client carries the key that course needs. Your key is
// used when you are an active teacher there; otherwise the owner's key is used.
func (s *Service) ForCourse(ctx context.Context, course domain.Course) (*Service, error) {
	if course.Owner == "" {
		return s, nil
	}
	owner, _, err := s.Owned(course)
	if err != nil {
		return nil, err
	}
	if s.Keys == nil {
		return s, nil
	}
	rs := s.routing()
	rs.mu.Lock()
	useOperator, decided := rs.operator[course.ID]
	client := rs.clients[course.ID]
	rs.mu.Unlock()
	if !decided {
		if useOperator, err = s.operatorTeaches(ctx, course.ID); err != nil {
			return nil, err
		}
		rs.mu.Lock()
		rs.operator[course.ID] = useOperator
		rs.mu.Unlock()
	}
	cp := *s
	cp.route = rs
	if useOperator {
		return &cp, nil
	}
	if client == nil {
		if client, err = s.teammateClient(rs, owner, course); err != nil {
			return nil, err
		}
		rs.mu.Lock()
		if existing := rs.clients[course.ID]; existing != nil {
			client = existing
		} else {
			rs.clients[course.ID] = client
		}
		rs.mu.Unlock()
	}
	s.noteExpiry(rs, owner)
	cp.Client, cp.team = client, &owner
	return &cp, nil
}

// operatorTeaches asks Canvas, with your key, whether you teach the course.
// A refusal means you don't; any other failure is returned and not cached.
func (s *Service) operatorTeaches(ctx context.Context, courseID int64) (bool, error) {
	var got struct {
		Enrollments []struct {
			Type            string `json:"type"`
			EnrollmentState string `json:"enrollment_state"`
		} `json:"enrollments"`
	}
	err := s.Client.Get(ctx, fmt.Sprintf("/api/v1/courses/%d", courseID), &got)
	if err != nil {
		var ce *canvas.Error
		if errors.As(err, &ce) && (ce.Status == http.StatusUnauthorized || ce.Status == http.StatusForbidden || ce.Status == http.StatusNotFound) {
			return false, nil
		}
		return false, err
	}
	for _, e := range got.Enrollments {
		if e.Type == "teacher" && e.EnrollmentState == "active" {
			return true, nil
		}
	}
	return false, nil
}

// TeammateClient builds a client from the teammate's saved key, for setup reads
// that aren't tied to a configured course.
func (s *Service) TeammateClient(owner workspace.Teammate) (*canvas.Client, error) {
	credential, err := s.Keys.Teammate(owner.CanvasUserID)
	if err != nil {
		if errors.Is(err, platform.ErrNoCredential) {
			return nil, errors.New(owner.MissingKeyMessage())
		}
		return nil, err
	}
	c, err := canvas.New(credential.Origin, credential.Token)
	credential.Token = ""
	if err != nil {
		return nil, err
	}
	if configured, e := canvas.New(s.Config.Instance, "validate-origin-only"); e != nil || configured.Origin() != c.Origin() {
		return nil, errors.New(workspace.OriginMismatchMessage(s.Config.Instance, c.Origin()))
	}
	return c, nil
}

func (s *Service) teammateClient(rs *routeState, owner workspace.Teammate, course domain.Course) (*canvas.Client, error) {
	c, err := s.TeammateClient(owner)
	if err != nil {
		return nil, err
	}
	root := s.InstallRoot
	c.OnWrite = func(method, path string, status int, uncertain bool) {
		if root == "" {
			return
		}
		entry := workspace.TeamLogEntry{Teammate: owner.Label, Course: course.Nickname, CourseID: course.ID, Method: method, Path: path, Status: status, Uncertain: uncertain}
		if err := workspace.AppendTeamLog(root, entry); err != nil {
			rs.logOnce.Do(func() { fmt.Fprintln(os.Stderr, "Note: the Canvas action log couldn't be written:", err) })
		}
	}
	c.MapError = func(err error) error {
		var ce *canvas.Error
		if errors.As(err, &ce) && ce.Kind == "authentication" {
			return &teammateAuthError{err: ce, name: owner.Name, course: course.Nickname}
		}
		return err
	}
	return c, nil
}

// noteExpiry prints the expiry line once per teammate per process.
func (s *Service) noteExpiry(rs *routeState, owner workspace.Teammate) {
	if s.Status == nil || owner.ExpiryState(time.Now()) == workspace.KeyOK {
		return
	}
	rs.mu.Lock()
	first := !rs.noted[owner.Label]
	rs.noted[owner.Label] = true
	rs.mu.Unlock()
	if first {
		fmt.Fprintf(s.Status, "Note: %s's Canvas key expires %s.\n", owner.Name, owner.KeyExpires)
	}
}

// teammateAuthError replaces "Canvas authentication failed" when a teammate's
// key is refused. It still unwraps to the *canvas.Error.
type teammateAuthError struct {
	err          *canvas.Error
	name, course string
}

func (e *teammateAuthError) Error() string {
	return fmt.Sprintf("Canvas didn't accept %s's key for %s. The key may have expired or been deleted, or %s may not teach that course. Nothing more was done in that course.", e.name, e.course, e.name)
}
func (e *teammateAuthError) Unwrap() error { return e.err }
