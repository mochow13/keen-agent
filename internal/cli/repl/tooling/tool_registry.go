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

	register := func(tool tools.Tool) {
		if excluded[tool.Name()] {
			return
		}
		_ = appState.RegisterTool(tool)
	}

	readFileTool := tools.NewReadFileTool(guard, permissionRequester)
	register(readFileTool)

	globTool := tools.NewGlobTool(guard, permissionRequester)
	register(globTool)

	grepTool := tools.NewGrepTool(guard, permissionRequester)
	register(grepTool)

	writeFileTool := tools.NewWriteFileTool(guard, diffEmitter, permissionRequester)
	register(writeFileTool)

	editFileTool := tools.NewEditFileTool(guard, diffEmitter, permissionRequester)
	register(editFileTool)

	bashTool := tools.NewBashTool(guard, permissionRequester)
	register(bashTool)

	webFetchTool := tools.NewWebFetchTool()
	register(webFetchTool)

	if mcpRuntime != nil && hasMCPConfigDirs(agentCfg) {
		register(tools.NewCallMCPTool(mcpRuntime, permissionRequester))
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
		register(tools.NewDelegateTool(runner))
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

func hasMCPConfigDirs(cfg *agentconfig.Config) bool {
	return cfg != nil && len(cfg.ResolvedMCPConfigDirs()) > 0
}

func hasSubagentsDirs(cfg *agentconfig.Config) bool {
	return cfg != nil && len(cfg.ResolvedSubagentsDirs()) > 0
}
