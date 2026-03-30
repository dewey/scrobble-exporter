package collector

import (
	"context"
	"fmt"

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

// ScrobbleCollector implements prometheus.Collector.
type ScrobbleCollector struct {
	scrapers    []Scraper
	playsTotal  *prometheus.Desc
	scrapeError *prometheus.Desc
}

func NewScrobbleCollector(scrapers []Scraper) *ScrobbleCollector {
	return &ScrobbleCollector{
		scrapers: scrapers,
		playsTotal: prometheus.NewDesc(
			"scrobble_plays_total",
			"Total number of plays/scrobbles for a user on a service.",
			[]string{"service", "username"},
			nil,
		),
		scrapeError: prometheus.NewDesc(
			"scrobble_scrape_errors_total",
			"1 if the last scrape failed, 0 on success. Label status_code carries the HTTP status code or '0' for non-HTTP errors.",
			[]string{"service", "username", "status_code"},
			nil,
		),
	}
}

func (c *ScrobbleCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.playsTotal
	ch <- c.scrapeError
}

func (c *ScrobbleCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	for _, s := range c.scrapers {
		counts, err := s.Scrape(ctx)
		if err != nil {
			statusCode := "0"
			if httpErr, ok := err.(*HTTPError); ok {
				statusCode = fmt.Sprintf("%d", httpErr.StatusCode)
			}
			ch <- prometheus.MustNewConstMetric(c.scrapeError, prometheus.GaugeValue, 1, s.ServiceName(), s.Username(), statusCode)
			continue
		}
		ch <- prometheus.MustNewConstMetric(c.playsTotal, prometheus.GaugeValue, float64(counts.AllTime), s.ServiceName(), s.Username())
		ch <- prometheus.MustNewConstMetric(c.scrapeError, prometheus.GaugeValue, 0, s.ServiceName(), s.Username(), "0")
	}
}

// Scrapers returns the list of registered scrapers (used by the index page).
func (c *ScrobbleCollector) Scrapers() []Scraper {
	return c.scrapers
}
