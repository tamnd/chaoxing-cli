// Package chaoxing is the library behind the cx command line:
// the HTTP client, pacing, auth-detection, and typed data models for
// Chaoxing (超星学习通), China's largest university e-learning platform.
//
// The public course search API at mooc1.chaoxing.com is accessible without
// login. Auth-gated endpoints redirect to passport.chaoxing.com; the client
// detects that and returns ErrBlocked, mapped to exit code 5 by the CLI.
package chaoxing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultUserAgent mimics a browser to avoid trivial bot filters.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Host is the primary Chaoxing hostname.
const Host = "chaoxing.com"

// BaseURL is the MOOC endpoint root.
const BaseURL = "https://mooc1.chaoxing.com"

// ErrBlocked is returned when auth is required or the site blocks the client.
// The CLI maps this to exit code 5 (KindRateLimited).
var ErrBlocked = errors.New("blocked: authentication required (exit 5)")

// ErrNotFound is returned when a course does not exist.
var ErrNotFound = errors.New("course not found")

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults for mooc1.chaoxing.com.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      300 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client is a rate-limited HTTP client for the Chaoxing public API.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: cfg.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Intercept login redirects so we can detect them.
				loc := req.URL.String()
				if strings.Contains(loc, "passport.chaoxing.com") ||
					(strings.Contains(loc, "login") && strings.Contains(loc, "chaoxing")) {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

// Get fetches rawURL and returns the body. Paces, retries, detects blocks.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	return c.do(ctx, rawURL)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		c.pace()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.cfg.UserAgent)
		req.Header.Set("Accept", "application/json,text/html;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		req.Header.Set("Referer", BaseURL+"/")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, rerr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if rerr != nil {
			lastErr = rerr
			continue
		}
		if isBlocked(resp, body) {
			return nil, ErrBlocked
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("http %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("http %d", resp.StatusCode)
		}
		return body, nil
	}
	return nil, fmt.Errorf("after %d attempts: %w", c.cfg.Retries, lastErr)
}

// pace blocks until at least Rate has elapsed since the last request.
func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

// isBlocked detects login redirects and auth errors from Chaoxing.
func isBlocked(resp *http.Response, body []byte) bool {
	if resp.StatusCode == http.StatusForbidden {
		return true
	}
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		loc := resp.Header.Get("Location")
		if strings.Contains(loc, "passport.chaoxing.com") ||
			(strings.Contains(loc, "login") && strings.Contains(loc, "chaoxing")) {
			return true
		}
	}
	s := string(body)
	if strings.Contains(s, `"result":false`) &&
		(strings.Contains(s, "未登录") || strings.Contains(s, "请登录") || strings.Contains(s, "登录")) {
		return true
	}
	return false
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
