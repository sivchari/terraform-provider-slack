package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	defaultMaxRetries = 3
	// Tier 1 methods such as apps.manifest.* legitimately ask for a full minute.
	defaultMaxWait    = 60 * time.Second
	defaultRetryAfter = time.Second
	// minRetryAfter keeps "Retry-After: 0" from producing back-to-back retries.
	minRetryAfter = time.Second
	// maxDrainBytes bounds how much of a 429 body is read to free the connection.
	maxDrainBytes = 64 << 10
)

// retryTransport replays requests that Slack answers with HTTP 429, waiting
// for the duration of the Retry-After header between attempts.
//
// Only 429 responses are retried: Slack guides clients to Retry-After alone,
// and a transport error says nothing about whether a non-idempotent call
// already took effect. A 429 is returned as is when the wait exceeds maxWait or
// the request deadline, or when the request body cannot be re-created
// (GetBody is nil), so streamed uploads are never buffered.
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

// RoundTrip implements http.RoundTripper. When a 429 is not retried, or
// retries are exhausted, it is returned unchanged so callers still see the
// rate limit.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	canRetry := replayable(req)

	for attempt := 0; ; attempt++ {
		attemptReq := req
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("rewind request body: %w", err)
			}
			attemptReq = req.Clone(ctx)
			attemptReq.Body = body
		}

		resp, err := t.base.RoundTrip(attemptReq)
		if err != nil || resp.StatusCode != http.StatusTooManyRequests || attempt >= t.maxRetries || !canRetry {
			return resp, err
		}

		wait, ok := t.retryAfter(resp)
		if !ok || exceedsDeadline(ctx, wait) {
			return resp, nil
		}

		// Draining lets the connection be reused for the retry.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
		resp.Body.Close()

		tflog.Warn(ctx, "Slack rate limited, waiting before retry", map[string]any{
			"wait":    wait.String(),
			"attempt": attempt + 1,
			"url":     req.URL.Path,
		})

		if err := t.sleep(ctx, wait); err != nil {
			return nil, fmt.Errorf("waiting %s for Slack rate limit to clear: %w", wait, err)
		}
	}
}

// retryAfter returns how long to wait before retrying, read from the
// Retry-After header as integer seconds or an HTTP-date. It reports false when
// the wait exceeds maxWait, since the limit will not clear before then.
func (t *retryTransport) retryAfter(resp *http.Response) (time.Duration, bool) {
	wait := defaultRetryAfter

	if v := resp.Header.Get("Retry-After"); v != "" {
		if seconds, err := strconv.Atoi(v); err == nil {
			if seconds >= 0 {
				wait = time.Duration(seconds) * time.Second
			}
		} else if date, err := http.ParseTime(v); err == nil {
			wait = time.Until(date)
		}
	}

	wait = max(wait, minRetryAfter)

	return wait, wait <= t.maxWait
}

// exceedsDeadline reports whether wait would run past the deadline of ctx.
func exceedsDeadline(ctx context.Context, wait time.Duration) bool {
	deadline, ok := ctx.Deadline()

	return ok && wait > time.Until(deadline)
}

// replayable reports whether req can be sent again: it has no body, or its
// body can be re-created through GetBody.
func replayable(req *http.Request) bool {
	return req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
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
