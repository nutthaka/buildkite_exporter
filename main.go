package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	namespace = "buildkite"
)

// Metric descriptors.
var (
	buildsTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "builds_total"),
		"Total number of buildkite builds",
		[]string{"state", "pipeline"}, nil,
	)
)

// Exporter for buildkite
type Exporter struct {
	URL, Organization, Token string
	totalScrapes             prometheus.Counter
	scrapeErrors             prometheus.Counter
}

// NewExporter returns a new Buildkite exporter
func NewExporter(uri, org, token string, timeout time.Duration) *Exporter {
	return &Exporter{
		URL:          uri,
		Organization: org,
		Token:        token,
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scrapes_total",
			Help:      "Total number of times Buildkite was scraped for metrics.",
		}),
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scrape_errors_total",
			Help:      "Total number of errors while attempting to scrape Buildkite.",
		}),
	}
}

// Describe describes all the metrics ever exported by the Buildkite exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- buildsTotalDesc
	ch <- e.totalScrapes.Desc()
	ch <- e.scrapeErrors.Desc()
}

// Collect fetches the stats from Buildkite and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)

	ch <- e.totalScrapes
	ch <- e.scrapeErrors
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	pipelines, err := e.fetch()

	if err != nil {
		log.Errorf("some error %s", err)
		e.scrapeErrors.Inc()
		return
	}

	for _, pipeline := range pipelines {
		for _, stat := range pipeline.Stats {
			ch <- prometheus.MustNewConstMetric(buildsTotalDesc, prometheus.UntypedValue, float64(stat.Count), stat.State, pipeline.Slug)
		}
	}
}

func (e *Exporter) fetchStateTypes() ([]string, error) {
	query := `
	{
		__type(name:"BuildStates") {
			enumValues {
				name
			}
		}
	}
	`

	var body struct {
		Data struct {
			Type struct {
				Values []struct {
					Name string `json:"name"`
				} `json:"enumValues"`
			} `json:"__type"`
		} `json:"data"`
	}

	b, err := doGraphQlRequest(e.URL, e.Token, query)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &body)
	if err != nil {
		return nil, err
	}

	var result []string

	for _, val := range body.Data.Type.Values {
		result = append(result, val.Name)
	}

	return result, nil
}

func (e *Exporter) fetch() ([]PipelineStats, error) {
	states, err := e.fetchStateTypes()
	if err != nil {
		return nil, err
	}

	statesQuery := ""
	for i := range states {
		statesQuery += fmt.Sprintf("%s: builds(state: %s) { count }\n", strings.ToLower(states[i]), states[i])
	}

	query := fmt.Sprintf(`
	query BuildStatsQuery {
		organization(slug: "%s") {
			pipelines(first: 100) {
				count
				pageInfo {
					hasNextPage
					hasPreviousPage
				}
				edges {
					node {
						slug
						%s
					}
				}
			}
		}
	}
	`, e.Organization, statesQuery)

	var body struct {
		Data struct {
			Organization struct {
				Pipelines struct {
					Edges []struct {
						Node map[string]json.RawMessage
					} `json:"edges"`
				} `json:"pipelines"`
			} `json:"organization"`
		} `json:"data"`
	}
	log.Infoln("do")

	b, err := doGraphQlRequest(e.URL, e.Token, query)

	if err != nil {
		log.Errorf("1 %s", err)
		return nil, err
	}
	json.Unmarshal(b, &body)

	var pipelines []PipelineStats
	var state struct{ Count int }

	for _, edge := range body.Data.Organization.Pipelines.Edges {
		pipeline := PipelineStats{}

		for key, value := range edge.Node {
			switch key {
			case "slug":
				json.Unmarshal(value, &pipeline.Slug)
			default:
				json.Unmarshal(value, &state)
				stat := BuildStat{State: key, Count: state.Count}
				pipeline.Stats = append(pipeline.Stats, stat)
			}
		}
		pipelines = append(pipelines, pipeline)
	}
	return pipelines, nil
}

type graphQLRequest struct {
	Query string `json:"query"`
}

func doGraphQlRequest(url, token, query string) ([]byte, error) {
	b, err := json.Marshal(graphQLRequest{query})
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(b)

	req, err := http.NewRequest("POST", url, buf)

	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	return body, err
}

func main() {
	var (
		listenAddress         = flag.String("web.listen-address", ":9101", "Address to listen on for web interface and telemetry.")
		metricsPath           = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
		buildkiteScrapeURL    = flag.String("buildkite.scrape-url", "https://graphql.buildkite.com/v1", "graphql URL on which to scrape Buildkite.")
		buildkiteOrganization = flag.String("buildkite.organization", "", "Buildkite organization to scrape.")
		buildkiteToken        = flag.String("buildkite.token", "", "Buildkite graphql token.")
		buildkiteTimeout      = flag.Duration("buildkite.timeout", 10*time.Second, "Timeout for trying to get stats from Buildkite.")
	)
	flag.Parse()

	if *buildkiteOrganization == "" {
		log.Fatal("-buildkite.organization is required")
	}
	if *buildkiteToken == "" {
		log.Fatal("-buildkite.token is required")
	}

	exporter := NewExporter(*buildkiteScrapeURL, *buildkiteOrganization, *buildkiteToken, *buildkiteTimeout)

	prometheus.MustRegister(exporter)

	log.Infoln("Listening on", *listenAddress)
	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
           <head><title>Buildkite Exporter</title></head>
           <body>
           <h1>Buildkite Exporter</h1>
           <p><a href='` + *metricsPath + `'>Metrics</a></p>
           </body>
           </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

// BuildStat contains count for a build state
type BuildStat struct {
	State string
	Count int
}

// PipelineStats a collection of stats for a pipeline
type PipelineStats struct {
	Slug  string
	Stats []BuildStat
}
