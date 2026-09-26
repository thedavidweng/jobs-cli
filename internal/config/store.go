package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Store struct {
	Path string
}

func NewStore(path string) *Store {
	if path == "" {
		path = DefaultConfigPath()
	}
	return &Store{Path: path}
}

func (s *Store) readRaw() (map[string]any, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	raw := map[string]any{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Store) writeRaw(raw map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		return err
	}
	return os.Chmod(s.Path, 0o600)
}

func profilesMap(raw map[string]any) map[string]any {
	p, ok := raw["profiles"].(map[string]any)
	if !ok {
		p = map[string]any{}
		raw["profiles"] = p
	}
	return p
}

func (s *Store) SetProfileField(profile, key string, value any) error {
	raw, err := s.readRaw()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	profiles := profilesMap(raw)
	p, ok := profiles[profile].(map[string]any)
	if !ok {
		p = map[string]any{}
		profiles[profile] = p
	}
	p[key] = value
	return s.writeRaw(raw)
}

func (s *Store) SetDefaultProfile(name string) error {
	raw, err := s.readRaw()
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	raw["default_profile"] = name
	return s.writeRaw(raw)
}
