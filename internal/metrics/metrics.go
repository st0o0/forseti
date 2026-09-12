package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
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

	// reconcile (toggle: reconcile)
	reconcileRuns     *prometheus.CounterVec
	reconcileDuration *prometheus.HistogramVec
	reconcileChanges  *prometheus.CounterVec
	reconcileDrift    *prometheus.GaugeVec

	// always registered
	targetReachable    *prometheus.GaugeVec
	configAdlists      prometheus.Gauge
	configDenyDomains  prometheus.Gauge
	configAllowDomains prometheus.Gauge

	// stats (toggle: stats)
	statsQueries          *prometheus.GaugeVec
	statsBlocked          *prometheus.GaugeVec
	statsBlockedPct       *prometheus.GaugeVec
	statsGravityLastUpdate *prometheus.GaugeVec
	statsDomainsBlocked   *prometheus.GaugeVec
	statsQueriesForwarded *prometheus.GaugeVec
	statsQueriesCached    *prometheus.GaugeVec
	statsUniqueDomains    *prometheus.GaugeVec
	statsRequestFrequency *prometheus.GaugeVec
	statsClientsActive    *prometheus.GaugeVec
	statsClientsTotal     *prometheus.GaugeVec

	// query_types (toggle: query_types)
	queryTypes  *prometheus.GaugeVec
	queryStatus *prometheus.GaugeVec
	replyTypes  *prometheus.GaugeVec

	// upstreams (toggle: upstreams)
	upstreamQueries  *prometheus.GaugeVec
	upstreamResponse *prometheus.GaugeVec
	upstreamVariance *prometheus.GaugeVec

	// blocking (toggle: blocking)
	blockingStatus *prometheus.GaugeVec

	// gravity (toggle: gravity)
	gravityRuns     *prometheus.CounterVec
	gravityDuration *prometheus.HistogramVec
	gravityLastRun  *prometheus.GaugeVec
	gravityErrors   *prometheus.CounterVec

	// always registered
	collectorDuration  prometheus.Histogram
	collectorFetches   *prometheus.CounterVec
	collectorCacheHits *prometheus.CounterVec

	// sessions (toggle: sessions)
	sessionReauth *prometheus.CounterVec
	sessionActive prometheus.Gauge

	// always registered
	buildInfo     *prometheus.GaugeVec
	configReload  *prometheus.CounterVec
	targetHealth  *prometheus.GaugeVec

	// settings_drift (toggle: settings_drift)
	settingsDrift *prometheus.GaugeVec

	// dhcp (toggle: dhcp)
	dhcpLeasesActive *prometheus.GaugeVec

	mu               sync.Mutex
	knownQueryTypes  map[string]map[string]bool
	knownQueryStatus map[string]map[string]bool
	knownReplyTypes  map[string]map[string]bool
	knownUpstreams   map[string]map[string]bool
}

func NewServer(port int, path string, toggles config.CollectorToggles) *Server {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())

	s := &Server{
		registry:         reg,
		knownQueryTypes:  make(map[string]map[string]bool),
		knownQueryStatus: make(map[string]map[string]bool),
		knownReplyTypes:  make(map[string]map[string]bool),
		knownUpstreams:   make(map[string]map[string]bool),
	}

	// Always registered
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

	s.buildInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "forseti_build_info",
		Help: "Build information",
	}, []string{"version", "mode"})
	s.configReload = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "forseti_config_reload_total",
		Help: "Total config reload attempts",
	}, []string{"result"})

	s.targetHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "forseti_target_health",
		Help: "Target health state (0=healthy, 1=degraded, 2=down)",
	}, []string{"target"})

	reg.MustRegister(
		s.targetReachable,
		s.configAdlists, s.configDenyDomains, s.configAllowDomains,
		s.collectorDuration, s.collectorFetches, s.collectorCacheHits,
		s.buildInfo, s.configReload, s.targetHealth,
	)

	// Reconcile
	if toggles.IsEnabled("reconcile") {
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
		reg.MustRegister(s.reconcileRuns, s.reconcileDuration, s.reconcileChanges, s.reconcileDrift)
	}

	// Stats
	if toggles.IsEnabled("stats") {
		s.statsQueries = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_dns_queries",
			Help: "Total DNS queries",
		}, []string{"target"})
		s.statsBlocked = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_dns_queries_blocked",
			Help: "Total blocked queries",
		}, []string{"target"})
		s.statsBlockedPct = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_blocked_percentage",
			Help: "Blocked query percentage",
		}, []string{"target"})
		s.statsGravityLastUpdate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_gravity_last_update_timestamp",
			Help: "Unix timestamp of last gravity update",
		}, []string{"target"})
		s.statsDomainsBlocked = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_domains_blocked",
			Help: "Unique domains on blocklists",
		}, []string{"target"})
		s.statsQueriesForwarded = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_queries_forwarded",
			Help: "Forwarded query count",
		}, []string{"target"})
		s.statsQueriesCached = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_queries_cached",
			Help: "Cached query count",
		}, []string{"target"})
		s.statsUniqueDomains = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_unique_domains",
			Help: "Distinct domains queried",
		}, []string{"target"})
		s.statsRequestFrequency = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_request_frequency",
			Help: "DNS requests per second",
		}, []string{"target"})
		s.statsClientsActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_clients_active",
			Help: "Active clients (24h)",
		}, []string{"target"})
		s.statsClientsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_clients_seen",
			Help: "Cumulative unique clients",
		}, []string{"target"})
		reg.MustRegister(
			s.statsQueries, s.statsBlocked, s.statsBlockedPct,
			s.statsGravityLastUpdate, s.statsDomainsBlocked,
			s.statsQueriesForwarded, s.statsQueriesCached,
			s.statsUniqueDomains, s.statsRequestFrequency,
			s.statsClientsActive, s.statsClientsTotal,
		)
	}

	// Query types
	if toggles.IsEnabled("query_types") {
		s.queryTypes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_dns_queries_by_type",
			Help: "Queries by DNS record type",
		}, []string{"target", "query_type"})
		s.queryStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_dns_queries_by_status",
			Help: "Queries by resolution status",
		}, []string{"target", "status"})
		s.replyTypes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_dns_replies_by_type",
			Help: "Replies by type",
		}, []string{"target", "reply_type"})
		reg.MustRegister(s.queryTypes, s.queryStatus, s.replyTypes)
	}

	// Upstreams
	if toggles.IsEnabled("upstreams") {
		s.upstreamQueries = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_upstream_queries",
			Help: "Queries routed to each upstream",
		}, []string{"target", "upstream", "name", "port"})
		s.upstreamResponse = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_upstream_response_seconds",
			Help: "Average upstream response time",
		}, []string{"target", "upstream", "name", "port"})
		s.upstreamVariance = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_upstream_response_variance",
			Help: "Upstream response time variance",
		}, []string{"target", "upstream", "name", "port"})
		reg.MustRegister(s.upstreamQueries, s.upstreamResponse, s.upstreamVariance)
	}

	// Blocking
	if toggles.IsEnabled("blocking") {
		s.blockingStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_blocking_status",
			Help: "Pi-hole blocking status (1=enabled, 0=disabled)",
		}, []string{"target"})
		reg.MustRegister(s.blockingStatus)
	}

	// Gravity
	if toggles.IsEnabled("gravity") {
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
		reg.MustRegister(s.gravityRuns, s.gravityDuration, s.gravityLastRun, s.gravityErrors)
	}

	// Sessions
	if toggles.IsEnabled("sessions") {
		s.sessionReauth = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "forseti_session_reauth_total",
			Help: "Total session re-authentications",
		}, []string{"target"})
		s.sessionActive = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "forseti_session_active",
			Help: "Number of active sessions in pool",
		})
		reg.MustRegister(s.sessionReauth, s.sessionActive)
	}

	// Settings drift
	if toggles.IsEnabled("settings_drift") {
		s.settingsDrift = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_settings_drift",
			Help: "Settings drift from desired state (1=drifted, 0=in sync)",
		}, []string{"target", "setting"})
		reg.MustRegister(s.settingsDrift)
	}

	// DHCP
	if toggles.IsEnabled("dhcp") {
		s.dhcpLeasesActive = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forseti_dhcp_leases_active",
			Help: "Number of active DHCP leases",
		}, []string{"target"})
		reg.MustRegister(s.dhcpLeasesActive)
	}

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
	Changes  map[string]map[string]int
	Drift    map[string]int
}

func (s *Server) RecordReconcile(r ReconcileResult) {
	if s.reconcileRuns == nil {
		return
	}
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
	if s.statsQueries == nil {
		return
	}
	s.targetReachable.WithLabelValues(target).Set(1)
	s.statsQueries.WithLabelValues(target).Set(float64(stats.QueriesTotal))
	s.statsBlocked.WithLabelValues(target).Set(float64(stats.BlockedTotal))
	s.statsBlockedPct.WithLabelValues(target).Set(stats.BlockedPercentage)
	s.statsDomainsBlocked.WithLabelValues(target).Set(float64(stats.DomainsBlocked))
	s.statsGravityLastUpdate.WithLabelValues(target).Set(float64(stats.GravityLastUpdate))

	s.statsQueriesForwarded.WithLabelValues(target).Set(float64(stats.Forwarded))
	s.statsQueriesCached.WithLabelValues(target).Set(float64(stats.Cached))
	s.statsUniqueDomains.WithLabelValues(target).Set(float64(stats.UniqueDomains))
	s.statsRequestFrequency.WithLabelValues(target).Set(stats.Frequency)
	s.statsClientsActive.WithLabelValues(target).Set(float64(stats.ClientsActive))
	s.statsClientsTotal.WithLabelValues(target).Set(float64(stats.ClientsTotal))

	if s.queryTypes != nil {
		s.mu.Lock()
		s.knownQueryTypes[target] = updateGaugeMap(s.queryTypes, s.knownQueryTypes[target], target, stats.QueryTypes)
		s.knownQueryStatus[target] = updateGaugeMap(s.queryStatus, s.knownQueryStatus[target], target, stats.QueryStatus)
		s.knownReplyTypes[target] = updateGaugeMap(s.replyTypes, s.knownReplyTypes[target], target, stats.ReplyTypes)
		s.mu.Unlock()
	}
}

func (s *Server) UpdateBlockingStatus(target string, enabled bool) {
	if s.blockingStatus == nil {
		return
	}
	s.blockingStatus.WithLabelValues(target).Set(boolToFloat(enabled))
}

func (s *Server) UpdateUpstreams(target string, upstreams []pihole.UpstreamStats) {
	if s.upstreamQueries == nil {
		return
	}
	newKeys := make(map[string]bool, len(upstreams))
	for _, u := range upstreams {
		port := fmt.Sprintf("%d", u.Port)
		key := u.IP + "|" + u.Name + "|" + port
		newKeys[key] = true
		s.upstreamQueries.WithLabelValues(target, u.IP, u.Name, port).Set(float64(u.Count))
		s.upstreamResponse.WithLabelValues(target, u.IP, u.Name, port).Set(u.ResponseTime)
		s.upstreamVariance.WithLabelValues(target, u.IP, u.Name, port).Set(u.ResponseVariance)
	}

	s.mu.Lock()
	if prev, ok := s.knownUpstreams[target]; ok {
		for key := range prev {
			if !newKeys[key] {
				parts := splitUpstreamKey(key)
				s.upstreamQueries.DeleteLabelValues(target, parts[0], parts[1], parts[2])
				s.upstreamResponse.DeleteLabelValues(target, parts[0], parts[1], parts[2])
				s.upstreamVariance.DeleteLabelValues(target, parts[0], parts[1], parts[2])
			}
		}
	}
	s.knownUpstreams[target] = newKeys
	s.mu.Unlock()
}

func (s *Server) RecordGravityRun(target string, trigger string, duration time.Duration, err error) {
	if s.gravityRuns == nil {
		return
	}
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

func (s *Server) RecordTargetHealth(target string, state string) {
	var val float64
	switch state {
	case "healthy":
		val = 0
	case "degraded":
		val = 1
	case "down":
		val = 2
	}
	s.targetHealth.WithLabelValues(target).Set(val)
}

func (s *Server) RecordReconcileResult(target string, duration time.Duration, success bool, changes map[string]map[string]int, drift map[string]int) {
	s.RecordReconcile(ReconcileResult{
		Target:   target,
		Duration: duration,
		Success:  success,
		Changes:  changes,
		Drift:    drift,
	})
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
	if s.sessionReauth == nil {
		return
	}
	s.sessionReauth.WithLabelValues(target).Inc()
}

func (s *Server) IncSessionActive() {
	if s.sessionActive == nil {
		return
	}
	s.sessionActive.Inc()
}

func (s *Server) ResetSessionActive() {
	if s.sessionActive == nil {
		return
	}
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

func (s *Server) UpdateSettingsDrift(target string, drifted map[string]bool) {
	if s.settingsDrift == nil {
		return
	}
	for setting, isDrifted := range drifted {
		s.settingsDrift.WithLabelValues(target, setting).Set(boolToFloat(isDrifted))
	}
}

func (s *Server) UpdateDHCPLeases(target string, count int) {
	if s.dhcpLeasesActive == nil {
		return
	}
	s.dhcpLeasesActive.WithLabelValues(target).Set(float64(count))
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
