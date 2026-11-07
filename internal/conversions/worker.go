package conversions

import (
	"context"
	"fmt"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/matching"
	"time"
)

// Step serializes work and mutations. Together with the Store's process lock this
// prevents overlapping writers, including after an application restart.
func (s *Service) Step(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.Store.Get(ctx, id)
	if err != nil {
		return err
	}
	if !active(c) || c.PauseReason != "" || time.Now().Before(c.NextAttempt) {
		return nil
	}
	switch c.Status {
	case "queued", "reading":
		err = s.read(ctx, c)
	case "matching":
		err = s.match(ctx, c)
	case "publishing":
		err = s.publish(ctx, c)
	}
	return err
}
func (s *Service) Start(ctx context.Context) {
	s.start.Do(func() {
		go func() {
			timer := time.NewTicker(100 * time.Millisecond)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					jobs, err := s.Store.List(ctx, "")
					if err != nil {
						continue
					}
					for _, c := range jobs {
						if ctx.Err() != nil {
							return
						}
						if err := s.Step(ctx, c.ID); ctx.Err() == nil {
							// Shutdown cancellations are expected and not reported.
							s.reportStep(c.ID, err)
						}
					}
				}
			}
		}()
	})
}
func (s *Service) read(ctx context.Context, c domain.Conversion) error {
	token, err := s.credentials(ctx, c, c.SourceProvider)
	if err != nil {
		return s.handle(ctx, c, err)
	}
	c.Status = "reading"
	if _, err = s.save(ctx, c); err != nil {
		return err
	}
	p, err := s.Providers[c.SourceProvider].ReadPlaylist(ctx, token, c.SourceID)
	if err != nil {
		return s.handle(ctx, c, err)
	}
	if p.OwnerID != c.SourceIdentity {
		return s.reject(ctx, c, "playlist_not_owned")
	}
	if len(p.Songs) == 0 {
		return s.reject(ctx, c, "empty_playlist")
	}
	if len(p.Songs) > s.MaxItems {
		return s.reject(ctx, c, "playlist_too_large")
	}
	// Revisions can detect edits across paginated reads; providers without revisions
	// expose a best-effort snapshot rather than an atomic read guarantee.
	if p.Revision != "" {
		check, e := s.Providers[c.SourceProvider].ReadPlaylist(ctx, token, c.SourceID)
		if e != nil {
			return s.handle(ctx, c, e)
		}
		if check.Revision != p.Revision {
			return s.reject(ctx, c, "source_changed")
		}
	}
	items := make([]domain.Item, len(p.Songs))
	for i, song := range p.Songs {
		items[i] = domain.Item{ID: fmt.Sprintf("%s-%d", c.ID, i), Position: i, Source: song, Candidates: []domain.Candidate{}, Decision: "pending"}
		if song.Unavailable != "" {
			items[i].Decision = "unavailable"
			items[i].Reason = song.Unavailable
		}
	}
	c.SourceName = p.Name
	c.Items = items
	c.Status = "matching"
	_, err = s.save(ctx, c)
	return err
}
func (s *Service) reject(ctx context.Context, c domain.Conversion, reason string) error {
	c.Status = "failed"
	c.Error = reason
	if _, err := s.save(ctx, c); err != nil {
		return err
	}
	s.event("conversion.failed", c, reason)
	return nil
}
func (s *Service) match(ctx context.Context, c domain.Conversion) error {
	for i, item := range c.Items {
		if item.Decision != "pending" {
			continue
		}
		token, err := s.credentials(ctx, c, c.TargetProvider)
		if err != nil {
			return s.handle(ctx, c, err)
		}
		// Reuse search evidence only within this immutable source snapshot.
		candidates := []domain.Candidate(nil)
		for j := 0; j < i; j++ {
			prior := c.Items[j]
			if item.Source.ID != "" && prior.Source.ID == item.Source.ID && prior.Decision != "pending" {
				candidates = append([]domain.Candidate{}, prior.Candidates...)
				break
			}
		}
		if candidates == nil {
			songs, e := s.Providers[c.TargetProvider].Search(ctx, token, item.Source)
			if e != nil {
				return s.handle(ctx, c, e)
			}
			candidates = matching.Rank(item.Source, songs)
		}
		decision, id, reason := matching.Decide(item.Source, candidates)
		item.Candidates = candidates
		item.Decision = decision
		item.SelectedID = id
		item.Reason = reason
		items := append([]domain.Item(nil), c.Items...)
		items[i] = item
		c.Items = items
		c.Revision++
		c.Status = reviewState(items)
		if c.Status == "needs_review" {
			for _, v := range items {
				if v.Decision == "pending" {
					c.Status = "matching"
					break
				}
			}
		}
		_, err = s.save(ctx, c)
		return err
	}
	c.Status = reviewState(c.Items)
	_, err := s.save(ctx, c)
	return err
}
