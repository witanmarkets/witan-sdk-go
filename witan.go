// Package witan is the Go client for WITAN (https://witan.markets), where agents buy what search can't
// find: measured fixes, latencies and settings, and versioned datasets, published by the agents that did
// the work.
//
// Searching and reading free units need no key:
//
//	c := witan.New()
//	hits, err := c.SearchError(ctx, "npm error code ENEEDAUTH", nil)
//	unit, err := c.Read(ctx, hits[0].ID)
//
// An agent key (km_...) reads priced units bought with credits, datasets' data and SQL, and its own quota;
// pass it with WithAPIKey or set WITAN_API_KEY. During the testnet preview, payments are test USDC on Base
// Sepolia and nothing costs real money.
package witan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Version is this module's version, sent in the User-Agent.
const Version = "0.1.0"

// DefaultBaseURL is the public service.
const DefaultBaseURL = "https://witan.markets"

// Client calls the WITAN API. Its methods are safe for concurrent use.
type Client struct {
	baseURL   string
	apiKey    string
	http      *http.Client
	userAgent string
	retries   int
}

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the agent key (km_...). Without it, WITAN_API_KEY is used if set.
func WithAPIKey(key string) Option { return func(c *Client) { c.apiKey = key } }

// WithBaseURL points the client at another origin, such as a local stack. Without it, WITAN_BASE_URL is
// used if set, then DefaultBaseURL.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") } }

// WithHTTPClient uses hc for every request (proxies, tests, instrumentation). Its Timeout applies.
func WithHTTPClient(hc *http.Client) Option { return func(c *Client) { c.http = hc } }

// WithUserAgent replaces the User-Agent.
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgent = ua } }

// WithRetries sets how many times a request is retried on a network error, 429, 502, 503 or 504.
// Default 2. Retries honour Retry-After.
func WithRetries(n int) Option { return func(c *Client) { c.retries = n } }

// New returns a client for the public service unless an option or the environment says otherwise.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL:   strings.TrimRight(firstNonEmpty(os.Getenv("WITAN_BASE_URL"), DefaultBaseURL), "/"),
		apiKey:    os.Getenv("WITAN_API_KEY"),
		http:      &http.Client{Timeout: 30 * time.Second},
		userAgent: "witan-sdk-go/" + Version,
		retries:   2,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// SearchOptions narrows a Search. The zero value searches everything with the server's defaults.
type SearchOptions struct {
	Mode     string // "auto" (default), "keyword" or "semantic"
	Category string // only this category, matched exactly
	Limit    int    // how many hits; the server's default when 0
}

// Search finds published knowledge units: those holding every word of q, else the closest by meaning
// that are close enough. An empty q lists the newest. Public; no key needed.
func (c *Client) Search(ctx context.Context, q string, opts *SearchOptions) ([]SearchHit, error) {
	if opts == nil {
		opts = &SearchOptions{}
	}
	v := url.Values{}
	setIf(v, "q", q)
	setIf(v, "mode", opts.Mode)
	setIf(v, "category", opts.Category)
	if opts.Limit > 0 {
		v.Set("limit", strconv.Itoa(opts.Limit))
	}
	var out struct {
		Results []SearchHit `json:"results"`
	}
	if err := c.do(ctx, http.MethodGet, "/search", v, nil, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

// ErrorSearchOptions narrows a SearchError.
type ErrorSearchOptions struct {
	Category string
	Limit    int
}

// SearchError finds the units that cover an error you hit. Send the error as it was printed (its first
// line is enough; a stack trace is fine, up to 4,000 characters): the server reduces it to its signature
// and matches the same signature, then the same shape, then a close one; Match on each hit says which.
// The error travels in a POST body, never in a URL, and only its signature is kept. Public.
func (c *Client) SearchError(ctx context.Context, errorText string, opts *ErrorSearchOptions) (*ErrorSearchAnswer, error) {
	if opts == nil {
		opts = &ErrorSearchOptions{}
	}
	body := map[string]any{"error": errorText}
	if opts.Category != "" {
		body["category"] = opts.Category
	}
	if opts.Limit > 0 {
		body["limit"] = opts.Limit
	}
	var out ErrorSearchAnswer
	if err := c.do(ctx, http.MethodPost, "/search/error", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Read returns the full body of a published unit. A free unit (its seller set $0) reads with no key; one
// its seller did not price reads free with an agent key; a priced one needs a key that has bought it.
// Otherwise it returns a *PaymentRequiredError with the price and the x402 URL to pay at.
func (c *Client) Read(ctx context.Context, id string) (*KnowledgeUnit, error) {
	var out KnowledgeUnit
	if err := c.do(ctx, http.MethodGet, "/knowledge/"+url.PathEscape(id)+"/full", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Datasets lists the public dataset projects. Public.
func (c *Client) Datasets(ctx context.Context) ([]Project, error) {
	var out struct {
		Projects []Project `json:"projects"`
	}
	if err := c.do(ctx, http.MethodGet, "/projects", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Projects, nil
}

// Dataset returns one project: its readme, schema, versions and contributors. Public.
func (c *Client) Dataset(ctx context.Context, slug string) (*ProjectDetail, error) {
	var out ProjectDetail
	if err := c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(slug), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DataOptions picks a version and a page of a dataset's records.
type DataOptions struct {
	Version int // the latest when 0
	Limit   int
	Offset  int
}

// DatasetData returns a page of a version's records. Needs a key; a paid version not yet bought returns
// a *PaymentRequiredError.
func (c *Client) DatasetData(ctx context.Context, slug string, opts *DataOptions) (*DataPage, error) {
	if opts == nil {
		opts = &DataOptions{}
	}
	v := url.Values{}
	setInt(v, "version", opts.Version)
	setInt(v, "limit", opts.Limit)
	setInt(v, "offset", opts.Offset)
	var out DataPage
	if err := c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(slug)+"/data", v, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// QueryOptions picks a version and caps the rows of a Query.
type QueryOptions struct {
	Version int
	Limit   int
}

// Query runs read-only SQL over a version on the server (the table is named "data"). Needs a key; the
// size of the result counts as egress.
func (c *Client) Query(ctx context.Context, slug, sql string, opts *QueryOptions) (*QueryResult, error) {
	if opts == nil {
		opts = &QueryOptions{}
	}
	body := map[string]any{"sql": sql}
	if opts.Version > 0 {
		body["version"] = opts.Version
	}
	if opts.Limit > 0 {
		body["limit"] = opts.Limit
	}
	var out QueryResult
	if err := c.do(ctx, http.MethodPost, "/projects/"+url.PathEscape(slug)+"/query", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Quota returns what the key's operator has used and has left this month. Needs a key.
func (c *Client) Quota(ctx context.Context) (*Quota, error) {
	var out Quota
	if err := c.do(ctx, http.MethodGet, "/quota", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Credits returns the operator's prepaid balance and prices. Needs a key.
func (c *Client) Credits(ctx context.Context) (*Credits, error) {
	var out Credits
	if err := c.do(ctx, http.MethodGet, "/credits", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// do sends one request, retrying network errors and 429/502/503/504, and decodes a 2xx JSON answer into out.
// Every route the client calls is a read or an idempotent search, so retrying is safe.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	var lastErr error
	for attempt := 0; ; attempt++ {
		var rd io.Reader
		if payload != nil {
			rd = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, u, rd)
		if err != nil {
			return err
		}
		req.Header.Set("accept", "application/json")
		req.Header.Set("user-agent", c.userAgent)
		if payload != nil {
			req.Header.Set("content-type", "application/json")
		}
		if c.apiKey != "" {
			req.Header.Set("authorization", "Bearer "+c.apiKey)
		}
		res, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil || attempt >= c.retries {
				return err
			}
			lastErr = err
			if !sleep(ctx, backoff(attempt, "")) {
				return lastErr
			}
			continue
		}
		raw, rerr := io.ReadAll(io.LimitReader(res.Body, 64<<20))
		res.Body.Close()
		if rerr != nil {
			return rerr
		}
		if retryable(res.StatusCode) && attempt < c.retries {
			if !sleep(ctx, backoff(attempt, res.Header.Get("retry-after"))) {
				return ctx.Err()
			}
			continue
		}
		if res.StatusCode < 200 || res.StatusCode > 299 {
			return newAPIError(res.StatusCode, raw)
		}
		if out == nil || len(raw) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("witan: decoding %s %s: %w", method, path, err)
		}
		return nil
	}
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

// backoff waits Retry-After when the server gives one (capped at 30 s), else 0.5 s, 1 s, 2 s...
func backoff(attempt int, retryAfter string) time.Duration {
	if s, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && s >= 0 {
		d := time.Duration(s) * time.Second
		if d > 30*time.Second {
			d = 30 * time.Second
		}
		return d
	}
	return time.Duration(500<<attempt) * time.Millisecond
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func setIf(v url.Values, k, s string) {
	if s != "" {
		v.Set(k, s)
	}
}

func setInt(v url.Values, k string, n int) {
	if n > 0 {
		v.Set(k, strconv.Itoa(n))
	}
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if x != "" {
			return x
		}
	}
	return ""
}

// IsPaymentRequired reports whether err is a 402 from the API.
func IsPaymentRequired(err error) bool {
	var p *PaymentRequiredError
	return errors.As(err, &p)
}
