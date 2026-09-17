package server

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/terracotta4u/golem/release"
)

func (s *Server) reportUpdate(ctx context.Context, w io.Writer) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	st := s.updateStatus(ctx)
	if !st.Available {
		return
	}
	fmt.Fprintf(w, "update available: %s (current %s)\n", release.Display(st.Latest), release.Display(st.Current))
}

func (s *Server) updateStatus(ctx context.Context) release.Status {
	s.updateOnce.Do(func() {
		if s.opts.Release == nil {
			s.update = release.Status{Current: s.opts.Version}
			return
		}
		c := *s.opts.Release
		if c.Current == "" {
			c.Current = s.opts.Version
		}
		st, err := c.Check(ctx)
		if err != nil {
			s.update = release.Status{Current: s.opts.Version}
			return
		}
		s.update = st
	})
	return s.update
}
