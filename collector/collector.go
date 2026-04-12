package collector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PlayCounts returned by every scraper.
// Period fields are pointers — nil means "not fetched / not supported yet"
type PlayCounts struct {
	AllTime   int64
	LastWeek  *int64
	LastMonth *int64
}

// Scraper is the only interface a new service must implement.
type Scraper interface {
	ServiceName() string
	Username() string
	Scrape(ctx context.Context) (PlayCounts, error)
}

// HTTPError carries an HTTP status code so the collector can surface it in the status_code label.
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

type cachedResult struct {
	counts     PlayCounts
	err        error
	statusCode string
	fetchedAt  time.Time
}

// ScrobbleCollector implements prometheus.Collector.
// It maintains an internal cache refreshed by a background loop so that
// Collect() never blocks on external API calls.
type ScrobbleCollector struct {
	scrapers []Scraper
	interval time.Duration

	mu    sync.RWMutex
	cache map[string]cachedResult

	playsTotal  *prometheus.Desc
	scrapeError *prometheus.Desc
	lastFetch   *prometheus.Desc
}

func NewScrobbleCollector(scrapers []Scraper, interval time.Duration) *ScrobbleCollector {
	return &ScrobbleCollector{
		scrapers: scrapers,
		interval: interval,
		cache:    make(map[string]cachedResult),
		playsTotal: prometheus.NewDesc(
			"scrobble_plays_total",
			"Total number of plays/scrobbles for a user on a service.",
			[]string{"service", "username"},
			nil,
		),
		scrapeError: prometheus.NewDesc(
			"scrobble_scrape_errors_total",
			"1 if the last scrape failed, 0 on success. Label status_code carries the HTTP status code; '0' means a non-HTTP error (network failure, parse error, etc.).",
			[]string{"service", "username", "status_code"},
			nil,
		),
		lastFetch: prometheus.NewDesc(
			"scrobble_last_fetch_timestamp_seconds",
			"Unix timestamp of the last completed cache refresh per service and user.",
			[]string{"service", "username"},
			nil,
		),
	}
}

// Start performs an initial blocking refresh so the cache is warm, then
// launches a background goroutine that refreshes every interval.
// Cancel ctx to stop the background loop.
func (c *ScrobbleCollector) Start(ctx context.Context) {
	c.refresh(ctx)
	go func() {
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.refresh(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// refresh calls all scrapers (with a per-refresh timeout) and updates the cache.
func (c *ScrobbleCollector) refresh(ctx context.Context) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for _, s := range c.scrapers {
		counts, err := s.Scrape(reqCtx)
		result := cachedResult{fetchedAt: time.Now()}
		if err != nil {
			result.err = err
			result.statusCode = "0"
			if httpErr, ok := err.(*HTTPError); ok {
				result.statusCode = fmt.Sprintf("%d", httpErr.StatusCode)
			}
		} else {
			result.counts = counts
		}
		c.mu.Lock()
		c.cache[s.ServiceName()+s.Username()] = result
		c.mu.Unlock()
	}
}

func (c *ScrobbleCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.playsTotal
	ch <- c.scrapeError
	ch <- c.lastFetch
}

// Collect serves metrics from the in-memory cache. It never performs network I/O.
func (c *ScrobbleCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, s := range c.scrapers {
		result, ok := c.cache[s.ServiceName()+s.Username()]
		if !ok {
			continue
		}
		ch <- prometheus.MustNewConstMetric(c.lastFetch, prometheus.GaugeValue, float64(result.fetchedAt.Unix()), s.ServiceName(), s.Username())
		if result.err != nil {
			ch <- prometheus.MustNewConstMetric(c.scrapeError, prometheus.GaugeValue, 1, s.ServiceName(), s.Username(), result.statusCode)
			continue
		}
		ch <- prometheus.MustNewConstMetric(c.playsTotal, prometheus.GaugeValue, float64(result.counts.AllTime), s.ServiceName(), s.Username())
		ch <- prometheus.MustNewConstMetric(c.scrapeError, prometheus.GaugeValue, 0, s.ServiceName(), s.Username(), "200")
	}
}

// Scrapers returns the list of registered scrapers (used by the index page).
func (c *ScrobbleCollector) Scrapers() []Scraper {
	return c.scrapers
}
