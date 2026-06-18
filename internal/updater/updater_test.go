package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"0.6.1", "0.6.2", true},
		{"0.6.1", "0.6.1", false},
		{"0.6.2", "0.6.1", false},
		{"0.6.1", "0.7.0", true},
		{"0.6.1", "1.0.0", true},
		{"1.0.0", "0.9.9", false},
		{"0.6.1", "0.6.1.1", true},
		{"0.6.1", "0.6.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.current+"_"+tt.latest, func(t *testing.T) {
			got := isNewer(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestCheckURL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := release{TagName: "v0.7.0"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	latest, newer, err := checkURL(context.Background(), http.DefaultClient, "0.6.1", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latest != "0.7.0" {
		t.Errorf("latest = %q, want %q", latest, "0.7.0")
	}
	if !newer {
		t.Error("expected newer to be true")
	}
}

func TestCheckURL_NotNewer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := release{TagName: "v0.6.1"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	latest, newer, err := checkURL(context.Background(), http.DefaultClient, "0.6.1", server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if latest != "0.6.1" {
		t.Errorf("latest = %q, want %q", latest, "0.6.1")
	}
	if newer {
		t.Error("expected newer to be false")
	}
}

func TestCheckURL_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, _, err := checkURL(context.Background(), http.DefaultClient, "0.6.1", server.URL)
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestCheckURL_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	_, _, err := checkURL(context.Background(), http.DefaultClient, "0.6.1", server.URL)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
