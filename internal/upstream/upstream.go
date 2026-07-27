// Package upstream is the shared HTTP plumbing for aether's outbound calls to
// third-party services (Cover Art Archive, MusicBrainz, fanart.tv,
// TheAudioDB, radio-browser.info).
//
// It exists so every one of those clients handles a flaky provider the same
// way: fair-use throttling, a bounded retry with exponential backoff for
// transient failures (5xx, 429, timeouts), Retry-After compliance, and a typed
// *Error that carries both the technical detail (logs) and a human-readable
// sentence naming the service (the UI). Handlers map the error to a status code
// with HTTPStatus and a message with UserMessage, so an upstream hiccup never
// surfaces as a raw "status 500" to the user.
package upstream

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

// Kind classifies an upstream failure. It decides both the status code aether
// answers with and the sentence the user reads.
type Kind int

const (
	// KindUnavailable is a persistent upstream server error (5xx) that
	// outlived the retries.
	KindUnavailable Kind = iota
	// KindRateLimited is a 429: we are being throttled by the provider.
	KindRateLimited
	// KindTimeout is a request that did not complete in time.
	KindTimeout
	// KindUnreachable is a transport-level failure (DNS, refused, reset).
	KindUnreachable
	// KindRejected is a 4xx other than 429 — the request itself was refused,
	// so retrying is pointless.
	KindRejected
	// KindBadResponse is a 2xx whose body could not be parsed.
	KindBadResponse
)

// defaults for a new Doer. Three attempts over ~1.5s of backoff keeps a
// user-facing lookup responsive while still riding out a single bad gateway.
const (
	defaultMaxAttempts   = 3
	defaultBackoff       = 500 * time.Millisecond
	defaultTimeout       = 20 * time.Second
	maxRetryAfterWait    = 5 * time.Second
	defaultRetryAfterCap = maxRetryAfterWait
)

// Error is a failed call to an external service. Message carries the technical
// detail for logs; UserMessage renders the sentence shown to a person.
type Error struct {
	// Service is the provider's display name, e.g. "Cover Art Archive".
	Service string
	Kind    Kind
	// Status is the upstream HTTP status, or 0 when the request never
	// produced a response.
	Status int
	// Attempts is how many requests were made before giving up.
	Attempts int
	Err      error

	// retryable marks a failure worth another attempt; retryAfter is the delay
	// the provider asked for, if any. Both are internal to the retry loop.
	retryable  bool
	retryAfter time.Duration
}

func (e *Error) Error() string {
	svc := e.Service
	if svc == "" {
		svc = "upstream service"
	}
	switch {
	case e.Status > 0 && e.Err != nil:
		return fmt.Sprintf("%s: status %d after %d attempt(s): %v", svc, e.Status, e.Attempts, e.Err)
	case e.Status > 0:
		return fmt.Sprintf("%s: status %d after %d attempt(s)", svc, e.Status, e.Attempts)
	case e.Err != nil:
		return fmt.Sprintf("%s: %v", svc, e.Err)
	default:
		return svc + ": request failed"
	}
}

func (e *Error) Unwrap() error { return e.Err }

// WrapError builds an *Error for a provider whose client does not use Doer —
// a hand-rolled or libs/ client that classified its own failure but cannot
// import this package. Status may be 0 when no response was received.
//
// The retry bookkeeping is deliberately left zero: the caller already finished
// trying, and this only exists so the failure reaches handlers in the shape
// HTTPStatus and UserMessage understand.
func WrapError(service string, kind Kind, status int, err error) *Error {
	return &Error{Service: service, Kind: kind, Status: status, Err: err, Attempts: 1}
}

// UserMessage is a complete, human-readable sentence naming the service and
// what to do about it. It deliberately omits status codes and Go error text.
func (e *Error) UserMessage() string {
	svc := e.Service
	if svc == "" {
		svc = "The external service"
	}
	switch e.Kind {
	case KindRateLimited:
		return svc + " is receiving too many requests right now. Wait a moment and try again."
	case KindTimeout:
		return svc + " took too long to respond. Try again in a moment."
	case KindUnreachable:
		return svc + " could not be reached. Check the server's internet connection and try again."
	case KindRejected:
		return svc + " rejected this request. The identifier may be wrong or no longer exist."
	case KindBadResponse:
		return svc + " returned a response aether could not read. Try again in a moment."
	default:
		return svc + " is temporarily unavailable. Try again in a few minutes."
	}
}

// HTTPStatus is the status aether should answer with for err: 502 by default,
// mirroring the upstream condition where a more precise code exists.
func HTTPStatus(err error) int {
	var uerr *Error
	if !errors.As(err, &uerr) {
		return http.StatusBadGateway
	}
	switch uerr.Kind {
	case KindRateLimited:
		return http.StatusTooManyRequests
	case KindTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusBadGateway
	}
}

// IsRejected reports whether err is the provider refusing the request itself
// (a 4xx other than 429). Callers use it to treat "no data for this id" as an
// empty result while still propagating genuine outages.
func IsRejected(err error) bool {
	var uerr *Error
	return errors.As(err, &uerr) && uerr.Kind == KindRejected
}

// UserMessage returns err's human-readable sentence, or fallback when err is
// not an upstream error (so callers can hand any error to the UI safely).
func UserMessage(err error, fallback string) string {
	var uerr *Error
	if errors.As(err, &uerr) {
		return uerr.UserMessage()
	}
	return fallback
}

// Doer performs throttled, retrying GET requests against one external service.
// Construct it with New; the exported fields are overridable in tests.
type Doer struct {
	// Service is the provider's display name, used in error messages.
	Service   string
	UserAgent string
	Client    *http.Client
	Limiter   *rate.Limiter
	// MaxAttempts bounds the total number of requests per call (1 = no retry).
	MaxAttempts int
	// Backoff is the delay before the second attempt; it doubles thereafter.
	Backoff time.Duration
	// RetryAfterCap bounds how long a Retry-After header can make us wait —
	// a provider asking for minutes must not hang a user-facing request.
	RetryAfterCap time.Duration
	// Wait sleeps for d, honouring ctx. Overridable so tests assert backoff
	// timing without real delay.
	Wait func(ctx context.Context, d time.Duration) error
}

// New returns a Doer for the named service, throttled to rps (burst 1).
func New(service, userAgent string, rps rate.Limit) *Doer {
	return &Doer{
		Service:       service,
		UserAgent:     userAgent,
		Client:        &http.Client{Timeout: defaultTimeout},
		Limiter:       rate.NewLimiter(rps, 1),
		MaxAttempts:   defaultMaxAttempts,
		Backoff:       defaultBackoff,
		RetryAfterCap: defaultRetryAfterCap,
		Wait:          sleep,
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Get issues a throttled GET, retrying transient failures. Any status in
// allowStatus is returned to the caller as a successful response (used for
// "404 = no data here", which is not an error for most of our providers);
// every other non-2xx becomes an *Error.
//
// On success the caller owns resp.Body and must close it.
func (d *Doer) Get(ctx context.Context, url string, header http.Header, allowStatus ...int) (*http.Response, error) {
	attempts := d.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	backoff := d.Backoff
	var last *Error

	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			wait := backoff
			if last != nil && last.retryAfter > 0 {
				wait = last.retryAfter
			}
			if err := d.wait(ctx, wait); err != nil {
				return nil, err
			}
			backoff *= 2
		}

		resp, err := d.do(ctx, url, header)
		if err != nil {
			// Throttle/context failures are ours, not the provider's: report
			// them as-is and do not burn retries on them.
			var uerr *Error
			if !errors.As(err, &uerr) {
				return nil, err
			}
			uerr.Attempts = attempt
			if !uerr.retryable {
				return nil, uerr
			}
			last = uerr
			continue
		}

		if resp.StatusCode < 300 || containsStatus(allowStatus, resp.StatusCode) {
			return resp, nil
		}

		serr := d.statusError(resp)
		_ = resp.Body.Close()
		serr.Attempts = attempt
		if !serr.retryable {
			return nil, serr
		}
		last = serr
	}

	if last == nil { // unreachable: the loop always runs at least once
		last = &Error{Service: d.Service, Kind: KindUnavailable, Attempts: attempts}
	}
	last.Attempts = attempts
	return nil, last
}

func (d *Doer) wait(ctx context.Context, dur time.Duration) error {
	if d.Wait != nil {
		return d.Wait(ctx, dur)
	}
	return sleep(ctx, dur)
}

// do performs one throttled request, classifying transport failures.
func (d *Doer) do(ctx context.Context, url string, header http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if d.UserAgent != "" {
		req.Header.Set("User-Agent", d.UserAgent)
	}
	for k, vals := range header {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	if d.Limiter != nil {
		if err := d.Limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("%s throttle: %w", d.Service, err)
		}
	}
	client := d.Client
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		// A cancelled/expired caller context is the caller's business, not a
		// provider fault — don't retry it and don't relabel it.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		kind := KindUnreachable
		if isTimeout(err) {
			kind = KindTimeout
		}
		return nil, &Error{Service: d.Service, Kind: kind, Err: err, retryable: true}
	}
	return resp, nil
}

// statusError classifies a non-2xx response. 5xx and 429 are transient (worth
// a retry); every other 4xx is a refusal we must not hammer.
func (d *Doer) statusError(resp *http.Response) *Error {
	e := &Error{Service: d.Service, Status: resp.StatusCode}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		e.Kind = KindRateLimited
		e.retryable = true
		e.retryAfter = d.retryAfter(resp)
	case resp.StatusCode >= 500:
		e.Kind = KindUnavailable
		e.retryable = true
		e.retryAfter = d.retryAfter(resp)
	default:
		e.Kind = KindRejected
	}
	return e
}

// retryAfter reads the Retry-After header (delay-seconds or HTTP-date),
// clamped to RetryAfterCap. Zero means "use the normal backoff".
func (d *Doer) retryAfter(resp *http.Response) time.Duration {
	raw := resp.Header.Get("Retry-After")
	if raw == "" {
		return 0
	}
	var wait time.Duration
	if secs, err := strconv.Atoi(raw); err == nil {
		wait = time.Duration(secs) * time.Second
	} else if when, err := http.ParseTime(raw); err == nil {
		// An HTTP-date is absolute; relative waits need the response's own
		// clock reference, so fall back to the backoff when it is in the past.
		wait = time.Until(when)
	}
	if wait <= 0 {
		return 0
	}
	limit := d.RetryAfterCap
	if limit <= 0 {
		limit = defaultRetryAfterCap
	}
	if wait > limit {
		wait = limit
	}
	return wait
}

// BadResponse wraps a parse failure on an otherwise-successful response, so a
// malformed provider payload reads like any other upstream problem.
func (d *Doer) BadResponse(err error) error {
	return &Error{Service: d.Service, Kind: KindBadResponse, Err: err}
}

func isTimeout(err error) bool {
	var terr interface{ Timeout() bool }
	return errors.As(err, &terr) && terr.Timeout()
}

func containsStatus(list []int, status int) bool {
	for _, s := range list {
		if s == status {
			return true
		}
	}
	return false
}
