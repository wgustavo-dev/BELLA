package startup

import (
	"fmt"
	"os/exec"
)

const runKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
const appName = "BELLA"

func InstallStartup(exePath string) error {
	cmd := exec.Command("reg", "add", runKey, "/v", appName, "/t", "REG_SZ", "/d", exePath, "/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("erro ao instalar inicialização automática: %s", string(out))
	}
	return nil
}

func UninstallStartup() error {
	cmd := exec.Command("reg", "delete", runKey, "/v", appName, "/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("erro ao remover inicialização automática: %s", string(out))
	}
	return nil
}
