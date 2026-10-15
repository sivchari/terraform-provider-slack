package internal

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// rateLimitServer answers the first limited requests with 429 and the given
// Retry-After header (omitted when empty), then with 200, recording every
// request body it receives.
type rateLimitServer struct {
	*httptest.Server
	mu         sync.Mutex
	bodies     []string
	retryAfter string
	limited    int
	status     int
}

func newRateLimitServer(t *testing.T, limited int, retryAfter string) *rateLimitServer {
	t.Helper()

	s := &rateLimitServer{limited: limited, retryAfter: retryAfter, status: http.StatusOK}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		s.mu.Lock()
		s.bodies = append(s.bodies, string(body))
		n := len(s.bodies)
		s.mu.Unlock()

		if n <= s.limited {
			if s.retryAfter != "" {
				w.Header().Set("Retry-After", s.retryAfter)
			}
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(s.status)
	}))
	t.Cleanup(s.Close)

	return s
}

func (s *rateLimitServer) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.bodies...)
}

// recordingTransport returns a retryTransport whose sleep records the
// requested durations instead of waiting.
func recordingTransport(slept *[]time.Duration) *retryTransport {
	rt := newRetryTransport(nil)
	rt.sleep = func(_ context.Context, d time.Duration) error {
		*slept = append(*slept, d)
		return nil
	}

	return rt
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

	server := newRateLimitServer(t, 1, "2")
	var slept []time.Duration
	rt := recordingTransport(&slept)

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
	if got := server.requests(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("request bodies = %q, want %q", got, want)
	}
	if len(slept) != 1 || slept[0] != 2*time.Second {
		t.Errorf("slept = %v, want [2s]", slept)
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
		{name: "zero retries immediately", retryAfter: "0", want: 0},
		{name: "missing header uses the default", retryAfter: "", want: defaultRetryAfter},
		{name: "unparsable header uses the default", retryAfter: "soon", want: defaultRetryAfter},
		{name: "negative header uses the default", retryAfter: "-5", want: defaultRetryAfter},
		{name: "above maxWait is capped", retryAfter: "3600", want: defaultMaxWait},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newRateLimitServer(t, 1, tt.retryAfter)
			var slept []time.Duration
			rt := recordingTransport(&slept)

			resp, err := post(context.Background(), t, rt, server.URL, "a=b")
			if err != nil {
				t.Errorf("RoundTrip() error = %v", err)
				return
			}
			defer resp.Body.Close()

			if len(slept) != 1 || slept[0] != tt.want {
				t.Errorf("slept = %v, want [%v]", slept, tt.want)
			}
		})
	}
}

func TestRetryTransport_RetriesExhausted(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 1000, "1")
	var slept []time.Duration
	rt := recordingTransport(&slept)

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
	if got, want := len(slept), rt.maxRetries; got != want {
		t.Errorf("sleeps = %d, want %d", got, want)
	}
}

func TestRetryTransport_ContextCancelledDuringWait(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 1000, "1")
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

			server := newRateLimitServer(t, 0, "")
			server.status = tt.status
			var slept []time.Duration
			rt := recordingTransport(&slept)

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
			if len(slept) != 0 {
				t.Errorf("slept = %v, want none", slept)
			}
		})
	}
}

// opaqueBody hides the concrete reader type so http.NewRequest cannot set
// GetBody, as with a streamed request body.
type opaqueBody struct{ io.Reader }

func TestRetryTransport_BuffersBodyWithoutGetBody(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 1, "0")
	var slept []time.Duration
	rt := recordingTransport(&slept)

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

	want := []string{"streamed=body", "streamed=body"}
	if got := server.requests(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("request bodies = %q, want %q", got, want)
	}
	if req.GetBody != nil {
		t.Error("RoundTrip() modified the caller's request")
	}
}

func TestRetryTransport_BodylessRequest(t *testing.T) {
	t.Parallel()

	server := newRateLimitServer(t, 1, "0")
	var slept []time.Duration
	rt := recordingTransport(&slept)

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
