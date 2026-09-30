package domain

import "encoding/json"

type Workplace string

const (
	WorkplaceRemote  Workplace = "remote"
	WorkplaceHybrid  Workplace = "hybrid"
	WorkplaceOnsite  Workplace = "onsite"
	WorkplaceUnknown Workplace = "unknown"
)

type CompensationInterval string

const (
	IntervalYear    CompensationInterval = "year"
	IntervalMonth   CompensationInterval = "month"
	IntervalWeek    CompensationInterval = "week"
	IntervalDay     CompensationInterval = "day"
	IntervalHour    CompensationInterval = "hour"
	IntervalUnknown CompensationInterval = "unknown"
)

type Compensation struct {
	Amounts  []CompensationAmount `json:"amounts,omitempty"`
	Currency string               `json:"currency,omitempty"`
	Interval CompensationInterval `json:"interval,omitempty"`
	Summary  string               `json:"summary,omitempty"`
}

type CompensationAmount struct {
	Kind string   `json:"kind,omitempty"`
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
}

type Job struct {
	ID               string             `json:"id"`
	Source           Source             `json:"source"`
	SourceJobID      string             `json:"source_job_id"`
	Title            string             `json:"title"`
	Employer         string             `json:"employer,omitempty"`
	Location         string             `json:"location,omitempty"`
	Workplace        Workplace          `json:"workplace"`
	Remote           bool               `json:"remote"`
	Description      string             `json:"description,omitempty"`
	ClosingDate      string             `json:"closing_date,omitempty"`
	EmploymentType   string             `json:"employment_type,omitempty"`
	EmploymentLength string             `json:"employment_length,omitempty"`
	Level            string             `json:"level,omitempty"`
	Tags             []string           `json:"tags,omitempty"`
	PostedDate       string             `json:"posted_date,omitempty"`
	Compensation     *Compensation      `json:"compensation,omitempty"`
	SourceURL        string             `json:"source_url,omitempty"`
	ApplicationURL   string             `json:"application_url,omitempty"`
	Application      *ApplicationTarget `json:"application,omitempty"`
	Diagnostics      *Diagnostics       `json:"diagnostics,omitempty"`
}

type Diagnostics struct {
	SourcePayload json.RawMessage `json:"source_payload,omitempty"`
}

func NewJob(source Source, sourceJobID string) Job {
	return Job{
		ID:          FormatJobID(source, sourceJobID),
		Source:      source,
		SourceJobID: sourceJobID,
		Workplace:   WorkplaceUnknown,
	}
}

func (j *Job) CompoundID() string {
	return FormatJobID(j.Source, j.SourceJobID)
}
