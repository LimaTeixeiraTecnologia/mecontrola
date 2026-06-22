# Input DTO Validate — Regra Obrigatória

- Rule ID: R-DTO-VALIDATE-001
- Severidade: hard
- Escopo: `internal/<modulo>/application/dtos/input/`

## Objetivo

Garantir que toda struct de input DTO possua um ponto único e previsível de validação na fronteira da aplicação, antes de qualquer IO ou construção de command.

## Regras Hard

### R-DTO-001 — Todo input DTO deve ter Validate() [HARD]

Todo struct em `internal/<modulo>/application/dtos/input/` DEVE ter método `Validate() error`.

```go
func (i *InputType) Validate() error {
    var errs []error
    // checks...
    return errors.Join(errs...)
}
```

Requisitos obrigatórios:
- `errors.Join` para coletar TODOS os erros simultaneamente
- Mensagem de erro DEVE nomear o campo: `fmt.Errorf("field_name: %w", err)` ou `errors.New("field_name: mensagem")`
- Receiver pointer `*T`
- Zero comentários (R-ADAPTER-001.1)
- Puro: sem IO, sem `context.Context` (DMMF Princípio 6)
- Delegar a VO smart constructors quando existirem (DMMF Princípio 1)

### R-DTO-002 — Use case DEVE chamar Validate() logo após abertura do span [HARD]

```go
func (uc *Foo) Execute(ctx context.Context, in input.FooInput) (output.FooOutput, error) {
    ctx, span := uc.o11y.Tracer().Start(ctx, "module.usecase.foo")
    defer span.End()

    if err := in.Validate(); err != nil {
        return output.FooOutput{}, err
    }
    ...
```

Posição: **imediatamente após** `defer span.End()`, antes de command construction e qualquer IO.
Motivo: o span deve estar aberto para que o erro de validação apareça no trace.

### R-DTO-003 — Não duplicar validação semântica de enum [HARD]

DTOs com campos Raw (ex: `transactions/Raw*`) validam apenas superfície:
- Strings não-vazias para campos obrigatórios
- Numéricos > 0 para amounts
- `time.Time` não-zero quando obrigatório

Validação semântica de enum (Direction, PaymentMethod, Frequency) permanece nos command constructors.

### R-DTO-004 — Não duplicar whitelists do use case no DTO [HARD]

Whitelists de valores válidos que vivem no use case (ex: gatewayReasons) NÃO devem ser replicados no DTO. Risco de divergência em manutenção. Essa validação permanece no use case.

## Gate de Verificação

```bash
# Todo input DTO deve ter Validate()
for f in $(find internal -path "*/application/dtos/input/*.go" ! -name "*_test.go" ! -name "errors.go"); do
  grep -q "func.*Validate().*error" "$f" || echo "FAIL: sem Validate() em $f"
done

# Zero comentários
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "^[[:space:]]*//" internal/*/application/dtos/input/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentários proibidos" || true
```

## Exceções

- Structs de query/filter com todos os campos opcionais e já tipados como VOs podem ter `Validate()` que retorna `nil` (ex: `AlertQuery`)
- DTOs construídos exclusivamente internamente pelo use case (não recebidos de adapter) são isentos

## Referências

- DMMF Princípio 1: smart constructors — `.agents/skills/go-implementation/references/domain-modeling.md`
- DMMF Princípio 5: workflow pipeline — mesma referência
- R-ADAPTER-001.1: zero comentários — `.claude/rules/go-adapters.md`
- R-TXN-002: validação em smart constructors — `.claude/rules/transactions-workflows.md`
