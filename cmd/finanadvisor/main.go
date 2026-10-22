//go:build js && wasm

// Comando finanadvisor: o consultor com IA para o app do celular sem
// computador (internal/askapi), num WebAssembly separado do finance.wasm
// porque leva a rede do Go. Registra o objeto global "finanAdvisor".
// O tools/mobilebuild compila e põe em mobile/www/static/advisor.wasm.
package main

import (
	"context"
	"syscall/js"

	"github.com/alexandreserra1/finan_rich/internal/askapi"
)

func main() {
	str := func(args []js.Value, i int) string {
		if i < len(args) {
			return args[i].String()
		}
		return ""
	}
	api := map[string]any{
		// ask chama a IA pela rede: devolve uma Promise (a tela não pode travar).
		"ask": js.FuncOf(func(_ js.Value, a []js.Value) any {
			req := str(a, 0)
			return js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, args []js.Value) any {
				resolve := args[0]
				go func() { resolve.Invoke(askapi.Ask(context.Background(), req)) }()
				return nil
			}))
		}),
		"cloudPresets":    js.FuncOf(func(js.Value, []js.Value) any { return askapi.CloudPresets() }),
		"proposalApplied": js.FuncOf(func(_ js.Value, a []js.Value) any { return askapi.ProposalApplied(str(a, 0), str(a, 1)) }),
	}
	js.Global().Set("finanAdvisor", js.ValueOf(api))
	js.Global().Call("dispatchEvent", js.Global().Get("Event").New("finan-advisor-ready"))
	select {}
}
