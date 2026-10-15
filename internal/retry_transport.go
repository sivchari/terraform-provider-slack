package internal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultMaxRetries = 3
	// Tier 1 methods such as apps.manifest.* legitimately ask for a full minute.
	defaultMaxWait    = 60 * time.Second
	defaultRetryAfter = time.Second
)

// retryTransport replays requests that Slack answers with HTTP 429, waiting
// for the duration of the Retry-After header between attempts.
//
// Only 429 responses are retried: Slack guides clients to Retry-After alone,
// and a transport error says nothing about whether a non-idempotent call
// already took effect.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
	maxWait    time.Duration
	sleep      func(ctx context.Context, d time.Duration) error
}

// newRetryTransport returns a retryTransport over base, or over
// http.DefaultTransport when base is nil.
func newRetryTransport(base http.RoundTripper) *retryTransport {
	if base == nil {
		base = http.DefaultTransport
	}

	return &retryTransport{
		base:       base,
		maxRetries: defaultMaxRetries,
		maxWait:    defaultMaxWait,
		sleep:      sleepContext,
	}
}

// RoundTrip implements http.RoundTripper. Once retries are exhausted the last
// 429 response is returned unchanged so callers still see the rate limit.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req, err := replayable(req)
	if err != nil {
		return nil, err
	}

	for attempt := 0; ; attempt++ {
		attemptReq := req
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("rewind request body: %w", err)
			}
			attemptReq = req.Clone(req.Context())
			attemptReq.Body = body
		}

		resp, err := t.base.RoundTrip(attemptReq)
		if err != nil || resp.StatusCode != http.StatusTooManyRequests || attempt >= t.maxRetries {
			return resp, err
		}

		wait := t.retryAfter(resp)
		// Draining lets the connection be reused for the retry.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if err := t.sleep(req.Context(), wait); err != nil {
			return nil, err
		}
	}
}

// retryAfter reads the Retry-After header (integer seconds), falling back to
// defaultRetryAfter when it is missing or invalid, and caps it at maxWait.
func (t *retryTransport) retryAfter(resp *http.Response) time.Duration {
	wait := defaultRetryAfter
	if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds >= 0 {
		wait = time.Duration(seconds) * time.Second
	}

	return min(wait, t.maxWait)
}

// replayable returns req itself when its body can be re-created, and
// otherwise a copy whose body is buffered in memory. Slack API bodies are
// small form posts, so buffering is cheap.
func replayable(req *http.Request) (*http.Request, error) {
	if req.Body == nil || req.Body == http.NoBody || req.GetBody != nil {
		return req, nil
	}

	buf, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("buffer request body: %w", err)
	}

	clone := req.Clone(req.Context())
	clone.Body = io.NopCloser(bytes.NewReader(buf))
	clone.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf)), nil
	}

	return clone, nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
