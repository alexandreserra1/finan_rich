package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// openApp abre o finan_rich em janela própria, sem abas nem barra de
// endereço, como um aplicativo: usa o modo --app de um navegador baseado no
// Chromium (Edge vem no Windows; Chrome, Brave ou Chromium nos outros). Sem
// nenhum deles, abre no navegador padrão.
func openApp(url string) error {
	if exe := appBrowser(); exe != "" {
		cmd := exec.Command(exe, "--app="+url)
		detach(cmd) // o Ctrl+C no finan_rich não pode fechar o navegador junto
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
			return nil
		}
	}
	return openBrowser(url)
}

// appBrowser procura um navegador que tenha o modo --app.
func appBrowser() string {
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		for _, dir := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LOCALAPPDATA")} {
			if dir != "" {
				candidates = append(candidates,
					filepath.Join(dir, `Microsoft\Edge\Application\msedge.exe`),
					filepath.Join(dir, `Google\Chrome\Application\chrome.exe`),
					filepath.Join(dir, `BraveSoftware\Brave-Browser\Application\brave.exe`))
			}
		}
	case "darwin":
		for _, app := range []string{"Google Chrome", "Microsoft Edge", "Brave Browser", "Chromium"} {
			candidates = append(candidates, "/Applications/"+app+".app/Contents/MacOS/"+app)
		}
	default:
		for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
			if p, err := exec.LookPath(name); err == nil {
				candidates = append(candidates, p)
			}
		}
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

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
