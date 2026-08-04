# Registro de Decisao Arquitetural (ADR)

## Metadados

- **Titulo:** STT dedicado OpenRouter antes do runtime agentivo
- **Data:** 2026-08-04
- **Status:** Aceita
- **Decisores:** Engenharia Me Controla
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O PRD exige entrada de audio no WhatsApp com as mesmas funcionalidades do texto, sem novo agente,
sem fallback chain e sem audio bruto no modelo principal. O codebase atual tem `llm.Provider` com
chat, stream e embeddings em `internal/platform/llm/provider.go:5`, e OpenRouter sem endpoint STT em
`internal/platform/llm/openrouter.go:22`.

A documentacao oficial do OpenRouter oferece STT por `/api/v1/audio/transcriptions` e tambem audio
input em chat completions. O PRD escolheu STT dedicado para produzir texto canonico antes do
`AgentRuntime`.

## Decisao

Usar o endpoint dedicado de STT do OpenRouter antes do `HandleInbound`. O audio nunca sera enviado ao
chat completions do agente financeiro no primeiro corte. O resultado aprovado vira texto canonico; se
for incerto ou falhar, o fluxo termina com resposta textual segura.

## Alternativas Consideradas

- Audio direto em chat completions: rejeitado porque mistura transcricao e decisao financeira no mesmo
  prompt, aumenta custo e dificulta bloquear falso-sucesso.
- Provider STT fora do OpenRouter: rejeitado por violar provider unico.
- Novo agente de voz: rejeitado por duplicar tools, memoria, confirmacoes e scorers.

## Consequencias

### Beneficios Esperados

- Separacao clara entre transcricao tecnica e decisao financeira.
- Menor risco de alucinacao por audio incerto.
- Reuso integral do runtime, tools e workflows atuais.

### Trade-offs e Custos

- Uma chamada externa a mais antes do agente.
- Necessidade de benchmark de modelo STT antes da implementacao.

### Riscos e Mitigacoes

- Risco: modelo STT indisponivel ou caro.
- Mitigacao: benchmark versionado, timeout de 20s, budget maximo e fail-closed.

## Plano de Implementacao

1. Adicionar `llm.Transcriber`.
2. Implementar OpenRouter STT.
3. Integrar no use case de audio antes do `HandleInbound`.
4. Validar com unit tests, real STT flag e golden pareado.

## Monitoramento e Validacao

- `agents_audio_transcription_latency_seconds`
- `agents_audio_inbound_total{outcome,reason}`
- `agents_audio_cost_microusd_total`
- Gate: `0` chamada a `HandleInbound` em `TranscriptionUncertain`.

## Impacto em Documentacao e Operacao

Atualizar runbook de audio, `.env.example`, golden real e dashboard agentivo.

## Revisao Futura

Revisar quando o primeiro beta tiver dados reais de custo, latencia e incerteza por pelo menos uma
janela operacional completa.
