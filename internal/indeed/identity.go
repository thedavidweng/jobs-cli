package indeed

import "strings"

// Mobile client identity bundle, taken verbatim from the pinned upstream
// reference:
// https://github.com/speedyapply/JobSpy/blob/4ec308a302e35b2a765a6bb73cee659c4011ff91/jobspy/indeed/constant.py
// These values were live-verified against the Indeed mobile GraphQL service in
// September 2026; replacing any of them triggers Cloudflare anti-bot challenges.
// The remote filter key comes from jobspy/indeed/__init__.py at the same commit.
const (
	graphQLEndpoint = "https://apis.indeed.com/graphql"
	apiKey          = "161092c2017b5bbab13edb12461a62d5a833871e7cad6d9d475304573de67ac8"
	userAgent       = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Indeed App 193.1"
	appInfo         = "appv=193.1; appid=com.indeed.jobsearch; osv=16.6.1; os=ios; dtype=phone"
	remoteFilterKey = "DSQF7"
)

// Market parameters are profile-driven (country/locale; default US/en-US, the
// values live-verified for the pinned bundle above; the September 2026
// Vancouver verification used indeed-locale: en-CA with indeed-co: CA).
type market struct {
	country string
	locale  string
}

func marketFor(country, locale string) market {
	m := market{country: strings.ToUpper(strings.TrimSpace(country)), locale: strings.TrimSpace(locale)}
	language, region, split := strings.Cut(m.locale, "-")
	if split && m.country == "" && region != "" {
		m.country = strings.ToUpper(region)
	}
	if !split {
		m.locale = ""
	}
	if m.country == "" {
		m.country = "US"
	}
	if m.locale == "" {
		language = strings.ToLower(strings.TrimSpace(language))
		if language == "" {
			language = "en"
		}
		m.locale = language + "-" + m.country
	}
	return m
}

func (m market) acceptLanguage() string {
	lang, _, _ := strings.Cut(m.locale, "-")
	return m.locale + "," + lang + ";q=0.9"
}

func (m market) host() string {
	switch m.country {
	case "US":
		return "www.indeed.com"
	case "GB":
		return "uk.indeed.com"
	default:
		return strings.ToLower(m.country) + ".indeed.com"
	}
}

func (m market) viewJobURL(key string) string {
	return "https://" + m.host() + "/viewjob?jk=" + key
}
