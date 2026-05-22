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

type Git struct {
	GitPath string
}

type CommandResult struct {
	Output string
	Error  error
}

func New(gitPath string) *Git {
	if strings.TrimSpace(gitPath) == "" {
		gitPath = "git"
	}

	return &Git{
		GitPath: gitPath,
	}
}

func (g *Git) Run(repoPath string, args ...string) CommandResult {
	cmd := exec.Command(g.GitPath, args...)
	cmd.Dir = repoPath

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := strings.TrimSpace(stdout.String())
	errorOutput := strings.TrimSpace(stderr.String())

	if errorOutput != "" {
		if output != "" {
			output += "\n"
		}
		output += errorOutput
	}

	return CommandResult{
		Output: output,
		Error:  err,
	}
}

func (g *Git) RunGlobal(args ...string) CommandResult {
	cmd := exec.Command(g.GitPath, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := strings.TrimSpace(stdout.String())
	errorOutput := strings.TrimSpace(stderr.String())

	if errorOutput != "" {
		if output != "" {
			output += "\n"
		}
		output += errorOutput
	}

	return CommandResult{
		Output: output,
		Error:  err,
	}
}

func (g *Git) IsGitInstalled() bool {
	cmd := exec.Command(g.GitPath, "--version")
	return cmd.Run() == nil
}

func IsRepository(path string) bool {
	gitDir := filepath.Join(path, ".git")

	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func EnsureFolder(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

func (g *Git) Clone(repoURL string, branch string, destination string) error {
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}

	parentDir := filepath.Dir(destination)

	if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
		return err
	}

	cmd := exec.Command(g.GitPath, "clone", "-b", branch, repoURL, destination)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		return fmt.Errorf("erro ao clonar repositório: %s", output)
	}

	return nil
}

func (g *Git) InitRepository(path string, repoURL string, branch string) error {
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}

	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}

	commands := [][]string{
		{"init"},
		{"branch", "-M", branch},
	}

	for _, args := range commands {
		result := g.Run(path, args...)
		if result.Error != nil {
			return fmt.Errorf("erro ao executar git %s: %s", strings.Join(args, " "), result.Output)
		}
	}

	remoteResult := g.Run(path, "remote")
	if remoteResult.Error == nil && strings.Contains(remoteResult.Output, "origin") {
		setURLResult := g.Run(path, "remote", "set-url", "origin", repoURL)
		if setURLResult.Error != nil {
			return fmt.Errorf("erro ao atualizar remote origin: %s", setURLResult.Output)
		}

		return nil
	}

	addRemoteResult := g.Run(path, "remote", "add", "origin", repoURL)
	if addRemoteResult.Error != nil {
		return fmt.Errorf("erro ao adicionar remote origin: %s", addRemoteResult.Output)
	}

	return nil
}

func (g *Git) HasChanges(path string) (bool, error) {
	result := g.Run(path, "status", "--porcelain")

	if result.Error != nil {
		return false, errors.New(result.Output)
	}

	return strings.TrimSpace(result.Output) != "", nil
}

func (g *Git) AddAll(path string) error {
	result := g.Run(path, "add", ".")

	if result.Error != nil {
		return errors.New(result.Output)
	}

	return nil
}

func (g *Git) Commit(path string, message string) error {
	result := g.Run(path, "commit", "-m", message)

	if result.Error != nil {
		output := strings.ToLower(result.Output)

		if strings.Contains(output, "nothing to commit") ||
			strings.Contains(output, "nada a confirmar") ||
			strings.Contains(output, "working tree clean") {
			return nil
		}

		return errors.New(result.Output)
	}

	return nil
}

func (g *Git) PullRebase(path string, branch string) error {
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}

	result := g.Run(path, "pull", "--rebase", "origin", branch)

	if result.Error != nil {
		return errors.New(result.Output)
	}

	return nil
}

func (g *Git) Push(path string, branch string) error {
	if strings.TrimSpace(branch) == "" {
		branch = "main"
	}

	result := g.Run(path, "push", "origin", branch)

	if result.Error != nil {
		return errors.New(result.Output)
	}

	return nil
}

func (g *Git) AddSafeDirectory(path string) error {
	normalized := filepath.ToSlash(path)
	result := g.RunGlobal("config", "--global", "--add", "safe.directory", normalized)

	if result.Error != nil {
		return errors.New(result.Output)
	}

	return nil
}

func HasConflictMessage(output string) bool {
	text := strings.ToLower(output)

	conflictTerms := []string{
		"conflict",
		"conflito",
		"merge conflict",
		"could not apply",
		"fix conflicts",
		"resolve all conflicts",
	}

	for _, term := range conflictTerms {
		if strings.Contains(text, term) {
			return true
		}
	}

	return false
}

func IsSafeDirectoryError(output string) bool {
	text := strings.ToLower(output)

	return strings.Contains(text, "dubious ownership") ||
		strings.Contains(text, "safe.directory") ||
		strings.Contains(text, "does not record ownership")
}
