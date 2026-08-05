# Tarefa 5.0: Auditoria Postgres e WAMID terminal

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar migration e repository de auditoria de áudio com outcome terminal único por WAMID, sem persistir áudio bruto. A tabela deve sustentar idempotência, troubleshooting e privacidade.

<requirements>
- Cobrir RF-21, RF-22, RF-23, RF-24, RF-25, RF-26 e RF-27.
- Criar migration `000015_agents_whatsapp_audio_messages`.
- Usar `PRIMARY KEY (wamid)` e `CHECK` para outcomes/reasons fechados.
- Inserir auditoria antes de download/STT quando o consumer identificar áudio.
- Não remover dedup para falhas terminalizadas de áudio.
- Não persistir áudio bruto, base64 ou URL temporária.
</requirements>

## Subtarefas

- [x] 5.1 Carregar `postgresql-production-standards` antes de editar migration ou schema.
- [x] 5.2 Criar migration up/down `000015` conforme PostgreSQL 16 confirmado.
- [x] 5.3 Implementar repository Postgres de auditoria com insert terminal por WAMID e update transacional curto.
- [x] 5.4 Modelar erros de duplicidade terminal e falha de persistência com `errors.Is`/`errors.As` quando necessário.
- [x] 5.5 Criar testes de integração de migration up/down, PK, checks e fluxo terminal.
- [x] 5.6 Adicionar grep/teste para impedir colunas ou arquivos de áudio bruto.

## Detalhes de Implementação

Referenciar `techspec.md` nas seções `Persistencia de Auditoria` e `Idempotencia e Outcome Terminal`.

Evidências de codebase a respeitar:
- `deployment/compose/compose.yml:11`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:190`
- `internal/platform/whatsapp/dedup/postgres/consumer_repository.go:44`
- `.specs/prd-agente-audio-openrouter/techspec.md`

## Critérios de Sucesso

- Mesmo WAMID tem exatamente um registro terminal de áudio.
- Falha de download/STT/validação não permite replay automático do mesmo WAMID.
- Repository não contém regra financeira nem lógica de STT.
- Migration é reversível e testada.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — A auditoria protege o inbound agentivo e a idempotência antes do runtime.
- `domain-modeling-production` — Outcomes e reasons persistidos devem refletir estados fechados.
- `design-patterns-mandatory` — A implementação deve permanecer repository direto, sem pattern formal novo.
- `postgresql-production-standards` — A tarefa cria migration, tabela, constraints e repository PostgreSQL.

## Testes da Tarefa

- [x] Teste de migration up/down.
- [x] Teste de PK por WAMID.
- [x] Teste de `CHECK outcome` e `CHECK reason`.
- [x] `go test -race -count=1 ./internal/agents/...`
- [x] `go test -race -count=1 ./internal/platform/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/infrastructure/persistence/`
- `internal/agents/infrastructure/repositories/postgres/`
- `migrations/`
- `deployment/compose/compose.yml`
