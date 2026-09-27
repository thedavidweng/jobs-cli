package market

import "testing"

func TestLocationSuffixesDoNotCollide(t *testing.T) {
	owner := map[string]string{}
	claim := func(name, country string) {
		if previous, ok := owner[name]; ok && previous != country {
			t.Errorf("location suffix %q maps to both %s and %s", name, previous, country)
		}
		if name != normalize(name) {
			t.Errorf("location suffix %q is not normalized", name)
		}
		owner[name] = country
	}
	for _, name := range usStates {
		claim(name, "US")
	}
	for _, name := range caProvinces {
		claim(name, "CA")
	}
	for code, names := range countryNames {
		for _, name := range names {
			claim(name, code)
		}
	}
	for code := range subdomains {
		if _, ok := countryNames[code]; !ok {
			t.Errorf("subdomain override for %s, which is not an Indeed market", code)
		}
	}
}
