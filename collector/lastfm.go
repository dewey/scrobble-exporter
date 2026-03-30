package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// LastfmScraper fetches all-time play counts from the Last.fm API.
type LastfmScraper struct {
	apiKey   string
	username string
	client   *http.Client
}

func NewLastfmScraper(apiKey, username string) *LastfmScraper {
	return &LastfmScraper{
		apiKey:   apiKey,
		username: username,
		client:   &http.Client{},
	}
}

func (s *LastfmScraper) ServiceName() string { return "lastfm" }
func (s *LastfmScraper) Username() string     { return s.username }

func (s *LastfmScraper) Scrape(ctx context.Context) (PlayCounts, error) {
	params := url.Values{}
	params.Set("method", "user.getInfo")
	params.Set("user", s.username)
	params.Set("api_key", s.apiKey)
	params.Set("format", "json")

	reqURL := "https://ws.audioscrobbler.com/2.0/?" + params.Encode()
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
			Message:    fmt.Sprintf("lastfm API returned status %d", resp.StatusCode),
		}
	}

	var result struct {
		User struct {
			Playcount string `json:"playcount"`
		} `json:"user"`
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return PlayCounts{}, fmt.Errorf("decode lastfm response: %w", err)
	}
	// API-level errors in the JSON body are treated as non-HTTP errors (status_code="0").
	if result.Error != 0 {
		return PlayCounts{}, fmt.Errorf("lastfm API error %d: %s", result.Error, result.Message)
	}

	count, err := strconv.ParseInt(result.User.Playcount, 10, 64)
	if err != nil {
		return PlayCounts{}, fmt.Errorf("parse lastfm playcount %q: %w", result.User.Playcount, err)
	}

	return PlayCounts{AllTime: count}, nil
}
