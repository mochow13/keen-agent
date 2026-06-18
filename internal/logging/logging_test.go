package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     int
	}{
		{"debug", "debug", -4},
		{"info", "info", 0},
		{"warn", "warn", 4},
		{"warning", "warning", 4},
		{"error", "error", 8},
		{"empty", "", 0},
		{"default", "unknown", 0},
		{"uppercase", "DEBUG", -4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(logLevelEnvVar, tt.envValue)
				defer os.Unsetenv(logLevelEnvVar)
			}

			level := parseLogLevel()
			if int(level) != tt.want {
				t.Errorf("parseLogLevel() = %v, want %v", level, tt.want)
			}
		})
	}
}

func TestGetLogDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir, err := getLogDirectory()
	if err != nil {
		t.Fatalf("getLogDirectory() error = %v", err)
	}

	if !strings.Contains(dir, ".keen-agent") {
		t.Errorf("getLogDirectory() = %v, want to contain '.keen-agent'", dir)
	}

	if !strings.Contains(dir, "logs") {
		t.Errorf("getLogDirectory() = %v, want to contain 'logs'", dir)
	}
}

func TestCreateLogFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	file, logFile, err := createLogFile()
	if err != nil {
		t.Fatalf("createLogFile() error = %v", err)
	}
	defer file.Close()

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("createLogFile() did not create file: %v", logFile)
	}

	if !strings.HasPrefix(filepath.Base(logFile), "keen-") {
		t.Errorf("createLogFile() filename = %v, want prefix 'keen-'", logFile)
	}

	if !strings.HasSuffix(logFile, ".log") {
		t.Errorf("createLogFile() filename = %v, want suffix '.log'", logFile)
	}
}

func TestInit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cleanup, logFile, err := Init()
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer cleanup()

	if logFile == "" {
		t.Error("Init() logFile is empty")
	}

	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Init() created file not accessible: %v", err)
	}

	if info.IsDir() {
		t.Error("Init() created path is a directory, not a file")
	}
}
