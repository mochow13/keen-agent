package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/mochow13/keen-agent/internal/agentconfig"
	keenauth "github.com/mochow13/keen-agent/internal/auth"
	"github.com/mochow13/keen-agent/internal/cli/repl"
	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/llm"
	keenmcp "github.com/mochow13/keen-agent/internal/mcp"
	"github.com/mochow13/keen-agent/internal/providers"
	"github.com/mochow13/keen-agent/internal/session"
	"github.com/spf13/cobra"
)

var newMCPManager = func(opts ...keenmcp.Option) (keenmcp.Runtime, error) {
	return keenmcp.NewManager(opts...)
}

func NewRootCommand(version string) *cobra.Command {
	var resumeSessionID string
	var agentFile string
	var modeFlag string

	cmd := &cobra.Command{
		Use:   "keen-agent",
		Short: "Keen Agent - A generic agent harness",
		Long:  `Keen Agent is a terminal-based agent harness that runs configured agents with tools, skills, and subagents.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, loader, globalCfg, _, _, err := loadRootRuntime()
			if err != nil {
				return err
			}
			wd, err := os.Getwd()
			if err != nil {
				wd = "."
			}

			agentCfg, err := loadAgentConfig(wd, agentFile)
			if err != nil {
				return err
			}

			mode, err := resolveModeOverride(agentCfg, modeFlag)
			if err != nil {
				return err
			}

			resolvedCfg, needsSetup, modelWarning, err := resolveSessionConfig(globalCfg, registry, agentCfg)
			if err != nil {
				return err
			}

			var resumeSession *session.LoadedSession
			if resumeSessionID != "" {
				var agentSlug string
				if agentCfg != nil {
					agentSlug = agentCfg.AgentSlug()
				}
				resumeSession, err = loadResumeSession(wd, resumeSessionID, agentSlug)
				if err != nil {
					return err
				}
			}

			mcpManager, closeMCP, mcpErr := startMCPRuntime(context.Background(), agentCfg)
			defer closeMCP()
			if mcpErr != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "MCP unavailable: %v\n", mcpErr)
			}

			sessionID, err := repl.RunREPL(version, wd, resolvedCfg, loader, globalCfg, registry, needsSetup, mcpManager, resumeSession, agentCfg, modelWarning, mode)
			if err != nil {
				return err
			}
			if sessionID != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nRun `keen-agent --resume %s` to resume the session\n", sessionID)
			}
			return nil
		},
	}

	cmd.Version = version
	cmd.Flags().StringVar(&resumeSessionID, "resume", "", "resume a specific Keen Agent session by ID")
	cmd.Flags().StringVar(&agentFile, "agent", "", "path to agent.yaml config (required)")
	cmd.Flags().StringVar(&modeFlag, "mode", "", "active mode: plan or build")
	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newValidateCommand())
	return cmd
}

func startMCPRuntime(ctx context.Context, agentCfg *agentconfig.Config) (keenmcp.Runtime, func(), error) {
	var opts []keenmcp.Option
	if agentCfg != nil {
		if dirs := agentCfg.ResolvedMCPConfigDirs(); len(dirs) > 0 {
			opts = append(opts, keenmcp.WithConfigPaths(dirs))
		}
	}
	manager, err := newMCPManager(opts...)
	if err != nil {
		return nil, func() {}, err
	}
	if err := manager.Start(ctx); err != nil {
		if closeErr := manager.Close(); closeErr != nil {
			slog.Warn("MCP shutdown failed after startup error", "error", closeErr)
		}
		return nil, func() {}, err
	}
	slog.Debug("MCP manager started")
	return manager, func() {
		if err := manager.Close(); err != nil {
			slog.Warn("MCP shutdown failed", "error", err)
		}
	}, nil
}

func newRunCommand() *cobra.Command {
	var sessionID string
	var format string
	var providerID string
	var modelID string
	var agentFile string
	var modeFlag string

	runCmd := &cobra.Command{
		Use:   "run [flags] <message...>",
		Short: "Run one non-interactive Keen Agent turn",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, _, globalCfg, _, _, err := loadRootRuntime()
			if err != nil {
				return err
			}

			wd, err := os.Getwd()
			if err != nil {
				wd = "."
			}
			agentCfg, err := loadAgentConfig(wd, agentFile)
			if err != nil {
				return err
			}

			mode, err := resolveModeOverride(agentCfg, modeFlag)
			if err != nil {
				return err
			}

			resolvedCfg, _, modelWarning, err := resolveSessionConfig(globalCfg, registry, agentCfg)
			if err != nil {
				return err
			}
			if modelWarning != "" {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s\n", modelWarning)
			}

			if err := applyRunOverrides(globalCfg, resolvedCfg, providerID, modelID); err != nil {
				return err
			}
			if resolvedCfg.Provider == "" {
				return fmt.Errorf("LLM client not initialized. Run keen to configure a provider")
			}
			if resolvedCfg.AuthMode == config.AuthModeOAuth && !keenauth.NewOAuthManager(nil).HasCredential(resolvedCfg.Provider) {
				return fmt.Errorf("LLM client not initialized. Run keen to configure a provider")
			}

			stdin := ""
			if shouldReadStdin(os.Stdin) {
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
				stdin = string(data)
			}
			prompt := buildRunPrompt(args, stdin)
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			client, err := llm.NewClient(resolvedCfg)
			if err != nil {
				return err
			}
			_, closeMCP, mcpErr := startMCPRuntime(context.Background(), agentCfg)
			defer closeMCP()
			if mcpErr != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "MCP unavailable: %v\n", mcpErr)
			}

			_, err = repl.RunHeadless(context.Background(), repl.HeadlessRunOptions{
				WorkingDir: wd,
				Config:     resolvedCfg,
				AgentCfg:   agentCfg,
				Client:     client,
				SessionID:  sessionID,
				Prompt:     prompt,
				Format:     format,
				Out:        cmd.OutOrStdout(),
				Mode:       mode,
			})
			return err
		},
	}
	runCmd.Flags().StringVar(&sessionID, "session", "", "resume an existing Keen Agent session")
	runCmd.Flags().StringVar(&format, "format", repl.HeadlessFormatText, "output format: text or json")
	runCmd.Flags().StringVar(&providerID, "provider", "", "provider to use for this run")
	runCmd.Flags().StringVar(&modelID, "model", "", "model to use for this run")
	runCmd.Flags().StringVar(&agentFile, "agent", "", "path to agent.yaml config (required)")
	runCmd.Flags().StringVar(&modeFlag, "mode", "", "active mode: plan or build")
	return runCmd
}

func loadRootRuntime() (*providers.Registry, *config.Loader, *config.GlobalConfig, *config.ResolvedConfig, bool, error) {
	registry, err := providers.Load()
	if err != nil {
		return nil, nil, nil, nil, false, fmt.Errorf("failed to load provider registry: %w", err)
	}
	loader := config.NewLoader()
	globalCfg, err := loader.Load()
	if err != nil {
		return nil, nil, nil, nil, false, fmt.Errorf("failed to load config: %w", err)
	}

	resolvedCfg, needsSetup, err := resolveFromActiveProvider(globalCfg, registry)
	if err != nil {
		return nil, nil, nil, nil, false, err
	}
	return registry, loader, globalCfg, resolvedCfg, needsSetup, nil
}

func resolveFromActiveProvider(globalCfg *config.GlobalConfig, registry *providers.Registry) (*config.ResolvedConfig, bool, error) {
	if globalCfg.ActiveProvider == "" {
		return &config.ResolvedConfig{}, true, nil
	}

	_, ok := registry.GetProvider(globalCfg.ActiveProvider)
	if !ok {
		return nil, false, fmt.Errorf("configured provider %q not found in registry", globalCfg.ActiveProvider)
	}
	providerCfg, ok := globalCfg.GetProviderConfig(globalCfg.ActiveProvider)
	if !ok {
		return nil, false, fmt.Errorf("failed to get provider config for %q", globalCfg.ActiveProvider)
	}
	apiKey, err := config.ResolveProviderAPIKey(globalCfg.ActiveProvider, providerCfg)
	if err != nil {
		return nil, false, err
	}
	activeModel := globalCfg.ActiveModel
	if activeModel == "" && len(providerCfg.Models) > 0 {
		activeModel = providerCfg.Models[0]
	}
	resolvedCfg := &config.ResolvedConfig{
		Provider:       globalCfg.ActiveProvider,
		Model:          activeModel,
		APIKey:         apiKey,
		APIKeyHelper:   providerCfg.APIKeyHelper,
		ThinkingEffort: globalCfg.ThinkingEffort,
		BaseURL:        providerCfg.BaseURL,
		AuthMode:       config.AuthModeForProvider(globalCfg.ActiveProvider),
		Headers:        providerCfg.Headers,
	}
	needsSetup := resolvedCfg.AuthMode == config.AuthModeOAuth && !keenauth.NewOAuthManager(nil).HasCredential(globalCfg.ActiveProvider)
	return resolvedCfg, needsSetup, nil
}

func resolveSessionConfig(globalCfg *config.GlobalConfig, registry *providers.Registry, agentCfg *agentconfig.Config) (*config.ResolvedConfig, bool, string, error) {
	var warning string

	if agentCfg != nil && agentCfg.Model.IsComplete() {
		provider := agentCfg.Model.Provider
		modelID := agentCfg.Model.ModelID
		providerCfg, ok := globalCfg.GetProviderConfig(provider)
		if ok && slices.Contains(providerCfg.Models, modelID) {
			_, ok := registry.GetProvider(provider)
			if !ok {
				return nil, false, "", fmt.Errorf("configured provider %q not found in registry", provider)
			}
			apiKey, err := config.ResolveProviderAPIKey(provider, providerCfg)
			if err != nil {
				return nil, false, "", err
			}
			resolvedCfg := &config.ResolvedConfig{
				Provider:       provider,
				Model:          modelID,
				APIKey:         apiKey,
				APIKeyHelper:   providerCfg.APIKeyHelper,
				ThinkingEffort: globalCfg.ThinkingEffort,
				BaseURL:        providerCfg.BaseURL,
				AuthMode:       config.AuthModeForProvider(provider),
				Headers:        providerCfg.Headers,
			}
			needsSetup := resolvedCfg.AuthMode == config.AuthModeOAuth && !keenauth.NewOAuthManager(nil).HasCredential(provider)
			return resolvedCfg, needsSetup, "", nil
		}
		warning = fmt.Sprintf("Configured model %s/%s is not available in ~/.keen-agent/configs.json.", provider, modelID)
	} else if agentCfg != nil && agentCfg.Model.IsSet() {
		warning = "Agent config model block is incomplete (requires both provider and model_id)."
	}

	resolvedCfg, needsSetup, err := resolveFromActiveProvider(globalCfg, registry)
	if err != nil {
		return nil, false, "", err
	}
	if warning != "" && resolvedCfg.Provider != "" && resolvedCfg.Model != "" {
		warning = fmt.Sprintf("%s\n  Using active model %s/%s instead. Use /model to choose a different model.", warning, resolvedCfg.Provider, resolvedCfg.Model)
	} else if warning != "" {
		warning = warning + " No active model is selected. Use /model to choose one."
	}
	return resolvedCfg, needsSetup, warning, nil
}

func applyRunOverrides(globalCfg *config.GlobalConfig, resolvedCfg *config.ResolvedConfig, providerID string, modelID string) error {
	if providerID != "" {
		providerCfg, ok := globalCfg.GetProviderConfig(providerID)
		if !ok {
			return fmt.Errorf("provider %q is not configured", providerID)
		}
		apiKey, err := config.ResolveProviderAPIKey(providerID, providerCfg)
		if err != nil {
			return err
		}
		resolvedCfg.Provider = providerID
		resolvedCfg.APIKey = apiKey
		resolvedCfg.BaseURL = providerCfg.BaseURL
		resolvedCfg.AuthMode = config.AuthModeForProvider(providerID)
		resolvedCfg.Headers = providerCfg.Headers
		if modelID == "" && len(providerCfg.Models) > 0 {
			resolvedCfg.Model = providerCfg.Models[0]
		}
	}
	if modelID != "" {
		resolvedCfg.Model = modelID
	}
	return nil
}

func buildRunPrompt(args []string, stdin string) string {
	argText := strings.TrimSpace(strings.Join(args, " "))
	stdin = strings.TrimSpace(stdin)
	switch {
	case argText != "" && stdin != "":
		return argText + "\n" + stdin
	case argText != "":
		return argText
	default:
		return stdin
	}
}

func shouldReadStdin(stdin *os.File) bool {
	info, err := stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}

func resolveModeOverride(agentCfg *agentconfig.Config, modeFlag string) (llm.AgentMode, error) {
	if modeFlag != "" && modeFlag != agentconfig.ModePlan && modeFlag != agentconfig.ModeBuild {
		return "", fmt.Errorf("invalid --mode %q; must be %q or %q", modeFlag, agentconfig.ModePlan, agentconfig.ModeBuild)
	}
	mode := agentCfg.EffectiveDefaultMode()
	if modeFlag != "" {
		mode = modeFlag
	}
	if mode == agentconfig.ModePlan {
		return llm.ModePlan, nil
	}
	return llm.ModeBuild, nil
}

func newValidateCommand() *cobra.Command {
	var agentFile string

	validateCmd := &cobra.Command{
		Use:   "validate --agent ./agent.yaml",
		Short: "Validate an agent configuration file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				wd = "."
			}
			path, err := resolveAgentConfigPath(wd, agentFile)
			if err != nil {
				return err
			}

			cfg, err := agentconfig.Load(path)
			if err != nil {
				return err
			}

			res := agentconfig.Validate(cfg)
			if !res.OK() {
				for _, issue := range res.Errors {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s: %s\n", issue.Path, issue.Message)
				}
				return fmt.Errorf("agent config %q is invalid", path)
			}

			if len(res.Warnings) > 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Warnings:")
				for _, issue := range res.Warnings {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", issue.Path, issue.Message)
				}
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "agent config %q is valid\n", path)
			}
			return nil
		},
	}
	validateCmd.Flags().StringVar(&agentFile, "agent", "", "path to agent.yaml config (required)")
	validateCmd.SilenceUsage = true
	validateCmd.SilenceErrors = true
	return validateCmd
}

func loadAgentConfig(workingDir, explicitPath string) (*agentconfig.Config, error) {
	path, err := resolveAgentConfigPath(workingDir, explicitPath)
	if err != nil {
		return nil, err
	}

	cfg, err := agentconfig.Load(path)
	if err != nil {
		return nil, err
	}

	res := agentconfig.Validate(cfg)
	if !res.OK() {
		for _, issue := range res.Errors {
			fmt.Fprintf(os.Stderr, "agent config error: %s: %s\n", issue.Path, issue.Message)
		}
		return nil, fmt.Errorf("invalid agent config %q", path)
	}
	return cfg, nil
}

func resolveAgentConfigPath(_ string, explicitPath string) (string, error) {
	if explicitPath == "" {
		return "", fmt.Errorf("--agent flag is required")
	}
	if _, err := os.Stat(explicitPath); err != nil {
		return "", fmt.Errorf("agent config not found: %w", err)
	}
	return explicitPath, nil
}
