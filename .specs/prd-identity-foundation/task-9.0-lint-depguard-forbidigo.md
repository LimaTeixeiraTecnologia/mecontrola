# Tarefa 9.0: Lint depguard + forbidigo enforçando fronteiras e proibições

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar regras `depguard` (fronteiras hexagonais) e `forbidigo` (proibições materiais) no `.golangci.yml` existente, sem remover regras presentes. As regras enforçam: `domain` não importa `application`/`infrastructure`; `application` não importa `infrastructure`; ausência ativa de tokens `JWT|RBAC|role|is_admin` em `internal/identity/**/*.go` (RF-02 reforçado por análise estática); proibição de uso direto de `VO.String()` em chamadas de logger (ADR-003); proibição de imports/símbolos vetados (`internal/platform/uow`, `entities.IDGenerator`, `nullableString`, `writeError`/`writeJSON` locais) — gate de aderência ao runbook.

<requirements>
- RF-02: ausência ativa de atributo de autorização — `forbidigo` proíbe `JWT|RBAC|role|is_admin` no scope `internal/identity/`.
- RF-15: `depguard` impede: (a) `internal/identity/domain` importando `application` ou `infrastructure`; (b) `internal/identity/application` importando `infrastructure`. Mensagens explícitas citam a fronteira hexagonal.
- ADR-003: `forbidigo` proíbe `WhatsAppNumber.String()` e `Email.String()` em chamadas de logger (heurística — pega chamadas explícitas).
- Gate de aderência ao runbook: proibir `internal/platform/uow`, `entities.IDGenerator`/`idGen`, `nullableString`/`nullableEmail`, `func writeError`/`func writeJSON` em paths de identity.
- Regras existentes em `.golangci.yml` permanecem intactas.
- `golangci-lint run` deve passar verde no escopo alterado (`./internal/identity/...`).
</requirements>

## Subtarefas

- [ ] 9.1 Inspecionar `.golangci.yml` atual e identificar o bloco `linters-settings` (linhas 37–158 do working tree atual).
- [ ] 9.2 Adicionar regras `depguard.rules`:
  - `identity-domain-no-application`: `files: ["**/internal/identity/domain/**/*.go"]`, deny: `internal/identity/application`, `internal/identity/infrastructure` (F-11/RF-15).
  - `identity-application-no-infrastructure`: `files: ["**/internal/identity/application/**/*.go"]`, deny: `internal/identity/infrastructure`.
  - `identity-no-internal-platform-uow`: `files: ["**/internal/identity/**/*.go", "**/internal/platform/outbox/**/*.go"]`, deny: `internal/platform/uow` com mensagem "Consumir devkit-go/pkg/database/uow direto (R6.9)".
  - `identity-no-platform-id`: `files: ["**/internal/identity/**/*.go"]`, deny: `internal/platform/id` com mensagem "Domínio gera ID via entities.NewID() — sem IDGenerator (R6.8)".
- [ ] 9.3 Adicionar regras `forbidigo.forbid`:
  - `'WhatsAppNumber\)\.String\(\)'` — msg: "use WhatsAppNumber.Masked() em logs (ADR-003)".
  - `'Email\)\.String\(\)'` — msg: "use Email.Masked() em logs (ADR-003)".
  - `'\bJWT\b|\bRBAC\b|\brole\b|\bis_admin\b'` — msg: "atributos de autorização proibidos no MVP (RF-02)". Escopo: `internal/identity/`.
  - `'func\s+writeError\b|func\s+writeJSON\b'` — msg: "usar devkit-go/pkg/responses (R6.10)".
  - `'func\s+nullableString\b|func\s+nullableEmail\b'` — msg: "usar internal/platform/sqlnull.Str/Time (R6.11)".
- [ ] 9.4 Executar `golangci-lint run ./internal/identity/...` e `golangci-lint run ./internal/platform/...` — corrigir até verde.
- [ ] 9.5 Documentar em `.golangci.yml` (comentário no topo das novas regras) que elas são gates do runbook `docs/runbooks/handler-usecase-uow-repository.md` §13.

## Detalhes de Implementação

Referenciar:
- [`techspec.md` §12](./techspec.md) — snippet canônico das regras (linhas 985–1015 originais).
- [Runbook §13](../../docs/runbooks/handler-usecase-uow-repository.md) — checklist de aderência (cada item proibitivo tem regra correspondente).
- [ADR-003](./adr-003-pii-masking-vo-methods.md) — justificativa para `forbidigo` em logs.

**Bloco canônico em `.golangci.yml`:**

```yaml
linters-settings:
  depguard:
    rules:
      identity-domain-no-application:
        files:
          - "**/internal/identity/domain/**/*.go"
        deny:
          - pkg: github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application
            desc: "identity/domain não pode importar identity/application (F-11/RF-15)"
          - pkg: github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure
            desc: "identity/domain não pode importar identity/infrastructure (F-11/RF-15)"
      identity-application-no-infrastructure:
        files:
          - "**/internal/identity/application/**/*.go"
        deny:
          - pkg: github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure
            desc: "identity/application não pode importar identity/infrastructure (F-11/RF-15)"
      identity-no-internal-platform-uow:
        files:
          - "**/internal/identity/**/*.go"
          - "**/internal/platform/outbox/**/*.go"
        deny:
          - pkg: github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/uow
            desc: "Consumir devkit-go/pkg/database/uow direto (R6.9 — ADR-008)"
      identity-no-platform-id:
        files:
          - "**/internal/identity/**/*.go"
        deny:
          - pkg: github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id
            desc: "Domínio gera ID via entities.NewID() — sem IDGenerator injetado (R6.8)"

  forbidigo:
    forbid:
      - p: 'WhatsAppNumber\)\.String\(\)'
        msg: "use WhatsAppNumber.Masked() em logs (ADR-003)"
      - p: 'Email\)\.String\(\)'
        msg: "use Email.Masked() em logs (ADR-003)"
      - p: 'func\s+writeError\b'
        msg: "usar github.com/JailtonJunior94/devkit-go/pkg/responses (R6.10)"
      - p: 'func\s+writeJSON\b'
        msg: "usar github.com/JailtonJunior94/devkit-go/pkg/responses (R6.10)"
      - p: 'func\s+nullableString\b'
        msg: "usar internal/platform/sqlnull.Str (R6.11)"
      - p: 'func\s+nullableEmail\b'
        msg: "usar internal/platform/sqlnull.Str (R6.11)"
```

> **Nota sobre RF-02**: a verificação `grep -RInE "JWT|RBAC|\brole\b|is_admin" internal/identity/` permanece como gate manual em CA-03 (não embutida no forbidigo para não competir com casos legítimos — comentários históricos em prosa). A regra `forbidigo` cobre identificadores Go reais; a varredura textual cobre comentários.

## Critérios de Sucesso

- `golangci-lint run ./internal/identity/...` verde após 8.0 estar concluído.
- `golangci-lint run ./internal/platform/outbox/...` verde após 10.0 estar concluído.
- Regras existentes do `.golangci.yml` permanecem (nenhuma remoção).
- Teste sintético: introduzir temporariamente um import `internal/platform/id` em `internal/identity/module.go` e confirmar que `golangci-lint run` reporta a violação com a mensagem esperada; remover após validação.
- CA-02 (PRD) satisfeito: zero violações `depguard` em `internal/identity/`.
- CA-03 (PRD) satisfeito: `grep -RInE "JWT|RBAC|\brole\b|is_admin" internal/identity/**/*.go` retorna 0 matches (validado por execução manual aqui).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `golangci-lint run ./internal/identity/...` verde.
- [ ] Teste sintético de violação (manual): introduzir import banido, observar erro do linter, remover.
- [ ] Diff de `.golangci.yml` mostra apenas adições (regras existentes preservadas).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `.golangci.yml` (editar — adicionar regras, sem remover existentes)
- Dependências: 8.0 concluído (paths de `internal/identity/` precisam existir para o linter validar código real).
