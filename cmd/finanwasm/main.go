//go:build js && wasm

// Comando finanwasm: as regras do finan_rich compiladas para WebAssembly.
// Registra o objeto global "finan" no navegador. Compile com:
//
//	go generate ./internal/web   (ou: make wasm)
package main

import (
	"syscall/js"

	"github.com/alexandreserra1/finan_rich/internal/wasmapi"
)

func main() {
	str := func(args []js.Value, i int) string {
		if i < len(args) {
			return args[i].String()
		}
		return ""
	}
	api := map[string]any{
		"dashboard":   js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.Dashboard(str(a, 0), str(a, 1)) }),
		"debts":       js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.Debts(str(a, 0)) }),
		"validate":    js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.Validate(str(a, 0), str(a, 1)) }),
		"parseMoney":  js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.ParseMoney(str(a, 0)) }),
		"formatMoney": js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.FormatMoney(int64(a[0].Int())) }),
		"monthLabel":  js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.MonthLabel(str(a, 0)) }),
		"defaultCategories": js.FuncOf(func(_ js.Value, a []js.Value) any {
			return wasmapi.DefaultCategories(int64(a[0].Float()))
		}),
		"export": js.FuncOf(func(_ js.Value, a []js.Value) any {
			return wasmapi.Export(str(a, 0), str(a, 1), int64(a[2].Float()))
		}),
		"readBackup":   js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.ReadBackup(str(a, 0)) }),
		"goals":        js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.Goals(str(a, 0), str(a, 1)) }),
		"recurringDue": js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.RecurringDue(str(a, 0), str(a, 1)) }),
		"alerts":       js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.Alerts(str(a, 0), str(a, 1)) }),
		"insights":     js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.Insights(str(a, 0), str(a, 1)) }),
		"importPreviewText": js.FuncOf(func(_ js.Value, a []js.Value) any {
			return wasmapi.ImportPreviewText(str(a, 0), str(a, 1), str(a, 2), str(a, 3))
		}),
		"accounts":     js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.Accounts(str(a, 0), str(a, 1)) }),
		"netWorth":     js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.NetWorth(str(a, 0), str(a, 1)) }),
		"installments": js.FuncOf(func(_ js.Value, a []js.Value) any { return wasmapi.Installments(str(a, 0), a[1].Int()) }),
		"importPreview": js.FuncOf(func(_ js.Value, a []js.Value) any {
			return wasmapi.ImportPreview(str(a, 0), str(a, 1), str(a, 2))
		}),
	}
	js.Global().Set("finan", js.ValueOf(api))
	js.Global().Call("dispatchEvent", js.Global().Get("Event").New("finan-ready"))
	select {} // mantém as funções vivas
}
