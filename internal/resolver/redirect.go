package resolver

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/httpclient"
)

const maxRedirects = 5

func (r *Resolver) followRedirects(ctx context.Context, start string) (string, error) {
	current := start
	for hops := 0; ; hops++ {
		u, err := url.Parse(current)
		if err != nil {
			return "", resolutionError("application URL "+current+" is not a valid URL", err)
		}
		if u.Scheme != "https" {
			return "", resolutionError("refusing to follow a clear-text (non-HTTPS) application redirect to "+current, nil)
		}
		if err := r.checkReachableHost(ctx, u); err != nil {
			return "", err
		}
		resp, err := r.get(ctx, current)
		if err != nil {
			return "", resolutionError("could not resolve application redirect "+current, err)
		}
		location := ""
		if isRedirectStatus(resp.StatusCode) {
			location = resp.Header.Get("Location")
		}
		drainAndClose(resp)
		if location == "" {
			return current, nil
		}
		if hops >= maxRedirects {
			return "", resolutionError("application URL exceeded "+strconv.Itoa(maxRedirects)+" redirects", nil)
		}
		next, err := u.Parse(location)
		if err != nil {
			return "", resolutionError("application redirect location "+location+" is not a valid URL", err)
		}
		next.Fragment = ""
		current = next.String()
	}
}

func (r *Resolver) get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	return r.httpClient().Do(req)
}

func (r *Resolver) httpClient() *http.Client {
	client := r.Client
	if client == nil {
		client = new(http.Client)
	}
	cloned := *client
	cloned.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if cloned.Transport == nil {
		cloned.Transport = http.DefaultTransport
	}
	cloned.Transport = r.guardTransport(cloned.Transport)
	return &cloned
}

func (r *Resolver) guardTransport(rt http.RoundTripper) http.RoundTripper {
	switch t := rt.(type) {
	case *http.Transport:
		guarded := t.Clone()
		guarded.DialContext = r.guardedDialContext()
		return guarded
	case *httpclient.Transport:
		copied := *t
		copied.Base = r.guardTransport(t.Base)
		return &copied
	default:
		return rt
	}
}

func (r *Resolver) guardedDialContext() func(context.Context, string, string) (net.Conn, error) {
	var dialer net.Dialer
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil {
			if blockedIP(ip) {
				return nil, resolutionError("refusing to dial private, link-local, or metadata address "+host, nil)
			}
			return dialer.DialContext(ctx, network, addr)
		}
		ips, lerr := r.lookupIP(ctx, host)
		if lerr != nil || len(ips) == 0 {
			return dialer.DialContext(ctx, network, addr)
		}
		for _, ip := range ips {
			if blockedIP(ip) {
				return nil, resolutionError("refusing to dial private, link-local, or metadata address "+ip.String()+" for host "+host, nil)
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
}

func (r *Resolver) checkReachableHost(ctx context.Context, u *url.URL) error {
	if err := checkHostSyntax(u); err != nil {
		return err
	}
	host := strings.ToLower(u.Hostname())
	if net.ParseIP(host) != nil {
		return nil
	}
	ips, err := r.lookupIP(ctx, host)
	if err != nil {
		return resolutionError("could not resolve host "+host, err)
	}
	if len(ips) == 0 {
		return resolutionError("host "+host+" did not resolve to any address", nil)
	}
	for _, ip := range ips {
		if blockedIP(ip) {
			return resolutionError("refusing to contact private, link-local, or metadata address "+ip.String()+" for host "+host, nil)
		}
	}
	return nil
}

func (r *Resolver) lookupIP(ctx context.Context, host string) ([]net.IP, error) {
	if r.LookupIP != nil {
		return r.LookupIP(ctx, host)
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		ips = append(ips, addr.IP)
	}
	return ips, nil
}

func checkHostSyntax(u *url.URL) error {
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return resolutionError("application URL "+u.String()+" has no host", nil)
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return resolutionError("refusing to resolve local host "+host, nil)
	}
	if ip := net.ParseIP(host); ip != nil && blockedIP(ip) {
		return resolutionError("refusing to resolve private, link-local, or metadata address "+ip.String(), nil)
	}
	return nil
}

func blockedIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}

func isRedirectStatus(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func drainAndClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
}
