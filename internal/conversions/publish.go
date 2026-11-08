package conversions

import (
	"context"
	"errors"
	"playlistconverter/internal/domain"
	"playlistconverter/internal/store"
	"time"
)

func marker(c domain.Conversion) string { return "playlist-converter:" + c.ID }
func (s *Service) publish(ctx context.Context, c domain.Conversion) error {
	token, err := s.credentials(ctx, c, c.TargetProvider)
	if err != nil {
		return s.handle(ctx, c, err)
	}
	provider := s.Providers[c.TargetProvider]
	if _, err = s.Store.Operation(ctx, c.ID); err == nil {
		return s.reconcile(ctx, c, token)
	} else if !store.IsMissing(err) {
		return err
	}
	if c.DestinationID == "" {
		if err = s.Store.Intent(ctx, c.ID, store.Operation{Kind: "create"}); err != nil {
			return err
		}
		p, e := provider.CreatePlaylist(ctx, token, c.Name, marker(c))
		if e != nil {
			return s.mutationError(ctx, c, e)
		}
		if p.ID == "" || p.OwnerID != "" && p.OwnerID != c.TargetIdentity {
			return s.handle(ctx, c, &domain.ProviderError{Code: "unknown_remote_outcome", UnknownOutcome: true})
		}
		c.DestinationID = p.ID
		c.DestinationURL = p.URL
		c.DestinationCreatedAt = time.Now()
		if _, err = s.save(ctx, c); err != nil {
			return err
		}
		return s.Store.ClearIntent(ctx, c.ID)
	}
	ids := selected(c)
	p, e := provider.ReadPlaylist(ctx, token, c.DestinationID)
	if e != nil {
		return s.handle(ctx, c, e)
	}
	if p.OwnerID != c.TargetIdentity || !exactPrefix(p.Songs, ids, c.Published) {
		if p.OwnerID == c.TargetIdentity && lagging(c, p.Songs, ids, c.Published) {
			return s.retryLater(ctx, c, appendRetry)
		}
		return s.handle(ctx, c, &domain.ProviderError{Code: "destination_changed"})
	}
	if c.Published == len(ids) {
		c.Status = "completed"
		c.PauseReason = ""
		if _, err = s.save(ctx, c); err != nil {
			return err
		}
		s.event("conversion.completed", c, "")
		return nil
	}
	id := ids[c.Published]
	if err = s.Store.Intent(ctx, c.ID, store.Operation{Kind: "append", Position: c.Published, TargetID: id}); err != nil {
		return err
	}
	c.AppendedAt = time.Now()
	if c, err = s.save(ctx, c); err != nil {
		return err
	}
	if err = provider.Append(ctx, token, c.DestinationID, []string{id}); err != nil {
		return s.mutationError(ctx, c, err)
	}
	// Even an acknowledged write is verified before advancing the durable cursor.
	return s.reconcile(ctx, c, token)
}
func (s *Service) mutationError(ctx context.Context, c domain.Conversion, err error) error {
	var pe *domain.ProviderError
	// A definitive provider rejection can be retried after its cause is resolved;
	// transport failures and unknown errors must retain the durable intent.
	if errors.As(err, &pe) && !pe.UnknownOutcome {
		if e := s.Store.ClearIntent(ctx, c.ID); e != nil {
			return e
		}
	} else if !errors.As(err, &pe) {
		err = &domain.ProviderError{Code: "unknown_remote_outcome", UnknownOutcome: true}
	}
	return s.handle(ctx, c, err)
}
func exactPrefix(songs []domain.Song, ids []string, n int) bool {
	if n < 0 || n > len(ids) || len(songs) != n {
		return false
	}
	for i, song := range songs {
		if song.ID != ids[i] {
			return false
		}
	}
	return true
}
