package ignore

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var DefaultIgnorePatterns = []string{".bella/", "*.tmp", "*.log", "*.cache", "*.zip", "*.rar", "*.iso", "*.mp4", "node_modules/", "dist/", "build/"}

type LargeFile struct {
	Path   string
	SizeMB float64
}

func EnsureGitignore(localFolder string, configPatterns []string) error {
	if err := os.MkdirAll(localFolder, os.ModePerm); err != nil {
		return err
	}
	gitignorePath := filepath.Join(localFolder, ".gitignore")
	existing := ""
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existing = string(data)
	}
	patterns := mergePatterns(DefaultIgnorePatterns, configPatterns)
	var b strings.Builder
	if strings.TrimSpace(existing) != "" {
		b.WriteString(strings.TrimRight(existing, "\n"))
		b.WriteString("\n\n")
	}
	if !containsLine(existing, "# B.E.L.L.A. ignores") {
		b.WriteString("# B.E.L.L.A. ignores\n")
	}
	for _, p := range patterns {
		if !containsLine(existing, p) {
			b.WriteString(p)
			b.WriteString("\n")
		}
	}
	return os.WriteFile(gitignorePath, []byte(b.String()), 0644)
}

// AddPatternsToGitignore adiciona arquivos específicos ao .gitignore.
// Usamos isso para arquivos grandes: eles ficam no disco, mas não entram no GitHub.
func AddPatternsToGitignore(repoFolder string, patterns []string) error {
	if err := os.MkdirAll(repoFolder, os.ModePerm); err != nil {
		return err
	}
	gitignorePath := filepath.Join(repoFolder, ".gitignore")
	existing := ""
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existing = string(data)
	}
	var b strings.Builder
	if strings.TrimSpace(existing) != "" {
		b.WriteString(strings.TrimRight(existing, "\n"))
		b.WriteString("\n\n")
	}
	header := "# B.E.L.L.A. large files ignored automatically"
	if !containsLine(existing, header) {
		b.WriteString(header)
		b.WriteString("\n")
	}
	seen := map[string]bool{}
	for _, p := range patterns {
		normalized := normalizeGitignorePath(p)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		if !containsLine(existing, normalized) {
			b.WriteString(normalized)
			b.WriteString("\n")
		}
	}
	return os.WriteFile(gitignorePath, []byte(b.String()), 0644)
}

func FindLargeFiles(root string, maxSizeBytes int64) ([]LargeFile, error) {
	var files []LargeFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if ShouldIgnorePath(path) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxSizeBytes {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				rel = path
			}
			files = append(files, LargeFile{Path: normalizeGitignorePath(rel), SizeMB: float64(info.Size()) / 1024 / 1024})
		}
		return nil
	})
	return files, err
}

// ShouldIgnorePath evita que watcher e verificadores mexam em arquivos técnicos da B.E.L.L.A., do Git e do Git portátil.
func ShouldIgnorePath(path string) bool {
	normalized := filepath.ToSlash(path)
	parts := strings.Split(normalized, "/")
	folders := []string{".git", ".bella", "node_modules", "dist", "build", "PortableGit"}
	for _, part := range parts {
		for _, folder := range folders {
			if strings.EqualFold(part, folder) {
				return true
			}
		}
	}
	lower := strings.ToLower(normalized)
	for _, ext := range []string{".tmp", ".log", ".cache", ".zip", ".rar", ".iso", ".mp4"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	base := filepath.Base(normalized)
	for _, file := range []string{"Bella.exe", "config.json", "config.installed.json", "config.portable.json", "BELLA_START.bat", "BELLA_SYNC_NOW.bat"} {
		if strings.EqualFold(base, file) {
			return true
		}
	}
	return false
}

func mergePatterns(defaults []string, custom []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, list := range [][]string{defaults, custom} {
		for _, p := range list {
			p = strings.TrimSpace(p)
			if p != "" && !seen[p] {
				seen[p] = true
				result = append(result, p)
			}
		}
	}
	return result
}

func containsLine(content, expected string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == strings.TrimSpace(expected) {
			return true
		}
	}
	return false
}
func shouldSkipDir(name string) bool {
	for _, skipped := range []string{".git", ".bella", "node_modules", "dist", "build", "PortableGit"} {
		if strings.EqualFold(name, skipped) {
			return true
		}
	}
	return false
}
func normalizeGitignorePath(path string) string {
	clean := strings.TrimSpace(path)
	clean = filepath.ToSlash(clean)
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")
	return clean
}
