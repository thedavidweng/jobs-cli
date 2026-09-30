// Package resumejson validates JSON Resume against the vendored v1.0.0 schema.
package resumejson

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	"github.com/thedavidweng/jobs-cli/v2/internal/errors"
)

//go:embed schema.json
var schemaData []byte

func Parse(data []byte) (domain.Candidate, *errors.Error) {
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return domain.Candidate{}, invalid(err)
	}
	var schemaDoc any
	if err := json.Unmarshal(schemaData, &schemaDoc); err != nil {
		return domain.Candidate{}, invalid(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource("https://jobs-cli.local/resume-v1.json", schemaDoc); err != nil {
		return domain.Candidate{}, invalid(err)
	}
	schema, err := compiler.Compile("https://jobs-cli.local/resume-v1.json")
	if err != nil {
		return domain.Candidate{}, invalid(err)
	}
	if err := schema.Validate(document); err != nil {
		return domain.Candidate{}, invalid(err)
	}
	var resume struct {
		Basics struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Phone    string `json:"phone"`
			URL      string `json:"url"`
			Location struct {
				Address    string `json:"address"`
				PostalCode string `json:"postalCode"`
				City       string `json:"city"`
				Country    string `json:"countryCode"`
				Region     string `json:"region"`
			} `json:"location"`
			Profiles []struct {
				Network string `json:"network"`
				URL     string `json:"url"`
			} `json:"profiles"`
		} `json:"basics"`
		Work         []map[string]any `json:"work"`
		Education    []map[string]any `json:"education"`
		Skills       []map[string]any `json:"skills"`
		Languages    []map[string]any `json:"languages"`
		Certificates []map[string]any `json:"certificates"`
	}
	if err := json.Unmarshal(data, &resume); err != nil {
		return domain.Candidate{}, invalid(err)
	}
	b := resume.Basics
	candidate := domain.Candidate{FullName: b.Name, Email: b.Email, Phone: b.Phone, Website: b.URL, Location: b.Location.City, Address: map[string]string{"address_line": b.Location.Address, "postal_code": b.Location.PostalCode, "country": b.Location.Country, "region": b.Location.Region}, Work: resume.Work, Education: resume.Education, Skills: resume.Skills, Languages: resume.Languages, Certificates: resume.Certificates}
	for _, profile := range b.Profiles {
		if strings.EqualFold(strings.TrimSpace(profile.Network), "LinkedIn") {
			candidate.LinkedIn = profile.URL
		}
	}
	return candidate, nil
}

func invalid(err error) *errors.Error {
	return errors.New(errors.ValidationFailed, "JSON Resume v1.0.0: "+err.Error(), errors.CatValidation, false, err)
}
