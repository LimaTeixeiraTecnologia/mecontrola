# Tarefa 9.0: Beacon de jornada (`page_opened`/`whatsapp_opened`)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o endpoint beacon que registra os timestamps client-side da jornada, fora do caminho de leitura `GET /state`.

<requirements>
- RF-35: registrar `page_opened_at` e `whatsapp_opened_at` (set-once-if-null).
- Endpoint `POST /api/v1/onboarding/tokens/{token}/opened`, rate-limited, valida token, responde `204` mesmo em token inválido (não vaza estado).
- Não gravar timestamps em `GET /state` (sem escrita em leitura).
</requirements>

## Subtarefas

- [ ] 9.1 Criar o handler `POST /tokens/{token}/opened` (adapter fino) + rota no router público do onboarding, reusando `middleware.NewRateLimiter`.
- [ ] 9.2 Criar usecase `RecordJourneyTimestamp` que aplica set-once-if-null em `page_opened_at`/`whatsapp_opened_at` conforme o discriminador do body (`{event:"page_opened"|"whatsapp_opened"}`).
- [ ] 9.3 Adicionar métodos idempotentes de repo (`MarkPageOpened`/`MarkWhatsAppOpened`).

## Detalhes de Implementação

Ver techspec.md, "Endpoints de API" (beacon) e "Modelos de Dados" (colunas). Resposta `204` sempre; token inválido não revela estado.

## Critérios de Sucesso

- Beacon grava `page_opened_at`/`whatsapp_opened_at` uma única vez (set-once-if-null).
- Token inválido → `204` sem vazar estado; rate limit aplicado.
- `GET /state` permanece sem escrita.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unitários do handler/usecase (set-once-if-null, token inválido → 204, discriminador).
- [ ] Integração (testcontainers) da gravação idempotente.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/infrastructure/http/server/handlers/` (novo handler do beacon)
- `internal/onboarding/infrastructure/http/server/router.go`
- `internal/onboarding/application/usecases/record_journey_timestamp.go` (novo)
- `internal/onboarding/infrastructure/repositories/postgres/magic_token_repository.go`
