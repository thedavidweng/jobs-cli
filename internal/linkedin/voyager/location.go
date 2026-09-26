package voyager

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	joberrors "github.com/thedavidweng/jobs-cli/internal/errors"
)

// Geo URN resolution follows the donor yashiels/linkedin-cli resolveLocation:
// a raw urn:li: value passes through, built-in known names map to verified
// urn:li:fsd_geo IDs, and anything else fails with the supported values listed.
// Vancouver (103366113) is the geoId LinkedIn itself pairs with Vancouver,
// British Columbia, Canada; the South African IDs and the global remote geo ID
// (91000010) are the donor's live-tested values.
var knownGeoURNs = map[string]string{
	"remote":                              geoURN(91000010),
	"vancouver":                           geoURN(103366113),
	"vancouver, bc":                       geoURN(103366113),
	"vancouver, british columbia":         geoURN(103366113),
	"vancouver, british columbia, canada": geoURN(103366113),
	"south africa":                        geoURN(104035573),
	"cape town":                           geoURN(105013608),
	"johannesburg":                        geoURN(104273735),
	"joburg":                              geoURN(104273735),
	"pretoria":                            geoURN(105944906),
	"durban":                              geoURN(106463985),
}

func geoURN(id int64) string {
	return "urn:li:fsd_geo:" + strconv.FormatInt(id, 10)
}

func resolveLocation(location string) (string, *joberrors.Error) {
	value := strings.TrimSpace(location)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(strings.ToLower(value), "urn:li:") {
		return value, nil
	}
	if urn, ok := knownGeoURNs[strings.ToLower(value)]; ok {
		return urn, nil
	}
	names := make([]string, 0, len(knownGeoURNs))
	for name := range knownGeoURNs {
		names = append(names, name)
	}
	sort.Strings(names)
	return "", joberrors.New(
		joberrors.InvalidArguments,
		fmt.Sprintf("unknown LinkedIn location %q; pass a geo URN (urn:li:fsd_geo:<id>) or one of the known locations: %s", value, strings.Join(names, ", ")),
		joberrors.CatValidation,
		false,
		nil,
	)
}
