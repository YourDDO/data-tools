package compendium

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientTransportErrorDoesNotExposeRequestURL(t *testing.T) {
	t.Parallel()
	const sensitiveURL = "https://private.example/api.php?titles=Secret_Page"
	client, err := NewClient("https://private.example/api.php", WithRetries(0), WithHTTPClient(doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: sensitiveURL, Err: errors.New("connection refused")}
	})))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.FetchPageContent(context.Background(), "Secret Page")
	if err == nil || strings.Contains(err.Error(), sensitiveURL) || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("error = %v", err)
	}
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestFetchCategoryContentPaginatesWithoutDelayOrAuthentication(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	var sleepCalls atomic.Int32
	doer := doerFunc(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization header = %q, want empty", got)
		}
		if got := r.URL.Query().Get("gcmtitle"); got != "Category:Private Items" {
			t.Errorf("gcmtitle = %q", got)
		}
		if got := r.URL.Query().Get("gcmnamespace"); got != "0" {
			t.Errorf("gcmnamespace = %q, want 0", got)
		}
		if r.URL.Query().Get("gcmcontinue") == "" {
			return response(http.StatusOK, `{"continue":{"gcmcontinue":"next"},"query":{"pages":[{"title":"One","revisions":[{"slots":{"main":{"content":"first"}}}]}]}}`), nil
		}
		return response(http.StatusOK, `{"query":{"pages":[{"title":"Two","revisions":[{"slots":{"main":{"content":"second"}}}]}]}}`), nil
	})

	client, err := NewClient("http://private.example/api.php", WithHTTPClient(doer), WithSleeper(func(context.Context, time.Duration) error {
		sleepCalls.Add(1)
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	contents, err := client.FetchCategoryContent(context.Background(), "Private Items")
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 || contents["One"] != "first" || contents["Two"] != "second" {
		t.Fatalf("requests = %d, contents = %#v", requests.Load(), contents)
	}
	if sleepCalls.Load() != 0 {
		t.Fatalf("pagination invoked sleeper %d times", sleepCalls.Load())
	}
}

func TestClientRetriesServerFailure(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	doer := doerFunc(func(_ *http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			return response(http.StatusServiceUnavailable, "temporary"), nil
		}
		return response(http.StatusOK, `{"query":{"pages":[{"title":"Page","revisions":[{"slots":{"main":{"content":"ok"}}}]}]}}`), nil
	})

	client, err := NewClient("http://private.example/api.php", WithHTTPClient(doer), WithRetries(1), WithSleeper(func(context.Context, time.Duration) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	content, err := client.FetchPageContent(context.Background(), "Page")
	if err != nil {
		t.Fatal(err)
	}
	if content != "ok" || requests.Load() != 2 {
		t.Fatalf("content = %q, requests = %d", content, requests.Load())
	}
}

func TestFetchCategoryContentRejectsMissingRecordContent(t *testing.T) {
	t.Parallel()
	doer := doerFunc(func(_ *http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"query":{"pages":[{"title":"Broken Page","revisions":[]}]}}`), nil
	})
	client, err := NewClient("http://private.example/api.php", WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.FetchCategoryContent(context.Background(), "Items")
	if err == nil || !strings.Contains(err.Error(), `source record "Broken Page"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestFetchCategoryContentRejectsDuplicateSourceIdentifier(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	doer := doerFunc(func(_ *http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			return response(http.StatusOK, `{"continue":{"gcmcontinue":"next"},"query":{"pages":[{"title":"Same Page","revisions":[{"slots":{"main":{"content":"first"}}}]}]}}`), nil
		}
		return response(http.StatusOK, `{"query":{"pages":[{"title":"Same Page","revisions":[{"slots":{"main":{"content":"second"}}}]}]}}`), nil
	})
	client, err := NewClient("http://private.example/api.php", WithHTTPClient(doer), WithSleeper(func(context.Context, time.Duration) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.FetchCategoryContent(context.Background(), "Items")
	if err == nil || !strings.Contains(err.Error(), `duplicate source record identifier "Same Page"`) {
		t.Fatalf("error = %v", err)
	}
}
