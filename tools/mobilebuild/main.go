// Command mobilebuild prepara as telas do app do celular (pasta mobile/www),
// que o Capacitor empacota no app Android/iOS.
//
//	go run ./tools/mobilebuild [pasta]         (padrão: mobile/www)
//	go run ./tools/mobilebuild -site [pasta]   (o site, ver site/build.sh)
//
// Rode antes "go generate ./internal/web" para gerar o finance.wasm. O
// advisor.wasm (consultor sem computador) é compilado aqui.
package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/alexandreserra1/finan_rich/internal/web"
)

func main() {
	site := flag.Bool("site", false, "telas do site (dados no navegador, sem pareamento)")
	flag.Parse()
	dir := "mobile/www"
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	if err := web.ExportApp(dir, *site); err != nil {
		log.Fatalf("mobilebuild: %v", err)
	}
	if *site { // a política (o Google exige o link) e a página de baixar o app
		for _, name := range []string{"privacidade.html", "baixar.html"} {
			b, err := os.ReadFile(filepath.Join("site", name))
			if err == nil {
				err = os.WriteFile(filepath.Join(dir, name), b, 0o644)
			}
			if err != nil {
				log.Fatalf("mobilebuild: %s: %v", name, err)
			}
		}
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
