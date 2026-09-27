package indeed

import (
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/domain"
)

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

// detailMarket supplies the market headers for detail lookups. jobData is keyed
// by job key and returns the same job under any market (live-verified), so
// detail sends the pinned bundle's verified US/en-US values instead of asking
// for a market.
var detailMarket = domain.Market{Country: "US", Locale: "en-US"}

func acceptLanguage(locale string) string {
	language, _, _ := strings.Cut(locale, "-")
	return locale + "," + language + ";q=0.9"
}
