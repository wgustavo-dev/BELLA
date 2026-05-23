package pathresolver

import "path/filepath"

func Resolve(base, path string) (string, error) {
	if filepath.IsAbs(path) || path == "git" {
		return path, nil
	}
	return filepath.Abs(filepath.Join(base, path))
}
