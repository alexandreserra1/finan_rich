// Package drivesync tem a regra da sincronização pelo Google Drive: cada pessoa
// tem um arquivo (finan_rich.json, na pasta de dados do app no Drive dela) com
// todos os registros, inclusive os apagados. Cada aparelho baixa o arquivo
// quando ele muda, aplica o que for mais novo (como em store.ApplyChanges) e,
// se tiver algo que o arquivo não tem, regrava o arquivo inteiro.
package drivesync

import "encoding/json"

// FileName é o nome do arquivo na pasta de dados do app (appDataFolder).
const FileName = "finan_rich.json"

// Item é um registro com o tipo, como vai na sincronização e no arquivo.
type Item struct {
	Type   string          `json:"type"`
	Record json.RawMessage `json:"record"`
}

// File é o conteúdo do arquivo no Drive.
type File struct {
	Version int    `json:"version"`
	Items   []Item `json:"items"`
}

type meta struct {
	ID        string `json:"id"`
	UpdatedAt int64  `json:"updatedAt"`
}

// NeedsUpload diz se o aparelho tem algum registro que falta no arquivo ou é
// mais novo que o de lá. Compara só o updatedAt: o navegador e o computador
// escrevem o mesmo registro com os campos em ordem diferente, e desempatar pelo
// texto faria os dois regravarem o arquivo um depois do outro, sem parar.
func NeedsUpload(local, remote []Item) bool {
	seen := make(map[string]int64, len(remote))
	for _, it := range remote {
		var m meta
		if json.Unmarshal(it.Record, &m) == nil {
			seen[it.Type+"/"+m.ID] = m.UpdatedAt
		}
	}
	for _, it := range local {
		var m meta
		if json.Unmarshal(it.Record, &m) != nil || m.ID == "" {
			continue
		}
		if at, ok := seen[it.Type+"/"+m.ID]; !ok || m.UpdatedAt > at {
			return true
		}
	}
	return false
}
