// Comando wasmbuild compila cmd/finanwasm para WebAssembly e copia o
// wasm_exec.js da sua instalação do Go (precisam ser da mesma versão).
// Funciona em Windows, macOS e Linux, de qualquer pasta do projeto:
//
//	go run ./tools/wasmbuild   ou   go generate ./internal/web
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const outDir = "internal/web/static"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "wasmbuild:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := chdirModuleRoot(); err != nil {
		return err
	}
	wasm := filepath.Join(outDir, "finance.wasm")
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", wasm, "./cmd/finanwasm")
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return err
	}
	root := strings.TrimSpace(string(goroot))
	src := filepath.Join(root, "lib", "wasm", "wasm_exec.js")
	if _, err := os.Stat(src); err != nil {
		src = filepath.Join(root, "misc", "wasm", "wasm_exec.js") // Go < 1.24
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "js", "wasm_exec.js"), b, 0o644); err != nil {
		return err
	}
	info, _ := os.Stat(wasm)
	fmt.Printf("ok: %s (%.1f MB, %s)\n", wasm, float64(info.Size())/1e6, runtime.Version())
	return nil
}

// chdirModuleRoot sobe as pastas até achar o go.mod (go generate roda na pasta
// do pacote, não na raiz).
func chdirModuleRoot() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return os.Chdir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return fmt.Errorf("go.mod não encontrado")
		}
		dir = parent
	}
}
