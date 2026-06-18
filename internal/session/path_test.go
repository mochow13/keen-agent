package session

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSessionsRootDir_UsesHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := sessionsRootDir()
	if err != nil {
		t.Fatalf("sessionsRootDir() error = %v", err)
	}

	want := filepath.Join(tmp, ".keen-agent", "sessions")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSanitizeWorkingDir_EmptyBecomesRoot(t *testing.T) {
	if got := sanitizeWorkingDir("   "); got != "root" {
		t.Fatalf("expected root, got %q", got)
	}
}

func TestSanitizeWorkingDir_NormalizesUnsafeCharacters(t *testing.T) {
	got := sanitizeWorkingDir(` /Users/me/My Project/@tmp `)
	if got != "Users-me-My-Project--tmp" {
		t.Fatalf("unexpected slug: %q", got)
	}
}

func TestNamespaceDirName_ContainsSlugAndHash(t *testing.T) {
	workingDir := "/Users/me/src/keen-agent"
	got := namespaceDirName(workingDir)

	if !strings.HasPrefix(got, "Users-me-src-keen-agent-") {
		t.Fatalf("expected slug prefix, got %q", got)
	}

	parts := strings.Split(got, "-")
	hash := parts[len(parts)-1]
	if len(hash) != 10 {
		t.Fatalf("expected 10-char hash suffix, got %q", hash)
	}
}

func TestShortHash_IsStable(t *testing.T) {
	value := "/Users/me/src/keen-agent"
	hash := shortHash(value)
	if shortHash(value) != hash {
		t.Fatal("expected shortHash to be stable for the same input")
	}
}

func TestSessionDirName_UsesSessionID(t *testing.T) {
	got := sessionDirName("session-id")
	want := "session-id"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestGenerateSessionID_Format(t *testing.T) {
	got, err := generateSessionID()
	if err != nil {
		t.Fatalf("generateSessionID() error = %v", err)
	}

	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(got) {
		t.Fatalf("unexpected session id format: %q", got)
	}
}
