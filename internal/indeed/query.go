package indeed

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/thedavidweng/jobs-cli/v2/internal/domain"
)

const (
	searchLimitDefault = 25
	searchLimitMax     = 100
	radiusDefault      = 25
)

const searchQueryTemplate = `query GetJobData {
	jobSearch(%s) {
		pageInfo {
			nextCursor
		}
		results {
			job {
				key
				title
				datePublished
				employer {
					name
				}
				location {
					countryCode
					formatted {
						short
						long
					}
				}
				compensation {
					estimated {
						currencyCode
						baseSalary {
							unitOfWork
							range {
								... on Range {
									min
									max
								}
								... on Exactly {
									value
								}
							}
						}
					}
					baseSalary {
						unitOfWork
						range {
							... on Range {
								min
								max
							}
							... on Exactly {
								value
							}
						}
					}
					currencyCode
				}
				recruit {
					viewJobUrl
					detailedSalary
				}
				attributes {
					key
					label
				}
			}
		}
	}
}`

const detailQueryTemplate = `query GetJobData {
	jobData(jobKeys: [%s]) {
		results {
			job {
				key
				title
				datePublished
				description {
					html
				}
				employer {
					name
					relativeCompanyPageUrl
					dossier {
						links {
							corporateWebsite
						}
					}
				}
				location {
					countryCode
					formatted {
						short
						long
					}
				}
				compensation {
					estimated {
						currencyCode
						baseSalary {
							unitOfWork
							range {
								... on Range {
									min
									max
								}
								... on Exactly {
									value
								}
							}
						}
					}
					baseSalary {
						unitOfWork
						range {
							... on Range {
								min
								max
							}
							... on Exactly {
								value
							}
						}
					}
					currencyCode
				}
				recruit {
					viewJobUrl
					detailedSalary
				}
				attributes {
					key
					label
				}
			}
		}
	}
}`

func searchQuery(req *domain.SearchRequest) string {
	args := make([]string, 0, 6)
	if what := strings.TrimSpace(req.Keywords); what != "" {
		args = append(args, "what: "+graphqlString(what))
	}
	if where := strings.TrimSpace(req.Location); where != "" {
		args = append(args, "location: {where: "+graphqlString(where)+", radius: "+strconv.Itoa(searchRadius(req))+", radiusUnit: MILES}")
	}
	args = append(args, "limit: "+strconv.Itoa(searchLimit(req)))
	if req.Cursor != "" {
		args = append(args, "cursor: "+graphqlString(req.Cursor))
	}
	sort, _ := sortArgument(req.Sort)
	args = append(args, "sort: "+sort)
	if filters := remoteFilter(req.Remote); filters != "" {
		args = append(args, "filters: "+filters)
	}
	return fmt.Sprintf(searchQueryTemplate, strings.Join(args, ", "))
}

func detailQuery(key string) string {
	return fmt.Sprintf(detailQueryTemplate, graphqlString(key))
}

func sortArgument(sort string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "", "relevance":
		return "RELEVANCE", true
	case "date":
		return "DATE", true
	default:
		return "", false
	}
}

func remoteFilter(remote *bool) string {
	if remote == nil || !*remote {
		return ""
	}
	return fmt.Sprintf(`{composite: {filters: [{keyword: {field: "attributes", keys: [%q]}}]}}`, remoteFilterKey)
}

func searchLimit(req *domain.SearchRequest) int {
	limit := req.Limit
	if limit <= 0 {
		limit = searchLimitDefault
	}
	if limit > searchLimitMax {
		limit = searchLimitMax
	}
	return limit
}

func searchRadius(req *domain.SearchRequest) int {
	if req.Radius > 0 {
		return req.Radius
	}
	return radiusDefault
}

func graphqlString(value string) string {
	return `"` + graphqlEscaper.Replace(value) + `"`
}

var graphqlEscaper = strings.NewReplacer(
	`\`, `\\`,
	`"`, `\"`,
	"\n", `\n`,
	"\r", `\r`,
	"\t", `\t`,
)
