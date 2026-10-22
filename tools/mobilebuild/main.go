// Command mobilebuild prepara as telas do app do celular (pasta mobile/www),
// que o Capacitor empacota no app Android/iOS.
//
//	go run ./tools/mobilebuild [pasta]   (padrão: mobile/www)
//
// Rode antes "go generate ./internal/web" para gerar o finance.wasm. O
// advisor.wasm (consultor sem computador) é compilado aqui.
package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/alexandreserra1/finan_rich/internal/web"
)

func main() {
	dir := "mobile/www"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := web.ExportApp(dir); err != nil {
		log.Fatalf("mobilebuild: %v", err)
	}
	// O consultor sem computador (IA na nuvem) só vai no app do celular.
	wasm := filepath.Join(dir, "static", "advisor.wasm")
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", wasm, "./cmd/finanadvisor")
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("mobilebuild: compilando o consultor: %v", err)
	}
	log.Printf("telas do app gravadas em %s", dir)
}
