package pihole

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func authedServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/auth" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
			return
		}
		if r.Method == http.MethodDelete && r.URL.Path == "/api/auth" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "pw")
	_ = c.Login()
	return c
}

// HasSession

func TestHasSessionTrue(t *testing.T) {
	c := &Client{sid: "abc"}
	if !c.HasSession() {
		t.Error("HasSession() = false, want true")
	}
}

func TestHasSessionFalse(t *testing.T) {
	c := &Client{}
	if c.HasSession() {
		t.Error("HasSession() = true, want false")
	}
}

// Close edge case: server returns error

func TestCloseServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/auth" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
			return
		}
	}))
	c := NewClient(srv.URL, "pw")
	_ = c.Login()
	srv.Close()

	err := c.Close()
	if err == nil {
		t.Error("Close() should error when server is down")
	}
	if c.sid != "" {
		t.Errorf("sid should be cleared even on error, got %q", c.sid)
	}
}

// Domains

func TestListDomains(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/domains/deny/exact" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"domains": []map[string]any{
					{"id": 1, "domain": "ads.example.com", "type": "deny", "kind": "exact", "comment": "[forseti]"},
					{"id": 2, "domain": "tracker.com", "type": "deny", "kind": "exact", "comment": "[forseti]"},
				},
			})
			return
		}
	})

	domains, err := c.ListDomains("deny", "exact")
	if err != nil {
		t.Fatalf("ListDomains() error: %v", err)
	}
	if len(domains) != 2 {
		t.Fatalf("len = %d, want 2", len(domains))
	}
	if domains[0].Domain != "ads.example.com" {
		t.Errorf("domain = %q", domains[0].Domain)
	}
}

func TestListDomainsError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	})

	_, err := c.ListDomains("deny", "exact")
	if err == nil {
		t.Fatal("ListDomains() should error on 500")
	}
}

func TestCreateDomain(t *testing.T) {
	var gotBody map[string]any

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/domains/deny/exact" {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"domain": map[string]any{"id": 5, "domain": gotBody["domain"]},
			})
			return
		}
	})

	domain, err := c.CreateDomain("deny", "exact", "ads.example.com", "[forseti]", true, []int{0, 1})
	if err != nil {
		t.Fatalf("CreateDomain() error: %v", err)
	}
	if domain.ID != 5 {
		t.Errorf("ID = %d, want 5", domain.ID)
	}
	groups := gotBody["groups"].([]any)
	if len(groups) != 1 || int(groups[0].(float64)) != 1 {
		t.Errorf("groups should exclude 0 on create, got %v", groups)
	}
}

func TestCreateDomainError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	})

	_, err := c.CreateDomain("deny", "exact", "ads.example.com", "", true, nil)
	if err == nil {
		t.Fatal("CreateDomain() should error on 400")
	}
}

func TestDeleteDomains(t *testing.T) {
	var gotBody []any

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/domains:batchDelete" {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	})

	err := c.DeleteDomains([]string{"ads.example.com", "tracker.com"})
	if err != nil {
		t.Fatalf("DeleteDomains() error: %v", err)
	}
	if len(gotBody) != 2 {
		t.Errorf("batch items = %d, want 2", len(gotBody))
	}
}

// Groups

func TestListGroupsEmpty(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"groups": []any{}})
	})

	groups, err := c.ListGroups()
	if err != nil {
		t.Fatalf("ListGroups() error: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("len = %d, want 0", len(groups))
	}
}

func TestListGroupsError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	_, err := c.ListGroups()
	if err == nil {
		t.Fatal("ListGroups() should error on 500")
	}
}

func TestCreateGroup(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/groups" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"group": map[string]any{
					"id":      3,
					"name":    body["name"],
					"comment": body["comment"],
					"enabled": body["enabled"],
				},
			})
			return
		}
	})

	group, err := c.CreateGroup("my-group", "[forseti]", true)
	if err != nil {
		t.Fatalf("CreateGroup() error: %v", err)
	}
	if group.ID != 3 {
		t.Errorf("ID = %d, want 3", group.ID)
	}
	if group.Name != "my-group" {
		t.Errorf("Name = %q, want my-group", group.Name)
	}
}

func TestCreateGroupError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("duplicate"))
	})

	_, err := c.CreateGroup("dup", "", true)
	if err == nil {
		t.Fatal("CreateGroup() should error on 409")
	}
}

func TestDeleteGroups(t *testing.T) {
	var gotBody []any

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/groups:batchDelete" {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	})

	err := c.DeleteGroups([]string{"group1", "group2"})
	if err != nil {
		t.Fatalf("DeleteGroups() error: %v", err)
	}
	if len(gotBody) != 2 {
		t.Errorf("batch items = %d, want 2", len(gotBody))
	}
}

// Clients

func TestListClients(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/clients" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"clients": []map[string]any{
					{"id": 1, "client": "192.168.1.100", "comment": "[forseti]", "groups": []int{0}},
				},
			})
			return
		}
	})

	clients, err := c.ListClients()
	if err != nil {
		t.Fatalf("ListClients() error: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("len = %d, want 1", len(clients))
	}
	if clients[0].Client != "192.168.1.100" {
		t.Errorf("client = %q", clients[0].Client)
	}
}

func TestListClientsError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	_, err := c.ListClients()
	if err == nil {
		t.Fatal("ListClients() should error on 500")
	}
}

func TestCreateClient(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/clients" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"client": map[string]any{
					"id":      7,
					"client":  body["client"],
					"comment": body["comment"],
				},
			})
			return
		}
	})

	client, err := c.CreateClient("192.168.1.200", "[forseti]", []int{0, 2})
	if err != nil {
		t.Fatalf("CreateClient() error: %v", err)
	}
	if client.ID != 7 {
		t.Errorf("ID = %d, want 7", client.ID)
	}
}

func TestCreateClientError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad"))
	})

	_, err := c.CreateClient("bad", "", nil)
	if err == nil {
		t.Fatal("CreateClient() should error on 400")
	}
}

func TestDeleteClients(t *testing.T) {
	var gotBody []any

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/clients:batchDelete" {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	})

	err := c.DeleteClients([]string{"192.168.1.100", "192.168.1.200"})
	if err != nil {
		t.Fatalf("DeleteClients() error: %v", err)
	}
	if len(gotBody) != 2 {
		t.Errorf("batch items = %d, want 2", len(gotBody))
	}
}

// DNS Records

func TestListDNSRecords(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/config/dns/hosts" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"config": map[string]any{
					"dns": map[string]any{
						"hosts": []string{
							"192.168.1.1 router.local",
							"192.168.1.2 nas.local",
						},
					},
				},
			})
			return
		}
	})

	records, err := c.ListDNSRecords()
	if err != nil {
		t.Fatalf("ListDNSRecords() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len = %d, want 2", len(records))
	}
	if records[0].IP != "192.168.1.1" || records[0].Domain != "router.local" {
		t.Errorf("record[0] = %+v", records[0])
	}
}

func TestListDNSRecordsMalformed(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/config/dns/hosts" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"config": map[string]any{
					"dns": map[string]any{
						"hosts": []string{
							"192.168.1.1 router.local",
							"malformed-no-space",
						},
					},
				},
			})
			return
		}
	})

	records, err := c.ListDNSRecords()
	if err != nil {
		t.Fatalf("ListDNSRecords() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len = %d, want 1 (malformed entry should be skipped)", len(records))
	}
}

func TestListDNSRecordsError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	_, err := c.ListDNSRecords()
	if err == nil {
		t.Fatal("ListDNSRecords() should error on 500")
	}
}

func TestAddDNSRecord(t *testing.T) {
	var gotPath string
	var gotMethod string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusCreated)
	})

	err := c.AddDNSRecord("192.168.1.1", "router.local")
	if err != nil {
		t.Fatalf("AddDNSRecord() error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if !strings.Contains(gotPath, "/api/config/dns/hosts/") {
		t.Errorf("path = %q, should contain /api/config/dns/hosts/", gotPath)
	}
}

func TestDeleteDNSRecord(t *testing.T) {
	var gotMethod string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	err := c.DeleteDNSRecord("192.168.1.1", "router.local")
	if err != nil {
		t.Fatalf("DeleteDNSRecord() error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
}

// CNAME Records

func TestListCNAMERecords(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/config/dns/cnameRecords" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"config": map[string]any{
					"dns": map[string]any{
						"cnameRecords": []string{
							"app.local,server.local",
							"web.local,server.local",
						},
					},
				},
			})
			return
		}
	})

	records, err := c.ListCNAMERecords()
	if err != nil {
		t.Fatalf("ListCNAMERecords() error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len = %d, want 2", len(records))
	}
	if records[0].Domain != "app.local" || records[0].Target != "server.local" {
		t.Errorf("record[0] = %+v", records[0])
	}
}

func TestListCNAMERecordsMalformed(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/config/dns/cnameRecords" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"config": map[string]any{
					"dns": map[string]any{
						"cnameRecords": []string{
							"app.local,server.local",
							"malformed-no-comma",
						},
					},
				},
			})
			return
		}
	})

	records, err := c.ListCNAMERecords()
	if err != nil {
		t.Fatalf("ListCNAMERecords() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len = %d, want 1", len(records))
	}
}

func TestListCNAMERecordsError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	_, err := c.ListCNAMERecords()
	if err == nil {
		t.Fatal("ListCNAMERecords() should error on 500")
	}
}

func TestAddCNAMERecord(t *testing.T) {
	var gotMethod string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusCreated)
	})

	err := c.AddCNAMERecord("app.local", "server.local")
	if err != nil {
		t.Fatalf("AddCNAMERecord() error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
}

func TestDeleteCNAMERecord(t *testing.T) {
	var gotMethod string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	err := c.DeleteCNAMERecord("app.local", "server.local")
	if err != nil {
		t.Fatalf("DeleteCNAMERecord() error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
}

// CreateAdlist edge case: empty response list

func TestCreateAdlistEmptyResponse(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/lists") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lists": []any{},
			})
			return
		}
	})

	list, err := c.CreateAdlist("https://example.com/list.txt", "[forseti]", true, nil)
	if err != nil {
		t.Fatalf("CreateAdlist() error: %v", err)
	}
	if list.Address != "https://example.com/list.txt" {
		t.Errorf("Address = %q, want fallback address", list.Address)
	}
}

// Error response tests for Adlists

func TestListAdlistsError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	_, err := c.ListAdlists()
	if err == nil {
		t.Fatal("ListAdlists() should error on 500")
	}
}

func TestCreateAdlistError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad"))
	})

	_, err := c.CreateAdlist("bad", "", true, nil)
	if err == nil {
		t.Fatal("CreateAdlist() should error on 400")
	}
}

func TestDeleteAdlistsError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	err := c.DeleteAdlists([]string{"x"})
	if err == nil {
		t.Fatal("DeleteAdlists() should error on 500")
	}
}

// GetStats error

func TestGetStatsError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	_, err := c.GetStats()
	if err == nil {
		t.Fatal("GetStats() should error on 500")
	}
}

// GetUpstreams error

func TestGetUpstreamsError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	_, err := c.GetUpstreams()
	if err == nil {
		t.Fatal("GetUpstreams() should error on 500")
	}
}

// GetBlockingStatus error

func TestGetBlockingStatusError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	_, err := c.GetBlockingStatus()
	if err == nil {
		t.Fatal("GetBlockingStatus() should error on 500")
	}
}

func TestWithoutDefault(t *testing.T) {
	result := withoutDefault([]int{0, 1, 2, 0, 3})
	if len(result) != 3 {
		t.Fatalf("len = %d, want 3", len(result))
	}
	for _, v := range result {
		if v == 0 {
			t.Error("result should not contain 0")
		}
	}
}

func TestWithoutDefaultAllZeros(t *testing.T) {
	result := withoutDefault([]int{0, 0})
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

func TestWithoutDefaultEmpty(t *testing.T) {
	result := withoutDefault(nil)
	if len(result) != 0 {
		t.Errorf("len = %d, want 0", len(result))
	}
}

// doJSONOnce edge cases

func TestDoJSONOnceInvalidJSON(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	})

	var result map[string]any
	err := c.doJSON(http.MethodGet, "/api/test", nil, &result)
	if err == nil {
		t.Fatal("should error on invalid JSON response")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("error = %q, should contain 'decode response'", err.Error())
	}
}

func TestDoJSONOnceNon2xxNon401(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	})

	err := c.doJSON(http.MethodGet, "/api/test", nil, nil)
	if err == nil {
		t.Fatal("should error on 403")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
	if apiErr.Message != "forbidden" {
		t.Errorf("Message = %q, want forbidden", apiErr.Message)
	}
}

func TestDoJSONNilResponseTarget(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"key":"value"}`))
	})

	err := c.doJSON(http.MethodGet, "/api/test", nil, nil)
	if err != nil {
		t.Fatalf("should not error when respTarget is nil: %v", err)
	}
}

func TestDoJSONEmptyResponseBody(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	var result map[string]any
	err := c.doJSON(http.MethodGet, "/api/test", nil, &result)
	if err != nil {
		t.Fatalf("should not error on empty body: %v", err)
	}
}

// doRequest: request with body sets Content-Type

func TestDoRequestContentType(t *testing.T) {
	var gotContentType string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	})

	_, _ = c.doRequest(http.MethodPost, "/api/test", strings.NewReader(`{"a":1}`))
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
}

func TestDoRequestNoContentTypeWithoutBody(t *testing.T) {
	var gotContentType string

	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	})

	_, _ = c.doRequest(http.MethodGet, "/api/test", nil)
	if gotContentType != "" {
		t.Errorf("Content-Type = %q, want empty", gotContentType)
	}
}

func TestDoRequestNoSIDWithoutSession(t *testing.T) {
	var gotSID string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSID = r.Header.Get("X-FTL-SID")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "pw")
	_, _ = c.doRequest(http.MethodGet, "/api/test", nil)
	if gotSID != "" {
		t.Errorf("SID = %q, want empty when no session", gotSID)
	}
}

// doRequest: network error

func TestDoRequestNetworkError(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", "pw")
	_, err := c.doRequest(http.MethodGet, "/api/test", nil)
	if err == nil {
		t.Fatal("should error on unreachable server")
	}
}

// doJSON: network error propagation

func TestDoJSONNetworkError(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", "pw")
	err := c.doJSON(http.MethodGet, "/api/test", nil, nil)
	if err == nil {
		t.Fatal("should error on unreachable server")
	}
}

func TestDeleteAdlistsFallbackToIndividual(t *testing.T) {
	individualDeletes := 0
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/lists:batchDelete" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"key":"not_found","message":"Not found"}}`))
			return
		}
		if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/lists/") {
			individualDeletes++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	err := c.DeleteAdlists([]string{"http://list1.txt", "http://list2.txt"})
	if err != nil {
		t.Fatalf("DeleteAdlists should succeed with fallback: %v", err)
	}
	if individualDeletes != 2 {
		t.Errorf("expected 2 individual deletes, got %d", individualDeletes)
	}
}

// TriggerGravity error

func TestTriggerGravityError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	err := c.TriggerGravity()
	if err == nil {
		t.Fatal("TriggerGravity() should error on 500")
	}
}

// Stats unmarshal error

func TestStatsUnmarshalInvalidJSON(t *testing.T) {
	var s Stats
	err := json.Unmarshal([]byte(`{invalid`), &s)
	if err == nil {
		t.Fatal("should error on invalid JSON")
	}
}

// doJSONOnce: read body error is hard to trigger with httptest,
// but we can test marshal error with an unmarshalable body

type unmarshallable struct{}

func (u unmarshallable) MarshalJSON() ([]byte, error) {
	return nil, io.ErrUnexpectedEOF
}

func TestDoJSONMarshalError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	err := c.doJSON(http.MethodPost, "/api/test", unmarshallable{}, nil)
	if err == nil {
		t.Fatal("should error on marshal failure")
	}
	if !strings.Contains(err.Error(), "marshal request") {
		t.Errorf("error = %q, should contain 'marshal request'", err.Error())
	}
}

// APIError.Error()

func TestAPIErrorString(t *testing.T) {
	e := &APIError{StatusCode: 404, Message: "not found"}
	got := e.Error()
	if got != "pihole api: 404 not found" {
		t.Errorf("Error() = %q", got)
	}
}

// GetStats with malformed JSON response (covers UnmarshalJSON error path)

func TestGetStatsMalformedJSON(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	})

	_, err := c.GetStats()
	if err == nil {
		t.Fatal("GetStats() should error on malformed JSON")
	}
}

func TestStatsUnmarshalJSONError(t *testing.T) {
	var s Stats
	err := s.UnmarshalJSON([]byte(`{invalid json`))
	if err == nil {
		t.Fatal("UnmarshalJSON should error on invalid JSON")
	}
}

// doJSONOnce: ReadAll error via a response body that errors on read

func TestDoJSONOnceReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/auth" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "s"},
			})
			return
		}
		// Write headers then hijack to produce a truncated body
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "pw")
	_ = c.Login()

	var result map[string]any
	err := c.doJSON(http.MethodGet, "/api/test", nil, &result)
	if err == nil {
		t.Fatal("should error when body read fails")
	}
}

// doJSON: error from doJSONOnce after successful reauth

func TestDoJSONErrorAfterReauth(t *testing.T) {
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/auth" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "new-sid"},
			})
			return
		}
		if r.Method == http.MethodDelete && r.URL.Path == "/api/auth" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// After reauth, server is down - simulate network error by closing
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "pw")
	c.sid = "expired"

	err := c.doJSON(http.MethodGet, "/api/test", nil, nil)
	if err == nil {
		t.Fatal("should error when second doJSONOnce fails after reauth")
	}
}

// doJSON: 401 after successful reauth

func TestDoJSON401AfterReauth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/auth" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"sid": "new-sid"},
			})
			return
		}
		if r.Method == http.MethodDelete && r.URL.Path == "/api/auth" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "pw")
	c.sid = "expired"

	err := c.doJSON(http.MethodGet, "/api/test", nil, nil)
	if err == nil {
		t.Fatal("should error on 401 after re-auth")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
	if apiErr.Message != "unauthorized after re-auth" {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

// doRequest with invalid URL (triggers NewRequest error)

func TestDoRequestInvalidURL(t *testing.T) {
	c := &Client{baseURL: "://invalid"}
	_, err := c.doRequest(http.MethodGet, "/api/test", nil)
	if err == nil {
		t.Fatal("should error on invalid URL")
	}
}

// Close with invalid URL (triggers NewRequest error)

func TestCloseInvalidURL(t *testing.T) {
	c := &Client{baseURL: "://invalid", sid: "s", httpClient: http.DefaultClient}
	err := c.Close()
	if err == nil {
		t.Fatal("should error on invalid URL in Close")
	}
}

// DHCP Leases

func TestGetDHCPLeases(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/dhcp/leases" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"leases": []map[string]any{
					{"ip": "192.168.1.100", "name": "laptop", "hwaddr": "aa:bb:cc:dd:ee:ff", "expires": 1234567890},
					{"ip": "192.168.1.101", "name": "phone", "hwaddr": "11:22:33:44:55:66", "expires": 1234567900},
				},
			})
			return
		}
	})

	leases, err := c.GetDHCPLeases()
	if err != nil {
		t.Fatalf("GetDHCPLeases() error: %v", err)
	}
	if len(leases) != 2 {
		t.Fatalf("len = %d, want 2", len(leases))
	}
	if leases[0].IP != "192.168.1.100" {
		t.Errorf("IP = %q", leases[0].IP)
	}
	if leases[0].Hostname != "laptop" {
		t.Errorf("Hostname = %q", leases[0].Hostname)
	}
}

func TestGetDHCPLeasesError(t *testing.T) {
	c := authedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})

	_, err := c.GetDHCPLeases()
	if err == nil {
		t.Fatal("GetDHCPLeases() should error on 500")
	}
}
