package resolver

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/thedavidweng/jobs-cli/internal/domain"
)

type classification struct {
	provider      domain.ApplicationProvider
	providerJobID string
	boardToken    string
	tenant        string
	site          string
	url           string
}

func classify(u *url.URL) classification {
	host := strings.ToLower(u.Hostname())
	segments := pathSegments(u.Path)
	switch {
	case hostMatches(host, "greenhouse.io"):
		return classifyGreenhouse(u, segments)
	case hostMatches(host, "lever.co"):
		return classifyLever(segments)
	case hostMatches(host, "ashbyhq.com"):
		return classifyAshby(segments)
	case hostMatches(host, "myworkdayjobs.com"), hostMatches(host, "myworkdaysite.com"):
		return classifyWorkday(host, segments)
	case hostMatches(host, "smartrecruiters.com"):
		return classifySmartRecruiters(segments)
	case hostMatches(host, "icims.com"):
		return classifyICIMS(host, segments)
	case hostMatches(host, "linkedin.com"):
		if c, ok := classifyLinkedIn(segments, u.Query()); ok {
			return c
		}
	case hostMatches(host, "indeed.com"):
		if c, ok := classifyIndeed(u, segments); ok {
			return c
		}
	}
	return classifyGeneric(u)
}

func requiresRedirect(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	if hostMatches(host, "grnh.se") {
		return true
	}
	if hostMatches(host, "greenhouse.io") && strings.HasPrefix(u.Path, "/embed/") {
		query := u.Query()
		return query.Get("for") == "" || firstNonEmpty(query.Get("token"), query.Get("gh_jid")) == ""
	}
	return false
}

func classifyGreenhouse(u *url.URL, segments []string) classification {
	c := classification{provider: domain.ProviderGreenhouse}
	query := u.Query()
	switch {
	case len(segments) >= 2 && segments[0] == "embed":
		boardToken := query.Get("for")
		jobID := firstNonEmpty(query.Get("token"), query.Get("gh_jid"))
		if boardToken != "" && jobID != "" {
			c.boardToken = boardToken
			c.providerJobID = jobID
			c.url = greenhouseJobURL(boardToken, jobID)
		}
	case len(segments) >= 3 && segments[1] == "jobs":
		c.boardToken = segments[0]
		c.providerJobID = segments[2]
	case len(segments) >= 2 && segments[0] == "jobs":
		c.providerJobID = segments[1]
		c.boardToken = query.Get("for")
	case len(segments) == 1 && query.Get("gh_jid") != "":
		c.boardToken = segments[0]
		c.providerJobID = query.Get("gh_jid")
		c.url = greenhouseJobURL(c.boardToken, c.providerJobID)
	}
	return c
}

func classifyLever(segments []string) classification {
	c := classification{provider: domain.ProviderLever}
	if len(segments) >= 2 {
		c.boardToken = segments[0]
		c.providerJobID = segments[1]
	}
	return c
}

func classifyAshby(segments []string) classification {
	c := classification{provider: domain.ProviderAshby}
	if len(segments) >= 2 {
		c.boardToken = segments[0]
		c.providerJobID = segments[1]
	}
	return c
}

func classifyWorkday(host string, segments []string) classification {
	c := classification{provider: domain.ProviderWorkday}
	if labels := strings.Split(host, "."); len(labels) > 0 {
		c.tenant = labels[0]
	}
	rest := segments
	if len(rest) > 0 && localeSegment(rest[0]) {
		rest = rest[1:]
	}
	if len(rest) > 0 && rest[0] != "job" && rest[0] != "jobs" {
		c.site = rest[0]
	}
	if len(segments) > 0 {
		c.providerJobID = workdayRequisition(segments[len(segments)-1])
	}
	return c
}

func classifySmartRecruiters(segments []string) classification {
	c := classification{provider: domain.ProviderSmartRecruiters}
	if len(segments) >= 2 {
		c.boardToken = segments[0]
		c.providerJobID = smartRecruitersJobID(segments[1])
	}
	return c
}

func classifyICIMS(host string, segments []string) classification {
	c := classification{provider: domain.ProviderICIMS}
	if label, _, ok := strings.Cut(host, "."); ok {
		c.boardToken = strings.TrimPrefix(label, "careers-")
	} else {
		c.boardToken = host
	}
	for i, segment := range segments {
		if segment == "jobs" && i+1 < len(segments) && isDigits(segments[i+1]) {
			c.providerJobID = segments[i+1]
			break
		}
	}
	return c
}

func classifyLinkedIn(segments []string, query url.Values) (classification, bool) {
	c := classification{provider: domain.ProviderLinkedIn}
	if len(segments) >= 3 && segments[0] == "jobs" && segments[1] == "view" {
		c.providerJobID = trailingDigits(segments[2])
		return c, true
	}
	if jobID := query.Get("currentJobId"); jobID != "" {
		c.providerJobID = jobID
		return c, true
	}
	return classification{}, false
}

func classifyIndeed(u *url.URL, segments []string) (classification, bool) {
	query := u.Query()
	jobID := firstNonEmpty(query.Get("jk"), query.Get("vjk"))
	if jobID == "" {
		return classification{}, false
	}
	c := classification{provider: domain.ProviderIndeed, providerJobID: jobID}
	if len(segments) > 0 && segments[len(segments)-1] == "viewjob" {
		c.url = "https://" + canonicalHost(u) + "/viewjob?jk=" + url.QueryEscape(jobID)
	}
	return c, true
}

func classifyGeneric(u *url.URL) classification {
	if net.ParseIP(u.Hostname()) != nil {
		return classification{provider: domain.ProviderUnknown}
	}
	if looksLikeApplicationURL(u) {
		return classification{provider: domain.ProviderExternal}
	}
	return classification{provider: domain.ProviderUnknown}
}

func looksLikeApplicationURL(u *url.URL) bool {
	haystack := strings.ToLower(u.Path + "?" + u.RawQuery)
	for _, marker := range []string{"job", "apply", "career", "position", "vacanc", "opening", "posting", "gh_jid"} {
		if strings.Contains(haystack, marker) {
			return true
		}
	}
	return false
}

var providerCapabilities = map[domain.ApplicationProvider]domain.Capabilities{
	domain.ProviderGreenhouse:      {Inspect: true, Prepare: true, NativeSubmit: true},
	domain.ProviderLinkedIn:        {Inspect: true, Prepare: true, AuthRequired: true},
	domain.ProviderIndeed:          {BrowserRequired: true},
	domain.ProviderLever:           {BrowserRequired: true},
	domain.ProviderAshby:           {BrowserRequired: true},
	domain.ProviderWorkday:         {BrowserRequired: true},
	domain.ProviderSmartRecruiters: {BrowserRequired: true},
	domain.ProviderICIMS:           {BrowserRequired: true},
	domain.ProviderExternal:        {BrowserRequired: true},
	domain.ProviderUnknown:         {BrowserRequired: true},
}

func capabilitiesFor(provider domain.ApplicationProvider) domain.Capabilities {
	if capabilities, ok := providerCapabilities[provider]; ok {
		return capabilities
	}
	return domain.Capabilities{BrowserRequired: true}
}

func postureFor(provider domain.ApplicationProvider) domain.VerificationPosture {
	switch provider {
	case domain.ProviderGreenhouse:
		return domain.VerifiedWorking
	case domain.ProviderLinkedIn:
		return domain.VerifiedSourceImpl
	default:
		return domain.BrowserRequiredPosture
	}
}

func greenhouseJobURL(boardToken, jobID string) string {
	return "https://job-boards.greenhouse.io/" + boardToken + "/jobs/" + jobID
}

func cleanURL(u *url.URL) string {
	cleaned := *u
	cleaned.Scheme = strings.ToLower(cleaned.Scheme)
	cleaned.Host = canonicalHost(&cleaned)
	cleaned.Fragment = ""
	cleaned.RawPath = ""
	cleaned.RawQuery = stripTracking(cleaned.Query()).Encode()
	if cleaned.Path != "" && cleaned.Path != "/" {
		cleaned.Path = strings.TrimRight(cleaned.Path, "/")
	}
	return cleaned.String()
}

func canonicalHost(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port != "" && !defaultPort(u.Scheme, port) {
		return net.JoinHostPort(host, port)
	}
	return host
}

func defaultPort(scheme, port string) bool {
	return (scheme == "https" && port == "443") || (scheme == "http" && port == "80")
}

var trackingParams = map[string]bool{
	"gclid": true, "fbclid": true, "msclkid": true, "mc_cid": true, "mc_eid": true,
	"gh_src": true, "refid": true, "trackingid": true, "trk": true, "originalsubdomain": true,
}

func stripTracking(values url.Values) url.Values {
	cleaned := url.Values{}
	for key, list := range values {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || trackingParams[lower] {
			continue
		}
		cleaned[key] = list
	}
	return cleaned
}

func pathSegments(path string) []string {
	segments := make([]string, 0, 4)
	for _, part := range strings.Split(path, "/") {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

func hostMatches(host, suffix string) bool {
	return host == suffix || strings.HasSuffix(host, "."+suffix)
}

func localeSegment(segment string) bool {
	if _, err := strconv.Atoi(segment); err == nil {
		return false
	}
	parts := strings.Split(segment, "-")
	if len(parts) == 0 || len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if len(part) < 2 || len(part) > 3 {
			return false
		}
		for _, r := range part {
			if !unicode.IsLetter(r) {
				return false
			}
		}
	}
	return true
}

func workdayRequisition(segment string) string {
	index := strings.LastIndex(segment, "_")
	if index <= 0 || index == len(segment)-1 {
		return ""
	}
	candidate := segment[index+1:]
	if !strings.ContainsAny(candidate, "0123456789") {
		return ""
	}
	return candidate
}

func smartRecruitersJobID(segment string) string {
	head, _, _ := strings.Cut(segment, "-")
	if head != "" && isDigits(head) {
		return head
	}
	if isDigits(segment) {
		return segment
	}
	return ""
}

func trailingDigits(segment string) string {
	end := len(segment)
	for end > 0 && isDigit(segment[end-1]) {
		end--
	}
	return segment[end:]
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isDigit(r byte) bool {
	return r >= '0' && r <= '9'
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
