package startup

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const runKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
const appName = "BELLA"

func InstallStartup(exePath string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	quotedPath := `"` + exePath + `"`

	cmd := exec.Command("reg", "add", runKey, "/v", appName, "/t", "REG_SZ", "/d", quotedPath, "/f")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func UninstallStartup() error {
	if runtime.GOOS != "windows" {
		return nil
	}

	cmd := exec.Command("reg", "delete", runKey, "/v", appName, "/f")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func IsStartupInstalled() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	cmd := exec.Command("reg", "query", runKey, "/v", appName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), appName)
}

func CurrentExecutablePath() (string, error) {
	return os.Executable()
}
