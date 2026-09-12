package pihole

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	password   string
	httpClient *http.Client
	sid        string
	OnReauth   func()
}

func NewClient(baseURL, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("pihole api: %d %s", e.StatusCode, e.Message)
}

// Auth

type authResponse struct {
	Session struct {
		SID string `json:"sid"`
	} `json:"session"`
}

func (c *Client) Login() error {
	slog.Debug("authenticating", "url", c.baseURL)
	body := map[string]string{"password": c.password}
	var resp authResponse
	if err := c.doJSON(http.MethodPost, "/api/auth", body, &resp); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	c.sid = resp.Session.SID
	slog.Debug("authenticated", "url", c.baseURL)
	return nil
}

func (c *Client) HasSession() bool {
	return c.sid != ""
}

func (c *Client) Close() error {
	if c.sid == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/api/auth", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-FTL-SID", c.sid)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.sid = ""
		return err
	}
	resp.Body.Close()
	c.sid = ""
	return nil
}

// API types

type APIList struct {
	ID      int    `json:"id"`
	Address string `json:"address"`
	Comment string `json:"comment"`
	Enabled bool   `json:"enabled"`
	Groups  []int  `json:"groups"`
}

type APIDomain struct {
	ID      int    `json:"id"`
	Domain  string `json:"domain"`
	Type    string `json:"type"`
	Kind    string `json:"kind"`
	Comment string `json:"comment"`
	Enabled bool   `json:"enabled"`
	Groups  []int  `json:"groups"`
}

type APIGroup struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Comment string `json:"comment"`
	Enabled bool   `json:"enabled"`
}

type APIClient struct {
	ID      int    `json:"id"`
	Client  string `json:"client"`
	Comment string `json:"comment"`
	Groups  []int  `json:"groups"`
}

type APIDNSRecord struct {
	IP     string
	Domain string
}

type APICNAMERecord struct {
	Domain string `json:"domain"`
	Target string `json:"target"`
}

type Stats struct {
	QueriesTotal      int
	BlockedTotal      int
	BlockedPercentage float64
	DomainsBlocked    int
	GravityLastUpdate int64
	Forwarded         int
	Cached            int
	UniqueDomains     int
	Frequency         float64
	ClientsActive     int
	ClientsTotal      int
	QueryTypes        map[string]int
	QueryStatus       map[string]int
	ReplyTypes        map[string]int
}

func (s *Stats) UnmarshalJSON(data []byte) error {
	var raw struct {
		Queries struct {
			Total          int                `json:"total"`
			Blocked        int                `json:"blocked"`
			PercentBlocked float64            `json:"percent_blocked"`
			Forwarded      int                `json:"forwarded"`
			Cached         int                `json:"cached"`
			UniqueDomains  int                `json:"unique_domains"`
			Frequency      float64            `json:"frequency"`
			Types          map[string]float64 `json:"types"`
			Status         map[string]float64 `json:"status"`
			Replies        map[string]float64 `json:"replies"`
		} `json:"queries"`
		Clients struct {
			Active int `json:"active"`
			Total  int `json:"total"`
		} `json:"clients"`
		Gravity struct {
			DomainsBeingBlocked int   `json:"domains_being_blocked"`
			LastUpdate          int64 `json:"last_update"`
		} `json:"gravity"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.QueriesTotal = raw.Queries.Total
	s.BlockedTotal = raw.Queries.Blocked
	s.BlockedPercentage = raw.Queries.PercentBlocked
	s.Forwarded = raw.Queries.Forwarded
	s.Cached = raw.Queries.Cached
	s.UniqueDomains = raw.Queries.UniqueDomains
	s.Frequency = raw.Queries.Frequency
	s.ClientsActive = raw.Clients.Active
	s.ClientsTotal = raw.Clients.Total
	s.DomainsBlocked = raw.Gravity.DomainsBeingBlocked
	s.GravityLastUpdate = raw.Gravity.LastUpdate

	s.QueryTypes = toIntMap(raw.Queries.Types)
	s.QueryStatus = toIntMap(raw.Queries.Status)
	s.ReplyTypes = toIntMap(raw.Queries.Replies)
	return nil
}

func toIntMap(m map[string]float64) map[string]int {
	if m == nil {
		return nil
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = int(v)
	}
	return out
}

type UpstreamStats struct {
	IP               string
	Name             string
	Port             int
	Count            int
	ResponseTime     float64
	ResponseVariance float64
}

// Adlists

func (c *Client) ListAdlists() ([]APIList, error) {
	var resp struct {
		Lists []APIList `json:"lists"`
	}
	if err := c.doJSON(http.MethodGet, "/api/lists?type=block", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Lists, nil
}

func (c *Client) CreateAdlist(address, comment string, enabled bool, groups []int) (*APIList, error) {
	body := map[string]any{
		"address": address,
		"comment": comment,
		"enabled": enabled,
		"groups":  withoutDefault(groups),
	}
	var resp struct {
		Lists []APIList `json:"lists"`
	}
	if err := c.doJSON(http.MethodPost, "/api/lists?type=block", body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Lists) > 0 {
		return &resp.Lists[0], nil
	}
	return &APIList{Address: address}, nil
}

func (c *Client) DeleteAdlists(addresses []string) error {
	items := make([]map[string]string, len(addresses))
	for i, addr := range addresses {
		items[i] = map[string]string{"item": addr}
	}
	return c.doJSON(http.MethodPost, "/api/lists:batchDelete?type=block", items, nil)
}

// Domains

func (c *Client) ListDomains(domType, kind string) ([]APIDomain, error) {
	var resp struct {
		Domains []APIDomain `json:"domains"`
	}
	path := fmt.Sprintf("/api/domains/%s/%s", domType, kind)
	if err := c.doJSON(http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Domains, nil
}

func (c *Client) CreateDomain(domType, kind, domain, comment string, enabled bool, groups []int) (*APIDomain, error) {
	body := map[string]any{
		"domain":  domain,
		"comment": comment,
		"enabled": enabled,
		"groups":  withoutDefault(groups),
	}
	path := fmt.Sprintf("/api/domains/%s/%s", domType, kind)
	var resp struct {
		Domain APIDomain `json:"domain"`
	}
	if err := c.doJSON(http.MethodPost, path, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Domain, nil
}

func (c *Client) DeleteDomains(domains []string) error {
	items := make([]map[string]string, len(domains))
	for i, d := range domains {
		items[i] = map[string]string{"item": d}
	}
	return c.doJSON(http.MethodPost, "/api/domains:batchDelete", items, nil)
}

// Groups

func (c *Client) ListGroups() ([]APIGroup, error) {
	var resp struct {
		Groups []APIGroup `json:"groups"`
	}
	if err := c.doJSON(http.MethodGet, "/api/groups", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Groups, nil
}

func (c *Client) CreateGroup(name, comment string, enabled bool) (*APIGroup, error) {
	body := map[string]any{
		"name":    name,
		"comment": comment,
		"enabled": enabled,
	}
	var resp struct {
		Group APIGroup `json:"group"`
	}
	if err := c.doJSON(http.MethodPost, "/api/groups", body, &resp); err != nil {
		return nil, err
	}
	return &resp.Group, nil
}

func (c *Client) DeleteGroups(names []string) error {
	items := make([]map[string]string, len(names))
	for i, n := range names {
		items[i] = map[string]string{"item": n}
	}
	return c.doJSON(http.MethodPost, "/api/groups:batchDelete", items, nil)
}

// Clients

func (c *Client) ListClients() ([]APIClient, error) {
	var resp struct {
		Clients []APIClient `json:"clients"`
	}
	if err := c.doJSON(http.MethodGet, "/api/clients", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Clients, nil
}

func (c *Client) CreateClient(ip, comment string, groups []int) (*APIClient, error) {
	body := map[string]any{
		"client":  ip,
		"comment": comment,
		"groups":  withoutDefault(groups),
	}
	var resp struct {
		Client APIClient `json:"client"`
	}
	if err := c.doJSON(http.MethodPost, "/api/clients", body, &resp); err != nil {
		return nil, err
	}
	return &resp.Client, nil
}

func (c *Client) DeleteClients(ips []string) error {
	items := make([]map[string]string, len(ips))
	for i, ip := range ips {
		items[i] = map[string]string{"item": ip}
	}
	return c.doJSON(http.MethodPost, "/api/clients:batchDelete", items, nil)
}

func (c *Client) UpdateAdlist(address string, comment string, groups []int) error {
	body := map[string]any{"comment": comment, "groups": groups}
	path := "/api/lists/" + url.PathEscape(address) + "?type=block"
	return c.doJSON(http.MethodPut, path, body, nil)
}

func (c *Client) UpdateClient(ip string, comment string, groups []int) error {
	body := map[string]any{"comment": comment, "groups": groups}
	path := "/api/clients/" + url.PathEscape(ip)
	return c.doJSON(http.MethodPut, path, body, nil)
}

func (c *Client) UpdateDomain(domain string, comment string, groups []int) error {
	body := map[string]any{"comment": comment, "groups": groups}
	path := "/api/domains/deny/exact/" + url.PathEscape(domain)
	return c.doJSON(http.MethodPut, path, body, nil)
}

// Local DNS

func (c *Client) ListDNSRecords() ([]APIDNSRecord, error) {
	var resp struct {
		Config struct {
			DNS struct {
				Hosts []string `json:"hosts"`
			} `json:"dns"`
		} `json:"config"`
	}
	if err := c.doJSON(http.MethodGet, "/api/config/dns/hosts", nil, &resp); err != nil {
		return nil, err
	}
	var records []APIDNSRecord
	for _, entry := range resp.Config.DNS.Hosts {
		parts := strings.SplitN(entry, " ", 2)
		if len(parts) == 2 {
			records = append(records, APIDNSRecord{IP: parts[0], Domain: parts[1]})
		}
	}
	return records, nil
}

func (c *Client) AddDNSRecord(ip, domain string) error {
	entry := url.PathEscape(ip + " " + domain)
	return c.doJSON(http.MethodPut, "/api/config/dns/hosts/"+entry, nil, nil)
}

func (c *Client) DeleteDNSRecord(ip, domain string) error {
	entry := url.PathEscape(ip + " " + domain)
	return c.doJSON(http.MethodDelete, "/api/config/dns/hosts/"+entry, nil, nil)
}

func (c *Client) ListCNAMERecords() ([]APICNAMERecord, error) {
	var resp struct {
		Config struct {
			DNS struct {
				CNAMERecords []string `json:"cnameRecords"`
			} `json:"dns"`
		} `json:"config"`
	}
	if err := c.doJSON(http.MethodGet, "/api/config/dns/cnameRecords", nil, &resp); err != nil {
		return nil, err
	}
	var records []APICNAMERecord
	for _, entry := range resp.Config.DNS.CNAMERecords {
		parts := strings.SplitN(entry, ",", 2)
		if len(parts) == 2 {
			records = append(records, APICNAMERecord{Domain: parts[0], Target: parts[1]})
		}
	}
	return records, nil
}

func (c *Client) AddCNAMERecord(domain, target string) error {
	entry := url.PathEscape(domain + "," + target)
	return c.doJSON(http.MethodPut, "/api/config/dns/cnameRecords/"+entry, nil, nil)
}

func (c *Client) DeleteCNAMERecord(domain, target string) error {
	entry := url.PathEscape(domain + "," + target)
	return c.doJSON(http.MethodDelete, "/api/config/dns/cnameRecords/"+entry, nil, nil)
}

// DHCP

type APIDHCPLease struct {
	IP       string `json:"ip"`
	Hostname string `json:"name"`
	MAC      string `json:"hwaddr"`
	Expires  int64  `json:"expires"`
}

func (c *Client) GetDHCPLeases() ([]APIDHCPLease, error) {
	var resp struct {
		Leases []APIDHCPLease `json:"leases"`
	}
	if err := c.doJSON(http.MethodGet, "/api/dhcp/leases", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Leases, nil
}

// Stats

func (c *Client) GetStats() (*Stats, error) {
	var stats Stats
	if err := c.doJSON(http.MethodGet, "/api/stats/summary", nil, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

func (c *Client) GetUpstreams() ([]UpstreamStats, error) {
	var resp struct {
		Upstreams []struct {
			IP         string `json:"ip"`
			Name       string `json:"name"`
			Port       int    `json:"port"`
			Count      int    `json:"count"`
			Statistics struct {
				Response float64 `json:"response"`
				Variance float64 `json:"variance"`
			} `json:"statistics"`
		} `json:"upstreams"`
	}
	if err := c.doJSON(http.MethodGet, "/api/stats/upstreams", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]UpstreamStats, len(resp.Upstreams))
	for i, u := range resp.Upstreams {
		out[i] = UpstreamStats{
			IP:               u.IP,
			Name:             u.Name,
			Port:             u.Port,
			Count:            u.Count,
			ResponseTime:     u.Statistics.Response,
			ResponseVariance: u.Statistics.Variance,
		}
	}
	return out, nil
}

func (c *Client) GetBlockingStatus() (bool, error) {
	var resp struct {
		Blocking string `json:"blocking"`
	}
	if err := c.doJSON(http.MethodGet, "/api/dns/blocking", nil, &resp); err != nil {
		return false, err
	}
	return resp.Blocking == "enabled", nil
}

// Config

func (c *Client) GetConfig() (map[string]any, error) {
	var resp struct {
		Config map[string]any `json:"config"`
	}
	if err := c.doJSON(http.MethodGet, "/api/config", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Config, nil
}

func (c *Client) PatchConfig(path string, value any) error {
	parts := strings.Split(path, "/")
	body := value
	for i := len(parts) - 1; i >= 0; i-- {
		body = map[string]any{parts[i]: body}
	}
	return c.doJSON(http.MethodPatch, "/api/config", map[string]any{"config": body}, nil)
}

// Readiness

func (c *Client) WaitForReady(maxWait time.Duration, interval time.Duration) error {
	deadline := time.Now().Add(maxWait)
	for {
		req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/info", nil)
		if err != nil {
			return fmt.Errorf("wait ready: %w", err)
		}
		resp, err := c.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("wait ready: timeout after %s: %w", maxWait, err)
			}
			return fmt.Errorf("wait ready: timeout after %s: status %d", maxWait, resp.StatusCode)
		}
		time.Sleep(interval)
	}
}

// Actions

func (c *Client) TriggerGravity() error {
	return c.doJSON(http.MethodPost, "/api/action/gravity", nil, nil)
}

// Internal helpers

func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if c.sid != "" {
		req.Header.Set("X-FTL-SID", c.sid)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *Client) doJSON(method, path string, reqBody any, respTarget any) error {
	slog.Debug("api request", "method", method, "path", path)
	resp, err := c.doJSONOnce(method, path, reqBody, respTarget)
	if err != nil {
		return err
	}

	if resp != http.StatusUnauthorized {
		return nil
	}

	if c.password == "" || path == "/api/auth" {
		return &APIError{StatusCode: http.StatusUnauthorized, Message: "unauthorized"}
	}

	slog.Debug("session expired, re-authenticating", "url", c.baseURL)
	if loginErr := c.Login(); loginErr != nil {
		return fmt.Errorf("re-auth failed: %w", loginErr)
	}
	if c.OnReauth != nil {
		c.OnReauth()
	}
	resp, err = c.doJSONOnce(method, path, reqBody, respTarget)
	if err != nil {
		return err
	}
	if resp == http.StatusUnauthorized {
		return &APIError{StatusCode: resp, Message: "unauthorized after re-auth"}
	}

	return nil
}

func (c *Client) doJSONOnce(method, path string, reqBody any, respTarget any) (statusCode int, _ error) {
	var bodyReader io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return 0, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	resp, err := c.doRequest(method, path, bodyReader)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return http.StatusUnauthorized, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respData),
		}
	}

	if respTarget != nil && len(respData) > 0 {
		if err := json.Unmarshal(respData, respTarget); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response: %w", err)
		}
	}

	return resp.StatusCode, nil
}

// Pi-hole auto-assigns new entries to group 0 (Default) on creation.
// Including 0 in a POST causes a UNIQUE constraint violation.
// PUT (update) replaces the full group list, so 0 must be included there.
func withoutDefault(groups []int) []int {
	out := make([]int, 0, len(groups))
	for _, g := range groups {
		if g != 0 {
			out = append(out, g)
		}
	}
	return out
}
