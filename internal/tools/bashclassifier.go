package tools

import (
	"regexp"
	"strings"
)

// alwaysDangerousBaseCommands are commands that are always gated, regardless of flags.
var alwaysDangerousBaseCommands = map[string]struct{}{
	"rm": {}, "rmdir": {}, "shred": {}, "dd": {}, "unlink": {},

	"install": {},
	"chown":   {},

	"sudo": {}, "su": {}, "doas": {}, "pkexec": {}, "chroot": {},

	"kill": {}, "killall": {}, "pkill": {}, "xkill": {},
	"shutdown": {}, "reboot": {}, "halt": {}, "poweroff": {},

	"eval": {},
}

// dangerousGitSubcommands are git subcommands that are always gated.
var dangerousGitSubcommands = map[string]struct{}{
	"rm": {}, "clean": {}, "push": {}, "reset": {},
	"rebase": {}, "merge": {}, "cherry-pick": {},
}

// dangerousDockerSubcommands are docker subcommands that are always gated.
var dangerousDockerSubcommands = map[string]struct{}{
	"rm": {}, "rmi": {}, "kill": {},
}

// dangerousKubectlSubcommands are kubectl subcommands that are always gated.
var dangerousKubectlSubcommands = map[string]struct{}{
	"apply": {}, "delete": {}, "patch": {}, "edit": {},
}

// dangerousSystemctlSubcommands are systemctl subcommands that are always gated.
var dangerousSystemctlSubcommands = map[string]struct{}{
	"stop": {}, "restart": {}, "disable": {},
}

// dangerousPackageManagerRemovalFlags maps package-manager base commands to removal flags.
var dangerousPackageManagerRemovalFlags = map[string]map[string]struct{}{
	"apt-get": {"remove": {}, "purge": {}, "autoremove": {}},
	"apt":     {"remove": {}, "purge": {}, "autoremove": {}},
	"yum":     {"remove": {}},
	"dnf":     {"remove": {}},
	"pacman":  {"-R": {}, "-Rs": {}, "-Rns": {}},
	"brew":    {"uninstall": {}},
	"pip":     {"uninstall": {}},
	"pip3":    {"uninstall": {}},
	"npm":     {"uninstall": {}},
}

// conditionalDangerousFlags maps base commands to flags that make them dangerous.
var conditionalDangerousFlags = map[string]map[string]struct{}{
	"cp":    {"-f": {}, "--force": {}},
	"rsync": {"--delete": {}, "--force": {}},
	"chmod": {"-R": {}, "777": {}},
}

// sensitiveFilePathPrefixes are path prefixes that indicate sensitive files.
// Any command argument matching these prefixes is gated.
var sensitiveFilePathPrefixes = []string{
	"~/.ssh/",
	"~/.aws/",
	"~/.netrc",
	"~/.git-credentials",
	"/etc/shadow",
	"/etc/sudoers",
	"/proc/",
}

// sensitiveEnvVarPattern matches references to likely-secret environment variables.
var sensitiveEnvVarPattern = regexp.MustCompile(`\$(?:[A-Z_]*(?:AWS|SECRET|TOKEN|PASSWORD|PRIVATE|GITHUB_TOKEN)[A-Z_0-9]*|\{[A-Z_]*(?:AWS|SECRET|TOKEN|PASSWORD|PRIVATE|GITHUB_TOKEN)[A-Z_0-9]*\})`)

// IsDangerousCommand returns true if the command should require explicit user approval.
func IsDangerousCommand(command string) bool {
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}

	if containsPrivilegeEscalation(command) {
		return true
	}

	if sensitiveEnvVarPattern.MatchString(command) {
		return true
	}

	for _, segment := range splitCommandSegments(command) {
		if isDangerousSegment(segment) {
			return true
		}
	}

	for _, sub := range extractSubshells(command) {
		if IsDangerousCommand(sub) {
			return true
		}
	}

	return false
}

func containsPrivilegeEscalation(command string) bool {
	for _, cmd := range []string{"sudo", "su", "doas", "pkexec", "chroot"} {
		if isCommandWord(command, cmd) {
			return true
		}
	}
	return false
}

// isCommandWord reports whether name appears as a distinct command word in command.
func isCommandWord(command, name string) bool {
	for _, seg := range splitCommandSegments(command) {
		tokens := tokenize(seg)
		if len(tokens) > 0 && tokens[0] == name {
			return true
		}
	}
	return false
}

func splitCommandSegments(command string) []string {
	separators := []string{"&&", "||", ";", "|"}
	segments := []string{command}
	for _, sep := range separators {
		var next []string
		for _, s := range segments {
			parts := strings.Split(s, sep)
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					next = append(next, p)
				}
			}
		}
		segments = next
	}
	return segments
}

func tokenize(segment string) []string {
	return strings.Fields(segment)
}

func isDangerousSegment(segment string) bool {
	tokens := tokenize(segment)
	if len(tokens) == 0 {
		return false
	}

	base := tokens[0]

	if _, ok := alwaysDangerousBaseCommands[base]; ok {
		return true
	}

	// mkfs and variants (mkfs.ext4, mkfs.xfs, etc.).
	if base == "mkfs" || strings.HasPrefix(base, "mkfs.") {
		return true
	}

	if isDangerousGit(tokens) {
		return true
	}

	if base == "docker" && len(tokens) > 1 {
		if _, ok := dangerousDockerSubcommands[tokens[1]]; ok {
			return true
		}
	}

	if base == "kubectl" && len(tokens) > 1 {
		if _, ok := dangerousKubectlSubcommands[tokens[1]]; ok {
			return true
		}
	}

	if base == "systemctl" && len(tokens) > 1 {
		if _, ok := dangerousSystemctlSubcommands[tokens[1]]; ok {
			return true
		}
	}

	if isDangerousPackageManager(tokens) {
		return true
	}

	if base == "go" && len(tokens) > 1 && tokens[1] == "clean" {
		if hasFlag(tokens[2:], "-cache", "-modcache") {
			return true
		}
	}

	if base == "make" {
		if hasFlag(tokens[1:], "install", "clean", "distclean") {
			return true
		}
	}

	if isDangerousInterpreter(tokens) {
		return true
	}

	if hasOverwriteRedirect(tokens) {
		return true
	}

	if isDangerousEnvCommand(tokens) {
		return true
	}

	if flags, ok := conditionalDangerousFlags[base]; ok {
		for _, tok := range tokens[1:] {
			if _, ok := flags[tok]; ok {
				return true
			}
		}
	}

	if hasSensitiveFilePath(tokens[1:]) {
		return true
	}

	return false
}

func isDangerousGit(tokens []string) bool {
	if tokens[0] != "git" || len(tokens) < 2 {
		return false
	}
	sub := tokens[1]

	if _, ok := dangerousGitSubcommands[sub]; ok {
		return true
	}

	if sub == "checkout" && hasFlag(tokens[2:], "-f", "--force", "--hard") {
		return true
	}

	if sub == "branch" && hasFlag(tokens[2:], "-D") {
		return true
	}

	if sub == "tag" && hasFlag(tokens[2:], "-d") {
		return true
	}

	return false
}

func isDangerousPackageManager(tokens []string) bool {
	base := tokens[0]
	flags, ok := dangerousPackageManagerRemovalFlags[base]
	if !ok {
		return false
	}
	for _, tok := range tokens[1:] {
		if _, ok := flags[tok]; ok {
			return true
		}
	}
	return false
}

func isDangerousInterpreter(tokens []string) bool {
	base := tokens[0]
	switch base {
	case "bash", "sh", "zsh":
		return hasFlag(tokens[1:], "-c")
	case "python", "python3":
		return hasFlag(tokens[1:], "-c")
	case "perl":
		return hasFlag(tokens[1:], "-e")
	case "ruby":
		return hasFlag(tokens[1:], "-e")
	}
	return false
}

func isDangerousEnvCommand(tokens []string) bool {
	base := tokens[0]

	if base == "env" {
		// bare env, or env with only assignments and no command
		if len(tokens) == 1 {
			return true
		}
		hasCommand := false
		for _, tok := range tokens[1:] {
			if !strings.Contains(tok, "=") {
				hasCommand = true
				break
			}
		}
		return !hasCommand
	}

	if base == "printenv" {
		if len(tokens) == 1 {
			return true // bare printenv dumps all env
		}
		for _, tok := range tokens[1:] {
			if isSensitiveEnvVarName(tok) {
				return true
			}
		}
	}

	if base == "export" {
		for _, tok := range tokens[1:] {
			name := strings.SplitN(tok, "=", 2)[0]
			if isSensitiveEnvVarName(name) {
				return true
			}
		}
	}

	return false
}

func isSensitiveEnvVarName(name string) bool {
	upper := strings.ToUpper(name)
	for _, prefix := range []string{"AWS", "SECRET", "TOKEN", "PASSWORD", "PRIVATE", "GITHUB_TOKEN", "API_KEY", "KEY"} {
		if strings.Contains(upper, prefix) {
			return true
		}
	}
	return false
}

func hasOverwriteRedirect(tokens []string) bool {
	for _, tok := range tokens {
		if isAppendRedirect(tok) {
			continue
		}
		if tok == ">" || tok == ">|" || tok == "1>" || tok == "2>" || tok == "&>" {
			return true
		}
		for _, prefix := range []string{">|", "1>", "2>", "&>"} {
			if strings.HasPrefix(tok, prefix) {
				return true
			}
		}
		if strings.HasPrefix(tok, ">") {
			return true
		}
	}
	return false
}

func isAppendRedirect(tok string) bool {
	if tok == ">>" || tok == "1>>" || tok == "2>>" || tok == "&>>" {
		return true
	}
	for _, prefix := range []string{">>", "1>>", "2>>", "&>>"} {
		if strings.HasPrefix(tok, prefix) {
			return true
		}
	}
	return false
}

func hasSensitiveFilePath(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		for _, prefix := range sensitiveFilePathPrefixes {
			if strings.HasPrefix(arg, prefix) || arg == prefix {
				return true
			}
		}
	}
	return false
}

func hasFlag(flags []string, targets ...string) bool {
	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		set[t] = struct{}{}
	}
	for _, f := range flags {
		if _, ok := set[f]; ok {
			return true
		}
	}
	return false
}

// extractSubshells extracts the contents of $(...) and `...` subshell expressions.
func extractSubshells(command string) []string {
	var results []string

	for i := 0; i < len(command); i++ {
		if command[i] == '$' && i+1 < len(command) && command[i+1] == '(' {
			content := extractBalancedParens(command[i+2:])
			if content != "" {
				results = append(results, content)
			}
		} else if command[i] == '`' {
			end := strings.IndexByte(command[i+1:], '`')
			if end >= 0 {
				content := command[i+1 : i+1+end]
				if content != "" {
					results = append(results, content)
				}
			}
		}
	}

	return results
}

// extractBalancedParens extracts content from a position after "$(" up to the matching ")".
func extractBalancedParens(s string) string {
	depth := 1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[:i]
			}
		}
	}
	return ""
}
