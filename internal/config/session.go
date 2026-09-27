package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	SessionSchemaVersion = "1"

	CookieLiAt       = "li_at"
	CookieJSessionID = "JSESSIONID"
	CookieLiAtCSRF   = "li_atcsrf"
)

var ErrSessionNotFound = errors.New("no LinkedIn session stored")

type LinkedInSession struct {
	SchemaVersion string            `json:"schema_version"`
	Provider      string            `json:"provider"`
	Profile       string            `json:"profile"`
	CapturedAt    string            `json:"captured_at"`
	Browser       string            `json:"browser,omitempty"`
	CSRFToken     string            `json:"csrf_token,omitempty"`
	Cookies       map[string]string `json:"cookies"`
}

func NewLinkedInSession(profile, browser string, cookies map[string]string) *LinkedInSession {
	return &LinkedInSession{
		SchemaVersion: SessionSchemaVersion,
		Provider:      ProviderLinkedIn,
		Profile:       profile,
		CapturedAt:    time.Now().UTC().Format(time.RFC3339),
		Browser:       browser,
		Cookies:       cookies,
	}
}

func (s *LinkedInSession) Cookie(name string) string {
	if s == nil || s.Cookies == nil {
		return ""
	}
	return s.Cookies[name]
}

func (s *LinkedInSession) CSRFTokenValue() string {
	if s == nil {
		return ""
	}
	if s.CSRFToken != "" {
		return s.CSRFToken
	}
	return TrimCookieQuotes(s.Cookie(CookieJSessionID))
}

func (s *LinkedInSession) Complete() bool {
	return s.Cookie(CookieLiAt) != "" && s.Cookie(CookieJSessionID) != ""
}

func (s *LinkedInSession) CookieNames() []string {
	if s == nil {
		return nil
	}
	names := make([]string, 0, len(s.Cookies))
	for name := range s.Cookies {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type SessionStatus struct {
	Present       bool     `json:"present"`
	Complete      bool     `json:"complete"`
	Invalid       bool     `json:"invalid"`
	InvalidReason string   `json:"invalid_reason,omitempty"`
	Path          string   `json:"path"`
	CapturedAt    string   `json:"captured_at,omitempty"`
	Browser       string   `json:"browser,omitempty"`
	CookieNames   []string `json:"cookie_names,omitempty"`
}

type SessionStore struct {
	baseDir   string
	profile   string
	overrides map[string]string
}

func NewSessionStore(configDir string) *SessionStore {
	if configDir == "" {
		configDir = DefaultDir()
	}
	return &SessionStore{baseDir: configDir, profile: "default", overrides: map[string]string{}}
}

func (s *SessionStore) WithProfile(profile string) *SessionStore {
	if profile == "" {
		profile = "default"
	}
	return &SessionStore{baseDir: s.baseDir, profile: profile, overrides: s.overrides}
}

func (s *SessionStore) SetOverride(profile, path string) {
	if path == "" {
		return
	}
	if s.overrides == nil {
		s.overrides = map[string]string{}
	}
	s.overrides[profile] = path
}

func (s *SessionStore) ConfigDir() string {
	return s.baseDir
}

func (s *SessionStore) Dir() string {
	return filepath.Join(s.baseDir, "sessions")
}

func (s *SessionStore) Profile() string {
	return s.profile
}

func (s *SessionStore) Path() string {
	if path, ok := s.overrides[s.profile]; ok && path != "" {
		return path
	}
	return filepath.Join(s.Dir(), s.profile+".json")
}

func (s *SessionStore) Load() (*LinkedInSession, error) {
	data, err := os.ReadFile(s.Path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	session := &LinkedInSession{}
	if err := json.Unmarshal(data, session); err != nil {
		return nil, fmt.Errorf("session file is not valid JSON: %w", err)
	}
	if err := ValidateLinkedInSession(session, s.profile); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *SessionStore) Save(session *LinkedInSession) error {
	if session == nil {
		return errors.New("nil session")
	}
	if session.SchemaVersion == "" {
		session.SchemaVersion = SessionSchemaVersion
	}
	if session.Provider == "" {
		session.Provider = ProviderLinkedIn
	}
	if session.Profile == "" {
		session.Profile = s.profile
	}
	if session.CapturedAt == "" {
		session.CapturedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir(), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(s.Dir(), 0o700); err != nil {
		return err
	}
	target := s.Path()
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

func (s *SessionStore) Remove() (bool, error) {
	err := os.Remove(s.Path())
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *SessionStore) Status() SessionStatus {
	status := SessionStatus{Path: s.Path()}
	if _, err := os.Stat(s.Path()); err != nil {
		if !os.IsNotExist(err) {
			status.Present = true
			status.Invalid = true
			status.InvalidReason = err.Error()
		}
		return status
	}
	status.Present = true
	session, err := s.Load()
	if err != nil {
		status.Invalid = true
		status.InvalidReason = err.Error()
		return status
	}
	status.Complete = session.Complete()
	status.CapturedAt = session.CapturedAt
	status.Browser = session.Browser
	status.CookieNames = session.CookieNames()
	return status
}
