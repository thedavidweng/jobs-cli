package indeed

import "encoding/json"

type graphQLRequest struct {
	Query string `json:"query"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type searchPayload struct {
	JobSearch *searchConnection `json:"jobSearch"`
}

type searchConnection struct {
	PageInfo *pageInfo    `json:"pageInfo"`
	Results  []searchEdge `json:"results"`
}

type pageInfo struct {
	NextCursor *string `json:"nextCursor"`
}

type searchEdge struct {
	Job *jobNode `json:"job"`
}

type detailPayload struct {
	JobData *detailConnection `json:"jobData"`
}

type detailConnection struct {
	Results []searchEdge `json:"results"`
}

type jobNode struct {
	Key           string            `json:"key"`
	Title         string            `json:"title"`
	DatePublished *int64            `json:"datePublished"`
	Description   *descriptionNode  `json:"description"`
	Location      *locationNode     `json:"location"`
	Compensation  *compensationNode `json:"compensation"`
	Attributes    []attributeNode   `json:"attributes"`
	Employer      *employerNode     `json:"employer"`
	Recruit       *recruitNode      `json:"recruit"`
}

type descriptionNode struct {
	HTML *string `json:"html"`
}

type locationNode struct {
	Formatted *formattedLocation `json:"formatted"`
}

type formattedLocation struct {
	Short *string `json:"short"`
	Long  *string `json:"long"`
}

type compensationNode struct {
	Estimated    *estimatedNode `json:"estimated"`
	BaseSalary   *salaryNode    `json:"baseSalary"`
	CurrencyCode *string        `json:"currencyCode"`
}

type estimatedNode struct {
	CurrencyCode *string     `json:"currencyCode"`
	BaseSalary   *salaryNode `json:"baseSalary"`
}

type salaryNode struct {
	UnitOfWork *string          `json:"unitOfWork"`
	Range      *salaryRangeNode `json:"range"`
}

type salaryRangeNode struct {
	Min   *float64 `json:"min"`
	Max   *float64 `json:"max"`
	Value *float64 `json:"value"`
}

type attributeNode struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type employerNode struct {
	Name                   *string      `json:"name"`
	RelativeCompanyPageURL *string      `json:"relativeCompanyPageUrl"`
	Dossier                *dossierNode `json:"dossier"`
}

type dossierNode struct {
	Links *dossierLinksNode `json:"links"`
}

type dossierLinksNode struct {
	CorporateWebsite *string `json:"corporateWebsite"`
}

type recruitNode struct {
	ViewJobURL     *string `json:"viewJobUrl"`
	DetailedSalary *string `json:"detailedSalary"`
}
