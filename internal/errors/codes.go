package errors

type Code string

const (
	AuthRequired         Code = "AUTH_REQUIRED"
	AuthTokenInvalid     Code = "AUTH_TOKEN_INVALID"
	APIAccessForbidden   Code = "API_ACCESS_FORBIDDEN"
	NetworkUnreachable   Code = "NETWORK_UNREACHABLE"
	NetworkTimeout       Code = "NETWORK_TIMEOUT"
	RateLimited          Code = "RATE_LIMITED"
	APIError             Code = "API_ERROR"
	APISchemaChanged     Code = "API_SCHEMA_CHANGED"
	ValidationFailed     Code = "VALIDATION_FAILED"
	ReadOnlyViolation    Code = "READ_ONLY_VIOLATION"
	ConfirmationRequired Code = "CONFIRMATION_REQUIRED"
	ResourceNotFound     Code = "RESOURCE_NOT_FOUND"
	InternalError        Code = "INTERNAL_ERROR"
	InvalidArguments     Code = "INVALID_ARGUMENTS"
	NotImplemented       Code = "NOT_IMPLEMENTED"

	SourceUnavailable           Code = "SOURCE_UNAVAILABLE"
	ATSResolutionFailed         Code = "ATS_RESOLUTION_FAILED"
	NativeApplyUnsupported      Code = "NATIVE_APPLY_UNSUPPORTED"
	BrowserRequired             Code = "BROWSER_REQUIRED"
	ApplicationIncomplete       Code = "APPLICATION_INCOMPLETE"
	ArtifactStale               Code = "ARTIFACT_STALE"
	LinkedInSessionRequired     Code = "LINKEDIN_SESSION_REQUIRED"
	LinkedInEasyApplyUnverified Code = "LINKEDIN_EASY_APPLY_UNVERIFIED"
)

type Category string

const (
	CatAuth       Category = "auth"
	CatNetwork    Category = "network"
	CatAPI        Category = "api"
	CatValidation Category = "validation"
	CatSafety     Category = "safety"
	CatInternal   Category = "internal"
)

var exitCodes = map[Code]int{
	AuthRequired:                3,
	AuthTokenInvalid:            3,
	LinkedInSessionRequired:     3,
	ReadOnlyViolation:           4,
	RateLimited:                 5,
	NetworkUnreachable:          5,
	NetworkTimeout:              5,
	SourceUnavailable:           5,
	APIError:                    6,
	APISchemaChanged:            6,
	APIAccessForbidden:          6,
	ResourceNotFound:            6,
	ATSResolutionFailed:         6,
	NativeApplyUnsupported:      6,
	BrowserRequired:             6,
	LinkedInEasyApplyUnverified: 6,
	ValidationFailed:            7,
	ApplicationIncomplete:       7,
	ArtifactStale:               7,
	ConfirmationRequired:        10,
	InvalidArguments:            2,
	NotImplemented:              1,
	InternalError:               1,
}

func ExitCodeFor(code Code) int {
	if code, ok := exitCodes[code]; ok {
		return code
	}
	return 1
}
