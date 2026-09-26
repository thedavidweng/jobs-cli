package httpclient

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

var ErrResponseTooLarge = errors.New("remote response exceeded the configured size limit")

type Policy struct {
	ReadRetries  int
	Backoff      time.Duration
	MaxBodyBytes int64
}

func DefaultPolicy() Policy {
	return Policy{
		ReadRetries:  2,
		Backoff:      250 * time.Millisecond,
		MaxBodyBytes: 10 << 20,
	}
}

type Transport struct {
	Base   http.RoundTripper
	Policy Policy
	Sleep  func(time.Duration)
}

func New(base http.RoundTripper, policy Policy) *http.Client {
	if base == nil {
		base = http.DefaultTransport
	}
	if policy.ReadRetries == 0 && policy.Backoff == 0 && policy.MaxBodyBytes == 0 {
		policy = DefaultPolicy()
	}
	return &http.Client{
		Transport: &Transport{Base: base, Policy: policy},
	}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	attempts := t.Policy.ReadRetries + 1
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		resp, err := t.Base.RoundTrip(req)
		return t.limit(resp, err)
	}
	var lastResp *http.Response
	for attempt := 0; attempt < attempts; attempt++ {
		cloned := req.Clone(req.Context())
		resp, err := t.Base.RoundTrip(cloned)
		if err != nil {
			if attempt == attempts-1 {
				return nil, err
			}
			t.wait(RetryAfter(nil, t.Policy.Backoff))
			continue
		}
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			return t.limit(resp, nil)
		}
		if attempt == attempts-1 {
			lastResp = resp
			break
		}
		delay := RetryAfter(resp, t.Policy.Backoff)
		drain(resp)
		t.wait(delay)
	}
	return t.limit(lastResp, nil)
}

func (t *Transport) limit(resp *http.Response, err error) (*http.Response, error) {
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	if t.Policy.MaxBodyBytes > 0 {
		resp.Body = &limitedBody{reader: resp.Body, remaining: t.Policy.MaxBodyBytes}
	}
	return resp, nil
}

func (t *Transport) wait(d time.Duration) {
	if d <= 0 {
		return
	}
	if t.Sleep != nil {
		t.Sleep(d)
		return
	}
	time.Sleep(d)
}

func RetryAfter(resp *http.Response, fallback time.Duration) time.Duration {
	if resp == nil {
		return fallback
	}
	value := resp.Header.Get("Retry-After")
	if value == "" {
		return fallback
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return fallback
}

func drain(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	_ = resp.Body.Close()
}

type limitedBody struct {
	reader    io.ReadCloser
	remaining int64
	overflow  bool
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.overflow {
		return 0, ErrResponseTooLarge
	}
	if b.remaining <= 0 {
		var probe [1]byte
		n, err := b.reader.Read(probe[:])
		if n > 0 {
			b.overflow = true
			return 0, ErrResponseTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.reader.Read(p)
	b.remaining -= int64(n)
	return n, err
}

func (b *limitedBody) Close() error {
	return b.reader.Close()
}
