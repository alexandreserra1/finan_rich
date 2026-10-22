package domain

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Tipos de registro sincronizados.
const (
	TypeCategory    = "category"
	TypeTransaction = "transaction"
	TypeDebt        = "debt"
	TypePref        = "pref"
	TypeNote        = "note"
	TypeGoal        = "goal"
	TypeRecurring   = "recurring"
	TypeAccount     = "account"
)

// ErrInvalidRecord indica um registro com tipo ou conteúdo inválido.
var ErrInvalidRecord = errors.New("registro inválido")

type validator interface{ Validate() error }

// DecodeRecord lê um registro em JSON conforme o tipo, valida e devolve seus
// metadados e o JSON normalizado (só os campos conhecidos). É usado pelo
// servidor ao receber uma sincronização e pelo navegador antes de salvar.
func DecodeRecord(typ string, raw []byte) (Meta, []byte, error) {
	var (
		meta Meta
		v    validator
		err  error
	)
	switch typ {
	case TypeCategory:
		var r Category
		err, meta, v = json.Unmarshal(raw, &r), r.Meta, &r
	case TypeTransaction:
		var r Transaction
		err, meta, v = json.Unmarshal(raw, &r), r.Meta, &r
	case TypeDebt:
		var r Debt
		err, meta, v = json.Unmarshal(raw, &r), r.Meta, &r
	case TypePref:
		var r Pref
		err, meta, v = json.Unmarshal(raw, &r), r.Meta, &r
	case TypeNote:
		var r Note
		err, meta, v = json.Unmarshal(raw, &r), r.Meta, &r
	case TypeGoal:
		var r Goal
		err, meta, v = json.Unmarshal(raw, &r), r.Meta, &r
	case TypeRecurring:
		var r Recurring
		err, meta, v = json.Unmarshal(raw, &r), r.Meta, &r
	case TypeAccount:
		var r Account
		err, meta, v = json.Unmarshal(raw, &r), r.Meta, &r
	default:
		return meta, nil, fmt.Errorf("%w: tipo %q", ErrInvalidRecord, typ)
	}
	if err != nil {
		return meta, nil, fmt.Errorf("%w: %v", ErrInvalidRecord, err)
	}
	meta = metaOf(v)
	if err := v.Validate(); err != nil {
		return meta, nil, fmt.Errorf("%w: %v", ErrInvalidRecord, err)
	}
	data, err := json.Marshal(v)
	return meta, data, err
}

func metaOf(v validator) Meta {
	switch r := v.(type) {
	case *Category:
		return r.Meta
	case *Transaction:
		return r.Meta
	case *Debt:
		return r.Meta
	case *Pref:
		return r.Meta
	case *Note:
		return r.Meta
	case *Goal:
		return r.Meta
	case *Recurring:
		return r.Meta
	case *Account:
		return r.Meta
	}
	return Meta{}
}
