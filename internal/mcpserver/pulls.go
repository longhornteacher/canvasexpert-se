package mcpserver

import (
	"context"
	"strings"
	"time"
)

// Agent apps stop waiting on a tool after about 60 seconds (Claude's limit is
// not configurable), while a whole-course pull can take longer. Long pulls run
// on the server's context and the call returns before that limit; repeating
// the same call waits on the same run instead of starting a second pull.
var pullWait = 45 * time.Second

// An uncollected result older than this is dropped so a later request pulls
// fresh Canvas work instead of reporting an old run.
const pullResultTTL = 15 * time.Minute

type pullJob struct {
	done     chan struct{}
	out      TextOutput
	err      error
	finished time.Time
}

func (s *Server) longCommand(ctx context.Context, args ...string) (TextOutput, error) {
	key := strings.Join(args, "\x00")
	s.mu.Lock()
	if s.pulls == nil {
		s.pulls = map[string]*pullJob{}
	}
	job := s.pulls[key]
	if job != nil && !job.finished.IsZero() && time.Since(job.finished) > pullResultTTL {
		job = nil
	}
	if job == nil {
		job = &pullJob{done: make(chan struct{})}
		s.pulls[key] = job
		run := s.run
		if run == nil {
			run = s.command
		}
		go func() {
			out, err := run(s.ctx, args...)
			s.mu.Lock()
			job.out, job.err, job.finished = out, err, time.Now()
			s.mu.Unlock()
			close(job.done)
		}()
	}
	s.mu.Unlock()

	timer := time.NewTimer(pullWait)
	defer timer.Stop()
	select {
	case <-job.done:
		s.mu.Lock()
		if s.pulls[key] == job {
			delete(s.pulls, key)
		}
		s.mu.Unlock()
		return job.out, job.err
	case <-timer.C:
	case <-ctx.Done():
	}
	return TextOutput{State: "pending", Text: "Still pulling from Canvas; nothing is wrong. Call this tool again with the same inputs to keep waiting. It continues the same run and does not start a second pull."}, nil
}
