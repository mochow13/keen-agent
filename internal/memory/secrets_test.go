package memory

import "testing"

func TestContainsSecret(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"aws key", "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE", true},
		{"api key", "api_key: abc123", true},
		{"apikey no separator", "apikey abc123", true},
		{"bearer token", "Bearer eyJhbGci...", true},
		{"password", "password=hunter2", true},
		{"github pat", "ghp_0123456789abcdefghijklmnopqrstuvwxyz0123", true},
		{"slack token", "xoxb-0123456789-012345", true},
		{"openai key", "sk-abcdefghijklmnopqrstuvwxyz0123456789", true},
		{"private key block", "-----BEGIN RSA PRIVATE KEY-----\nMIIE", true},
		{"clean text", "- User prefers brief responses.", false},
		{"normal note", "- Run go test -race ./... after changes", false},
		{"token mention", "user rotates tokens monthly", false},
		{"password mention", "passwordless auth", false},
		{"secret mention", "keep secrets out of logs", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsSecret(tt.content); got != tt.want {
				t.Fatalf("ContainsSecret(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
