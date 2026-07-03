package main

import (
	"net/url"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestParseEnv(t *testing.T) {
	input := `# comment
KEY1=value1

KEY2=value with spaces
NO_EQUALS_LINE
`
	entries, err := parseEnv(input)
	if err != nil {
		t.Fatalf("parse inesperado: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("esperado 5 entradas, obteve %d", len(entries))
	}
	if c, ok := entries[0].(commentLine); !ok || c.raw != "# comment" {
		t.Errorf("entrada 0 deveria ser comentário: %T", entries[0])
	}
	if v, ok := entries[1].(variable); !ok || v.key != "KEY1" || v.value != "value1" {
		t.Errorf("entrada 1 incorreta: %T %+v", entries[1], entries[1])
	}
	if _, ok := entries[2].(blankLine); !ok {
		t.Errorf("entrada 2 deveria ser linha em branco: %T", entries[2])
	}
	if v, ok := entries[3].(variable); !ok || v.key != "KEY2" || v.value != "value with spaces" {
		t.Errorf("entrada 3 incorreta: %T %+v", entries[3], entries[3])
	}
	if c, ok := entries[4].(commentLine); !ok {
		t.Errorf("linha sem '=' deveria ser tratada como comentário: %T", entries[4])
	} else if c.raw != "NO_EQUALS_LINE" {
		t.Errorf("comentário preserva texto original: %q", c.raw)
	}
}

func TestParseEnvRejectsEmptyKey(t *testing.T) {
	input := "=value\n"
	_, err := parseEnv(input)
	if err == nil {
		t.Error("esperado erro para chave vazia")
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	input := `# comment
KEY1=value1

KEY2=value2
`
	entries, err := parseEnv(input)
	if err != nil {
		t.Fatal(err)
	}
	out := serialize(entries)
	if out != input {
		t.Errorf("roundtrip falhou.\nesperado:\n%s\nobteve:\n%s", input, out)
	}
}

func TestApplyForm(t *testing.T) {
	entries := []envLine{
		variable{key: "KEEP", value: "old"},
		variable{key: "UPDATE", value: "old"},
		variable{key: "DELETE", value: "old"},
	}
	form := url.Values{}
	form.Set("val_KEEP", "old")
	form.Set("val_UPDATE", "new")
	form.Add("del_DELETE", "1")
	form.Add("new_keys", "NEW")
	form.Add("new_values", "value")

	result, err := applyForm(entries, form)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Fatalf("esperado 3 entradas, obteve %d", len(result))
	}
	m := make(map[string]string)
	for _, e := range result {
		if v, ok := e.(variable); ok {
			m[v.key] = v.value
		}
	}
	if m["KEEP"] != "old" {
		t.Errorf("KEEP deveria permanecer 'old', obteve %q", m["KEEP"])
	}
	if m["UPDATE"] != "new" {
		t.Errorf("UPDATE deveria ser 'new', obteve %q", m["UPDATE"])
	}
	if _, ok := m["DELETE"]; ok {
		t.Error("DELETE deveria ter sido removido")
	}
	if m["NEW"] != "value" {
		t.Errorf("NEW deveria ser 'value', obteve %q", m["NEW"])
	}
}

func TestApplyFormRejectsEmptyNewKey(t *testing.T) {
	form := url.Values{}
	form.Add("new_keys", "")
	form.Add("new_values", "x")
	_, err := applyForm(nil, form)
	if err != nil {
		t.Errorf("chave vazia deve ser ignorada, não erro: %v", err)
	}
}

func TestAuthOKWithHash(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	cfg = settings{PasswordHash: string(hash)}
	staticPassword = ""

	if authOK("admin", "secret123") != true {
		t.Error("autenticação correta deveria passar")
	}
	if authOK("admin", "wrong") != false {
		t.Error("senha errada deveria falhar")
	}
	if authOK("other", "secret123") != false {
		t.Error("usuário errado deveria falhar")
	}
}

func TestAuthOKWithStaticPassword(t *testing.T) {
	staticPassword = "tmp123"
	cfg.PasswordHash = ""

	if authOK("admin", "tmp123") != true {
		t.Error("senha temporária correta deveria passar")
	}
	if authOK("admin", "wrong") != false {
		t.Error("senha temporária errada deveria falhar")
	}
}
