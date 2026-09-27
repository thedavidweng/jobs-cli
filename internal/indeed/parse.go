package indeed

import (
	"encoding/json"
	"fmt"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
	joberrors "github.com/thedavidweng/jobs-cli/v2/internal/errors"
)

func parseSearchResponse(body []byte, marketCountry string) ([]domain.Job, string, *joberrors.Error) {
	var envelope graphQLResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, "", schemaDrift("response is not valid JSON: " + err.Error())
	}
	if failure := graphQLFailure(envelope.Errors); failure != nil {
		return nil, "", failure
	}
	var payload searchPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return nil, "", schemaDrift("jobSearch payload could not be decoded: " + err.Error())
	}
	if payload.JobSearch == nil {
		return nil, "", schemaDrift("response has no jobSearch connection")
	}
	if payload.JobSearch.PageInfo == nil {
		return nil, "", schemaDrift("jobSearch has no pageInfo object")
	}
	jobs, err := jobsFromResults(payload.JobSearch.Results, marketCountry)
	if err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if payload.JobSearch.PageInfo.NextCursor != nil {
		nextCursor = *payload.JobSearch.PageInfo.NextCursor
	}
	return jobs, nextCursor, nil
}

func jobsFromResults(results []searchEdge, marketCountry string) ([]domain.Job, *joberrors.Error) {
	if results == nil {
		return nil, schemaDrift("jobSearch has no results array")
	}
	jobs := make([]domain.Job, 0, len(results))
	for index, edge := range results {
		if edge.Job == nil {
			return nil, schemaDrift(fmt.Sprintf("jobSearch result %d has no job node", index))
		}
		job, err := normalizeJob(edge.Job, marketCountry)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, nil
}

func parseDetailResponse(body []byte, key, marketCountry string) (*domain.Job, *joberrors.Error) {
	var envelope graphQLResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, schemaDrift("response is not valid JSON: " + err.Error())
	}
	if failure := graphQLFailure(envelope.Errors); failure != nil {
		return nil, failure
	}
	var payload detailPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return nil, schemaDrift("jobData payload could not be decoded: " + err.Error())
	}
	if payload.JobData == nil {
		return nil, schemaDrift("response has no jobData connection")
	}
	if payload.JobData.Results == nil {
		return nil, schemaDrift("jobData has no results array")
	}
	for index, edge := range payload.JobData.Results {
		if edge.Job == nil {
			return nil, schemaDrift(fmt.Sprintf("jobData result %d has no job node", index))
		}
		if edge.Job.Key != key {
			continue
		}
		return normalizeJob(edge.Job, marketCountry)
	}
	return nil, joberrors.New(
		joberrors.ResourceNotFound,
		"job "+string(domain.SourceIndeed)+":"+key+" was not found on indeed",
		joberrors.CatAPI,
		false,
		nil,
	)
}

func graphQLFailure(errs []graphQLError) *joberrors.Error {
	if len(errs) == 0 {
		return nil
	}
	message := errs[0].Message
	if message == "" {
		message = "unknown GraphQL error"
	}
	return joberrors.New(joberrors.APIError, "indeed GraphQL error: "+message, joberrors.CatAPI, false, nil)
}
