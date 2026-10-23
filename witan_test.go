package witan

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func server(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	t.Setenv("WITAN_API_KEY", "")
	return New(WithBaseURL(ts.URL), WithRetries(2))
}

func TestSearchErrorSendsTheErrorInTheBody(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/search/error" || r.URL.RawQuery != "" {
			t.Errorf("got %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		var b map[string]any
		json.NewDecoder(r.Body).Decode(&b)
		if b["error"] != "npm error code ENEEDAUTH" || b["limit"] != float64(3) {
			t.Errorf("body %v", b)
		}
		if ua := r.Header.Get("user-agent"); ua != "witan-sdk-go/"+Version {
			t.Errorf("user-agent %q", ua)
		}
		w.Write([]byte(`{"signature":"npm error code ENEEDAUTH","mode":"error","results":[{"id":"u1","title":"T","score":"78.00","price":"$0","priceMicro":"0","match":{"kind":"exact","similarity":1}}]}`))
	})
	a, err := c.SearchError(context.Background(), "npm error code ENEEDAUTH", &ErrorSearchOptions{Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	h := a.Results[0]
	if h.ID != "u1" || h.Score.Value != 78 || !h.Score.Valid || h.Match.Kind != "exact" || h.Match.Similarity.Value != 1 || h.PriceMicro.Value != 0 {
		t.Errorf("hit %+v", h)
	}
}

func TestSearchQueryAndKey(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("q") != "redis throughput" || q.Get("mode") != "keyword" || q.Get("limit") != "5" {
			t.Errorf("query %v", q)
		}
		if got := r.Header.Get("authorization"); got != "Bearer km_test" {
			t.Errorf("authorization %q", got)
		}
		w.Write([]byte(`{"results":[{"id":"a","score":null},{"id":"b","score":73}],"mode":"keyword"}`))
	})
	WithAPIKey("km_test")(c)
	hits, err := c.Search(context.Background(), "redis throughput", &SearchOptions{Mode: "keyword", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].Score.Valid || hits[1].Score.Value != 73 {
		t.Errorf("hits %+v", hits)
	}
}

func TestReadPaymentRequired(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(`{"error":"payment required","price":"$0.01","pay":"https://witan.markets/paid/knowledge?id=x","freeWithKey":true}`))
	})
	_, err := c.Read(context.Background(), "x")
	var p *PaymentRequiredError
	if !errors.As(err, &p) || p.Price != "$0.01" || p.Pay == "" || !p.FreeWithKey {
		t.Fatalf("err %v", err)
	}
	var a *APIError
	if !errors.As(err, &a) || a.Status != 402 || !IsPaymentRequired(err) {
		t.Fatalf("as APIError: %v", err)
	}
}

func TestRetriesOn503ThenSucceeds(t *testing.T) {
	var n atomic.Int32
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 3 {
			w.Header().Set("retry-after", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"id":"u1","title":"T","body":"B"}`))
	})
	u, err := c.Read(context.Background(), "u1")
	if err != nil || u.Body != "B" || n.Load() != 3 {
		t.Fatalf("u=%+v err=%v calls=%d", u, err, n.Load())
	}
}

func TestGivesUpAfterRetries(t *testing.T) {
	var n atomic.Int32
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.Header().Set("retry-after", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"slow down"}`))
	})
	_, err := c.Datasets(context.Background())
	var a *APIError
	if !errors.As(err, &a) || a.Status != 429 || a.Message != "slow down" || n.Load() != 3 {
		t.Fatalf("err %v calls %d", err, n.Load())
	}
}

func TestNoRetryOn404(t *testing.T) {
	var n atomic.Int32
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		http.NotFound(w, r)
	})
	if _, err := c.Dataset(context.Background(), "nope"); err == nil || n.Load() != 1 {
		t.Fatalf("err %v calls %d", err, n.Load())
	}
}

func TestQueryAndDataPaths(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/ai%2Fincidents/query", "/projects/ai/incidents/query":
			t.Errorf("slug not escaped as one segment: %s", r.URL.Path)
		case "/projects/agent-api-observatory/query":
			var b map[string]any
			json.NewDecoder(r.Body).Decode(&b)
			if b["sql"] != "SELECT 1" || b["version"] != float64(79) {
				t.Errorf("body %v", b)
			}
			w.Write([]byte(`{"columns":["x"],"rows":[[1]],"count":1}`))
		case "/projects/agent-api-observatory/data":
			if r.URL.Query().Get("limit") != "2" {
				t.Errorf("query %v", r.URL.Query())
			}
			w.Write([]byte(`{"count":2,"next":2,"records":[{"a":1},{"a":2}]}`))
		default:
			t.Errorf("path %s", r.URL.Path)
		}
	})
	ctx := context.Background()
	q, err := c.Query(ctx, "agent-api-observatory", "SELECT 1", &QueryOptions{Version: 79})
	if err != nil || q.Count != 1 || q.Columns[0] != "x" {
		t.Fatalf("q=%+v err=%v", q, err)
	}
	d, err := c.DatasetData(ctx, "agent-api-observatory", &DataOptions{Limit: 2})
	if err != nil || len(d.Records) != 2 || d.Next == nil || *d.Next != 2 {
		t.Fatalf("d=%+v err=%v", d, err)
	}
}

func TestEnvironmentDefaults(t *testing.T) {
	t.Setenv("WITAN_BASE_URL", "http://localhost:3000/")
	t.Setenv("WITAN_API_KEY", "km_env")
	c := New()
	if c.baseURL != "http://localhost:3000" || c.apiKey != "km_env" {
		t.Errorf("base %q key %q", c.baseURL, c.apiKey)
	}
}
