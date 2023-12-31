package lfsc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/superfly/ltx"
)

// DefaultURL is the default URL set by NewClient().
const DefaultURL = "https://litefs.fly.io"

// Client represents a client for connecting to LiteFS Cloud.
type Client struct {
	// Base URL of the remote LiteFS Cloud service.
	URL string

	// Authentication token. Required for most API calls.
	Token string
}

// NewClient returns a new instance of Client with the default URL.
func NewClient() *Client {
	return &Client{
		URL: DefaultURL,
	}
}

// ListClusters returns a list of clusters for the current organization.
// An org-scoped authentication token with read permissions is required.
func (c *Client) ListClusters(ctx context.Context, input *ListClustersInput) (*ListClustersOutput, error) {
	q := make(url.Values)
	if input != nil && input.Offset > 0 {
		q.Set("offset", strconv.Itoa(input.Offset))
	}
	if input != nil && input.Limit > 0 {
		q.Set("limit", strconv.Itoa(input.Limit))
	}

	var output ListClustersOutput
	if err := c.do(ctx, "GET", url.URL{Path: "/clusters", RawQuery: q.Encode()}, nil, &output); err != nil {
		return nil, err
	}
	return &output, nil
}

type ListClustersInput struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type ListClustersOutput struct {
	Offset   int        `json:"offset"`
	Limit    int        `json:"limit"`
	Total    int        `json:"total"`
	Clusters []*Cluster `json:"clusters"`
}

// CreateCluster creates a new cluster in a given region.
// An org-scoped authentication token with write permissions is required.
func (c *Client) CreateCluster(ctx context.Context, name, region string) (*Cluster, error) {
	input := createClusterInput{Cluster: name, Region: region}

	var output Cluster
	if err := c.do(ctx, "POST", url.URL{Path: "/clusters"}, &input, &output); err != nil {
		return nil, err
	}
	return &output, nil
}

type createClusterInput struct {
	Cluster string `json:"cluster"`
	Region  string `json:"region"`
}

// DeleteCluster permanently deletes an existing cluster.
// An org-scoped authentication token with write permissions is required.
func (c *Client) DeleteCluster(ctx context.Context, name string) error {
	q := make(url.Values)
	q.Set("cluster", name)
	return c.do(ctx, "DELETE", url.URL{Path: "/clusters", RawQuery: q.Encode()}, nil, nil)
}

// Pos returns a map of database names with their current replication position.
// A cluster-scoped authentication token with read permissions is required.
func (c *Client) Pos(ctx context.Context) (map[string]ltx.Pos, error) {
	output := make(map[string]ltx.Pos)
	if err := c.do(ctx, "GET", url.URL{Path: "/pos"}, nil, &output); err != nil {
		return nil, err
	}
	return output, nil
}

// HWM returns a map of database names with their current high-water mark.
// A cluster-scoped authentication token with read permissions is required.
func (c *Client) HWM(ctx context.Context) (map[string]ltx.TXID, error) {
	output := make(map[string]ltx.TXID)
	if err := c.do(ctx, "GET", url.URL{Path: "/hwm"}, nil, &output); err != nil {
		return nil, err
	}
	return output, nil
}

// RestoreDatabaseToTimestamp reverts the database to the state closest to the given timestamp.
// A cluster-scoped authentication token with write permissions is required.
func (c *Client) RestoreDatabaseToTimestamp(ctx context.Context, database string, timestamp time.Time) (ltx.TXID, error) {
	q := make(url.Values)
	q.Set("db", database)
	q.Set("timestamp", timestamp.Format(time.RFC3339))

	var output restoreDatabaseOutput
	if err := c.do(ctx, "POST", url.URL{Path: "/db/restore", RawQuery: q.Encode()}, nil, &output); err != nil {
		return 0, err
	}
	return output.TXID, nil
}

type restoreDatabaseOutput struct {
	TXID ltx.TXID `json:"txID"`
}

// ExportDatabase returns a reader that contains the current database state.
// A cluster-scoped authentication token with read permissions is required.
func (c *Client) ExportDatabase(ctx context.Context, database string) (io.ReadCloser, error) {
	q := make(url.Values)
	q.Set("db", database)
	q.Set("format", "sqlite")

	req, err := c.newRequest(ctx, "GET", url.URL{Path: "/db/snapshot", RawQuery: q.Encode()}, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if err := decodeResponseError(resp); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	return resp.Body, nil
}

// ImportDatabase creates a new database with the SQLite database contained in r.
// A cluster-scoped authentication token with write permissions is required.
func (c *Client) ImportDatabase(ctx context.Context, database string, r io.Reader) (ltx.TXID, error) {
	q := make(url.Values)
	q.Set("db", database)

	req, err := c.newRequest(ctx, "POST", url.URL{Path: "/db/upload", RawQuery: q.Encode()}, r)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}

	if err := decodeResponseError(resp); err != nil {
		_ = resp.Body.Close()
		return 0, err
	}

	var output importDatabaseOutput
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return 0, fmt.Errorf("cannot parse response body (%d): %w", resp.StatusCode, err)
	}
	return output.TXID, nil
}

type importDatabaseOutput struct {
	TXID ltx.TXID `json:"txID"`
}

// Regions returns a list of available LiteFS Cloud regions.
func (c *Client) Regions(ctx context.Context) ([]string, error) {
	req, err := c.newRequest(ctx, "GET", url.URL{Path: "/regions"}, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if err := decodeResponseError(resp); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}

	var output regionsOutput
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return nil, fmt.Errorf("cannot parse response body (%d): %w", resp.StatusCode, err)
	}
	return output.Regions, nil
}

type regionsOutput struct {
	Regions []string `json:"regions"`
}

func (c *Client) newRequest(ctx context.Context, method string, u url.URL, body io.Reader) (*http.Request, error) {
	if c.URL == "" {
		return nil, fmt.Errorf("lfsc.Client: URL required")
	}

	baseURL, err := url.Parse(c.URL)
	if err != nil {
		return nil, err
	}
	u.Scheme, u.Host = baseURL.Scheme, baseURL.Host

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	if c.Token != "" {
		req.Header.Set("Authorization", c.Token)
	}

	return req, nil
}

func (c *Client) do(ctx context.Context, method string, u url.URL, input, output any) error {
	var requestBody io.Reader
	if input != nil {
		buf, err := json.Marshal(input)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(buf)
	}

	req, err := c.newRequest(ctx, method, u, requestBody)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if err := decodeResponseError(resp); err != nil {
		return err
	}

	if output != nil {
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return fmt.Errorf("cannot parse response body (%d): %w", resp.StatusCode, err)
		}
	}
	return nil
}

func decodeResponseError(resp *http.Response) error {
	if isSuccessfulStatusCode(resp.StatusCode) {
		return nil
	}

	var errorResp errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
		return fmt.Errorf("internal error: code=%d", resp.StatusCode)
	}
	return errorResp.err()
}

type errorResponse struct {
	Code  string   `json:"code"`
	Error string   `json:"error"`
	Pos   *ltx.Pos `json:"pos,omitempty"`
}

func (r *errorResponse) err() error {
	switch r.Code {
	case "EPOSMISMATCH":
		var pos ltx.Pos
		if r.Pos != nil {
			pos = *r.Pos
		}
		return ltx.NewPosMismatchError(pos)

	default:
		return &Error{Code: r.Code, Message: r.Error}
	}
}

func isSuccessfulStatusCode(code int) bool {
	return code >= 200 && code < 300
}
