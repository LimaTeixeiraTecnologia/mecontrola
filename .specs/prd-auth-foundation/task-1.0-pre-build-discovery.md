# Tarefa 1.0: Pre-build discovery (framework de teste, headers, config canonical, user.deleted publish)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Esta tarefa **não escreve código de produção**. Realiza 4 descobertas obrigatórias que informam decisões de implementação das tarefas 2.0–10.0. Os achados MUST ser documentados em comentários no PR (ou em `docs/discoveries/technical-auth-foundation-mvp-whatsapp-llm/findings.md` se preferível) e usados como base nas tarefas seguintes.

<requirements>
- Detectar o framework de integration test em uso (`dockertest` vs `testcontainers-go`).
- Auditar headers HTTP atualmente lidos por handlers em `internal/*/infrastructure/http/server/handlers/**`.
- Identificar o pacote canônico de configuração (provavelmente `configs/`).
- Verificar se `MarkUserDeleted` em `internal/identity/application/usecases/mark_user_deleted.go` já publica `user.deleted` via outbox.
</requirements>

## Subtarefas

- [ ] 1.1 **PRE-01**: `cat migrations/migrations_integration_test.go | head -40`. Identificar `dockertest` vs `testcontainers-go`. Documentar decisão e padrão a seguir nos integration tests de 3.0, 4.0, 5.0, 7.0, 8.0.
- [ ] 1.2 **PRE-02**: `grep -rEn 'r\.Header\.(Get|Values)' internal/*/infrastructure/http/server/handlers/`. Listar headers legítimos não inclusos na allowlist `{X-Request-ID, Content-Type, Idempotency-Key}`. Documentar allowlist expandida com justificativa por header.
- [ ] 1.3 **PRE-03**: `ls internal/platform/config 2>/dev/null; ls configs/ 2>/dev/null; grep -rl "package config\|package configs" --include="*.go" .`. Identificar o pacote canônico. Apontar regra `forbidigo` em 2.0 para o pacote real (não presumido).
- [ ] 1.4 **PRE-04**: Ler `internal/identity/application/usecases/mark_user_deleted.go` por completo. Verificar se publica `outbox.Event{Type:"user.deleted"}`. Se sim, documentar payload atual; se não, registrar que 4.0 deve adicionar.

## Detalhes de Implementação

Ver techspec `## Sequenciamento de Desenvolvimento > Tarefas pre-build obrigatórias`.

## Critérios de Sucesso

- 4 descobertas documentadas em arquivo único de findings ou comentário no PR de 2.0.
- Cada descoberta tem **resposta concreta** (não "talvez", não "provavelmente"); se houver ambiguidade, registrar como item bloqueante.
- Allowlist expandida de headers tem justificativa de 1 linha por header.
- Estado de `user.deleted` em `MarkUserDeleted` é uma das duas: "já publica com payload X" ou "não publica — 4.0 adiciona".

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] N/A (tarefa de descoberta, sem código novo de produção)
- [ ] Os achados são re-verificáveis (comandos exatos no PR)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/migrations_integration_test.go` (leitura)
- `internal/*/infrastructure/http/server/handlers/**` (leitura via grep)
- `internal/platform/config/`, `configs/` (leitura via ls/grep)
- `internal/identity/application/usecases/mark_user_deleted.go` (leitura)
- Documento de findings: `docs/discoveries/technical-auth-foundation-mvp-whatsapp-llm/findings.md` (criar) ou comentário no PR de 2.0
