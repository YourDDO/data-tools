package compendium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultUserAgent = "YourDDO-DataPipeline/2.0"

// Source is the Compendium boundary consumed by the pipeline.
type Source interface {
	FetchCategoryContent(context.Context, string) (map[string]string, error)
	FetchPageContent(context.Context, string) (string, error)
}

// HTTPDoer permits tests and production wiring to supply their own transport.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type SleepFunc func(context.Context, time.Duration) error

type Client struct {
	endpoint  *url.URL
	http      HTTPDoer
	sleep     SleepFunc
	userAgent string
	retries   int
}

type Option func(*Client)

func WithHTTPClient(client HTTPDoer) Option {
	return func(c *Client) { c.http = client }
}

func WithSleeper(sleep SleepFunc) Option {
	return func(c *Client) { c.sleep = sleep }
}

func WithRetries(retries int) Option {
	return func(c *Client) { c.retries = retries }
}

func NewClient(apiURL string, options ...Option) (*Client, error) {
	endpoint, err := url.Parse(strings.TrimSpace(apiURL))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, fmt.Errorf("invalid Compendium API URL %q", apiURL)
	}
	c := &Client{
		endpoint: endpoint,
		http:     &http.Client{Timeout: 30 * time.Second},
		sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
		userAgent: defaultUserAgent,
		retries:   4,
	}
	for _, option := range options {
		option(c)
	}
	if c.http == nil || c.sleep == nil || c.retries < 0 {
		return nil, errors.New("invalid Compendium client dependency")
	}
	return c, nil
}

func (c *Client) FetchPageContent(ctx context.Context, pageTitle string) (string, error) {
	values := url.Values{
		"action":        {"query"},
		"format":        {"json"},
		"formatversion": {"2"},
		"prop":          {"revisions"},
		"redirects":     {"1"},
		"rvprop":        {"content"},
		"rvslots":       {"main"},
		"titles":        {pageTitle},
	}
	response, err := c.query(ctx, values)
	if err != nil {
		return "", fmt.Errorf("fetch page %q: %w", pageTitle, err)
	}
	for _, page := range response.Query.Pages {
		if content, ok := page.content(); ok {
			return content, nil
		}
	}
	return "", fmt.Errorf("fetch page %q: no revision content", pageTitle)
}

func (c *Client) FetchCategoryContent(ctx context.Context, categoryName string) (map[string]string, error) {
	contents := make(map[string]string)
	continuation := ""
	for {
		values := url.Values{
			"action":        {"query"},
			"format":        {"json"},
			"formatversion": {"2"},
			"gcmlimit":      {"50"},
			"gcmnamespace":  {"0"},
			"gcmtitle":      {"Category:" + categoryName},
			"generator":     {"categorymembers"},
			"prop":          {"revisions"},
			"redirects":     {"1"},
			"rvprop":        {"content"},
			"rvslots":       {"main"},
		}
		if continuation != "" {
			values.Set("gcmcontinue", continuation)
		}
		response, err := c.query(ctx, values)
		if err != nil {
			return nil, fmt.Errorf("fetch category %q: %w", categoryName, err)
		}
		for _, page := range response.Query.Pages {
			if strings.TrimSpace(page.Title) == "" {
				return nil, fmt.Errorf("fetch category %q: source record has an empty title", categoryName)
			}
			content, ok := page.content()
			if !ok {
				return nil, fmt.Errorf("fetch category %q source record %q: no revision content", categoryName, page.Title)
			}
			if _, exists := contents[page.Title]; exists {
				return nil, fmt.Errorf("fetch category %q: duplicate source record identifier %q", categoryName, page.Title)
			}
			contents[page.Title] = content
		}
		continuation = response.Continue["gcmcontinue"]
		if continuation == "" {
			return contents, nil
		}
		if err := c.sleep(ctx, 495*time.Millisecond); err != nil {
			return nil, fmt.Errorf("fetch category %q: pagination delay: %w", categoryName, err)
		}
	}
}

func (c *Client) query(ctx context.Context, values url.Values) (apiResponse, error) {
	requestURL := *c.endpoint
	requestURL.RawQuery = values.Encode()
	var lastStatus int
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			if err := c.sleep(ctx, 5*time.Second*time.Duration(1<<uint(attempt-1))); err != nil {
				return apiResponse{}, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
		if err != nil {
			return apiResponse{}, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			var requestErr *url.Error
			if errors.As(err, &requestErr) {
				err = requestErr.Err
			}
			return apiResponse{}, fmt.Errorf("Compendium API request: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return apiResponse{}, fmt.Errorf("read response: %w", readErr)
		}
		lastStatus = resp.StatusCode
		if resp.StatusCode == http.StatusOK {
			var decoded apiResponse
			if err := json.Unmarshal(body, &decoded); err != nil {
				return apiResponse{}, fmt.Errorf("decode response: %w", err)
			}
			return decoded, nil
		}
		if !retryable(resp.StatusCode, body) {
			return apiResponse{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(body, 512))
		}
	}
	return apiResponse{}, fmt.Errorf("HTTP %d after %d attempts", lastStatus, c.retries+1)
}

func retryable(status int, body []byte) bool {
	if status == http.StatusTooManyRequests || status >= 500 {
		return true
	}
	lower := strings.ToLower(string(body))
	return (status == http.StatusForbidden || status == http.StatusServiceUnavailable) &&
		(strings.Contains(lower, "cloudflare") || strings.Contains(lower, "challenge"))
}

func truncate(body []byte, limit int) string {
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit]) + "... (" + strconv.Itoa(len(body)) + " bytes)"
}

type apiResponse struct {
	Continue map[string]string `json:"continue"`
	Query    struct {
		Pages []page `json:"pages"`
	} `json:"query"`
}

type page struct {
	Title     string `json:"title"`
	Revisions []struct {
		Slots *struct {
			Main *struct {
				Content *string `json:"content"`
			} `json:"main"`
		} `json:"slots"`
	} `json:"revisions"`
}

func (p page) content() (string, bool) {
	if len(p.Revisions) == 0 || p.Revisions[0].Slots == nil || p.Revisions[0].Slots.Main == nil || p.Revisions[0].Slots.Main.Content == nil {
		return "", false
	}
	return *p.Revisions[0].Slots.Main.Content, true
}
