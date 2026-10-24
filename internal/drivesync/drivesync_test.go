package drivesync

import (
	"encoding/json"
	"fmt"
	"testing"
)

func item(typ, id string, at int64, extra string) Item {
	return Item{Type: typ, Record: json.RawMessage(fmt.Sprintf(`{"id":%q,"updatedAt":%d%s}`, id, at, extra))}
}

func TestNeedsUpload(t *testing.T) {
	remote := []Item{item("transaction", "a", 10, ""), item("category", "c", 5, `,"deleted":true`)}
	cases := []struct {
		name  string
		local []Item
		want  bool
	}{
		{"igual ao arquivo", []Item{item("transaction", "a", 10, ""), item("category", "c", 5, `,"deleted":true`)}, false},
		{"campos em outra ordem, mesma data", []Item{{Type: "transaction", Record: json.RawMessage(`{"updatedAt":10,"id":"a","x":1}`)}}, false},
		{"mais antigo que o arquivo", []Item{item("transaction", "a", 9, "")}, false},
		{"mais novo aqui", []Item{item("transaction", "a", 11, "")}, true},
		{"registro novo aqui", []Item{item("goal", "g", 1, "")}, true},
		{"mesmo id, outro tipo", []Item{item("debt", "a", 10, "")}, true},
		{"apagado aqui depois", []Item{item("transaction", "a", 12, `,"deleted":true`)}, true},
		{"aparelho vazio", nil, false},
	}
	for _, c := range cases {
		if got := NeedsUpload(c.local, remote); got != c.want {
			t.Errorf("%s: NeedsUpload = %v, quer %v", c.name, got, c.want)
		}
	}
	if !NeedsUpload([]Item{item("transaction", "a", 1, "")}, nil) {
		t.Error("arquivo vazio (primeira vez): o aparelho com dados grava")
	}
}
