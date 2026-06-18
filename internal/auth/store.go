package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"time"
)

type OAuthCredential struct {
	Type         string    `json:"type"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	AccountID    string    `json:"account_id,omitempty"`
}

type Store struct {
	path string
}

func NewStore() *Store {
	return &Store{path: AuthPath()}
}

func NewStoreAt(path string) *Store {
	return &Store{path: path}
}

func AuthPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".keen-agent", "auth.json")
}

func (s *Store) Get(provider string) (OAuthCredential, bool, error) {
	data, err := s.load()
	if err != nil {
		return OAuthCredential{}, false, err
	}
	cred, ok := data[provider]
	return cred, ok, nil
}

func (s *Store) All() (map[string]OAuthCredential, error) {
	data, err := s.load()
	if err != nil {
		return nil, err
	}
	result := make(map[string]OAuthCredential, len(data))
	maps.Copy(result, data)
	return result, nil
}

func (s *Store) Set(provider string, cred OAuthCredential) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	if data == nil {
		data = make(map[string]OAuthCredential)
	}
	if cred.Type == "" {
		cred.Type = "oauth"
	}
	data[provider] = cred
	return s.save(data)
}

func (s *Store) Remove(provider string) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}
	delete(data, provider)
	return s.save(data)
}

func (s *Store) load() (map[string]OAuthCredential, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]OAuthCredential{}, nil
		}
		return nil, fmt.Errorf("read auth store: %w", err)
	}
	if len(data) == 0 {
		return map[string]OAuthCredential{}, nil
	}

	var result map[string]OAuthCredential
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse auth store: %w", err)
	}
	if result == nil {
		result = map[string]OAuthCredential{}
	}
	return result, nil
}

func (s *Store) save(data map[string]OAuthCredential) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create auth directory: %w", err)
	}
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal auth store: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".auth-*.json")
	if err != nil {
		return fmt.Errorf("create temporary auth store: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary auth store: %w", err)
	}
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure temporary auth store permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary auth store: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("write auth store: %w", err)
	}
	if err := os.Chmod(s.path, 0600); err != nil {
		return fmt.Errorf("secure auth store permissions: %w", err)
	}
	return nil
}
