package tools

import "testing"

func TestIsDangerousCommand_AlwaysDangerous(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"rm", "rm file.txt"},
		{"rmdir", "rmdir dir"},
		{"shred", "shred file.txt"},
		{"dd", "dd if=/dev/zero of=/dev/sda"},
		{"mkfs", "mkfs.ext4 /dev/sda1"},
		{"unlink", "unlink file.txt"},
		{"chown", "chown user:group file"},
		{"kill", "kill 1234"},
		{"shutdown", "shutdown now"},
		{"eval", "eval rm -rf /"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !IsDangerousCommand(tt.command) {
				t.Errorf("expected %q to be dangerous", tt.command)
			}
		})
	}
}

func TestIsDangerousCommand_PrivilegeEscalation(t *testing.T) {
	tests := []string{
		"sudo ls",
		"su - user",
		"doas command",
		"pkexec program",
		"chroot /path command",
	}

	for _, command := range tests {
		t.Run(command, func(t *testing.T) {
			if !IsDangerousCommand(command) {
				t.Errorf("expected %q to be dangerous", command)
			}
		})
	}
}

func TestIsDangerousCommand_GitDangerous(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"push", "git push"},
		{"push force", "git push --force"},
		{"reset", "git reset HEAD~1"},
		{"rebase", "git rebase main"},
		{"merge", "git merge feature"},
		{"rm", "git rm file.txt"},
		{"clean", "git clean -fd"},
		{"checkout force", "git checkout -f file.txt"},
		{"checkout hard", "git checkout --hard"},
		{"branch force delete", "git branch -D feature"},
		{"tag delete", "git tag -d v1.0"},
		{"cherry-pick", "git cherry-pick abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !IsDangerousCommand(tt.command) {
				t.Errorf("expected %q to be dangerous", tt.command)
			}
		})
	}
}

func TestIsDangerousCommand_GitSafe(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"status", "git status"},
		{"log", "git log"},
		{"diff", "git diff"},
		{"checkout branch", "git checkout feature"},
		{"checkout new branch", "git checkout -b feature"},
		{"branch list", "git branch"},
		{"branch safe delete", "git branch -d feature"},
		{"add", "git add file.txt"},
		{"commit", "git commit -m msg"},
		{"revert", "git revert abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsDangerousCommand(tt.command) {
				t.Errorf("expected %q to be safe", tt.command)
			}
		})
	}
}

func TestIsDangerousCommand_ConditionalFlags(t *testing.T) {
	tests := []struct {
		name    string
		command string
		danger  bool
	}{
		{"cp safe", "cp a b", false},
		{"cp force", "cp -f a b", true},
		{"cp force long", "cp --force a b", true},
		{"rsync safe", "rsync -av a b", false},
		{"rsync delete", "rsync --delete a b", true},
		{"rsync force", "rsync --force a b", true},
		{"docker run", "docker run nginx", false},
		{"docker stop", "docker stop mycontainer", false},
		{"docker exec", "docker exec mycontainer ls", false},
		{"docker rm", "docker rm mycontainer", true},
		{"docker rmi", "docker rmi myimage", true},
		{"docker kill", "docker kill mycontainer", true},
		{"chmod safe", "chmod +x script.sh", false},
		{"chmod 755", "chmod 755 file", false},
		{"chmod recursive", "chmod -R 755 dir", true},
		{"chmod 777", "chmod 777 file", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDangerousCommand(tt.command)
			if got != tt.danger {
				t.Errorf("IsDangerousCommand(%q) = %v, want %v", tt.command, got, tt.danger)
			}
		})
	}
}

func TestIsDangerousCommand_Redirection(t *testing.T) {
	tests := []struct {
		name    string
		command string
		danger  bool
	}{
		{"overwrite", "echo test > file.txt", true},
		{"overwrite stderr", "cmd 2> file.txt", true},
		{"overwrite clobber", "echo test >| file.txt", true},
		{"append", "echo test >> file.txt", false},
		{"stdout only", "echo test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDangerousCommand(tt.command)
			if got != tt.danger {
				t.Errorf("IsDangerousCommand(%q) = %v, want %v", tt.command, got, tt.danger)
			}
		})
	}
}

func TestIsDangerousCommand_SafeCommands(t *testing.T) {
	tests := []string{
		"ls -la",
		"cat file.txt",
		"echo hello",
		"go test ./...",
		"go build",
		"npm test",
		"make",
		"mkdir dir",
		"touch file",
		"pwd",
		"find . -name '*.go'",
		"grep foo file.txt",
		"python3 -m pytest",
		"mv a b",
		"chmod +x script.sh",
	}

	for _, command := range tests {
		t.Run(command, func(t *testing.T) {
			if IsDangerousCommand(command) {
				t.Errorf("expected %q to be safe", command)
			}
		})
	}
}

func TestIsDangerousCommand_Compound(t *testing.T) {
	tests := []struct {
		name    string
		command string
		danger  bool
	}{
		{"one dangerous", "echo hello && rm file", true},
		{"all safe", "echo hello && echo world", false},
		{"piped dangerous", "cat file | rm -", true},
		{"or dangerous", "false || sudo ls", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDangerousCommand(tt.command)
			if got != tt.danger {
				t.Errorf("IsDangerousCommand(%q) = %v, want %v", tt.command, got, tt.danger)
			}
		})
	}
}

func TestIsDangerousCommand_SensitiveFilePaths(t *testing.T) {
	tests := []struct {
		name    string
		command string
		danger  bool
	}{
		{"ssh key", "cat ~/.ssh/id_rsa", true},
		{"ssh known_hosts", "cat ~/.ssh/known_hosts", true},
		{"aws credentials", "cat ~/.aws/credentials", true},
		{"netrc", "cat ~/.netrc", true},
		{"git credentials", "cat ~/.git-credentials", true},
		{"etc shadow", "cat /etc/shadow", true},
		{"etc sudoers", "cat /etc/sudoers", true},
		{"proc environ", "cat /proc/1/environ", true},
		{"less sensitive", "less ~/.ssh/id_rsa", true},
		{"head sensitive", "head ~/.aws/credentials", true},
		{"grep sensitive", "grep token ~/.netrc", true},
		{"safe file", "cat file.txt", false},
		{"safe path", "cat /etc/hosts", false},
		{"safe home", "cat ~/project/readme.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDangerousCommand(tt.command)
			if got != tt.danger {
				t.Errorf("IsDangerousCommand(%q) = %v, want %v", tt.command, got, tt.danger)
			}
		})
	}
}

func TestIsDangerousCommand_Subshells(t *testing.T) {
	tests := []struct {
		name    string
		command string
		danger  bool
	}{
		{"dollar paren rm", "echo $(rm file.txt)", true},
		{"dollar paren sudo", "result=$(sudo ls)", true},
		{"backtick rm", "echo `rm file.txt`", true},
		{"backtick sudo", "result=`sudo ls`", true},
		{"nested safe", "echo $(echo hello)", false},
		{"backtick safe", "echo `date`", false},
		{"safe no subshell", "echo hello", false},
		{"chained with subshell", "ls && echo $(chmod 777 f)", true},
		{"subshell with eval", "cat $(eval echo hi)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDangerousCommand(tt.command)
			if got != tt.danger {
				t.Errorf("IsDangerousCommand(%q) = %v, want %v", tt.command, got, tt.danger)
			}
		})
	}
}

func TestIsDangerousCommand_EnvSecrets(t *testing.T) {
	tests := []struct {
		name    string
		command string
		danger  bool
	}{
		{"bare env", "env", true},
		{"env with command", "env GOOS=linux go build", false},
		{"bare printenv", "printenv", true},
		{"printenv common", "printenv HOME", false},
		{"printenv secret", "printenv GITHUB_TOKEN", true},
		{"export safe", "export PATH=$PATH:/foo", false},
		{"export sensitive", "export AWS_SECRET_KEY=abc", true},
		{"unset", "unset VAR", false},
		{"sensitive var", "echo $AWS_SECRET_ACCESS_KEY", true},
		{"common var", "echo $HOME", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDangerousCommand(tt.command)
			if got != tt.danger {
				t.Errorf("IsDangerousCommand(%q) = %v, want %v", tt.command, got, tt.danger)
			}
		})
	}
}
