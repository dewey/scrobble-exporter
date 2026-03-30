package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dewey/scrobble-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var indexTmpl = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html><head><title>scrobble-exporter</title></head>
<body>
<h1>scrobble-exporter</h1>
<h2>Active collectors:</h2>
<ul>
{{range .Scrapers}}<li>{{.ServiceName}} / {{.Username}}</li>
{{end}}
</ul>
<p><a href="{{.MetricsPath}}">{{.MetricsPath}}</a></p>
</body></html>`))

func main() {
	rootCmd := &cobra.Command{
		Use:   "scrobble-exporter",
		Short: "Prometheus exporter for music scrobble counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			listenAddress := viper.GetString("listen-address")
			metricsPath := viper.GetString("metrics-path")

			scrapers := buildScrapers()
			if len(scrapers) == 0 {
				log.Println("warning: no scrapers configured, exporter will emit no metrics")
			}

			col := collector.NewScrobbleCollector(scrapers)
			reg := prometheus.NewRegistry()
			reg.MustRegister(col)

			mux := http.NewServeMux()
			mux.Handle(metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
			mux.HandleFunc("/up", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "OK")
			})
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				data := struct {
					Scrapers    []collector.Scraper
					MetricsPath string
				}{
					Scrapers:    col.Scrapers(),
					MetricsPath: metricsPath,
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if err := indexTmpl.Execute(w, data); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			})

			log.Printf("listening on %s, metrics at %s", listenAddress, metricsPath)
			return http.ListenAndServe(listenAddress, mux)
		},
	}

	rootCmd.Flags().String("listen-address", ":9101", "Address to expose metrics on (env: LISTEN_ADDRESS)")
	rootCmd.Flags().String("metrics-path", "/metrics", "Path to expose metrics on (env: METRICS_PATH)")

	viper.BindPFlags(rootCmd.Flags())
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildScrapers() []collector.Scraper {
	var scrapers []collector.Scraper

	apiKey := os.Getenv("LASTFM_API_KEY")
	lastfmUsers := os.Getenv("LASTFM_USERNAMES")
	if apiKey == "" || lastfmUsers == "" {
		log.Println("warning: LASTFM_API_KEY or LASTFM_USERNAMES not set, skipping Last.fm")
	} else {
		for _, u := range strings.Split(lastfmUsers, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				scrapers = append(scrapers, collector.NewLastfmScraper(apiKey, u))
			}
		}
	}

	lbUsers := os.Getenv("LISTENBRAINZ_USERNAMES")
	if lbUsers == "" {
		log.Println("warning: LISTENBRAINZ_USERNAMES not set, skipping Listenbrainz")
	} else {
		for _, u := range strings.Split(lbUsers, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				scrapers = append(scrapers, collector.NewListenbrainzScraper(u))
			}
		}
	}

	return scrapers
}
