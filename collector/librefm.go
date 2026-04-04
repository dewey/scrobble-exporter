package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// LibrefmScraper fetches all-time play counts from the Libre.fm API.
type LibrefmScraper struct {
	apiKey   string
	username string
	client   *http.Client
}

func NewLibrefmScraper(apiKey, username string) *LibrefmScraper {
	return &LibrefmScraper{
		apiKey:   apiKey,
		username: username,
		client:   &http.Client{},
	}
}

func (s *LibrefmScraper) ServiceName() string { return "librefm" }
func (s *LibrefmScraper) Username() string     { return s.username }

func (s *LibrefmScraper) Scrape(ctx context.Context) (PlayCounts, error) {
	params := url.Values{}
	params.Set("method", "user.getInfo")
	params.Set("user", s.username)
	params.Set("api_key", s.apiKey)
	params.Set("format", "json")

	reqURL := "https://libre.fm/2.0/?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return PlayCounts{}, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return PlayCounts{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return PlayCounts{}, &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("librefm API returned status %d", resp.StatusCode),
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
		return PlayCounts{}, fmt.Errorf("decode librefm response: %w", err)
	}
	if result.Error != 0 {
		return PlayCounts{}, fmt.Errorf("librefm API error %d: %s", result.Error, result.Message)
	}

	count, err := strconv.ParseInt(result.User.Playcount, 10, 64)
	if err != nil {
		return PlayCounts{}, fmt.Errorf("parse librefm playcount %q: %w", result.User.Playcount, err)
	}

	return PlayCounts{AllTime: count}, nil
}
