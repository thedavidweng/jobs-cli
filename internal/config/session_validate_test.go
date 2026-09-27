package config_test

import (
	"strings"
	"testing"

	"github.com/thedavidweng/jobs-cli/v2/internal/config"
)

func validatableSession() *config.LinkedInSession {
	session := config.NewLinkedInSession("default", "chrome", map[string]string{
		config.CookieLiAt:       "li-at-secret",
		config.CookieJSessionID: `"ajax:1234567890"`,
	})
	session.CSRFToken = "ajax:1234567890"
	return session
}

func TestValidateLinkedInSession(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*config.LinkedInSession)
		profile string
		wantErr string
	}{
		{
			name:    "valid with stored csrf token",
			mutate:  func(*config.LinkedInSession) {},
			wantErr: "",
		},
		{
			name: "valid deriving csrf from quoted jsessionid",
			mutate: func(s *config.LinkedInSession) {
				s.CSRFToken = ""
			},
			wantErr: "",
		},
		{
			name: "valid deriving csrf from smart-quoted jsessionid",
			mutate: func(s *config.LinkedInSession) {
				s.CSRFToken = ""
				s.Cookies[config.CookieJSessionID] = "“ajax:1234567890”"
			},
			wantErr: "",
		},
		{
			name: "stored csrf token takes precedence over jsessionid",
			mutate: func(s *config.LinkedInSession) {
				s.Cookies[config.CookieJSessionID] = `"not-a-csrf-token"`
			},
			wantErr: "",
		},
		{
			name: "wrong schema version",
			mutate: func(s *config.LinkedInSession) {
				s.SchemaVersion = "2"
			},
			wantErr: "schema_version",
		},
		{
			name: "wrong provider",
			mutate: func(s *config.LinkedInSession) {
				s.Provider = "greenhouse"
			},
			wantErr: "provider",
		},
		{
			name:    "profile mismatch",
			mutate:  func(*config.LinkedInSession) {},
			profile: "work",
			wantErr: "does not match the active profile",
		},
		{
			name: "missing li_at",
			mutate: func(s *config.LinkedInSession) {
				delete(s.Cookies, config.CookieLiAt)
			},
			wantErr: "cookies.li_at is missing",
		},
		{
			name: "missing jsessionid",
			mutate: func(s *config.LinkedInSession) {
				delete(s.Cookies, config.CookieJSessionID)
			},
			wantErr: "cookies.JSESSIONID is missing",
		},
		{
			name: "bad captured_at",
			mutate: func(s *config.LinkedInSession) {
				s.CapturedAt = "yesterday"
			},
			wantErr: "captured_at",
		},
		{
			name: "non-ajax csrf token",
			mutate: func(s *config.LinkedInSession) {
				s.CSRFToken = "not-an-ajax-token"
			},
			wantErr: `csrf_token is not a well-formed "ajax:" token`,
		},
		{
			name: "empty ajax csrf token",
			mutate: func(s *config.LinkedInSession) {
				s.CSRFToken = "ajax:"
			},
			wantErr: `csrf_token is not a well-formed "ajax:" token`,
		},
		{
			name: "jsessionid still malformed after quote normalization",
			mutate: func(s *config.LinkedInSession) {
				s.CSRFToken = ""
				s.Cookies[config.CookieJSessionID] = "“a“jax:1234567890”"
			},
			wantErr: "JSESSIONID",
		},
		{
			name: "jsessionid empty after quote normalization",
			mutate: func(s *config.LinkedInSession) {
				s.CSRFToken = ""
				s.Cookies[config.CookieJSessionID] = "“ajax:”"
			},
			wantErr: "JSESSIONID",
		},
		{
			name: "jsessionid with quote inside csrf remainder",
			mutate: func(s *config.LinkedInSession) {
				s.CSRFToken = ""
				s.Cookies[config.CookieJSessionID] = `"ajax:1234"5678"`
			},
			wantErr: "JSESSIONID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := validatableSession()
			tt.mutate(session)
			profile := tt.profile
			if profile == "" {
				profile = "default"
			}
			err := config.ValidateLinkedInSession(session, profile)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateLinkedInSession() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateLinkedInSession() = %v, want error containing %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), "li-at-secret") || strings.Contains(err.Error(), "ajax:1234567890") {
				t.Fatalf("validation error leaked session material: %v", err)
			}
		})
	}
}

func TestValidateLinkedInSessionNilSession(t *testing.T) {
	if err := config.ValidateLinkedInSession(nil, "default"); err == nil {
		t.Fatal("nil session must fail validation")
	}
}

func TestCSRFTokenValueTrimsSurroundingQuotes(t *testing.T) {
	tests := []struct {
		name      string
		csrfToken string
		jsession  string
		want      string
	}{
		{"stored token wins", "ajax:stored", `"ajax:other"`, "ajax:stored"},
		{"ascii quotes", "", "\"ajax:1234\"", "ajax:1234"},
		{"smart quotes", "", "“ajax:1234”", "ajax:1234"},
		{"single smart quotes", "", "‘ajax:1234’", "ajax:1234"},
		{"interior quotes kept", "", "\"ajax:12\"34\"", "ajax:12\"34"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := config.NewLinkedInSession("default", "chrome", map[string]string{
				config.CookieJSessionID: tt.jsession,
			})
			session.CSRFToken = tt.csrfToken
			if got := session.CSRFTokenValue(); got != tt.want {
				t.Fatalf("CSRFTokenValue() = %q, want %q", got, tt.want)
			}
		})
	}
}
