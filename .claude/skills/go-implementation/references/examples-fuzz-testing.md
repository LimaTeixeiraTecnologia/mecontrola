# Exemplos: Fuzz Testing em Go

<!-- TL;DR
Exemplo dedicado de fuzz testing em Go. Carregar somente quando o usuario pedir fuzz testing,
fuzz test, fuzzing ou teste fuzz. O runtime de Go exige `func FuzzXxx(*testing.F)` top-level;
essa e uma excecao tecnica exclusiva ao registrador `suite.Run` de R4.
Keywords: fuzz, fuzzing, fuzz test, fuzz testing, parser, validador, propriedade
Load complete when: tarefa requer criar, revisar ou explicar fuzz tests em Go.
-->

## Regra de uso

Fuzz testing so deve ser usado quando houver input arbitrario relevante: parsers, validadores,
normalizadores, codecs, value objects ou funcoes puras com invariantes claras. Para testes comuns,
usar sempre o modelo canonico de `examples-testing.md`.

Excecao tecnica exclusiva: arquivos de fuzz continuam em `package <pacote>_test`, mas usam
`func FuzzXxx(f *testing.F)` porque o runtime de Go so descobre fuzz tests nesse formato. Nao usar
`func TestXxx(t *testing.T)` avulso para cenarios comuns.

## Fuzz test para parser/validador

```go
package valueobjects_test

import (
    "testing"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/<modulo>/domain/valueobjects"
)

func FuzzParseMoney(f *testing.F) {
    seeds := []string{
        "100.00",
        "0",
        "-1",
        "",
        "99999999.99",
        "not-a-number",
    }

    for _, seed := range seeds {
        f.Add(seed)
    }

    f.Fuzz(func(t *testing.T, input string) {
        money, err := valueobjects.ParseMoney(input)
        if err != nil {
            return
        }

        reparsed, err := valueobjects.ParseMoney(money.String())
        if err != nil {
            t.Fatalf("reparse money: %v", err)
        }

        if !money.Equal(reparsed) {
            t.Fatalf("round-trip mismatch: got %s, want %s", reparsed.String(), money.String())
        }
    })
}
```

## Validacao

- Rodar seed corpus: `go test -run=FuzzParseMoney ./internal/<modulo>/domain/valueobjects`.
- Rodar fuzz por tempo controlado: `go test -run=FuzzParseMoney -fuzz=FuzzParseMoney -fuzztime=30s ./internal/<modulo>/domain/valueobjects`.
- Manter seeds pequenos, deterministas e representativos.
- Nao fazer IO real, chamadas externas, sleeps ou dependencia de hora atual dentro de `f.Fuzz`.
