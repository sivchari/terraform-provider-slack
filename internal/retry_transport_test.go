package internal

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// rateLimitServer answers the first limited requests with 429, then with
// status, recording every request body it receives. The n-th 429 carries
// retryAfters[n] as Retry-After, the last entry repeating; an empty entry or
// no entries omits the header.
type rateLimitServer struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []string
}

func newRateLimitServer(t *testing.T, limited, status int, retryAfters ...string) *rateLimitServer {
	t.Helper()

	s := &rateLimitServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		s.mu.Lock()
		s.bodies = append(s.bodies, string(body))
		n := len(s.bodies)
		s.mu.Unlock()

		if n <= limited {
			if len(retryAfters) > 0 {
				if ra := retryAfters[min(n-1, len(retryAfters)-1)]; ra != "" {
					w.Header().Set("Retry-After", ra)
				}
			}
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(s.Close)

	return s
}

func (s *rateLimitServer) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.bodies)
}

// sleepRecorder collects the durations a retryTransport asked to wait.
type sleepRecorder struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (r *sleepRecorder) sleep(_ context.Context, d time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.waits = append(r.waits, d)

	return nil
}

func (r *sleepRecorder) recorded() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.waits)
}

// recordingTransport returns a retryTransport whose sleep records the
// requested durations instead of waiting.
func recordingTransport() (*retryTransport, *sleepRecorder) {
	rec := &sleepRecorder{}
	rt := newRetryTransport(nil)
	rt.sleep = rec.sleep

	return rt, rec
}

func post(ctx context.Context, t *testing.T, rt http.RoundTripper, url, body string) (*http.Response, error) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Errorf("NewRequestWithContext() error = %v", err)
		return nil, err
	}

	return rt.RoundTrip(req)
}

func TestRetryTransport_RetriesAfter429(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 1, http.StatusOK, "2")
	rt, rec := recordingTransport()

	resp, err := post(context.Background(), t, rt, server.URL, "token=abc&text=hello")
	if err != nil {
		t.Errorf("RoundTrip() error = %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	want := []string{"token=abc&text=hello", "token=abc&text=hello"}
	if got := server.requests(); !slices.Equal(got, want) {
		t.Errorf("request bodies = %q, want %q", got, want)
	}
	if got, want := rec.recorded(), []time.Duration{2 * time.Second}; !slices.Equal(got, want) {
		t.Errorf("slept = %v, want %v", got, want)
	}
}

func TestRetryTransport_MultipleRetries(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 2, http.StatusOK, "1", "3")
	rt, rec := recordingTransport()

	resp, err := post(context.Background(), t, rt, server.URL, "token=abc&text=hello")
	if err != nil {
		t.Errorf("RoundTrip() error = %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	want := []string{"token=abc&text=hello", "token=abc&text=hello", "token=abc&text=hello"}
	if got := server.requests(); !slices.Equal(got, want) {
		t.Errorf("request bodies = %q, want %q", got, want)
	}
	if got, want := rec.recorded(), []time.Duration{time.Second, 3 * time.Second}; !slices.Equal(got, want) {
		t.Errorf("slept = %v, want %v", got, want)
	}
}

func TestRetryTransport_RetryAfter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		retryAfter string
		want       time.Duration
	}{
		{name: "seconds are honored", retryAfter: "2", want: 2 * time.Second},
		{name: "zero is floored", retryAfter: "0", want: minRetryAfter},
		{name: "missing header uses the default", retryAfter: "", want: defaultRetryAfter},
		{name: "unparsable header uses the default", retryAfter: "soon", want: defaultRetryAfter},
		{name: "negative header uses the default", retryAfter: "-5", want: defaultRetryAfter},
		{name: "date in the past is floored", retryAfter: time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), want: minRetryAfter},
		{name: "exactly maxWait is honored", retryAfter: "60", want: defaultMaxWait},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newRateLimitServer(t, 1, http.StatusOK, tt.retryAfter)
			rt, rec := recordingTransport()

			resp, err := post(context.Background(), t, rt, server.URL, "a=b")
			if err != nil {
				t.Errorf("RoundTrip() error = %v", err)
				return
			}
			defer resp.Body.Close()

			if got, want := rec.recorded(), []time.Duration{tt.want}; !slices.Equal(got, want) {
				t.Errorf("slept = %v, want %v", got, want)
			}
		})
	}
}

func TestRetryTransport_RetryAfterHTTPDate(t *testing.T) {
	t.Parallel()

	const target = 30 * time.Second

	server := newRateLimitServer(t, 1, http.StatusOK, time.Now().Add(target).UTC().Format(http.TimeFormat))
	rt, rec := recordingTransport()

	resp, err := post(context.Background(), t, rt, server.URL, "a=b")
	if err != nil {
		t.Errorf("RoundTrip() error = %v", err)
		return
	}
	defer resp.Body.Close()

	waits := rec.recorded()
	if len(waits) != 1 {
		t.Errorf("slept = %v, want one wait", waits)
		return
	}
	// The date has one-second resolution and time passes between formatting
	// and parsing, so the wait lands just under target.
	if waits[0] > target || waits[0] < target-5*time.Second {
		t.Errorf("slept = %v, want close to %v", waits[0], target)
	}
}

func TestRetryTransport_RetryAfterAboveMaxWait(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		retryAfter string
	}{
		{name: "seconds", retryAfter: "3600"},
		{name: "http-date", retryAfter: time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newRateLimitServer(t, 1000, http.StatusOK, tt.retryAfter)
			rt, rec := recordingTransport()

			resp, err := post(context.Background(), t, rt, server.URL, "a=b")
			if err != nil {
				t.Errorf("RoundTrip() error = %v", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusTooManyRequests {
				t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
			}
			if got := resp.Header.Get("Retry-After"); got != tt.retryAfter {
				t.Errorf("Retry-After = %q, want the response unchanged", got)
			}
			if got := len(server.requests()); got != 1 {
				t.Errorf("requests = %d, want 1 (no retry)", got)
			}
			if got := rec.recorded(); len(got) != 0 {
				t.Errorf("slept = %v, want none", got)
			}
		})
	}
}

func TestRetryTransport_RetriesExhausted(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 1000, http.StatusOK, "1")
	rt, rec := recordingTransport()

	resp, err := post(context.Background(), t, rt, server.URL, "a=b")
	if err != nil {
		t.Errorf("RoundTrip() error = %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	if got := resp.Header.Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want the final response unchanged", got)
	}
	if got, want := len(server.requests()), rt.maxRetries+1; got != want {
		t.Errorf("requests = %d, want %d", got, want)
	}
	if got, want := len(rec.recorded()), rt.maxRetries; got != want {
		t.Errorf("sleeps = %d, want %d", got, want)
	}
}

func TestRetryTransport_WaitExceedsContextDeadline(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 1000, http.StatusOK, "30")
	rt, rec := recordingTransport()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	resp, err := post(ctx, t, rt, server.URL, "a=b")
	if err != nil {
		t.Errorf("RoundTrip() error = %v, want the 429 response", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	if got := len(server.requests()); got != 1 {
		t.Errorf("requests = %d, want 1 (no retry)", got)
	}
	if got := rec.recorded(); len(got) != 0 {
		t.Errorf("slept = %v, want none", got)
	}
}

func TestRetryTransport_WaitFitsContextDeadline(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 1, http.StatusOK, "1")
	rt, rec := recordingTransport()

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	t.Cleanup(cancel)

	resp, err := post(ctx, t, rt, server.URL, "a=b")
	if err != nil {
		t.Errorf("RoundTrip() error = %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got, want := rec.recorded(), []time.Duration{time.Second}; !slices.Equal(got, want) {
		t.Errorf("slept = %v, want %v", got, want)
	}
}

func TestRetryTransport_ContextCancelledDuringWait(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 1000, http.StatusOK, "1")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	rt := newRetryTransport(nil)
	rt.sleep = func(ctx context.Context, _ time.Duration) error {
		cancel()
		<-ctx.Done()
		return ctx.Err()
	}

	resp, err := post(ctx, t, rt, server.URL, "a=b")
	if err == nil {
		resp.Body.Close()
		t.Error("RoundTrip() error = nil, want context.Canceled")
		return
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("RoundTrip() error = %v, want context.Canceled", err)
	}
	if want := "waiting 1s for Slack rate limit to clear"; !strings.Contains(err.Error(), want) {
		t.Errorf("RoundTrip() error = %q, want it to contain %q", err.Error(), want)
	}
	if got := len(server.requests()); got != 1 {
		t.Errorf("requests = %d, want 1 (no retry after cancel)", got)
	}
}

func TestRetryTransport_DefaultSleepHonorsContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := newRetryTransport(nil).sleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("sleep() error = %v, want context.Canceled", err)
	}
}

func TestRetryTransport_NoRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
	}{
		{name: "200 passes through", status: http.StatusOK},
		{name: "500 is not retried", status: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newRateLimitServer(t, 0, tt.status)
			rt, rec := recordingTransport()

			resp, err := post(context.Background(), t, rt, server.URL, "a=b")
			if err != nil {
				t.Errorf("RoundTrip() error = %v", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.status {
				t.Errorf("StatusCode = %d, want %d", resp.StatusCode, tt.status)
			}
			if got := len(server.requests()); got != 1 {
				t.Errorf("requests = %d, want 1", got)
			}
			if got := rec.recorded(); len(got) != 0 {
				t.Errorf("slept = %v, want none", got)
			}
		})
	}
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// countingBody is an endless response body that counts the bytes read.
type countingBody struct {
	read   int
	closed bool
}

func (b *countingBody) Read(p []byte) (int, error) {
	b.read += len(p)
	return len(p), nil
}

func (b *countingBody) Close() error {
	b.closed = true
	return nil
}

func TestRetryTransport_BoundsDrainOf429Body(t *testing.T) {
	t.Parallel()

	var limited *countingBody
	calls := 0
	rt, _ := recordingTransport()
	rt.base = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls > 1 {
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		}
		limited = &countingBody{}

		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"1"}},
			Body:       limited,
		}, nil
	})

	resp, err := post(context.Background(), t, rt, "http://slack.test/api", "a=b")
	if err != nil {
		t.Errorf("RoundTrip() error = %v", err)
		return
	}
	defer resp.Body.Close()

	if limited == nil {
		t.Error("the 429 was never served")
		return
	}
	if limited.read > maxDrainBytes {
		t.Errorf("drained %d bytes, want at most %d", limited.read, maxDrainBytes)
	}
	if !limited.closed {
		t.Error("429 body not closed")
	}
}

// opaqueBody hides the concrete reader type so http.NewRequest cannot set
// GetBody, as with a streamed request body.
type opaqueBody struct{ io.Reader }

func TestRetryTransport_OpaqueBodyIsNotRetried(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 1, http.StatusOK, "1")
	rt, rec := recordingTransport()

	req, err := http.NewRequest(http.MethodPost, server.URL, opaqueBody{strings.NewReader("streamed=body")})
	if err != nil {
		t.Errorf("NewRequest() error = %v", err)
		return
	}
	if req.GetBody != nil {
		t.Error("GetBody != nil, want nil for an opaque body")
		return
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Errorf("RoundTrip() error = %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusTooManyRequests)
	}
	if got, want := server.requests(), []string{"streamed=body"}; !slices.Equal(got, want) {
		t.Errorf("request bodies = %q, want %q", got, want)
	}
	if got := rec.recorded(); len(got) != 0 {
		t.Errorf("slept = %v, want none", got)
	}
	if req.GetBody != nil {
		t.Error("RoundTrip() modified the caller's request")
	}
}

func TestRetryTransport_BodylessRequest(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 1, http.StatusOK, "0")
	rt, _ := recordingTransport()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Errorf("NewRequest() error = %v", err)
		return
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Errorf("RoundTrip() error = %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := len(server.requests()); got != 2 {
		t.Errorf("requests = %d, want 2", got)
	}
}
