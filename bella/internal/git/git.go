package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Git struct{ GitPath string }
type CommandResult struct {
	Output string
	Error  error
}

type RepoStatus struct {
	Path             string
	Branch           string
	IsRepository     bool
	HasLocalChanges  bool
	HasRemoteChanges bool
	HasCommitsToPush bool
}

func New(gitPath string) *Git {
	if strings.TrimSpace(gitPath) == "" {
		gitPath = "git"
	}
	return &Git{GitPath: gitPath}
}

func (g *Git) Run(repoPath string, args ...string) CommandResult {
	cmd := exec.Command(g.GitPath, args...)
	cmd.Dir = repoPath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())
	if errOut != "" {
		if out != "" {
			out += "\n"
		}
		out += errOut
	}
	return CommandResult{Output: out, Error: err}
}

func (g *Git) RunGlobal(args ...string) CommandResult {
	cmd := exec.Command(g.GitPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())
	if errOut != "" {
		if out != "" {
			out += "\n"
		}
		out += errOut
	}
	return CommandResult{Output: out, Error: err}
}

func (g *Git) IsGitInstalled() bool { return exec.Command(g.GitPath, "--version").Run() == nil }
func IsRepository(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}
func EnsureFolder(path string) error { return os.MkdirAll(path, os.ModePerm) }

func (g *Git) Clone(repoURL, branch, destination string) error {
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	if err := os.MkdirAll(filepath.Dir(destination), os.ModePerm); err != nil {
		return err
	}
	cmd := exec.Command(g.GitPath, "clone", "-b", branch, repoURL, destination)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("erro ao clonar repositório: %s", strings.TrimSpace(stdout.String()+"\n"+stderr.String()))
	}
	return nil
}

func (g *Git) InitRepository(path, repoURL, branch string) error {
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}
	for _, args := range [][]string{{"init"}, {"branch", "-M", branch}} {
		res := g.Run(path, args...)
		if res.Error != nil {
			return fmt.Errorf("erro ao executar git %s: %s", strings.Join(args, " "), res.Output)
		}
	}
	remote := g.Run(path, "remote")
	if remote.Error == nil && strings.Contains(remote.Output, "origin") {
		res := g.Run(path, "remote", "set-url", "origin", repoURL)
		if res.Error != nil {
			return fmt.Errorf("erro ao atualizar remote origin: %s", res.Output)
		}
	} else {
		res := g.Run(path, "remote", "add", "origin", repoURL)
		if res.Error != nil {
			return fmt.Errorf("erro ao adicionar remote origin: %s", res.Output)
		}
	}
	_ = g.SetUpstream(path, branch)
	return nil
}

func (g *Git) SetUpstream(path, branch string) error {
	if branch == "" {
		branch = "main"
	}
	_ = g.Run(path, "config", "--unset-all", "branch."+branch+".merge")
	_ = g.Run(path, "config", "--add", "branch."+branch+".merge", "refs/heads/"+branch)
	res := g.Run(path, "config", "branch."+branch+".remote", "origin")
	if res.Error != nil {
		return errors.New(res.Output)
	}
	return nil
}

func (g *Git) HasChanges(path string) (bool, error) {
	res := g.Run(path, "status", "--porcelain")
	if res.Error != nil {
		return false, errors.New(res.Output)
	}
	return strings.TrimSpace(res.Output) != "", nil
}

func (g *Git) Fetch(path, branch string) error {
	if branch == "" {
		branch = "main"
	}
	res := g.Run(path, "fetch", "origin", branch)
	if res.Error != nil {
		return errors.New(res.Output)
	}
	return nil
}
func (g *Git) HasRemoteChanges(path, branch string) (bool, error) {
	if branch == "" {
		branch = "main"
	}
	res := g.Run(path, "rev-list", "--count", "HEAD..origin/"+branch)
	if res.Error != nil {
		return false, nil
	}
	return strings.TrimSpace(res.Output) != "0" && strings.TrimSpace(res.Output) != "", nil
}
func (g *Git) HasCommitsToPush(path, branch string) (bool, error) {
	if branch == "" {
		branch = "main"
	}
	res := g.Run(path, "rev-list", "--count", "origin/"+branch+"..HEAD")
	if res.Error != nil {
		return false, nil
	}
	return strings.TrimSpace(res.Output) != "0" && strings.TrimSpace(res.Output) != "", nil
}

// GetRepoStatus faz a checagem explícita do estado local/remoto antes do sync.
// Isso permite saber se precisa commitar, puxar do GitHub, fazer push ou não fazer nada.
func (g *Git) GetRepoStatus(path, branch string) (*RepoStatus, error) {
	st := &RepoStatus{Path: path, Branch: branch, IsRepository: IsRepository(path)}
	if !st.IsRepository {
		return st, nil
	}
	local, err := g.HasChanges(path)
	if err != nil {
		return st, err
	}
	st.HasLocalChanges = local
	if err := g.Fetch(path, branch); err != nil {
		return st, err
	}
	remote, _ := g.HasRemoteChanges(path, branch)
	push, _ := g.HasCommitsToPush(path, branch)
	st.HasRemoteChanges = remote
	st.HasCommitsToPush = push
	return st, nil
}

func (g *Git) AddAll(path string) error {
	res := g.Run(path, "add", ".")
	if res.Error != nil {
		return errors.New(res.Output)
	}
	return nil
}
func (g *Git) AddFile(path, file string) error {
	res := g.Run(path, "add", "--", filepath.ToSlash(file))
	if res.Error != nil {
		return errors.New(res.Output)
	}
	return nil
}
func (g *Git) Commit(path, message string) error {
	res := g.Run(path, "commit", "-m", message)
	if res.Error != nil {
		out := strings.ToLower(res.Output)
		if strings.Contains(out, "nothing to commit") || strings.Contains(out, "nada a confirmar") || strings.Contains(out, "nothing added to commit") {
			return nil
		}
		return errors.New(res.Output)
	}
	return nil
}
func (g *Git) PullRebase(path, branch string) error {
	if branch == "" {
		branch = "main"
	}
	if err := g.Fetch(path, branch); err != nil {
		return err
	}
	res := g.Run(path, "rebase", "origin/"+branch)
	if res.Error != nil {
		return errors.New(res.Output)
	}
	return nil
}
func (g *Git) Push(path, branch string) error {
	if branch == "" {
		branch = "main"
	}
	res := g.Run(path, "push", "origin", branch)
	if res.Error != nil {
		return errors.New(res.Output)
	}
	return nil
}
func (g *Git) IsFileTracked(repoPath, relativePath string) bool {
	return g.Run(repoPath, "ls-files", "--error-unmatch", "--", filepath.ToSlash(relativePath)).Error == nil
}
func (g *Git) RemoveCached(repoPath, relativePath string) error {
	res := g.Run(repoPath, "rm", "--cached", "--", filepath.ToSlash(relativePath))
	if res.Error != nil {
		out := strings.ToLower(res.Output)
		if strings.Contains(out, "pathspec") || strings.Contains(out, "did not match") || strings.Contains(out, "unmatch") {
			return nil
		}
		return errors.New(res.Output)
	}
	return nil
}
func (g *Git) ConfigureSafeDirectory(path string) error {
	res := g.RunGlobal("config", "--global", "--add", "safe.directory", filepath.ToSlash(path))
	if res.Error != nil {
		return errors.New(res.Output)
	}
	return nil
}
func (g *Git) AddSafeDirectory(path string) error { return g.ConfigureSafeDirectory(path) }

func (g *Git) EnsureUserIdentity(path, name, email string) error {
	currentName := strings.TrimSpace(g.Run(path, "config", "user.name").Output)
	currentEmail := strings.TrimSpace(g.Run(path, "config", "user.email").Output)
	if currentName == "" && strings.TrimSpace(name) != "" {
		if res := g.Run(path, "config", "user.name", name); res.Error != nil {
			return errors.New(res.Output)
		}
	}
	if currentEmail == "" && strings.TrimSpace(email) != "" {
		if res := g.Run(path, "config", "user.email", email); res.Error != nil {
			return errors.New(res.Output)
		}
	}
	currentName = strings.TrimSpace(g.Run(path, "config", "user.name").Output)
	currentEmail = strings.TrimSpace(g.Run(path, "config", "user.email").Output)
	if currentName == "" || currentEmail == "" {
		return fmt.Errorf("identidade Git ausente. Configure gitUserName/gitUserEmail no config.json ou rode git config user.name/user.email neste repositório")
	}
	return nil
}

func HasConflictMessage(output string) bool {
	t := strings.ToLower(output)
	for _, term := range []string{"conflict", "conflito", "merge conflict", "could not apply", "fix conflicts", "resolve all conflicts"} {
		if strings.Contains(t, term) {
			return true
		}
	}
	return false
}
func HasDubiousOwnershipMessage(output string) bool {
	t := strings.ToLower(output)
	return strings.Contains(t, "dubious ownership") || strings.Contains(t, "safe.directory") || strings.Contains(t, "does not record ownership")
}
func HasDubiousOwnership(output string) bool { return HasDubiousOwnershipMessage(output) }
