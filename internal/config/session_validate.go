package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const csrfTokenPrefix = "ajax:"

// Surrounding ASCII and Unicode quotation marks found around pasted cookie
// values. Only these outer characters are ever removed; interior characters are
// never rewritten.
const (
	cookieQuotes      = "\"'‘’“”"
	smartCookieQuotes = "‘’“”"
	csrfForbidden     = cookieQuotes + " \t\r\n"
)

// ValidateLinkedInSession is the single shared validator for LinkedIn
// sessions. The guided browser import, the manual `auth linkedin import
// --from-json -` command, and SessionStore.Load/Status all call it, so every
// stored session meets the same rules.
func ValidateLinkedInSession(session *LinkedInSession, profile string) error {
	if session == nil {
		return errors.New("session is missing")
	}
	if session.SchemaVersion != SessionSchemaVersion {
		return fmt.Errorf("schema_version %q is not supported (want %q)", session.SchemaVersion, SessionSchemaVersion)
	}
	if session.Provider != ProviderLinkedIn {
		return fmt.Errorf("provider %q is not %q", session.Provider, ProviderLinkedIn)
	}
	if session.Profile != profile {
		return fmt.Errorf("profile %q does not match the active profile %q", session.Profile, profile)
	}
	if _, err := time.Parse(time.RFC3339, session.CapturedAt); err != nil {
		return fmt.Errorf("captured_at %q is not a valid RFC3339 timestamp", session.CapturedAt)
	}
	if session.Cookie(CookieLiAt) == "" {
		return fmt.Errorf("cookies.%s is missing", CookieLiAt)
	}
	if session.Cookie(CookieJSessionID) == "" {
		return fmt.Errorf("cookies.%s is missing", CookieJSessionID)
	}
	if !wellFormedCSRFToken(session.CSRFTokenValue()) {
		if session.CSRFToken != "" {
			return errors.New(`csrf_token is not a well-formed "ajax:" token`)
		}
		return errors.New(`JSESSIONID is not a well-formed "ajax:" csrf token after removing surrounding quotes`)
	}
	return nil
}

func wellFormedCSRFToken(token string) bool {
	rest, ok := strings.CutPrefix(token, csrfTokenPrefix)
	if !ok || rest == "" {
		return false
	}
	return !strings.ContainsAny(rest, csrfForbidden)
}

// TrimCookieQuotes removes surrounding ASCII and Unicode quotation marks from a
// stored cookie value, leaving interior characters untouched.
func TrimCookieQuotes(value string) string {
	return strings.Trim(value, cookieQuotes)
}

// TrimSmartCookieQuotes removes only surrounding Unicode smart quotes, the
// artifact left by pasting cookie values through rich-text editors.
func TrimSmartCookieQuotes(value string) string {
	return strings.Trim(value, smartCookieQuotes)
}
