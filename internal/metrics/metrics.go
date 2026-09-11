package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/st0o0/forseti/internal/config"
	"github.com/st0o0/forseti/internal/pihole"
)

type CollectFunc func(ctx context.Context)

type Server struct {
	httpServer  *http.Server
	registry    *prometheus.Registry
	collectFunc CollectFunc

	reconcileRuns     *prometheus.CounterVec
	reconcileDuration *prometheus.HistogramVec
	reconcileChanges  *prometheus.CounterVec
	reconcileDrift    *prometheus.GaugeVec
	targetReachable   *prometheus.GaugeVec

	configAdlists     prometheus.Gauge
	configDenyDomains prometheus.Gauge
	configAllowDomains prometheus.Gauge

	piholeQueries          *prometheus.GaugeVec
	piholeBlocked          *prometheus.GaugeVec
	piholeBlockedPct       *prometheus.GaugeVec
	piholeGravityLastUpdate *prometheus.GaugeVec
	piholeStatus           *prometheus.GaugeVec
	piholeDomainsBlocked   *prometheus.GaugeVec

	piholeQueriesForwarded *prometheus.GaugeVec
	piholeQueriesCached    *prometheus.GaugeVec
	piholeUniqueDomains    *prometheus.GaugeVec
	piholeRequestFrequency *prometheus.GaugeVec
	piholeClientsActive    *prometheus.GaugeVec
	piholeClientsTotal     *prometheus.GaugeVec

	piholeQueryTypes  *prometheus.GaugeVec
	piholeQueryStatus *prometheus.GaugeVec
	piholeReplyTypes  *prometheus.GaugeVec

	piholeUpstreamQueries  *prometheus.GaugeVec
	piholeUpstreamResponse *prometheus.GaugeVec
	piholeUpstreamVariance *prometheus.GaugeVec

	gravityRuns     *prometheus.CounterVec
	gravityDuration *prometheus.HistogramVec
	gravityLastRun  *prometheus.GaugeVec
	gravityErrors   *prometheus.CounterVec

	collectorDuration  prometheus.Histogram
	collectorFetches   *prometheus.CounterVec
	collectorCacheHits *prometheus.CounterVec

	sessionReauth *prometheus.CounterVec
	sessionActive prometheus.Gauge

	buildInfo    *prometheus.GaugeVec
	configReload *prometheus.CounterVec

	knownQueryTypes  map[string]map[string]bool // target -> set of keys
	knownQueryStatus map[string]map[string]bool
	knownReplyTypes  map[string]map[string]bool
	knownUpstreams   map[string]map[string]bool // target -> set of "ip|name|port"
}

func NewServer(port int, path string) *Server {
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector())

	s := &Server{
		registry:         reg,
		knownQueryTypes:  make(map[string]map[string]bool),
		knownQueryStatus: make(map[string]map[string]bool),
		knownReplyTypes:  make(map[string]map[string]bool),
		knownUpstreams:   make(map[string]map[string]bool),
	}

	s.reconcileRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "forseti_reconcile_runs_total",
		Help: "Total reconcile cycles",
	}, []string{"target", "status"})

	s.reconcileDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "forseti_reconcile_duration_seconds",
		Help:    "Duration of reconcile cycles",
		Buckets: prometheus.DefBuckets,
	}, []string{"target"})

	s.reconcileChanges = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "forseti_reconcile_changes_total",
		Help: "Total changes applied",
	}, []string{"target", "type", "action"})

	s.reconcileDrift = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "forseti_reconcile_drift",
		Help: "Items differing from desired state",
	}, []string{"target", "type"})

	s.targetReachable = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "forseti_target_reachable",
		Help: "Whether target is reachable (1=yes, 0=no)",
	}, []string{"target"})

	s.configAdlists = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "forseti_config_adlists",
		Help: "Number of configured adlists",
	})
	s.configDenyDomains = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "forseti_config_deny_domains",
		Help: "Number of configured deny domains",
	})
	s.configAllowDomains = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "forseti_config_allow_domains",
		Help: "Number of configured allow domains",
	})

	s.piholeQueries = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_dns_queries",
		Help: "Total DNS queries",
	}, []string{"target"})
	s.piholeBlocked = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_dns_queries_blocked",
		Help: "Total blocked queries",
	}, []string{"target"})
	s.piholeBlockedPct = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_blocked_percentage",
		Help: "Blocked query percentage",
	}, []string{"target"})
	s.piholeGravityLastUpdate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_gravity_last_update",
		Help: "Unix timestamp of last gravity update",
	}, []string{"target"})
	s.piholeStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_status",
		Help: "Pi-hole status (1=enabled, 0=disabled)",
	}, []string{"target"})
	s.piholeDomainsBlocked = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_domains_blocked",
		Help: "Unique domains on blocklists",
	}, []string{"target"})

	s.piholeQueriesForwarded = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_queries_forwarded",
		Help: "Forwarded query count",
	}, []string{"target"})
	s.piholeQueriesCached = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_queries_cached",
		Help: "Cached query count",
	}, []string{"target"})
	s.piholeUniqueDomains = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_unique_domains",
		Help: "Distinct domains queried",
	}, []string{"target"})
	s.piholeRequestFrequency = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_request_frequency",
		Help: "DNS requests per second",
	}, []string{"target"})
	s.piholeClientsActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_clients_active",
		Help: "Active clients (24h)",
	}, []string{"target"})
	s.piholeClientsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_clients_seen",
		Help: "Cumulative unique clients",
	}, []string{"target"})

	s.piholeQueryTypes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_dns_queries_by_type",
		Help: "Queries by DNS record type",
	}, []string{"target", "query_type"})
	s.piholeQueryStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_dns_queries_by_status",
		Help: "Queries by resolution status",
	}, []string{"target", "status"})
	s.piholeReplyTypes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_dns_replies_by_type",
		Help: "Replies by type",
	}, []string{"target", "reply_type"})

	s.piholeUpstreamQueries = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_upstream_queries",
		Help: "Queries routed to each upstream",
	}, []string{"target", "upstream", "name", "port"})
	s.piholeUpstreamResponse = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_upstream_response_seconds",
		Help: "Average upstream response time",
	}, []string{"target", "upstream", "name", "port"})
	s.piholeUpstreamVariance = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "pihole_upstream_response_variance",
		Help: "Upstream response time variance",
	}, []string{"target", "upstream", "name", "port"})

	s.gravityRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "forseti_gravity_runs_total",
		Help: "Total gravity update triggers",
	}, []string{"target", "trigger"})

	s.gravityDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "forseti_gravity_duration_seconds",
		Help:    "Duration of gravity updates",
		Buckets: prometheus.DefBuckets,
	}, []string{"target"})

	s.gravityLastRun = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "forseti_gravity_last_run_timestamp",
		Help: "Unix timestamp of last gravity run",
	}, []string{"target"})

	s.gravityErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "forseti_gravity_errors_total",
		Help: "Total gravity update errors",
	}, []string{"target"})

	s.collectorDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "forseti_collector_duration_seconds",
		Help:    "Duration of stats collection cycles",
		Buckets: prometheus.DefBuckets,
	})

	s.collectorFetches = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "forseti_collector_fetches_total",
		Help: "Total stats API fetches",
	}, []string{"target", "status"})

	s.collectorCacheHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "forseti_collector_cache_hits_total",
		Help: "Total stats cache hits",
	}, []string{"target"})

	s.sessionReauth = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "forseti_session_reauth_total",
		Help: "Total session re-authentications",
	}, []string{"target"})

	s.sessionActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "forseti_session_active",
		Help: "Number of active sessions in pool",
	})

	s.buildInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "forseti_build_info",
		Help: "Build information",
	}, []string{"version", "mode"})

	s.configReload = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "forseti_config_reload_total",
		Help: "Total config reload attempts",
	}, []string{"result"})

	reg.MustRegister(
		s.reconcileRuns, s.reconcileDuration, s.reconcileChanges,
		s.reconcileDrift, s.targetReachable,
		s.configAdlists, s.configDenyDomains, s.configAllowDomains,
		s.piholeQueries, s.piholeBlocked, s.piholeBlockedPct,
		s.piholeGravityLastUpdate,
		s.piholeStatus, s.piholeDomainsBlocked,
		s.piholeQueriesForwarded, s.piholeQueriesCached,
		s.piholeUniqueDomains, s.piholeRequestFrequency,
		s.piholeClientsActive, s.piholeClientsTotal,
		s.piholeQueryTypes, s.piholeQueryStatus, s.piholeReplyTypes,
		s.piholeUpstreamQueries, s.piholeUpstreamResponse, s.piholeUpstreamVariance,
		s.gravityRuns, s.gravityDuration, s.gravityLastRun, s.gravityErrors,
		s.collectorDuration, s.collectorFetches, s.collectorCacheHits,
		s.sessionReauth, s.sessionActive,
		s.buildInfo, s.configReload,
	)

	promHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if s.collectFunc != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			s.collectFunc(ctx)
		}
		promHandler.ServeHTTP(w, r)
	})

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return s
}

func (s *Server) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *Server) SetCollectFunc(f CollectFunc) {
	s.collectFunc = f
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) SetConfigMetrics(cfg *config.Config) {
	s.configAdlists.Set(float64(len(cfg.Adlists)))
	s.configDenyDomains.Set(float64(len(cfg.Deny)))
	s.configAllowDomains.Set(float64(len(cfg.Allow)))
}

type ReconcileResult struct {
	Target   string
	Duration time.Duration
	Success  bool
	Changes  map[string]map[string]int // type -> action -> count
	Drift    map[string]int            // type -> count
}

func (s *Server) RecordReconcile(r ReconcileResult) {
	status := "success"
	if !r.Success {
		status = "error"
	}
	s.reconcileRuns.WithLabelValues(r.Target, status).Inc()
	s.reconcileDuration.WithLabelValues(r.Target).Observe(r.Duration.Seconds())
	s.targetReachable.WithLabelValues(r.Target).Set(boolToFloat(r.Success))

	for resType, actions := range r.Changes {
		for action, count := range actions {
			s.reconcileChanges.WithLabelValues(r.Target, resType, action).Add(float64(count))
		}
	}
	for resType, count := range r.Drift {
		s.reconcileDrift.WithLabelValues(r.Target, resType).Set(float64(count))
	}
}

func (s *Server) UpdateStats(target string, stats *pihole.Stats) {
	s.targetReachable.WithLabelValues(target).Set(1)
	s.piholeQueries.WithLabelValues(target).Set(float64(stats.QueriesTotal))
	s.piholeBlocked.WithLabelValues(target).Set(float64(stats.BlockedTotal))
	s.piholeBlockedPct.WithLabelValues(target).Set(stats.BlockedPercentage)
	s.piholeDomainsBlocked.WithLabelValues(target).Set(float64(stats.DomainsBlocked))
	s.piholeGravityLastUpdate.WithLabelValues(target).Set(float64(stats.GravityLastUpdate))

	s.piholeQueriesForwarded.WithLabelValues(target).Set(float64(stats.Forwarded))
	s.piholeQueriesCached.WithLabelValues(target).Set(float64(stats.Cached))
	s.piholeUniqueDomains.WithLabelValues(target).Set(float64(stats.UniqueDomains))
	s.piholeRequestFrequency.WithLabelValues(target).Set(stats.Frequency)
	s.piholeClientsActive.WithLabelValues(target).Set(float64(stats.ClientsActive))
	s.piholeClientsTotal.WithLabelValues(target).Set(float64(stats.ClientsTotal))

	s.knownQueryTypes[target] = updateGaugeMap(s.piholeQueryTypes, s.knownQueryTypes[target], target, stats.QueryTypes)
	s.knownQueryStatus[target] = updateGaugeMap(s.piholeQueryStatus, s.knownQueryStatus[target], target, stats.QueryStatus)
	s.knownReplyTypes[target] = updateGaugeMap(s.piholeReplyTypes, s.knownReplyTypes[target], target, stats.ReplyTypes)
}

func (s *Server) UpdateBlockingStatus(target string, enabled bool) {
	s.piholeStatus.WithLabelValues(target).Set(boolToFloat(enabled))
}

func (s *Server) UpdateUpstreams(target string, upstreams []pihole.UpstreamStats) {
	newKeys := make(map[string]bool, len(upstreams))
	for _, u := range upstreams {
		port := fmt.Sprintf("%d", u.Port)
		key := u.IP + "|" + u.Name + "|" + port
		newKeys[key] = true
		s.piholeUpstreamQueries.WithLabelValues(target, u.IP, u.Name, port).Set(float64(u.Count))
		s.piholeUpstreamResponse.WithLabelValues(target, u.IP, u.Name, port).Set(u.ResponseTime)
		s.piholeUpstreamVariance.WithLabelValues(target, u.IP, u.Name, port).Set(u.ResponseVariance)
	}

	if prev, ok := s.knownUpstreams[target]; ok {
		for key := range prev {
			if !newKeys[key] {
				parts := splitUpstreamKey(key)
				s.piholeUpstreamQueries.DeleteLabelValues(target, parts[0], parts[1], parts[2])
				s.piholeUpstreamResponse.DeleteLabelValues(target, parts[0], parts[1], parts[2])
				s.piholeUpstreamVariance.DeleteLabelValues(target, parts[0], parts[1], parts[2])
			}
		}
	}
	s.knownUpstreams[target] = newKeys
}

func (s *Server) RecordGravityRun(target string, trigger string, duration time.Duration, err error) {
	s.gravityRuns.WithLabelValues(target, trigger).Inc()
	s.gravityDuration.WithLabelValues(target).Observe(duration.Seconds())
	s.gravityLastRun.WithLabelValues(target).Set(float64(time.Now().Unix()))
	if err != nil {
		s.gravityErrors.WithLabelValues(target).Inc()
	}
}

func (s *Server) MarkTargetUnreachable(target string) {
	s.targetReachable.WithLabelValues(target).Set(0)
}

func (s *Server) ObserveCollectorDuration(d time.Duration) {
	s.collectorDuration.Observe(d.Seconds())
}

func (s *Server) RecordCollectorFetch(target, status string) {
	s.collectorFetches.WithLabelValues(target, status).Inc()
}

func (s *Server) RecordCollectorCacheHit(target string) {
	s.collectorCacheHits.WithLabelValues(target).Inc()
}

func (s *Server) RecordSessionReauth(target string) {
	s.sessionReauth.WithLabelValues(target).Inc()
}

func (s *Server) IncSessionActive() {
	s.sessionActive.Inc()
}

func (s *Server) ResetSessionActive() {
	s.sessionActive.Set(0)
}

func (s *Server) SetBuildInfo(version, mode string) {
	s.buildInfo.WithLabelValues(version, mode).Set(1)
}

func (s *Server) RecordConfigReload(success bool) {
	result := "success"
	if !success {
		result = "failure"
	}
	s.configReload.WithLabelValues(result).Inc()
}

func updateGaugeMap(gauge *prometheus.GaugeVec, prev map[string]bool, target string, data map[string]int) map[string]bool {
	newKeys := make(map[string]bool, len(data))
	for key, count := range data {
		newKeys[key] = true
		gauge.WithLabelValues(target, key).Set(float64(count))
	}
	for key := range prev {
		if !newKeys[key] {
			gauge.DeleteLabelValues(target, key)
		}
	}
	return newKeys
}

func splitUpstreamKey(key string) [3]string {
	var parts [3]string
	i := 0
	start := 0
	for j := 0; j < len(key) && i < 2; j++ {
		if key[j] == '|' {
			parts[i] = key[start:j]
			i++
			start = j + 1
		}
	}
	parts[i] = key[start:]
	return parts
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
