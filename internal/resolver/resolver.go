package resolver

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	"github.com/thedavidweng/jobs-cli/internal/errors"
)

// Resolver maps an application URL to an Application Target. LookupIP is
// injectable so tests can control DNS without network access.
type Resolver struct {
	Client   *http.Client
	LookupIP func(ctx context.Context, host string) ([]net.IP, error)
}

func New(client *http.Client) domain.Resolver {
	return &Resolver{Client: client}
}

func (r *Resolver) Resolve(ctx context.Context, rawURL string) (*domain.ApplicationTarget, error) {
	input := strings.TrimSpace(rawURL)
	if input == "" {
		return nil, resolutionError("no application URL to resolve", nil)
	}
	parsed, err := url.Parse(input)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, resolutionError("application URL "+input+" is not an absolute http(s) URL", err)
	}
	if err := checkHostSyntax(parsed); err != nil {
		return nil, err
	}

	canonical := cleanURL(parsed)
	resolvedFrom := ""
	if requiresRedirect(parsed) {
		final, ferr := r.followRedirects(ctx, canonical)
		if ferr != nil {
			return nil, ferr
		}
		target, perr := url.Parse(final)
		if perr != nil {
			return nil, resolutionError("resolved URL "+final+" is not a valid URL", perr)
		}
		if final != canonical {
			resolvedFrom = canonical
		}
		parsed = target
		canonical = cleanURL(parsed)
	}

	c := classify(parsed)
	if c.url != "" {
		canonical = c.url
	}
	return &domain.ApplicationTarget{
		URL:           canonical,
		Provider:      c.provider,
		ProviderJobID: c.providerJobID,
		BoardToken:    c.boardToken,
		Tenant:        c.tenant,
		Site:          c.site,
		Capabilities:  capabilitiesFor(c.provider),
		Verification:  postureFor(c.provider),
		ResolvedFrom:  resolvedFrom,
	}, nil
}

func resolutionError(message string, err error) *errors.Error {
	return errors.New(errors.ATSResolutionFailed, message, errors.CatAPI, false, err)
}
