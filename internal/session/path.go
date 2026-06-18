package session

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const transcriptFileName = "transcript_events.jsonl"

var unsafePathChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func sessionsRootDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".keen-agent", "sessions"), nil
}

func namespaceDirName(workingDir string) string {
	slug := sanitizeWorkingDir(workingDir)
	return slug + "-" + shortHash(workingDir)
}

func sanitizeWorkingDir(workingDir string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", string(filepath.Separator), "-")
	slug := replacer.Replace(strings.TrimSpace(workingDir))
	slug = unsafePathChars.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-.")
	if slug == "" {
		return "root"
	}
	return slug
}

func shortHash(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:10]
}

func sessionDirName(sessionID string) string {
	return sessionID
}

func generateSessionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		raw[0:4],
		raw[4:6],
		raw[6:8],
		raw[8:10],
		raw[10:16],
	), nil
}
