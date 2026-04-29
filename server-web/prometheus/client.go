package promclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Host struct {
	Instance   string  `json:"instance"`
	CPU        float64 `json:"cpu"`
	Memory     float64 `json:"memory"`
	Status     string  `json:"status"`
	LastScrape string  `json:"lastScrape"`
}

type apiResponse struct {
	Status    string      `json:"status"`
	ErrorType string      `json:"errorType"`
	Error     string      `json:"error"`
	Data      queryResult `json:"data"`
}

type queryResult struct {
	ResultType string         `json:"resultType"`
	Result     []vectorResult `json:"result"`
}

type vectorResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

type metricValue struct {
	Instance  string
	Value     float64
	Timestamp time.Time
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Ready(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/-/ready", nil)
	if err != nil {
		return fmt.Errorf("build prometheus readiness request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("prometheus readiness check failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus readiness check returned status %d", response.StatusCode)
	}

	return nil
}

func (c *Client) GetHosts(ctx context.Context) ([]Host, error) {
	cpuValues, err := c.queryInstantVector(ctx, queryCPUUsage)
	if err != nil {
		return nil, err
	}

	memoryValues, err := c.queryInstantVector(ctx, queryMemoryUsage)
	if err != nil {
		return nil, err
	}

	hostsByInstance := map[string]*Host{}

	for _, item := range cpuValues {
		host := getOrCreateHost(hostsByInstance, item.Instance)
		host.CPU = item.Value
		host.Status = "healthy"
		host.LastScrape = item.Timestamp.UTC().Format(time.RFC3339)
	}

	for _, item := range memoryValues {
		host := getOrCreateHost(hostsByInstance, item.Instance)
		host.Memory = item.Value
		host.Status = "healthy"
		if host.LastScrape == "" || item.Timestamp.After(parseTime(host.LastScrape)) {
			host.LastScrape = item.Timestamp.UTC().Format(time.RFC3339)
		}
	}

	hosts := make([]Host, 0, len(hostsByInstance))
	for _, host := range hostsByInstance {
		if host.Status == "" {
			host.Status = "unknown"
		}
		hosts = append(hosts, *host)
	}

	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].Instance < hosts[j].Instance
	})

	return hosts, nil
}

func (c *Client) queryInstantVector(ctx context.Context, query string) ([]metricValue, error) {
	endpoint := c.baseURL + "/api/v1/query"
	values := url.Values{}
	values.Set("query", query)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build prometheus query request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("query prometheus failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query prometheus returned status %d", response.StatusCode)
	}

	var payload apiResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}

	if payload.Status != "success" {
		if payload.Error != "" {
			return nil, fmt.Errorf("prometheus query error: %s", payload.Error)
		}
		return nil, fmt.Errorf("prometheus query failed with status %s", payload.Status)
	}

	results := make([]metricValue, 0, len(payload.Data.Result))
	for _, item := range payload.Data.Result {
		metric, err := parseVectorResult(item)
		if err != nil {
			return nil, err
		}
		results = append(results, metric)
	}

	return results, nil
}

func parseVectorResult(item vectorResult) (metricValue, error) {
	instance := item.Metric["instance"]
	if instance == "" {
		instance = item.Metric["job"]
	}

	if len(item.Value) != 2 {
		return metricValue{}, fmt.Errorf("unexpected prometheus value format")
	}

	timestamp, err := parseTimestamp(item.Value[0])
	if err != nil {
		return metricValue{}, err
	}

	value, err := parseFloat(item.Value[1])
	if err != nil {
		return metricValue{}, err
	}

	return metricValue{
		Instance:  instance,
		Value:     value,
		Timestamp: timestamp,
	}, nil
}

func parseTimestamp(value interface{}) (time.Time, error) {
	floatValue, err := parseFloat(value)
	if err != nil {
		return time.Time{}, err
	}
	sec := int64(floatValue)
	nsec := int64((floatValue - float64(sec)) * 1e9)
	return time.Unix(sec, nsec), nil
}

func parseFloat(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0, fmt.Errorf("parse float value: %w", err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected value type %T", value)
	}
}

func getOrCreateHost(hosts map[string]*Host, instance string) *Host {
	host, ok := hosts[instance]
	if ok {
		return host
	}

	host = &Host{
		Instance: instance,
		Status:   "unknown",
	}
	hosts[instance] = host
	return host
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
