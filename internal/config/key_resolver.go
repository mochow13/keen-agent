package config

import "sync"

// APIKeyResolver caches a key resolved via apiKeyHelper and refreshes it on demand.
type APIKeyResolver struct {
	mu       sync.Mutex
	provider string
	helper   string
	cached   string
}

func NewAPIKeyResolver(provider, helper string) *APIKeyResolver {
	return &APIKeyResolver{provider: provider, helper: helper}
}

func (r *APIKeyResolver) Get() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cached != "" {
		return r.cached, nil
	}
	key, err := runAPIKeyHelper(r.provider, r.helper)
	if err != nil {
		return "", err
	}
	r.cached = key
	return key, nil
}

func (r *APIKeyResolver) Refresh() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key, err := runAPIKeyHelper(r.provider, r.helper)
	if err != nil {
		return "", err
	}
	r.cached = key
	return key, nil
}
