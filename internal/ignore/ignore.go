package ignore

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var DefaultIgnorePatterns = []string{
	".bella/",
	"*.tmp",
	"*.log",
	"*.cache",
	"*.zip",
	"*.rar",
	"*.iso",
	"*.mp4",
	"node_modules/",
	"dist/",
	"build/",
}

type LargeFile struct {
	Path   string
	SizeMB float64
}

func EnsureGitignore(localFolder string, configPatterns []string) error {
	if err := os.MkdirAll(localFolder, os.ModePerm); err != nil {
		return err
	}

	gitignorePath := filepath.Join(localFolder, ".gitignore")

	existingContent := ""

	if data, err := os.ReadFile(gitignorePath); err == nil {
		existingContent = string(data)
	}

	patterns := mergePatterns(DefaultIgnorePatterns, configPatterns)

	var builder strings.Builder

	if strings.TrimSpace(existingContent) != "" {
		builder.WriteString(strings.TrimRight(existingContent, "\n"))
		builder.WriteString("\n\n")
	}

	if !containsLine(existingContent, "# B.E.L.L.A. ignores") {
		builder.WriteString("# B.E.L.L.A. ignores\n")
	}

	for _, pattern := range patterns {
		if !containsLine(existingContent, pattern) {
			builder.WriteString(pattern)
			builder.WriteString("\n")
		}
	}

	return os.WriteFile(gitignorePath, []byte(builder.String()), 0644)
}

func mergePatterns(defaults []string, custom []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, pattern := range defaults {
		normalized := strings.TrimSpace(pattern)

		if normalized == "" || seen[normalized] {
			continue
		}

		seen[normalized] = true
		result = append(result, normalized)
	}

	for _, pattern := range custom {
		normalized := strings.TrimSpace(pattern)

		if normalized == "" || seen[normalized] {
			continue
		}

		seen[normalized] = true
		result = append(result, normalized)
	}

	return result
}

func containsLine(content string, expectedLine string) bool {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == strings.TrimSpace(expectedLine) {
			return true
		}
	}

	return false
}

func FindLargeFiles(root string, maxSizeBytes int64) ([]LargeFile, error) {
	var largeFiles []LargeFile

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			name := entry.Name()

			if shouldSkipDir(name) {
				return filepath.SkipDir
			}

			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if info.Size() > maxSizeBytes {
			relativePath, err := filepath.Rel(root, path)
			if err != nil {
				relativePath = path
			}

			largeFiles = append(largeFiles, LargeFile{
				Path:   relativePath,
				SizeMB: float64(info.Size()) / 1024 / 1024,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return largeFiles, nil
}

func shouldSkipDir(name string) bool {
	skippedDirs := []string{
		".git",
		".bella",
		"node_modules",
		"dist",
		"build",
	}

	for _, skipped := range skippedDirs {
		if name == skipped {
			return true
		}
	}

	return false
}
