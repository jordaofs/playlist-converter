package conversions

import (
	"context"
	"playlistconverter/internal/domain"
)

func (s *Service) reconcile(ctx context.Context, c domain.Conversion, token domain.Token) error {
	op, err := s.Store.Operation(ctx, c.ID)
	if err != nil {
		return err
	}
	provider := s.Providers[c.TargetProvider]
	switch op.Kind {
	case "create":
		found, e := provider.FindCreated(ctx, token, marker(c))
		if e != nil {
			return s.handle(ctx, c, e)
		}
		if len(found) != 1 || found[0].ID == "" || found[0].OwnerID != c.TargetIdentity {
			return s.handle(ctx, c, &domain.ProviderError{Code: "unknown_remote_outcome", UnknownOutcome: true})
		}
		if c.DestinationID != "" && c.DestinationID != found[0].ID {
			return s.handle(ctx, c, &domain.ProviderError{Code: "unknown_remote_outcome", UnknownOutcome: true})
		}
		c.DestinationID = found[0].ID
		c.DestinationURL = found[0].URL
	case "append":
		p, e := provider.ReadPlaylist(ctx, token, c.DestinationID)
		if e != nil {
			return s.handle(ctx, c, e)
		}
		ids := selected(c)
		if op.Position >= len(ids) || ids[op.Position] != op.TargetID || p.OwnerID != c.TargetIdentity || !exactPrefix(p.Songs, ids, op.Position+1) {
			return s.handle(ctx, c, &domain.ProviderError{Code: "unknown_remote_outcome", UnknownOutcome: true})
		}
		c.Published = op.Position + 1
	default:
		return s.handle(ctx, c, &domain.ProviderError{Code: "unknown_remote_outcome", UnknownOutcome: true})
	}
	c.Status = "publishing"
	c.PauseReason = ""
	c.Error = ""
	if _, err = s.save(ctx, c); err != nil {
		return err
	}
	return s.Store.ClearIntent(ctx, c.ID)
}
