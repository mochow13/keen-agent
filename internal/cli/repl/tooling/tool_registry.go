package tooling

import (
	"path/filepath"

	"github.com/mochow13/keen-agent/internal/agentconfig"
	replappstate "github.com/mochow13/keen-agent/internal/cli/repl/appstate"
	replpermissions "github.com/mochow13/keen-agent/internal/cli/repl/permissions"
	"github.com/mochow13/keen-agent/internal/config"
	"github.com/mochow13/keen-agent/internal/filesystem"
	"github.com/mochow13/keen-agent/internal/llm"
	keenmcp "github.com/mochow13/keen-agent/internal/mcp"
	"github.com/mochow13/keen-agent/internal/subagents"
	"github.com/mochow13/keen-agent/internal/tools"
)

func SetupToolRegistry(
	workingDir string,
	appState *replappstate.AppState,
	permissionRequester *replpermissions.Requester,
	diffEmitter *DiffEmitter,
	mcpRuntime keenmcp.Runtime,
	cfg *config.ResolvedConfig,
	agentCfg *agentconfig.Config,
) {
	gitAwareness := filesystem.NewGitAwareness()
	_ = gitAwareness.LoadGitignore(filepath.Join(workingDir, ".gitignore"))
	guard := filesystem.NewGuard(workingDir, gitAwareness)

	excluded := builtinToolsExcluded(agentCfg)

	registerExcludable := func(tool tools.Tool) {
		if excluded[tool.Name()] {
			return
		}
		_ = appState.RegisterTool(tool)
	}
	registerRequired := func(tool tools.Tool) {
		_ = appState.RegisterTool(tool)
	}

	readFileTool := tools.NewReadFileTool(guard, permissionRequester)
	registerExcludable(readFileTool)

	globTool := tools.NewGlobTool(guard, permissionRequester)
	registerExcludable(globTool)

	grepTool := tools.NewGrepTool(guard, permissionRequester)
	registerExcludable(grepTool)

	writeFileTool := tools.NewWriteFileTool(guard, diffEmitter, permissionRequester)
	registerExcludable(writeFileTool)

	editFileTool := tools.NewEditFileTool(guard, diffEmitter, permissionRequester)
	registerExcludable(editFileTool)

	bashTool := tools.NewBashTool(guard, permissionRequester)
	registerExcludable(bashTool)

	webFetchTool := tools.NewWebFetchTool()
	registerExcludable(webFetchTool)

	if mcpRuntime != nil && hasMCPConfigPaths(agentCfg) {
		registerRequired(tools.NewCallMCPTool(mcpRuntime, permissionRequester))
	}

	if hasSubagentsDirs(agentCfg) {
		runner := &subagents.Runner{
			WorkingDir: workingDir,
			Config:     cfg,
			GetProfiles: func() []subagents.Profile {
				return appState.GetSubagents().Profiles
			},
			NewClient: llm.NewClient,
			Registry:  appState.GetToolRegistry(),
		}
		registerRequired(tools.NewDelegateTool(runner))
	}
}

func builtinToolsExcluded(cfg *agentconfig.Config) map[string]bool {
	excluded := make(map[string]bool)
	if cfg == nil || cfg.BuiltinTools == nil {
		return excluded
	}
	for _, name := range cfg.BuiltinTools.Exclude {
		excluded[name] = true
	}
	return excluded
}

func hasMCPConfigPaths(cfg *agentconfig.Config) bool {
	return cfg != nil && len(cfg.ResolvedMCPConfigPaths()) > 0
}

func hasSubagentsDirs(cfg *agentconfig.Config) bool {
	return cfg != nil && len(cfg.ResolvedSubagentsDirs()) > 0
}
