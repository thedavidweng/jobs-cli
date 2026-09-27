package market_test

import (
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/market"
)

func TestFromLocation(t *testing.T) {
	cases := []struct {
		location string
		want     string
	}{
		{"Vancouver, BC", "CA"},
		{"vancouver, bc", "CA"},
		{"Toronto, Ontario", "CA"},
		{"Montréal, Québec", "CA"},
		{"Remote, Canada", "CA"},
		{"Canada", "CA"},
		{"Vancouver, BC, Canada", "CA"},
		{"Vancouver, BC,", "CA"},
		{"Seattle, WA", "US"},
		{"Austin, Texas", "US"},
		{"Washington, D.C.", "US"},
		{"Austin, TX, USA", "US"},
		{"Boston, U.S.", "US"},
		{"New York", "US"},
		{"San Francisco, CA", "US"},
		{"Berlin, DE", "US"},
		{"London, UK", "GB"},
		{"London, United  Kingdom", "GB"},
		{"Manchester, England", "GB"},
		{"Berlin, Germany", "DE"},
		{"Istanbul, Türkiye", "TR"},
		{"Kuala Lumpur, Malaysia", "MY"},
		{"Singapore", "SG"},
		{"Dubai, UAE", "AE"},
		{"London", ""},
		{"Vancouver", ""},
		{"Paris, FR", ""},
		{"Remote", ""},
		{"Worldwide", ""},
		{"Sofia, Bulgaria", ""},
		{"Seattle, WA 98101", ""},
		{"", ""},
		{" , ", ""},
	}
	for _, tc := range cases {
		got, ok := market.FromLocation(tc.location)
		if got != tc.want || ok != (tc.want != "") {
			t.Errorf("FromLocation(%q) = %q, %v; want %q", tc.location, got, ok, tc.want)
		}
	}
}

func TestResolveCountryPrecedence(t *testing.T) {
	cases := []struct {
		name   string
		in     market.Inputs
		want   string
		origin domain.MarketOrigin
	}{
		{
			name:   "flag beats everything",
			in:     market.Inputs{Country: "us", Location: "Vancouver, BC", EnvCountry: "GB", ProfileCountry: "CA"},
			want:   "US",
			origin: domain.MarketFromFlag,
		},
		{
			name:   "location beats environment and profile",
			in:     market.Inputs{Location: "Seattle, WA", EnvCountry: "GB", ProfileCountry: "CA"},
			want:   "US",
			origin: domain.MarketFromLocation,
		},
		{
			name:   "environment beats profile",
			in:     market.Inputs{Location: "London", EnvCountry: "GB", ProfileCountry: "CA"},
			want:   "GB",
			origin: domain.MarketFromEnv,
		},
		{
			name:   "profile is the last resort",
			in:     market.Inputs{ProfileCountry: "ca"},
			want:   "CA",
			origin: domain.MarketFromProfile,
		},
		{
			name:   "UK is accepted for GB",
			in:     market.Inputs{Country: "UK"},
			want:   "GB",
			origin: domain.MarketFromFlag,
		},
		{
			name:   "a valid higher layer shadows an invalid lower one",
			in:     market.Inputs{Country: "CA", EnvCountry: "XX", ProfileCountry: "YY"},
			want:   "CA",
			origin: domain.MarketFromFlag,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := market.Resolve(&tc.in)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got.Country != tc.want || got.Origin != tc.origin {
				t.Fatalf("market = %+v, want %s from %s", got, tc.want, tc.origin)
			}
		})
	}
}

func TestResolveRequiresAMarket(t *testing.T) {
	for _, in := range []market.Inputs{{}, {Location: "London"}, {Location: "Remote", Locale: "en-GB"}} {
		got, err := market.Resolve(&in)
		if got != nil {
			t.Fatalf("Resolve(%+v) = %+v, want no market", in, got)
		}
		if err == nil || err.Code != joberrors.MarketRequired || err.Category != joberrors.CatValidation || err.Retryable {
			t.Fatalf("Resolve(%+v) error = %+v, want a non-retryable validation MARKET_REQUIRED", in, err)
		}
		if err.ExitCode() != 2 {
			t.Errorf("exit code = %d, want 2", err.ExitCode())
		}
		for _, want := range []string{"--country", "--location", "JOBS_COUNTRY"} {
			if !strings.Contains(err.Message, want) {
				t.Errorf("message %q does not mention %s", err.Message, want)
			}
		}
		if in.Location != "" && !strings.Contains(err.Message, `"`+in.Location+`"`) {
			t.Errorf("message %q does not quote the location", err.Message)
		}
	}
}

func TestResolveRejectsCountriesOutsideTheMarketList(t *testing.T) {
	cases := []struct {
		in    market.Inputs
		label string
	}{
		{market.Inputs{Country: "XX"}, "--country"},
		{market.Inputs{Country: "Canada"}, "--country"},
		{market.Inputs{EnvCountry: "BG"}, "JOBS_COUNTRY"},
		{market.Inputs{ProfileCountry: "usa"}, "profile country"},
	}
	for _, tc := range cases {
		_, err := market.Resolve(&tc.in)
		if err == nil || err.Code != joberrors.InvalidArguments {
			t.Fatalf("Resolve(%+v) error = %+v, want INVALID_ARGUMENTS", tc.in, err)
		}
		if !strings.HasPrefix(err.Message, tc.label+" ") || !strings.Contains(err.Message, "CA, CH") {
			t.Errorf("message = %q, want it to name %s and list the markets", err.Message, tc.label)
		}
	}
}

func TestResolveLocale(t *testing.T) {
	cases := []struct {
		name string
		in   market.Inputs
		want string
	}{
		{name: "defaults to English in the market", in: market.Inputs{Country: "CA"}, want: "en-CA"},
		{name: "flag wins even across regions", in: market.Inputs{Country: "US", Locale: "fr-CA", EnvLocale: "es-US"}, want: "fr-CA"},
		{name: "flag is normalized", in: market.Inputs{Country: "CA", Locale: "FR_ca"}, want: "fr-CA"},
		{name: "environment locale in the market", in: market.Inputs{Country: "CA", EnvLocale: "fr-CA", ProfileLocale: "en-CA"}, want: "fr-CA"},
		{name: "profile locale when the environment is elsewhere", in: market.Inputs{Country: "CA", EnvLocale: "en-GB", ProfileLocale: "fr-CA"}, want: "fr-CA"},
		{name: "configured locales elsewhere are ignored", in: market.Inputs{Location: "Seattle, WA", EnvLocale: "fr-CA", ProfileLocale: "en-GB"}, want: "en-US"},
		{name: "malformed configured locales are ignored", in: market.Inputs{Country: "CA", EnvLocale: "french", ProfileLocale: "fr"}, want: "en-CA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := market.Resolve(&tc.in)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got.Locale != tc.want {
				t.Fatalf("locale = %q, want %q", got.Locale, tc.want)
			}
		})
	}

	for _, locale := range []string{"fr", "french-CA", "en-CAN", "e1-CA", "en-CA-x"} {
		_, err := market.Resolve(&market.Inputs{Country: "CA", Locale: locale})
		if err == nil || err.Code != joberrors.InvalidArguments || !strings.Contains(err.Message, "--locale") {
			t.Errorf("--locale %q error = %+v, want INVALID_ARGUMENTS naming --locale", locale, err)
		}
	}
}

func TestLookupAndHost(t *testing.T) {
	hosts := map[string]string{
		"US": "www.indeed.com",
		"GB": "uk.indeed.com",
		"uk": "uk.indeed.com",
		"CA": "ca.indeed.com",
		"MY": "malaysia.indeed.com",
		"MT": "malta.indeed.com",
		"de": "de.indeed.com",
	}
	for country, want := range hosts {
		if got, ok := market.Host(country); !ok || got != want {
			t.Errorf("Host(%q) = %q, %v; want %q", country, got, ok, want)
		}
	}
	for _, country := range []string{"", "XX", "BG", "PR", "USA"} {
		if got, ok := market.Host(country); ok {
			t.Errorf("Host(%q) = %q, want no Indeed market", country, got)
		}
	}
	if code, ok := market.Lookup(" ca "); !ok || code != "CA" {
		t.Errorf("Lookup(ca) = %q, %v", code, ok)
	}
}

func TestCountriesAreTheSortedMarketCodes(t *testing.T) {
	codes := market.Countries()
	if len(codes) != 63 {
		t.Fatalf("markets = %d, want the 63 of the pinned reference", len(codes))
	}
	for i, code := range codes {
		if len(code) != 2 || strings.ToUpper(code) != code {
			t.Errorf("code %q is not an uppercase ISO 3166-1 alpha-2 code", code)
		}
		if i > 0 && codes[i-1] >= code {
			t.Errorf("codes are not sorted: %q before %q", codes[i-1], code)
		}
	}
}
