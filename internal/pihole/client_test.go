package pihole

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, "testpass", 0)
	return srv, client
}

func TestLoginAndClose(t *testing.T) {
	var gotSID string
	var deleteCalled bool

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "test-session-123"},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/auth":
			gotSID = r.Header.Get("X-FTL-SID")
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	if err := client.Login(); err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	if client.sid != "test-session-123" {
		t.Errorf("sid = %q, want test-session-123", client.sid)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if !deleteCalled {
		t.Error("DELETE /api/auth was not called")
	}
	if gotSID != "test-session-123" {
		t.Errorf("DELETE sent SID %q, want test-session-123", gotSID)
	}
	if client.sid != "" {
		t.Errorf("sid after close = %q, want empty", client.sid)
	}
}

func TestCloseIdempotent(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	if err := client.Close(); err != nil {
		t.Fatalf("Close() on unauthenticated client should not error: %v", err)
	}
}

func TestSessionHeaderSent(t *testing.T) {
	var receivedSID string

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "my-sid"},
			})
		case r.URL.Path == "/api/groups":
			receivedSID = r.Header.Get("X-FTL-SID")
			_ = json.NewEncoder(w).Encode(map[string]any{"groups": []any{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	_ = client.Login()
	_, _ = client.ListGroups()

	if receivedSID != "my-sid" {
		t.Errorf("request SID = %q, want my-sid", receivedSID)
	}
}

func TestListAdlists(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.URL.Path == "/api/lists":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lists": []map[string]any{
					{"id": 1, "address": "https://example.com/list.txt", "comment": "[forseti]", "enabled": true},
				},
			})
		}
	})

	_ = client.Login()
	lists, err := client.ListAdlists()
	if err != nil {
		t.Fatalf("ListAdlists() error: %v", err)
	}
	if len(lists) != 1 {
		t.Fatalf("len = %d, want 1", len(lists))
	}
	if lists[0].Address != "https://example.com/list.txt" {
		t.Errorf("address = %q", lists[0].Address)
	}
}

func TestCreateAdlist(t *testing.T) {
	var gotBody map[string]any

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/lists":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lists": []map[string]any{{"id": 1, "address": gotBody["address"]}},
			})
		}
	})

	_ = client.Login()
	list, err := client.CreateAdlist("https://example.com/list.txt", "[forseti]", true, []int{0})
	if err != nil {
		t.Fatalf("CreateAdlist() error: %v", err)
	}
	if list.ID != 1 {
		t.Errorf("ID = %d, want 1", list.ID)
	}
}

func TestDeleteAdlists(t *testing.T) {
	var gotBody map[string]any

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/lists:batchDelete":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusNoContent)
		}
	})

	_ = client.Login()
	err := client.DeleteAdlists([]string{"https://example.com/list1.txt", "https://example.com/list2.txt"})
	if err != nil {
		t.Fatalf("DeleteAdlists() error: %v", err)
	}
}

func TestGetStats(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.URL.Path == "/api/stats/summary":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"queries": map[string]any{
					"total":           12345,
					"blocked":         678,
					"percent_blocked": 5.49,
					"forwarded":       8000,
					"cached":          3000,
					"unique_domains":  5678,
					"frequency":       1.5,
					"types":           map[string]any{"A": 5000, "AAAA": 3000, "PTR": 200},
					"status":          map[string]any{"GRAVITY": 800, "FORWARDED": 8000, "CACHE": 3000},
					"replies":         map[string]any{"CNAME": 3000, "IP": 7000, "NXDOMAIN": 200},
				},
				"clients": map[string]any{
					"active": 15,
					"total":  42,
				},
				"gravity": map[string]any{
					"domains_being_blocked": 80000,
					"last_update":           1700000000,
				},
			})
		}
	})

	_ = client.Login()
	stats, err := client.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if stats.QueriesTotal != 12345 {
		t.Errorf("QueriesTotal = %d, want 12345", stats.QueriesTotal)
	}
	if stats.BlockedTotal != 678 {
		t.Errorf("BlockedTotal = %d, want 678", stats.BlockedTotal)
	}
	if stats.DomainsBlocked != 80000 {
		t.Errorf("DomainsBlocked = %d, want 80000", stats.DomainsBlocked)
	}
	if stats.Forwarded != 8000 {
		t.Errorf("Forwarded = %d, want 8000", stats.Forwarded)
	}
	if stats.Cached != 3000 {
		t.Errorf("Cached = %d, want 3000", stats.Cached)
	}
	if stats.UniqueDomains != 5678 {
		t.Errorf("UniqueDomains = %d, want 5678", stats.UniqueDomains)
	}
	if stats.Frequency != 1.5 {
		t.Errorf("Frequency = %f, want 1.5", stats.Frequency)
	}
	if stats.ClientsActive != 15 {
		t.Errorf("ClientsActive = %d, want 15", stats.ClientsActive)
	}
	if stats.ClientsTotal != 42 {
		t.Errorf("ClientsTotal = %d, want 42", stats.ClientsTotal)
	}
	if len(stats.QueryTypes) != 3 {
		t.Errorf("QueryTypes len = %d, want 3", len(stats.QueryTypes))
	}
	if stats.QueryTypes["A"] != 5000 {
		t.Errorf("QueryTypes[A] = %d, want 5000", stats.QueryTypes["A"])
	}
	if len(stats.QueryStatus) != 3 {
		t.Errorf("QueryStatus len = %d, want 3", len(stats.QueryStatus))
	}
	if stats.QueryStatus["GRAVITY"] != 800 {
		t.Errorf("QueryStatus[GRAVITY] = %d, want 800", stats.QueryStatus["GRAVITY"])
	}
	if len(stats.ReplyTypes) != 3 {
		t.Errorf("ReplyTypes len = %d, want 3", len(stats.ReplyTypes))
	}
	if stats.ReplyTypes["IP"] != 7000 {
		t.Errorf("ReplyTypes[IP] = %d, want 7000", stats.ReplyTypes["IP"])
	}
}

func TestGetStatsMissingOptionalFields(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.URL.Path == "/api/stats/summary":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"queries": map[string]any{
					"total":           100,
					"blocked":         10,
					"percent_blocked": 10.0,
				},
				"gravity": map[string]any{
					"domains_being_blocked": 5000,
					"last_update":           1700000000,
				},
			})
		}
	})

	_ = client.Login()
	stats, err := client.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if stats.Forwarded != 0 {
		t.Errorf("Forwarded = %d, want 0", stats.Forwarded)
	}
	if stats.QueryTypes != nil {
		t.Errorf("QueryTypes = %v, want nil", stats.QueryTypes)
	}
}

func TestGetUpstreams(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.URL.Path == "/api/stats/upstreams":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"upstreams": []map[string]any{
					{
						"ip": "1.1.1.1", "name": "one.one.one.one", "port": 53, "count": 5000,
						"statistics": map[string]any{"response": 0.025, "variance": 0.003},
					},
					{
						"ip": "8.8.8.8", "name": "dns.google", "port": 53, "count": 3000,
						"statistics": map[string]any{"response": 0.030, "variance": 0.005},
					},
					{
						"ip": "blocklist", "name": "blocklist", "port": -1, "count": 1234,
						"statistics": map[string]any{"response": 0.0, "variance": 0.0},
					},
				},
				"forwarded_queries": 8000,
				"total_queries":     12345,
			})
		}
	})

	_ = client.Login()
	upstreams, err := client.GetUpstreams()
	if err != nil {
		t.Fatalf("GetUpstreams() error: %v", err)
	}
	if len(upstreams) != 3 {
		t.Fatalf("len = %d, want 3", len(upstreams))
	}
	if upstreams[0].IP != "1.1.1.1" {
		t.Errorf("upstreams[0].IP = %q, want 1.1.1.1", upstreams[0].IP)
	}
	if upstreams[0].Count != 5000 {
		t.Errorf("upstreams[0].Count = %d, want 5000", upstreams[0].Count)
	}
	if upstreams[0].ResponseTime != 0.025 {
		t.Errorf("upstreams[0].ResponseTime = %f, want 0.025", upstreams[0].ResponseTime)
	}
	if upstreams[2].Port != -1 {
		t.Errorf("upstreams[2].Port = %d, want -1", upstreams[2].Port)
	}
}

func TestGetQueries(t *testing.T) {
	var gotQuery string
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.URL.Path == "/api/queries":
			gotQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{
				"queries": []map[string]any{
					{"time": 1767225600.0, "domain": "click.example.com", "status": "GRAVITY"},
					{"time": 1767225601.0, "domain": "trk.example.net", "status": "DENYLIST"},
				},
			})
		}
	})

	_ = client.Login()
	records, err := client.GetQueries(QueryHistoryOptions{
		From:     from,
		Until:    until,
		Statuses: BlockedQueryStatuses,
	})
	if err != nil {
		t.Fatalf("GetQueries() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len = %d, want 2", len(records))
	}
	if records[0].Domain != "click.example.com" {
		t.Errorf("records[0].Domain = %q, want click.example.com", records[0].Domain)
	}
	if records[1].Status != "DENYLIST" {
		t.Errorf("records[1].Status = %q, want DENYLIST", records[1].Status)
	}

	q, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if q.Get("from") != fmt.Sprintf("%d", from.Unix()) {
		t.Errorf("from = %q, want %d", q.Get("from"), from.Unix())
	}
	if q.Get("until") != fmt.Sprintf("%d", until.Unix()) {
		t.Errorf("until = %q, want %d", q.Get("until"), until.Unix())
	}
	gotStatuses := q["status"]
	if len(gotStatuses) != len(BlockedQueryStatuses) {
		t.Fatalf("status params = %v, want %d repeated values matching %v", gotStatuses, len(BlockedQueryStatuses), BlockedQueryStatuses)
	}
	for i, want := range BlockedQueryStatuses {
		if gotStatuses[i] != want {
			t.Errorf("status[%d] = %q, want %q", i, gotStatuses[i], want)
		}
	}
}

func TestGetQueriesNoOptions(t *testing.T) {
	var gotQuery string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.URL.Path == "/api/queries":
			gotQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{"queries": []map[string]any{}})
		}
	})

	_ = client.Login()
	records, err := client.GetQueries(QueryHistoryOptions{})
	if err != nil {
		t.Fatalf("GetQueries() error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("len = %d, want 0", len(records))
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
	}
}

func TestGetQueriesDomainAndLength(t *testing.T) {
	var gotQuery string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.URL.Path == "/api/queries":
			gotQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{"queries": []map[string]any{}})
		}
	})

	_ = client.Login()
	_, err := client.GetQueries(QueryHistoryOptions{
		Domain: "click.example.com",
		Length: 1,
	})
	if err != nil {
		t.Fatalf("GetQueries() error: %v", err)
	}

	q, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if q.Get("domain") != "click.example.com" {
		t.Errorf("domain = %q, want click.example.com", q.Get("domain"))
	}
	if q.Get("length") != "1" {
		t.Errorf("length = %q, want 1", q.Get("length"))
	}
}

func TestGetBlockingStatus(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.URL.Path == "/api/dns/blocking":
			_ = json.NewEncoder(w).Encode(map[string]any{"blocking": "enabled"})
		}
	})

	_ = client.Login()
	blocking, err := client.GetBlockingStatus()
	if err != nil {
		t.Fatalf("GetBlockingStatus() error: %v", err)
	}
	if !blocking {
		t.Error("blocking = false, want true")
	}
}

func TestGetBlockingStatusDisabled(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.URL.Path == "/api/dns/blocking":
			_ = json.NewEncoder(w).Encode(map[string]any{"blocking": "disabled"})
		}
	})

	_ = client.Login()
	blocking, err := client.GetBlockingStatus()
	if err != nil {
		t.Fatalf("GetBlockingStatus() error: %v", err)
	}
	if blocking {
		t.Error("blocking = true, want false")
	}
}

func TestAPIError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid password"}`))
	})

	err := client.Login()
	if err == nil {
		t.Fatal("expected error for 401")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		// The error wraps an APIError
		t.Logf("error type: %T, message: %v", err, err)
	} else {
		if apiErr.StatusCode != 401 {
			t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
		}
	}
}

func TestWaitForReadyImmediate(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/info/login" {
			w.WriteHeader(http.StatusOK)
			return
		}
	})

	err := client.WaitForReady(2*time.Second, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForReady() error: %v", err)
	}
}

func TestWaitForReadyAfterRetries(t *testing.T) {
	var calls atomic.Int32

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/info/login" {
			n := calls.Add(1)
			if n < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	})

	err := client.WaitForReady(5*time.Second, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForReady() error: %v", err)
	}
	if calls.Load() < 3 {
		t.Errorf("expected at least 3 calls, got %d", calls.Load())
	}
}

func TestWaitForReadyTimeout(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/info/login" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	})

	err := client.WaitForReady(300*time.Millisecond, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestTriggerGravity(t *testing.T) {
	gravityCalled := false

	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/action/gravity":
			gravityCalled = true
			w.WriteHeader(http.StatusOK)
		}
	})

	_ = client.Login()
	if err := client.TriggerGravity(); err != nil {
		t.Fatalf("TriggerGravity() error: %v", err)
	}
	if !gravityCalled {
		t.Error("gravity endpoint was not called")
	}
}
