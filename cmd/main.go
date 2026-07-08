package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/mochow13/keen-agent/internal/agentconfig"
	clicmd "github.com/mochow13/keen-agent/internal/cli/cmd"
	"github.com/mochow13/keen-agent/internal/logging"
)

var version = "0.1.0"

func main() {
	var agentSlug string
	for i, arg := range os.Args {
		if arg == "--agent" && i+1 < len(os.Args) {
			if cfg, err := agentconfig.Load(os.Args[i+1]); err == nil {
				agentSlug = cfg.AgentSlug()
			}
			break
		} else if strings.HasPrefix(arg, "--agent=") {
			if cfg, err := agentconfig.Load(strings.TrimPrefix(arg, "--agent=")); err == nil {
				agentSlug = cfg.AgentSlug()
			}
			break
		}
	}

	cleanup, logFile, err := logging.Init(agentSlug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logging: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	slog.Debug("Logging initialized", "file", logFile)

	rootCmd := clicmd.NewRootCommand(version)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
