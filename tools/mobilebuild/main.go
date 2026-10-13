// Command mobilebuild prepara as telas do app do celular (pasta mobile/www),
// que o Capacitor empacota no app Android/iOS.
//
//	go run ./tools/mobilebuild [pasta]   (padrão: mobile/www)
//
// Rode antes "go generate ./internal/web" para gerar o finance.wasm.
package main

import (
	"log"
	"os"

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
	log.Printf("telas do app gravadas em %s", dir)
}
