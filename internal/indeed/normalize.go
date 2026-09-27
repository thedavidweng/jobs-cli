package indeed

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
	"github.com/thedavidweng/jobs-cli/v2/internal/market"
)

func normalizeJob(node *jobNode, marketCountry string) (*domain.Job, *joberrors.Error) {
	if node == nil {
		return nil, schemaDrift("job node is missing")
	}
	if node.Key == "" {
		return nil, schemaDrift("job node has no key")
	}
	if node.Title == "" {
		return nil, schemaDrift(fmt.Sprintf("job %s has no title", node.Key))
	}
	job := domain.NewJob(domain.SourceIndeed, node.Key)
	job.Title = node.Title
	job.Employer = employerName(node.Employer)
	job.Location = locationLabel(node.Location)
	job.Workplace, job.Remote = workplace(node.Attributes)
	job.PostedDate = postedDate(node.DatePublished)
	job.Compensation = compensationFor(node.Compensation, node.Recruit)
	job.SourceURL = viewJobURL(node, marketCountry)
	job.ApplicationURL = recruitViewURL(node.Recruit)
	job.Description = descriptionText(node.Description)
	raw, err := json.Marshal(node)
	if err != nil {
		return nil, internalError("encode indeed job diagnostics", err)
	}
	job.Diagnostics = &domain.Diagnostics{SourcePayload: raw}
	return &job, nil
}

// viewJobURL links a job on its own country's Indeed site, falling back to the
// market that returned it when the job carries no Indeed market country.
func viewJobURL(node *jobNode, marketCountry string) string {
	country := ""
	if node.Location != nil && node.Location.CountryCode != nil {
		country = *node.Location.CountryCode
	}
	host, ok := market.Host(country)
	if !ok {
		host, _ = market.Host(marketCountry)
	}
	return "https://" + host + "/viewjob?jk=" + node.Key
}

func employerName(node *employerNode) string {
	if node == nil || node.Name == nil {
		return ""
	}
	return *node.Name
}

func locationLabel(node *locationNode) string {
	if node == nil || node.Formatted == nil {
		return ""
	}
	if node.Formatted.Short != nil && *node.Formatted.Short != "" {
		return *node.Formatted.Short
	}
	if node.Formatted.Long != nil {
		return *node.Formatted.Long
	}
	return ""
}

func workplace(attrs []attributeNode) (domain.Workplace, bool) {
	for _, attr := range attrs {
		label := strings.ToLower(strings.TrimSpace(attr.Label))
		switch {
		case strings.Contains(label, "remote"):
			return domain.WorkplaceRemote, true
		case strings.Contains(label, "hybrid"):
			return domain.WorkplaceHybrid, false
		case strings.Contains(label, "on-site"), strings.Contains(label, "onsite"):
			return domain.WorkplaceOnsite, false
		}
	}
	return domain.WorkplaceUnknown, false
}

func postedDate(published *int64) string {
	if published == nil || *published <= 0 {
		return ""
	}
	return time.UnixMilli(*published).UTC().Format(time.RFC3339)
}

func compensationFor(node *compensationNode, recruit *recruitNode) *domain.Compensation {
	if node == nil {
		return nil
	}
	salary, currency := node.baseSalary()
	if salary == nil {
		return nil
	}
	out := &domain.Compensation{Interval: intervalFor(salary.UnitOfWork)}
	if currency != nil && *currency != "" {
		out.Currency = *currency
	}
	out.Amounts = amountsFor(salary.Range)
	if recruit != nil && recruit.DetailedSalary != nil {
		out.Summary = *recruit.DetailedSalary
	}
	return out
}

func (n *compensationNode) baseSalary() (salary *salaryNode, currency *string) {
	if n.BaseSalary != nil {
		return n.BaseSalary, n.CurrencyCode
	}
	if n.Estimated != nil {
		return n.Estimated.BaseSalary, n.Estimated.CurrencyCode
	}
	return nil, nil
}

func amountsFor(rng *salaryRangeNode) []domain.CompensationAmount {
	if rng == nil {
		return nil
	}
	switch {
	case rng.Min != nil && rng.Max != nil:
		return []domain.CompensationAmount{{Kind: "range", Min: rng.Min, Max: rng.Max}}
	case rng.Value != nil:
		exact := *rng.Value
		return []domain.CompensationAmount{{Kind: "exact", Min: &exact, Max: &exact}}
	case rng.Min != nil:
		return []domain.CompensationAmount{{Kind: "min", Min: rng.Min}}
	case rng.Max != nil:
		return []domain.CompensationAmount{{Kind: "max", Max: rng.Max}}
	default:
		return nil
	}
}

func intervalFor(unit *string) domain.CompensationInterval {
	if unit == nil {
		return domain.IntervalUnknown
	}
	switch strings.ToUpper(strings.TrimSpace(*unit)) {
	case "YEAR":
		return domain.IntervalYear
	case "MONTH":
		return domain.IntervalMonth
	case "WEEK":
		return domain.IntervalWeek
	case "DAY":
		return domain.IntervalDay
	case "HOUR":
		return domain.IntervalHour
	default:
		return domain.IntervalUnknown
	}
}

func recruitViewURL(node *recruitNode) string {
	if node == nil || node.ViewJobURL == nil {
		return ""
	}
	return *node.ViewJobURL
}

func descriptionText(node *descriptionNode) string {
	if node == nil || node.HTML == nil {
		return ""
	}
	return *node.HTML
}
