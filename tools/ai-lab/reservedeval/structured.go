package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/domain"
)

func jsonObject(raw json.RawMessage) (map[string]any, bool) {
	var object map[string]any
	var validated json.RawMessage
	if strictJSON(raw, &validated) != nil {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&object) != nil || object == nil {
		return nil, false
	}
	return object, true
}

func validCriterion(rule Criterion) bool {
	kinds := 0
	for _, present := range []bool{rule.Contains != "", rule.Forbids != "", rule.Calls != nil, rule.Proposals != nil} {
		if present {
			kinds++
		}
	}
	if kinds != 1 || len(rule.Contains)+len(rule.Forbids) > 512 {
		return false
	}
	if rule.Calls == nil && rule.Proposals == nil {
		return strings.TrimSpace(rule.Contains+rule.Forbids) != ""
	}
	if rule.Calls != nil {
		if len(*rule.Calls) > 64 {
			return false
		}
		for _, call := range *rule.Calls {
			if _, ok := jsonObject(call.Args); !ok || len(call.Args) > 4096 || !identifier.MatchString(call.Name) {
				return false
			}
		}
	}
	if rule.Proposals != nil {
		if len(*rule.Proposals) > 128 {
			return false
		}
		for _, proposal := range *rule.Proposals {
			record, ok := jsonObject(proposal.Record)
			if !ok || len(record) == 0 || len(proposal.Record) > 4096 ||
				(proposal.Action != advisor.ActionCreate && proposal.Action != advisor.ActionEdit && proposal.Action != advisor.ActionDelete) ||
				(proposal.Type != domain.TypeTransaction && proposal.Type != domain.TypeGoal && proposal.Type != domain.TypeDebt && proposal.Type != domain.TypeRecurring) {
				return false
			}
			if proposal.Action != advisor.ActionCreate {
				if id, ok := record["id"].(string); !ok || strings.TrimSpace(id) == "" {
					return false
				}
			}
			if len(proposal.Before) > 0 {
				before, ok := jsonObject(proposal.Before)
				if !ok || len(before) == 0 || len(proposal.Before) > 4096 ||
					(proposal.Action == advisor.ActionEdit && before["id"] != record["id"]) {
					return false
				}
			} else if proposal.Action == advisor.ActionEdit {
				return false
			}
		}
	}
	return true
}

func matchesObject(expected, actual json.RawMessage, subset bool) bool {
	want, validWant := jsonObject(expected)
	got, validGot := jsonObject(actual)
	if !validWant || !validGot || (!subset && len(want) != len(got)) {
		return false
	}
	for key, value := range want {
		actualValue, exists := got[key]
		if !exists || !reflect.DeepEqual(value, actualValue) {
			return false
		}
	}
	return true
}

func structuredPass(rule Criterion, calls []ExpectedCall, proposals []advisor.Proposal) bool {
	if rule.Calls != nil {
		if len(*rule.Calls) != len(calls) {
			return false
		}
		for index, want := range *rule.Calls {
			got := calls[index]
			if want.Name != got.Name || want.Error != got.Error || !matchesObject(want.Args, got.Args, false) {
				return false
			}
		}
		return true
	}
	if rule.Proposals != nil {
		if len(*rule.Proposals) != len(proposals) {
			return false
		}
		for index, want := range *rule.Proposals {
			got := proposals[index]
			if got.Applied || want.Action != got.Action || want.Type != got.Type || !matchesObject(want.Record, got.Record, true) ||
				(len(want.Before) > 0 && !matchesObject(want.Before, got.Before, true)) {
				return false
			}
		}
		return true
	}
	return false
}

func executorFailed(content string) bool {
	for _, prefix := range []string{"não deu", "argumentos inválidos", "ferramenta desconhecida", "essa ferramenta não é com você"} {
		if strings.HasPrefix(content, prefix) {
			return true
		}
	}
	return false
}
