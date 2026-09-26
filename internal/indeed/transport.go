package indeed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
	"github.com/thedavidweng/jobs-cli/internal/httpclient"
)

func (s *Source) call(ctx context.Context, query string, mkt market) ([]byte, *joberrors.Error) {
	payload, err := json.Marshal(graphQLRequest{Query: query})
	if err != nil {
		return nil, internalError("encode indeed GraphQL request", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphQLEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, internalError("build indeed GraphQL request", err)
	}
	req.Host = "apis.indeed.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("accept", "application/json")
	req.Header.Set("accept-language", mkt.acceptLanguage())
	req.Header.Set("user-agent", userAgent)
	req.Header.Set("indeed-api-key", apiKey)
	req.Header.Set("indeed-app-info", appInfo)
	req.Header.Set("indeed-locale", mkt.locale)
	req.Header.Set("indeed-co", mkt.country)

	resp, err := s.client().Do(req)
	if err != nil {
		return nil, networkError(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, networkError(err)
	}
	if failure := responseFailure(resp, body); failure != nil {
		return nil, failure
	}
	return body, nil
}

func (s *Source) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return http.DefaultClient
}

func responseFailure(resp *http.Response, body []byte) *joberrors.Error {
	if failure := statusFailure(resp, body); failure != nil {
		return failure
	}
	if looksLikeHTML(body) {
		return antiBotError(resp)
	}
	if !jsonBody(body) {
		return joberrors.New(
			joberrors.APIError,
			fmt.Sprintf("indeed returned a non-JSON response (status %d, content type %q)", resp.StatusCode, resp.Header.Get("Content-Type")),
			joberrors.CatAPI,
			false,
			nil,
		)
	}
	return nil
}

func statusFailure(resp *http.Response, body []byte) *joberrors.Error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusTooManyRequests:
		return joberrors.NewWithRetryAfter(joberrors.RateLimited, "indeed rate limited the request", joberrors.CatAPI, true, httpclient.RetryAfter(resp, time.Second), nil)
	case http.StatusUnauthorized, http.StatusForbidden:
		if looksLikeHTML(body) {
			return antiBotError(resp)
		}
		return joberrors.New(
			joberrors.APIAccessForbidden,
			fmt.Sprintf("indeed rejected the mobile client request (status %d)", resp.StatusCode),
			joberrors.CatAPI,
			false,
			nil,
		)
	default:
		return joberrors.New(
			joberrors.APIError,
			fmt.Sprintf("indeed returned status %d", resp.StatusCode),
			joberrors.CatAPI,
			resp.StatusCode >= 500,
			nil,
		)
	}
}

func antiBotError(resp *http.Response) *joberrors.Error {
	return joberrors.New(
		joberrors.APIError,
		fmt.Sprintf("indeed returned an anti-bot HTML challenge (status %d, content type %q) instead of JSON", resp.StatusCode, resp.Header.Get("Content-Type")),
		joberrors.CatAPI,
		false,
		nil,
	)
}

func looksLikeHTML(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	return bytes.HasPrefix(trimmed, []byte("<!DOCTYPE")) || bytes.HasPrefix(trimmed, []byte("<html")) || bytes.HasPrefix(trimmed, []byte("<body"))
}

func jsonBody(body []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(body), []byte("{"))
}

func networkError(err error) *joberrors.Error {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return joberrors.New(joberrors.NetworkTimeout, "indeed request timed out: "+err.Error(), joberrors.CatNetwork, true, err)
	}
	return joberrors.New(joberrors.NetworkUnreachable, "indeed request failed: "+err.Error(), joberrors.CatNetwork, true, err)
}

func schemaDrift(detail string) *joberrors.Error {
	return joberrors.New(joberrors.APISchemaChanged, "indeed response schema changed: "+detail, joberrors.CatAPI, false, nil)
}

func internalError(message string, err error) *joberrors.Error {
	return joberrors.New(joberrors.InternalError, message+": "+err.Error(), joberrors.CatInternal, false, err)
}
