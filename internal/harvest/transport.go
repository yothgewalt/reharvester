package harvest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	defaultRetries = 4
	retryBackoff   = 5 // first wait is 5×Delay: 15 s at the default, then 30, 60, 120
	maxRetryAfter  = 60 * time.Second
	maxBody        = 64 << 20
)

// get performs one GET and returns the body. Callers own the politeness delay;
// see Client.pause.
//
// A throttled response (429 or 503) or a request that times out — arXiv often
// stalls instead of answering 429 — is retried up to MaxRetries times, waiting
// for the server's Retry-After or, without one, retryBackoff×Delay doubled per
// attempt. Each retry is logged. Only when they run out does the error return,
// as "<host>: <status>".
func (c *Client) get(ctx context.Context, rawURL string, header http.Header) ([]byte, error) {
	retries := c.MaxRetries
	if retries <= 0 {
		retries = defaultRetries
	}
	for attempt := 0; ; attempt++ {
		body, wait, err := c.getOnce(ctx, rawURL, header)
		if wait < 0 || attempt == retries || ctx.Err() != nil {
			return body, err
		}
		if wait == 0 {
			wait = retryBackoff * c.Delay << attempt
		}
		log.Printf("harvest: %v — retrying in %s (%d/%d)", err, wait, attempt+1, retries)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// getOnce makes a single request. wait is negative when the result is final,
// and otherwise how long the server asked us to back off (zero: it did not say).
func (c *Client) getOnce(ctx context.Context, rawURL string, header http.Header) ([]byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, -1, err
	}
	for k, vs := range header {
		req.Header[k] = vs
	}
	req.Header.Set("User-Agent", "reharvester/0.1 (local-first literature discovery)")
	res, err := c.HTTP.Do(req)
	if err != nil {
		// The error embeds the request URL, which can carry an API key.
		if ue, ok := errors.AsType[*url.Error](err); ok {
			ue.URL = redactURL(ue.URL)
		}
		if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
			return nil, 0, err
		}
		return nil, -1, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		io.Copy(io.Discard, res.Body)
		err := fmt.Errorf("%s: %s", req.URL.Host, res.Status)
		if res.StatusCode != http.StatusTooManyRequests && res.StatusCode != http.StatusServiceUnavailable {
			return nil, -1, err
		}
		wait := time.Duration(0)
		if s, perr := strconv.Atoi(res.Header.Get("Retry-After")); perr == nil && s > 0 {
			wait = min(time.Duration(s)*time.Second, maxRetryAfter)
		}
		return nil, wait, err
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody))
	if err != nil {
		return nil, -1, fmt.Errorf("%s: reading body: %w", req.URL.Host, err)
	}
	return body, -1, nil
}

// secretParams are query parameters redactURL masks.
var secretParams = []string{"api_key", "apikey", "key", "token"}

// redactURL masks credentials in a URL so it can be logged or shown.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<unparseable url>"
	}
	q := u.Query()
	changed := false
	for _, k := range secretParams {
		if q.Has(k) {
			q.Set(k, "REDACTED")
			changed = true
		}
	}
	if changed {
		u.RawQuery = q.Encode()
	}
	return u.String()
}
