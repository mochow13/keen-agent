package repl

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func readGitBranch(dir string) string {
	if dir == "" {
		return ""
	}
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return ""
	}
	ref, err := repo.Reference(plumbing.HEAD, false)
	if err != nil {
		return ""
	}
	if ref.Type() == plumbing.SymbolicReference {
		name := ref.Target()
		if name.IsBranch() {
			return name.Short()
		}
		return ""
	}
	hash := ref.Hash().String()
	if len(hash) >= 7 {
		return hash[:7]
	}
	return ""
}

func (m *replModel) refreshGitBranch() {
	if m.ctx == nil {
		return
	}
	m.gitBranch = readGitBranch(m.ctx.workingDir)
}
