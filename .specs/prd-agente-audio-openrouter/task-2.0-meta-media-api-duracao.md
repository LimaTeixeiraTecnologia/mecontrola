# Tarefa 2.0: Cliente Meta Media API e duração determinística

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar fronteira fina para resolver URL de mídia e baixar áudio pela Meta Media API, com autenticação, timeout, limite de bytes, SHA-256 conferido, descarte do bruto e extração determinística de duração antes do STT.

<requirements>
- Cobrir RF-06, RF-07, RF-08 e RF-24.
- Baixar mídia apenas após autorização/dedup já feitos no fluxo WhatsApp.
- Rejeitar áudio acima de `AGENT_AUDIO_MAX_BYTES`.
- Rejeitar áudio acima de `AGENT_AUDIO_MAX_DURATION`.
- Rejeitar antes do STT quando a duração não puder ser determinada para `audio/ogg`/Opus ou M4A/AAC.
- Não persistir áudio bruto.
</requirements>

## Subtarefas

- [x] 2.1 Criar pacote `internal/platform/whatsapp/media` como adapter fino para Meta Media API.
- [x] 2.2 Implementar `Resolve(ctx, mediaID)` para `GET /{media-id}` com bearer token e timeout.
- [x] 2.3 Implementar `Download(ctx, url, maxBytes)` com limite `maxBytes + 1`, SHA-256 e descarte controlado.
- [x] 2.4 Implementar extração determinística de duração para `audio/ogg`/Opus e M4A/AAC, com menor dependência auditável possível quando necessário.
- [x] 2.5 Retornar erros tipados para media id inválido, URL ausente, download acima do limite, SHA mismatch, MIME não suportado e duração indeterminável.
- [x] 2.6 Criar testes unitários com servidor HTTP fake e fixtures pequenas.

## Detalhes de Implementação

Referenciar `techspec.md` nas seções `Contrato de Media Download`, `Contrato STT OpenRouter` e `Configuração`.

Evidências de codebase a respeitar:
- `internal/onboarding/infrastructure/http/client/meta/client.go:96`
- `deployment/compose/compose.yml:11`
- `.specs/prd-agente-audio-openrouter/whatsapp-audio-payload-evidence-2026-08-04.md`
- `.specs/prd-agente-audio-openrouter/stt-lote-30-results.json`

## Critérios de Sucesso

- Media API client não contém regra de negócio financeira.
- Download confere SHA-256 Meta vs bytes baixados.
- Áudio bruto não é persistido em disco, banco, log ou métrica.
- Duração ausente/indeterminável falha fechado antes de OpenRouter.
- Limite de bytes e duração são cobertos por testes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — O client alimenta o inbound agentivo sem reimplementar primitivos do runtime.
- `domain-modeling-production` — Erros técnicos e estados de rejeição devem ser modelados de forma fechada.
- `design-patterns-mandatory` — A fronteira deve permanecer adapter fino, sem pattern formal novo.

## Testes da Tarefa

- [x] `go test -race -count=1 ./internal/platform/whatsapp/media`
- [x] Teste de URL ausente, auth, timeout, maxBytes, MIME inválido e SHA mismatch.
- [x] Teste de duração para fixture `audio/ogg`/Opus.
- [x] Teste de duração para fixture M4A/AAC quando fixture existir.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/whatsapp/media/`
- `internal/platform/httpclient/`
- `.specs/prd-agente-audio-openrouter/stt-lote-30-results.json`
