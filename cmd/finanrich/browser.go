package main

import (
	"os"
	"os/exec"
	"runtime"
)

// openBrowser abre a URL no navegador padrão do sistema.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// isDoubleClicked é uma aproximação: no Windows, quem abre pelo Explorer não
// tem a variável PROMPT (que só existe no cmd.exe).
func isDoubleClicked() bool {
	return runtime.GOOS == "windows" && os.Getenv("PROMPT") == ""
}
