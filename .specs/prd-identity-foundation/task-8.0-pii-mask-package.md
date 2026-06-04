# Tarefa 8.0: Pacote `internal/platform/observability/mask` + extensão de `PIIFields` no `piiHandler`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o pacote `internal/platform/observability/mask` com funções puras `WhatsApp(s string) string` e `Email(s string) string` que produzem máscaras parciais (`+5511****8888`, `a***@dominio.com`). Estender `internal/platform/observability/redaction.go` adicionando `"whatsapp_number"`, `"email"` e `"display_name"` em `PIIFields` para servir como rede de segurança contra log acidental do valor cru (ADR-004). Convenção de uso: módulo `identity` loga sob chave canônica com sufixo `_masked` (ex.: `"whatsapp_number_masked"`).

<requirements>
- RF-19: mascaramento aplicado em ponto único reutilizável.
- ADR-004: defesa em profundidade — pacote `mask` + handler global como segurança.
- `mask.WhatsApp("+5511988887777")` → `"+5511****8888"` (preserva `+55DD` + 4 últimos).
- `mask.WhatsApp("")` → `"***"` (sentinel para entrada vazia).
- `mask.WhatsApp(curto<8chars)` → `"***"` (resiliência a input degenerado).
- `mask.Email("alice@example.com")` → `"a***@example.com"`.
- `mask.Email("a@x.com")` → `"a***@x.com"` (preserva inicial mesmo em local-part de 1 char).
- `mask.Email("sem-arroba")` → `"***"`.
- `PIIFields` ganha `"whatsapp_number"`, `"email"`, `"display_name"` (lowercase, conforme convenção atual).
- Receiver single-letter para qualquer struct introduzida; preferir funções top-level para funções puras sem estado (excepção R1 documentada).
- Pacote `mask` não importa nada além de `strings`.
</requirements>

## Subtarefas

- [ ] 8.1 Criar `internal/platform/observability/mask/whatsapp.go` com função pura `WhatsApp(s string) string`.
- [ ] 8.2 Criar `internal/platform/observability/mask/email.go` com função pura `Email(s string) string`.
- [ ] 8.3 Criar `internal/platform/observability/mask/whatsapp_test.go` com `WhatsAppMaskSuite` table-driven cobrindo: entrada canônica E.164 BR, entrada com formato humano, vazio, curto, sem `+`, com mais de 13 dígitos.
- [ ] 8.4 Criar `internal/platform/observability/mask/email_test.go` com `EmailMaskSuite` table-driven: válido com domínio simples, com subdomínio (`a@mail.foo.com`), local-part 1 char, vazio, sem `@`, com mais de 1 `@` (toma o último).
- [ ] 8.5 Editar `internal/platform/observability/redaction.go` adicionando `"whatsapp_number"`, `"email"`, `"display_name"` à slice `PIIFields`.
- [ ] 8.6 Verificar comportamento do `piiHandler_test.go` (existente) — adicionar caso garantindo que `slog.String("whatsapp_number", "+5511...")` resulta em `[REDACTED]` (sem o `mask`).

## Detalhes de Implementação

Ver techspec §"Visão Geral dos Componentes" subseção `internal/platform/observability/mask/*` e ADR-004. Convenção do call site (a documentar no AGENTS.md do identity — task 10.0): `slog.String("whatsapp_number_masked", mask.WhatsApp(n))`.

## Critérios de Sucesso

- `go test -cover ./internal/platform/observability/mask/...` ≥ 100% nas duas funções.
- `mask.WhatsApp` e `mask.Email` produzem saída determinística e nunca panicam (não há fuzz obrigatório mas o test cobre input degenerado).
- `PIIFields` atualizado e o `piiHandler` redacta as 3 novas chaves para `[REDACTED]`.
- `mask` package não tem dependência externa além de `strings`.

## Definition of Done (DoD)

- [ ] `go test -cover ./internal/platform/observability/mask/...` reporta 100%.
- [ ] `go test ./internal/platform/observability/...` (suite global) passa.
- [ ] `grep -E 'whatsapp_number|email|display_name' internal/platform/observability/redaction.go` retorna as 3 entradas em `PIIFields`.
- [ ] `golangci-lint run ./internal/platform/observability/...` passa.
- [ ] `grep -rE 'import' internal/platform/observability/mask/*.go | grep -v 'strings'` retorna vazio.
- [ ] Test do `piiHandler` valida `whatsapp_number` redactado para `[REDACTED]`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit table-driven para `mask.WhatsApp` e `mask.Email`.
- [ ] Test de regressão no `piiHandler` cobrindo as 3 novas chaves.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/platform/observability/mask/whatsapp.go` (novo)
- `internal/platform/observability/mask/email.go` (novo)
- `internal/platform/observability/mask/whatsapp_test.go` (novo)
- `internal/platform/observability/mask/email_test.go` (novo)
- `internal/platform/observability/redaction.go` (alterado)
- `internal/platform/observability/pii_handler_test.go` (alterado — novos cenários)
