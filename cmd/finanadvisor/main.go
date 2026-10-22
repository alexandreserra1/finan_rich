//go:build js && wasm

// Comando finanadvisor: o consultor com IA para o app do celular sem
// computador (internal/askapi), num WebAssembly separado do finance.wasm
// porque leva a rede do Go. Registra o objeto global "finanAdvisor".
// O tools/mobilebuild compila e põe em mobile/www/static/advisor.wasm.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/alexandreserra1/finan_rich/internal/askapi"
)

func main() {
	// promise roda fn fora da tela (a rede do Go bloqueia) e devolve uma Promise.
	// Um panic vira erro na tela, em vez de deixar "Pensando…" para sempre.
	promise := func(fn func() string) js.Value {
		var executor js.Func
		executor = js.FuncOf(func(_ js.Value, args []js.Value) any {
			resolve := args[0]
			go func() {
				defer executor.Release()
				defer func() {
					if r := recover(); r != nil {
						b, _ := json.Marshal(map[string]string{"error": fmt.Sprint("erro interno do consultor: ", r)})
						resolve.Invoke(string(b))
					}
				}()
				resolve.Invoke(fn())
			}()
			return nil
		})
		return js.Global().Get("Promise").New(executor)
	}
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
			return promise(func() string { return askapi.Ask(context.Background(), req) })
		}),
		// checkKey testa a chave (uma pergunta mínima, sem dados): Promise também.
		"checkKey": js.FuncOf(func(_ js.Value, a []js.Value) any {
			ai := str(a, 0)
			return promise(func() string { return askapi.Check(context.Background(), ai) })
		}),
		// categorize: "IA sugere" da importação de extrato, sem computador.
		"categorize": js.FuncOf(func(_ js.Value, a []js.Value) any {
			req := str(a, 0)
			return promise(func() string { return askapi.Categorize(context.Background(), req) })
		}),
		"cloudPresets":    js.FuncOf(func(js.Value, []js.Value) any { return askapi.CloudPresets() }),
		"proposalApplied": js.FuncOf(func(_ js.Value, a []js.Value) any { return askapi.ProposalApplied(str(a, 0), str(a, 1)) }),
	}
	js.Global().Set("finanAdvisor", js.ValueOf(api))
	js.Global().Call("dispatchEvent", js.Global().Get("Event").New("finan-advisor-ready"))
	select {}
}
