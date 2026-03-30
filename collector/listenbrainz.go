package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ListenbrainzScraper fetches all-time listen counts from the ListenBrainz API.
type ListenbrainzScraper struct {
	username string
	client   *http.Client
}

func NewListenbrainzScraper(username string) *ListenbrainzScraper {
	return &ListenbrainzScraper{
		username: username,
		client:   &http.Client{},
	}
}

func (s *ListenbrainzScraper) ServiceName() string { return "listenbrainz" }
func (s *ListenbrainzScraper) Username() string     { return s.username }

func (s *ListenbrainzScraper) Scrape(ctx context.Context) (PlayCounts, error) {
	reqURL := fmt.Sprintf("https://api.listenbrainz.org/1/user/%s/listen-count", s.username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return PlayCounts{}, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return PlayCounts{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return PlayCounts{}, &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("listenbrainz API returned status %d", resp.StatusCode),
		}
	}

	var result struct {
		Payload struct {
			Count int64 `json:"count"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return PlayCounts{}, fmt.Errorf("decode listenbrainz response: %w", err)
	}

	return PlayCounts{AllTime: result.Payload.Count}, nil
}
