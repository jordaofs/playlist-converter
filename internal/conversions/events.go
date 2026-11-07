package conversions

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"playlistconverter/internal/domain"
	"regexp"
	"time"
)

// Conversion events are the operator's view of what the worker did. They carry
// a closed set of fields: job id, direction, status, a reason code and counts.
// Playlist names, song data, URLs, provider identities, principals and raw
// provider errors stay out, because those logs leave the application boundary.

var reasonCode = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

func (s *Service) logger() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.Default()
}

func code(value string) string {
	if reasonCode.MatchString(value) {
		return value
	}
	return "other"
}

func (s *Service) event(name string, c domain.Conversion, reason string) {
	attrs := []any{
		"conversion_id", c.ID,
		"direction", c.SourceProvider + "_to_" + c.TargetProvider,
		"status", c.Status,
		"items", len(c.Items),
		"published", c.Published,
	}
	if reason != "" {
		attrs = append(attrs, "reason", code(reason))
	}
	if !c.CreatedAt.IsZero() {
		attrs = append(attrs, "duration_ms", time.Since(c.CreatedAt).Milliseconds())
	}
	s.logger().Info(name, attrs...)
}

type stepFailure struct {
	code, detail string
}

// reportStep logs a worker step failure once per distinct error per job. The
// worker revisits active jobs every 100 ms, so logging every failure would
// flood the bounded container log and evict useful lines.
func (s *Service) reportStep(id string, err error) {
	s.stepMu.Lock()
	defer s.stepMu.Unlock()
	if err == nil {
		delete(s.stepErrors, id)
		return
	}
	failure := classify(err)
	if s.stepErrors == nil {
		s.stepErrors = map[string]stepFailure{}
	}
	if previous, seen := s.stepErrors[id]; seen && previous == failure {
		return
	}
	s.stepErrors[id] = failure
	attrs := []any{"conversion_id", id, "error_code", failure.code}
	if failure.detail != "" {
		attrs = append(attrs, "error", failure.detail)
	}
	s.logger().Error("conversion.step_failed", attrs...)
}

func classify(err error) stepFailure {
	var pe *domain.ProviderError
	var ce *Error
	var ue *url.Error
	switch {
	case errors.As(err, &pe):
		return stepFailure{code: code(pe.Code)}
	case errors.As(err, &ce):
		return stepFailure{code: code(ce.Code)}
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return stepFailure{code: "canceled"}
	case errors.As(err, &ue):
		// Request URLs can include search terms taken from private playlists.
		return stepFailure{code: "network"}
	}
	detail := err.Error()
	if len(detail) > 200 {
		detail = detail[:200]
	}
	return stepFailure{code: "internal", detail: detail}
}
