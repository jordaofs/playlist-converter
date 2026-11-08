package domain

import (
	"context"
	"time"
)

type Song struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	DurationMS  int      `json:"durationMs"`
	URL         string   `json:"url"`
	ISRC        string   `json:"isrc,omitempty"`
	Explicit    *bool    `json:"explicit,omitempty"`
	Unavailable string   `json:"unavailable,omitempty"`
}
type Playlist struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	OwnerID  string `json:"ownerId"`
	Revision string `json:"revision,omitempty"`
	URL      string `json:"url"`
	Songs    []Song `json:"songs"`
}
type Identity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry"`
}
type Provider interface {
	Identity(context.Context, Token) (Identity, error)
	ReadPlaylist(context.Context, Token, string) (Playlist, error)
	Search(context.Context, Token, Song) ([]Song, error)
	CreatePlaylist(context.Context, Token, string, string) (Playlist, error)
	Append(context.Context, Token, string, []string) error
	FindCreated(context.Context, Token, string) ([]Playlist, error)
}
type ProviderError struct {
	Code           string
	RetryAfter     time.Duration
	UnknownOutcome bool
}

func (e *ProviderError) Error() string { return e.Code }

type Candidate struct {
	Song    Song     `json:"song"`
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons"`
}
type Item struct {
	ID         string      `json:"id"`
	Position   int         `json:"position"`
	Source     Song        `json:"source"`
	Candidates []Candidate `json:"candidates"`
	Decision   string      `json:"decision"`
	SelectedID string      `json:"selectedId,omitempty"`
	Reason     string      `json:"reason,omitempty"`
}
type Conversion struct {
	ID             string `json:"id"`
	PrincipalID    string `json:"-"`
	SourceProvider string `json:"sourceProvider"`
	TargetProvider string `json:"targetProvider"`
	SourceID       string `json:"sourceId"`
	SourceIdentity string `json:"-"`
	TargetIdentity string `json:"-"`
	SourceName     string `json:"sourceName"`
	Status         string `json:"status"`
	PauseReason    string `json:"pauseReason,omitempty"`
	Error          string `json:"error,omitempty"`
	Revision       int    `json:"revision"`
	Items          []Item `json:"items"`
	DestinationID  string `json:"destinationId,omitempty"`
	DestinationURL string `json:"destinationUrl,omitempty"`
	// DestinationCreatedAt is when this conversion created its destination;
	// zero for conversions created before the field existed.
	DestinationCreatedAt time.Time `json:"destinationCreatedAt,omitempty"`
	// AppendedAt is when the latest append was sent, saved before sending.
	AppendedAt  time.Time `json:"appendedAt,omitempty"`
	Name        string    `json:"name,omitempty"`
	Published   int       `json:"published"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	NextAttempt time.Time `json:"nextAttempt,omitempty"`
}
type Decision struct {
	ItemID      string `json:"itemId"`
	CandidateID string `json:"candidateId"`
	Action      string `json:"action"`
}
