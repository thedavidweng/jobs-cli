// Package market resolves the Indeed market (country and locale) a search runs
// in. Indeed serves each country from its own index, and a search sent to the
// wrong market returns plausible jobs from the wrong place instead of failing
// ("Vancouver" in the US market returns jobs around Portland, OR), so
// resolution never falls back to a default country.
package market

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thedavidweng/jobs-cli/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

// Inputs are the market settings visible to one search.
type Inputs struct {
	Country        string // --country
	Locale         string // --locale
	Location       string // --location
	EnvCountry     string // JOBS_COUNTRY
	EnvLocale      string // JOBS_LOCALE
	ProfileCountry string
	ProfileLocale  string
}

// Resolve picks the country from --country, then --location inference, then
// JOBS_COUNTRY, then the profile, and fails with MARKET_REQUIRED when none
// applies. The locale is --locale, else the first configured locale
// (environment, then profile) whose region is that country, else en-<country>.
func Resolve(in *Inputs) (*domain.Market, *joberrors.Error) {
	country, origin, err := resolveCountry(in)
	if err != nil {
		return nil, err
	}
	locale, err := resolveLocale(in, country)
	if err != nil {
		return nil, err
	}
	return &domain.Market{Country: country, Locale: locale, Origin: origin}, nil
}

func resolveCountry(in *Inputs) (string, domain.MarketOrigin, *joberrors.Error) {
	if value := strings.TrimSpace(in.Country); value != "" {
		return configuredCountry(value, "--country", domain.MarketFromFlag)
	}
	if country, ok := FromLocation(in.Location); ok {
		return country, domain.MarketFromLocation, nil
	}
	if value := strings.TrimSpace(in.EnvCountry); value != "" {
		return configuredCountry(value, "JOBS_COUNTRY", domain.MarketFromEnv)
	}
	if value := strings.TrimSpace(in.ProfileCountry); value != "" {
		return configuredCountry(value, "profile country", domain.MarketFromProfile)
	}
	return "", "", required(in.Location)
}

func configuredCountry(value, label string, origin domain.MarketOrigin) (string, domain.MarketOrigin, *joberrors.Error) {
	country, ok := Lookup(value)
	if !ok {
		return "", "", invalid(fmt.Sprintf("%s %q is not an Indeed market; use one of: %s", label, value, strings.Join(Countries(), ", ")))
	}
	return country, origin, nil
}

func required(location string) *joberrors.Error {
	message := "indeed search needs a market"
	if strings.TrimSpace(location) != "" {
		message += fmt.Sprintf(", and --location %q does not name one", location)
	}
	message += `; pass --country (for example US, CA, or GB), end --location with a US state, Canadian province, or country ("Austin, TX", "Toronto, ON", "London, United Kingdom"), or set JOBS_COUNTRY or the profile country`
	return joberrors.New(joberrors.MarketRequired, message, joberrors.CatValidation, false, nil)
}

func resolveLocale(in *Inputs, country string) (string, *joberrors.Error) {
	if value := strings.TrimSpace(in.Locale); value != "" {
		locale, ok := parseLocale(value)
		if !ok {
			return "", invalid(fmt.Sprintf("--locale %q is not a language-REGION tag such as en-CA or fr-CA", value))
		}
		return locale, nil
	}
	for _, value := range []string{in.EnvLocale, in.ProfileLocale} {
		if locale, ok := parseLocale(value); ok && strings.HasSuffix(locale, "-"+country) {
			return locale, nil
		}
	}
	return "en-" + country, nil
}

func parseLocale(value string) (string, bool) {
	language, region, ok := strings.Cut(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"), "-")
	if !ok || !asciiLetters(language, 2, 3) || !asciiLetters(region, 2, 2) {
		return "", false
	}
	return strings.ToLower(language) + "-" + strings.ToUpper(region), true
}

func asciiLetters(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

// FromLocation infers a market from the last comma-separated part of a
// location: a US state or Canadian province (code or name) or a country name.
// Two-letter parts other than US and UK are read only as state or province
// codes, so "San Francisco, CA" is California and "Paris, FR" infers nothing.
// Bare cities infer nothing; there is no city table.
func FromLocation(location string) (string, bool) {
	parts := strings.Split(location, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		suffix := normalize(parts[i])
		if suffix == "" {
			continue
		}
		country, ok := locationSuffixes[suffix]
		return country, ok
	}
	return "", false
}

func normalize(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(s, ".", ""))), " ")
}

// Lookup returns the canonical code of an Indeed market, accepting any letter
// case and UK for GB.
func Lookup(value string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(value))
	if code == "UK" {
		code = "GB"
	}
	if _, ok := countryNames[code]; !ok {
		return "", false
	}
	return code, true
}

// Host returns the Indeed site host for a market, such as ca.indeed.com.
func Host(country string) (string, bool) {
	code, ok := Lookup(country)
	if !ok {
		return "", false
	}
	if subdomain, ok := subdomains[code]; ok {
		return subdomain + ".indeed.com", true
	}
	return strings.ToLower(code) + ".indeed.com", true
}

// Countries returns every Indeed market code in sorted order.
func Countries() []string {
	codes := make([]string, 0, len(countryNames))
	for code := range countryNames {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func invalid(message string) *joberrors.Error {
	return joberrors.New(joberrors.InvalidArguments, message, joberrors.CatValidation, false, nil)
}
