package market

// countryNames lists every Indeed market by ISO 3166-1 alpha-2 code, which is
// also the indeed-co header value. The market list and names come from the
// Country enum of the pinned upstream reference:
// https://github.com/speedyapply/JobSpy/blob/4ec308a302e35b2a765a6bb73cee659c4011ff91/jobspy/model.py
// A few common English aliases (uae, england, ...) are added for --location
// inference.
var countryNames = map[string][]string{
	"AE": {"united arab emirates", "uae"},
	"AR": {"argentina"},
	"AT": {"austria"},
	"AU": {"australia"},
	"BE": {"belgium"},
	"BH": {"bahrain"},
	"BR": {"brazil"},
	"CA": {"canada"},
	"CH": {"switzerland"},
	"CL": {"chile"},
	"CN": {"china"},
	"CO": {"colombia"},
	"CR": {"costa rica"},
	"CZ": {"czech republic", "czechia"},
	"DE": {"germany"},
	"DK": {"denmark"},
	"EC": {"ecuador"},
	"EG": {"egypt"},
	"ES": {"spain"},
	"FI": {"finland"},
	"FR": {"france"},
	"GB": {"uk", "united kingdom", "great britain", "england", "scotland", "wales", "northern ireland"},
	"GR": {"greece"},
	"HK": {"hong kong"},
	"HU": {"hungary"},
	"ID": {"indonesia"},
	"IE": {"ireland"},
	"IL": {"israel"},
	"IN": {"india"},
	"IT": {"italy"},
	"JP": {"japan"},
	"KR": {"south korea"},
	"KW": {"kuwait"},
	"LU": {"luxembourg"},
	"MA": {"morocco"},
	"MT": {"malta"},
	"MX": {"mexico"},
	"MY": {"malaysia"},
	"NG": {"nigeria"},
	"NL": {"netherlands", "the netherlands"},
	"NO": {"norway"},
	"NZ": {"new zealand"},
	"OM": {"oman"},
	"PA": {"panama"},
	"PE": {"peru"},
	"PH": {"philippines"},
	"PK": {"pakistan"},
	"PL": {"poland"},
	"PT": {"portugal"},
	"QA": {"qatar"},
	"RO": {"romania"},
	"SA": {"saudi arabia"},
	"SE": {"sweden"},
	"SG": {"singapore"},
	"TH": {"thailand"},
	"TR": {"türkiye", "turkey"},
	"TW": {"taiwan"},
	"UA": {"ukraine"},
	"US": {"usa", "us", "united states", "united states of america"},
	"UY": {"uruguay"},
	"VE": {"venezuela"},
	"VN": {"vietnam"},
	"ZA": {"south africa"},
}

// subdomains holds the Indeed site subdomains that differ from the lowercase
// country code (indeed_domain_value in the pinned reference).
var subdomains = map[string]string{
	"GB": "uk",
	"MT": "malta",
	"MY": "malaysia",
	"US": "www",
}

// usStates and caProvinces are the codes and names of US states (plus DC) and
// Canadian provinces and territories.
var usStates = []string{
	"al", "alabama", "ak", "alaska", "az", "arizona", "ar", "arkansas",
	"ca", "california", "co", "colorado", "ct", "connecticut", "de", "delaware",
	"dc", "district of columbia", "fl", "florida", "ga", "georgia", "hi", "hawaii",
	"id", "idaho", "il", "illinois", "in", "indiana", "ia", "iowa",
	"ks", "kansas", "ky", "kentucky", "la", "louisiana", "me", "maine",
	"md", "maryland", "ma", "massachusetts", "mi", "michigan", "mn", "minnesota",
	"ms", "mississippi", "mo", "missouri", "mt", "montana", "ne", "nebraska",
	"nv", "nevada", "nh", "new hampshire", "nj", "new jersey", "nm", "new mexico",
	"ny", "new york", "nc", "north carolina", "nd", "north dakota", "oh", "ohio",
	"ok", "oklahoma", "or", "oregon", "pa", "pennsylvania", "ri", "rhode island",
	"sc", "south carolina", "sd", "south dakota", "tn", "tennessee", "tx", "texas",
	"ut", "utah", "vt", "vermont", "va", "virginia", "wa", "washington",
	"wv", "west virginia", "wi", "wisconsin", "wy", "wyoming",
}

var caProvinces = []string{
	"ab", "alberta", "bc", "british columbia", "mb", "manitoba",
	"nb", "new brunswick", "nl", "newfoundland and labrador", "newfoundland",
	"ns", "nova scotia", "nt", "northwest territories", "nu", "nunavut",
	"on", "ontario", "pe", "prince edward island", "qc", "quebec", "québec",
	"sk", "saskatchewan", "yt", "yukon",
}

// locationSuffixes maps every normalized location suffix the inference accepts
// to its market.
var locationSuffixes = buildLocationSuffixes()

func buildLocationSuffixes() map[string]string {
	out := map[string]string{}
	for _, name := range usStates {
		out[name] = "US"
	}
	for _, name := range caProvinces {
		out[name] = "CA"
	}
	for code, names := range countryNames {
		for _, name := range names {
			out[name] = code
		}
	}
	return out
}
