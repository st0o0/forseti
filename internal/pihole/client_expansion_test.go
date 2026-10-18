package pihole

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// --- Allowlists (2.6) ---

func TestListAllowlists(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/lists" && r.URL.Query().Get("type") == "allow" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lists": []map[string]any{
					{"id": 1, "address": "https://allow.example.com/list.txt", "comment": "[forseti]", "enabled": true, "groups": []int{0}},
				},
			})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
	})

	lists, err := c.ListAllowlists()
	if err != nil {
		t.Fatalf("ListAllowlists() error: %v", err)
	}
	if len(lists) != 1 {
		t.Fatalf("len = %d, want 1", len(lists))
	}
	if lists[0].Address != "https://allow.example.com/list.txt" {
		t.Errorf("address = %q", lists[0].Address)
	}
}

func TestCreateAllowlist(t *testing.T) {
	var gotBody map[string]any
	var gotPath, gotMethod string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.String()
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"lists": []map[string]any{
				{"id": 10, "address": gotBody["address"]},
			},
		})
	})

	list, err := c.CreateAllowlist("https://allow.example.com/list.txt", "[forseti]", true, []int{0, 2})
	if err != nil {
		t.Fatalf("CreateAllowlist() error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if !strings.Contains(gotPath, "type=allow") {
		t.Errorf("path should contain type=allow, got %s", gotPath)
	}
	if list.ID != 10 {
		t.Errorf("ID = %d, want 10", list.ID)
	}
	groups := gotBody["groups"].([]any)
	if len(groups) != 1 || int(groups[0].(float64)) != 2 {
		t.Errorf("groups should exclude 0 on create, got %v", groups)
	}
}

func TestUpdateAllowlist(t *testing.T) {
	var gotPath, gotMethod string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.String()
		w.WriteHeader(http.StatusOK)
	})

	err := c.UpdateAllowlist("https://allow.example.com/list.txt", "[forseti]", []int{0, 1})
	if err != nil {
		t.Fatalf("UpdateAllowlist() error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if !strings.Contains(gotPath, "type=allow") {
		t.Errorf("path should contain type=allow, got %s", gotPath)
	}
}

func TestDeleteAllowlists(t *testing.T) {
	var gotBody []any
	var gotPath string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	})

	err := c.DeleteAllowlists([]string{"https://a.com/list.txt", "https://b.com/list.txt"})
	if err != nil {
		t.Fatalf("DeleteAllowlists() error: %v", err)
	}
	if len(gotBody) != 2 {
		t.Errorf("batch items = %d, want 2", len(gotBody))
	}
	if !strings.Contains(gotPath, "type=allow") {
		t.Errorf("path should contain type=allow, got %s", gotPath)
	}
}

func TestDeleteAllowlistsBatchFallback(t *testing.T) {
	var deleteCalls int

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "batchDelete") {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"key": "not_found", "message": "not found"}})
			return
		}
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/api/lists/") {
			deleteCalls++
			w.WriteHeader(http.StatusNoContent)
			return
		}
	})

	err := c.DeleteAllowlists([]string{"https://a.com/list.txt", "https://b.com/list.txt"})
	if err != nil {
		t.Fatalf("DeleteAllowlists() error: %v", err)
	}
	if deleteCalls != 2 {
		t.Errorf("individual delete calls = %d, want 2", deleteCalls)
	}
}

// --- Top Stats (3.6) ---

func TestGetTopDomains(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/stats/top_domains" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}
		if r.URL.Query().Get("count") != "5" {
			t.Errorf("count = %s, want 5", r.URL.Query().Get("count"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"top_domains": []map[string]any{
				{"domain": "google.com", "count": 1000},
				{"domain": "github.com", "count": 500},
			},
			"blocked_domains": []map[string]any{
				{"domain": "ads.tracker.com", "count": 300},
			},
		})
	})

	result, err := c.GetTopDomains(5)
	if err != nil {
		t.Fatalf("GetTopDomains() error: %v", err)
	}
	if len(result.TopQueries) != 2 {
		t.Fatalf("TopQueries len = %d, want 2", len(result.TopQueries))
	}
	if result.TopQueries[0].Name != "google.com" || result.TopQueries[0].Count != 1000 {
		t.Errorf("TopQueries[0] = %+v", result.TopQueries[0])
	}
	if len(result.TopBlocked) != 1 {
		t.Fatalf("TopBlocked len = %d, want 1", len(result.TopBlocked))
	}
	if result.TopBlocked[0].Name != "ads.tracker.com" {
		t.Errorf("TopBlocked[0].Name = %q", result.TopBlocked[0].Name)
	}
}

func TestGetTopClients(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"top_clients": []map[string]any{
				{"name": "desktop", "ip": "192.168.1.10", "count": 800},
				{"name": "", "ip": "10.0.0.5", "count": 200},
			},
		})
	})

	result, err := c.GetTopClients(10)
	if err != nil {
		t.Fatalf("GetTopClients() error: %v", err)
	}
	if len(result.TopClients) != 2 {
		t.Fatalf("len = %d, want 2", len(result.TopClients))
	}
	if result.TopClients[0].Name != "desktop" {
		t.Errorf("[0].Name = %q, want desktop", result.TopClients[0].Name)
	}
	if result.TopClients[1].Name != "10.0.0.5" {
		t.Errorf("[1].Name = %q, want IP fallback 10.0.0.5", result.TopClients[1].Name)
	}
}

func TestGetHistory(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/history" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"history": []map[string]any{
				{"timestamp": 1700000000, "total": 100, "blocked": 20},
				{"timestamp": 1700000600, "total": 150, "blocked": 30},
			},
		})
	})

	entries, err := c.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if entries[0].Timestamp != 1700000000 || entries[0].Total != 100 || entries[0].Blocked != 20 {
		t.Errorf("entries[0] = %+v", entries[0])
	}
}

func TestGetSystemInfoWithTemp(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/info/system" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"system": map[string]any{
				"cpu": 12.5,
				"memory": map[string]any{
					"ram": map[string]any{"percent_used": 45.3},
				},
				"sensors": map[string]any{"cpu_temp": 52.0},
				"load":    []float64{0.5, 0.8, 1.2},
			},
		})
	})

	info, err := c.GetSystemInfo()
	if err != nil {
		t.Fatalf("GetSystemInfo() error: %v", err)
	}
	if info.CPU != 12.5 {
		t.Errorf("CPU = %f, want 12.5", info.CPU)
	}
	if info.Memory != 45.3 {
		t.Errorf("Memory = %f, want 45.3", info.Memory)
	}
	if !info.HasTemp {
		t.Error("HasTemp = false, want true")
	}
	if info.Temperature != 52.0 {
		t.Errorf("Temperature = %f, want 52.0", info.Temperature)
	}
	if info.Load1 != 0.5 || info.Load5 != 0.8 || info.Load15 != 1.2 {
		t.Errorf("Load = (%f, %f, %f)", info.Load1, info.Load5, info.Load15)
	}
}

func TestGetSystemInfoNoTemp(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"system": map[string]any{
				"cpu": 5.0,
				"memory": map[string]any{
					"ram": map[string]any{"percent_used": 30.0},
				},
				"sensors": map[string]any{"cpu_temp": 0.0},
				"load":    []float64{0.1, 0.2, 0.3},
			},
		})
	})

	info, err := c.GetSystemInfo()
	if err != nil {
		t.Fatalf("GetSystemInfo() error: %v", err)
	}
	if info.HasTemp {
		t.Error("HasTemp = true, want false when temp is 0")
	}
	if info.Temperature != 0 {
		t.Errorf("Temperature = %f, want 0", info.Temperature)
	}
}

// --- UpdateDomain parameterization (4.4) ---

func TestUpdateDomainDenyExact(t *testing.T) {
	var gotPath, gotMethod string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	err := c.UpdateDomain("deny", "exact", "ads.example.com", "[forseti]", []int{0, 1})
	if err != nil {
		t.Fatalf("UpdateDomain() error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/api/domains/deny/exact/ads.example.com" {
		t.Errorf("path = %s, want /api/domains/deny/exact/ads.example.com", gotPath)
	}
}

func TestUpdateDomainAllowRegex(t *testing.T) {
	var gotPath string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	err := c.UpdateDomain("allow", "regex", ".*tracker.*", "[forseti]", []int{0})
	if err != nil {
		t.Fatalf("UpdateDomain() error: %v", err)
	}
	if !strings.HasPrefix(gotPath, "/api/domains/allow/regex/") {
		t.Errorf("path = %s, want prefix /api/domains/allow/regex/", gotPath)
	}
}

func TestUpdateDomainDenyRegex(t *testing.T) {
	var gotPath string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	err := c.UpdateDomain("deny", "regex", "^ads\\.", "[forseti]", []int{0})
	if err != nil {
		t.Fatalf("UpdateDomain() error: %v", err)
	}
	if !strings.HasPrefix(gotPath, "/api/domains/deny/regex/") {
		t.Errorf("path = %s, want prefix /api/domains/deny/regex/", gotPath)
	}
}

func TestUpdateDomainAllowExact(t *testing.T) {
	var gotPath string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	err := c.UpdateDomain("allow", "exact", "safe.example.com", "[forseti]", []int{0, 2})
	if err != nil {
		t.Fatalf("UpdateDomain() error: %v", err)
	}
	if gotPath != "/api/domains/allow/exact/safe.example.com" {
		t.Errorf("path = %s, want /api/domains/allow/exact/safe.example.com", gotPath)
	}
}
