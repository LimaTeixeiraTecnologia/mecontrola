# Runbook — Agente de Áudio WhatsApp/STT (OpenRouter)

- PRD: `.specs/prd-agente-audio-openrouter/prd.md` (RF-28, RF-29, RF-46)
- TechSpec: `.specs/prd-agente-audio-openrouter/techspec.md` (seções `Monitoramento e Observabilidade`,
  `Alertas iniciais`, `Riscos e Mitigacoes`)
- ADRs: `.specs/prd-agente-audio-openrouter/adr-001-stt-dedicado-openrouter.md`,
  `adr-002-outcome-terminal-wamid-audio.md`, `adr-003-auditoria-sem-audio-bruto.md`,
  `adr-004-nao-aplicar-pattern-formal.md`
- Código: `internal/agents/application/usecases/process_audio_inbound.go` (orquestração),
  `internal/agents/application/usecases/decide_audio_transcription.go` (decisor puro),
  `internal/agents/infrastructure/persistence/audio_audit_repository.go` (auditoria),
  `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
  (integração no consumer), `internal/platform/whatsapp/media/` (Meta Media API),
  `internal/platform/llm/openrouter_stt.go` (transcrição OpenRouter)
- Migration: `migrations/000015_agents_whatsapp_audio_messages.up.sql`
- Dashboard: `deployment/dashboards/agent-audio-whatsapp.json`
- Alertas: `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` (grupo `audio`)
- Golden set: `internal/agents/application/golden/audio_case.go`,
  `audio_pipeline_test.go`, `harness_audio_realllm_test.go`
- Evidência de produção sanitizada: `.specs/prd-agente-audio-openrouter/prod-evidence-2026-08-04.md`,
  `.specs/prd-agente-audio-openrouter/whatsapp-audio-payload-evidence-2026-08-04.md`,
  `.specs/prd-agente-audio-openrouter/benchmark-stt.md`

## 1. Visão geral do fluxo (para triagem sem acesso a áudio bruto)

```text
WhatsApp webhook (type=audio)
  -> payload.ExtractMessages (Audio{MediaID, MimeType, SHA256, Voice})
  -> WhatsAppInboundConsumer.handleAudio
     -> ProcessAudioInbound.Execute
        1. checkExisting: idempotência por WAMID (auditoria já terminal? reenvia a mesma resposta)
        2. fetchAudio: Resolve (Meta Media API) -> Download (maxBytes) -> VerifyChecksum (SHA-256)
           -> DetermineDuration -> CheckMaxDuration -> audioFormatFromMime
        3. transcribeAndDecide: transcriber.Transcribe (OpenRouter STT)
           -> DecideAudioTranscription (decisor puro) -> outcome/reason
        4. terminal: persiste auditoria (PK wamid), registra métricas, loga outcome agregado
  -> se outcome == "approved": texto canônico segue para o pipeline textual existente
     (tryResume / onboarding / HandleInbound) — SEM diferença de tratamento vs texto digitado
  -> qualquer outro outcome: resposta textual segura de reenvio, SEM HandleInbound, SEM tool call
```

Nenhum passo desta triagem exige ouvir ou acessar o áudio bruto: o áudio nunca é persistido (ADR-003).
Toda investigação usa métricas, logs estruturados (outcome/reason/duration/size/model, sem texto
completo) e a tabela de auditoria `mecontrola.agents_whatsapp_audio_messages` (transcrição só quando
`outcome = 'approved'`, nunca o áudio em si).

### 1.1 Estados fechados (para ler outcome/reason sem ambiguidade)

| `outcome` | Significado | Chama `HandleInbound`? |
|---|---|---|
| `approved` | Transcrição aprovada; vira texto canônico | Sim (pipeline textual normal) |
| `rejected` | Falha técnica antes/durante download ou validação | Não |
| `transcription_uncertain` | STT respondeu, mas sinal técnico é fraco | Não |
| `transcription_failed` | STT retornou erro/timeout/indisponibilidade | Não |
| `dispatched` | Marca informativa pós-`approved` (já processado, resposta idempotente) | N/A (estado terminal informativo) |

| `reason` | Quando ocorre |
|---|---|
| `approved` | Aprovação; usado com `outcome=approved`/`dispatched` |
| `media_unavailable` | Falha ao resolver/baixar mídia na Meta Media API, ou SHA-256 não confere |
| `size_exceeded` | Download excedeu `AGENT_AUDIO_MAX_BYTES` |
| `duration_unavailable` | Duração não determinável (inclui MIME não suportado) |
| `duration_exceeded` | Duração acima de `AGENT_AUDIO_MAX_DURATION` |
| `invalid_payload` | `AGENT_AUDIO_MAX_BYTES` não configurado ou payload inválido |
| `cost_exceeded` | Preflight ou pós-STT excedeu `AGENT_AUDIO_MAX_COST_MICROUSD` |
| `stt_error` | Erro de upstream do OpenRouter STT (timeout, 4xx/5xx, modelo indisponível) |
| `truncated` | Provider sinalizou corte ou resposta incompleta |
| `empty_text` | Texto vazio, só pontuação ou repetição ininteligível |
| `incoherent` | Texto tecnicamente incoerente (menos de 3 caracteres alfanuméricos) |
| `language_unsupported` | Idioma diferente de `pt` (ou não detectável) |
| `low_confidence` | `Confidence` retornado pelo provider abaixo de `AGENT_AUDIO_MIN_CONFIDENCE` |

## 2. Feature flag e configuração

| Campo | Env | Default | Observação |
|---|---|---|---|
| `AudioEnabled` | `AGENT_AUDIO_ENABLED` | `false` | Gate mestre. `false` = consumer NÃO injeta `audioProcessor`; qualquer áudio recebido responde `"entrada por áudio ainda não está disponível por aqui, pode digitar sua mensagem?"` sem tocar Meta Media API nem OpenRouter (`internal/agents/module.go:391`, `whatsapp_inbound_consumer.go:251-252`). |
| `STTModel` | `AGENT_STT_MODEL` | vazio | Obrigatório em produção quando `AudioEnabled=true`. |
| `STTTimeout` | `AGENT_STT_TIMEOUT` | `20s` | `1s..20s`; obrigatório (>0) em produção quando habilitado. |
| `AudioMaxDuration` | `AGENT_AUDIO_MAX_DURATION` | `60s` | `1s..60s`. |
| `AudioMaxBytes` | `AGENT_AUDIO_MAX_BYTES` | sem default de produção | Obrigatório (>0) quando habilitado. |
| `AudioMaxCostMicrousd` | `AGENT_AUDIO_MAX_COST_MICROUSD` | sem default de produção | Obrigatório (>0) quando habilitado; teto inicial recomendado `2000` microusd/áudio (benchmark-stt.md). |
| `AudioMinConfidence` | `AGENT_AUDIO_MIN_CONFIDENCE` | `0.80` | **Controle INATIVO na prática.** Nenhum modelo STT do OpenRouter retorna `confidence` (verificado 2026-08-05 nos 5 modelos do benchmark); o código só aplica o piso quando o provider fornece o campo. Mantido para o caso de um provider passar a retornar. Não contar como proteção ativa. |
| `AudioUncertainReply` | `WA_MSG_AUDIO_UNCERTAIN_RETRY` | mensagem PT-BR curta | Obrigatório não vazio em produção quando habilitado. |
| `AudioRejectedReply` | `WA_MSG_AUDIO_REJECTED_RETRY` | mensagem PT-BR curta | Obrigatório não vazio em produção quando habilitado. |

Validação de produção (`configs/config.go`) falha o boot se `AGENT_AUDIO_ENABLED=true` e qualquer
campo obrigatório acima estiver vazio/zero — não é possível subir com configuração parcial.

### 2.1 Limitação conhecida: detecção de idioma não é um controle ativo

Verificação empírica de 2026-08-05 (review) com áudio PT-BR real contra os 5 modelos STT do
benchmark — `openai/whisper-large-v3`, `openai/gpt-4o-transcribe`, `openai/gpt-4o-mini-transcribe`,
`mistralai/voxtral-mini-transcribe`, `deepgram/nova-3` — mostrou que **nenhum** retorna o campo
`language` na resposta (todos devolvem `language=""` com transcrição correta). Trocar de modelo não
resolve.

Como consequência, `buildTranscriptionResponse` assume o idioma enviado no request (`pt`) quando o
provider omite o campo. Isso mantém a feature operante, mas significa que:

- o motivo `language_unsupported` **não ocorre em produção** para áudio real;
- áudio em outro idioma é transcrito à força como PT-BR pelo provider e a proteção efetiva passa a
  ser os gates de **texto vazio**, **incoerência** e **truncamento** (esses sim ativos e testados);
- RF-13/RF-14 estão formalmente emendados no PRD; não declarar detecção de idioma como atendida.

Se o alerta `mc-audio-transcription-uncertain-rate` disparar, **não** investigar idioma primeiro:
os motivos plausíveis em produção são `empty_text`, `incoherent` e `truncated`.
Regressão coberta por `TestRealSTT_Transcribe` e `TestGoldenAudioRealSTT` (ambos `-tags=integration`).

## 3. Triagem por sintoma

### 3.1 Falha de download (Meta Media API)

**Sintoma:** `agents_audio_inbound_total{outcome="rejected", reason="media_unavailable"}` ou
`reason="size_exceeded"` crescendo; `agents_audio_download_latency_seconds{outcome="error"}` com
amostras.

**Investigação:**

1. Confirmar taxa e volume:
   ```promql
   sum by (reason) (rate(agents_audio_inbound_total{outcome="rejected"}[15m]))
   sum(rate(agents_audio_download_latency_seconds_count{outcome="error"}[15m]))
   ```
2. Verificar logs estruturados (`WARN`) do consumer/usecase — campos disponíveis: `outcome`,
   `reason`, `mime_family`, `duration_ms`, `size_bytes`, `error_code`. Nunca contêm URL de mídia,
   token ou bytes do áudio:
   ```logql
   {service_name=~"mecontrola-.+"} | json | outcome=~"rejected" | reason=~"media_unavailable|size_exceeded"
   ```
3. `error_code` possíveis (ver `classifyMediaDownloadError` em `process_audio_inbound.go`):
   `media_resolve_failed`, `media_auth_failed` (token Meta inválido/expirado —
   verificar `META_ACCESS_TOKEN`), `media_server_error` (5xx da Meta), `media_bad_request`,
   `media_url_missing`, `sha256_mismatch` (integridade — investigar proxy/CDN intermediário),
   `media_download_failed` (genérico).
4. Correlacionar com traces no Tempo pelo span `agents.usecase.process_audio_inbound`.
5. Se `media_auth_failed` sustentado: verificar validade/rotação de `META_ACCESS_TOKEN`
   (mesmo token usado pelo client Meta existente, `internal/onboarding/infrastructure/http/client/meta`).

**Não é regressão de código se:** taxa de `media_unavailable` acompanha instabilidade conhecida da
Meta Graph API (5xx transitório) — auditoria com PK `wamid` garante que o mesmo WAMID não é
reprocessado automaticamente (RF-21/RF-22/RF-23); o usuário reenvia manualmente.

### 3.2 Falha STT

**Sintoma:** alerta `mc-audio-stt-error-rate` (`stt_error_rate_15m > 5%`) disparado; outcome
`transcription_failed` com `reason="stt_error"`.

**Investigação:**

1. ```promql
   100 * sum(rate(agents_audio_inbound_total{outcome="transcription_failed"}[15m]))
     / clamp_min(sum(rate(agents_audio_inbound_total[15m])), 1)
   ```
2. `error_code` possíveis (`classifySTTError`): `stt_timeout` (acima de `AGENT_STT_TIMEOUT`),
   `stt_model_unavailable` (modelo `AGENT_STT_MODEL` indisponível no OpenRouter),
   `stt_empty_text`, `stt_invalid_response`, `stt_upstream_error` (4xx/5xx/rate limit/no credit).
3. Verificar disponibilidade/saúde do OpenRouter (status page) e saldo de crédito da API key —
   `stt_upstream_error` com 402/no-credit é falha de billing, não de código.
4. Comparar com o benchmark real de referência (`benchmark-stt.md`): p50 `529ms`, p95 `775ms`,
   erro `0/30` em lote controlado — desvio grande sugere degradação externa do provider, não do
   nosso código.
5. Correlacionar span `agents.usecase.process_audio_inbound` no Tempo com o intervalo do erro.

**Ação:** nenhum fallback chain é permitido (R-AGENT-WF-001.4/techspec) — não trocar de provider
automaticamente. Se sustentado, considerar reverter `AGENT_AUDIO_ENABLED=false` (canário, seção 5)
até o provider normalizar.

### 3.3 Incerteza técnica alta

**Sintoma:** alerta `mc-audio-transcription-uncertain-rate` (`transcription_uncertain_rate_15m > 20%`).

**Investigação:**

1. ```promql
   sum by (reason) (rate(agents_audio_inbound_total{outcome="transcription_uncertain"}[15m]))
   ```
2. Distribuir por `reason`: `truncated` (resposta cortada — revisar limite interno de texto ou
   sinalização do provider), `empty_text`/`incoherent` (ruído, fala cortada, silêncio),
   `language_unsupported` (usuário falando outro idioma ou detecção instável),
   `low_confidence` (provider incerto sobre a própria transcrição).
3. Isto é um **sintoma técnico esperado em volume moderado**, não necessariamente um incidente:
   o decisor (`DecideAudioTranscription`, puro, sem IO) é conservador por design (RF-13..RF-17) —
   prefere rejeitar tecnicamente a arriscar enviar texto ambíguo para `HandleInbound`. Ambiguidade
   **financeira** com texto tecnicamente confiável não entra aqui (segue o fluxo textual normal).
4. Se a taxa subir de forma abrupta e sustentada (não apenas ruído normal de uso), suspeitar de:
   regressão no parser de idioma do provider, mudança de modelo (`AGENT_STT_MODEL`), ou aumento de
   áudios fora do português.

**Garantia auditável:** `transcription_uncertain` nunca chama `HandleInbound` nem tool financeira.
A prova real da fronteira de dispatch é
`TestHandleAudio_UncertainNeverCallsHandleInboundOrTools` em
`internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_test.go`,
que espiona o consumer de produção com `AssertNotCalled` sobre `HandleInbound.Execute`.
Os `TestNegative*` de `internal/agents/application/golden/audio_pipeline_test.go` complementam
validando outcome/reason por caminho de rejeição.
(Correção 2026-08-05: a versão anterior citava um `dispatchSpy` do pacote golden que reimplementava
a própria condição de produção e portanto não podia falhar; o spy foi removido.)

### 3.4 Latência alta

**Sintoma:** alerta `mc-audio-transcription-latency-p95` (`transcription_p95_15m > 8s`).

**Investigação:**

1. ```promql
   histogram_quantile(0.95, sum by (le) (rate(agents_audio_transcription_latency_seconds_bucket[15m])))
   histogram_quantile(0.95, sum by (le, outcome) (rate(agents_audio_download_latency_seconds_bucket[15m])))
   ```
2. Diferenciar latência de **download** (Meta Media API) vs latência de **transcrição** (OpenRouter
   STT) — painéis separados no dashboard `agent-audio-whatsapp.json`.
3. Baseline real: p50 `529ms`/p95 `775ms`/max `940ms` em lote de 30 áudios reais WhatsApp
   (`benchmark-stt.md`). `AGENT_STT_TIMEOUT=20s` é o teto absoluto; `8s` no alerta é um orçamento de
   degradação sustentada, bem abaixo do timeout.
4. Se p95 sobe de forma consistente: suspeitar de degradação do OpenRouter, aumento de duração
   média dos áudios recebidos (ver painel de duração), ou saturação de rede/egress do host.

### 3.5 Custo alto

**Sintoma:** alerta `mc-audio-cost-microusd-high` (`audio_cost_microusd_1h > 120000`, equivalente a
60 áudios no teto por-áudio de `2000` microusd em uma hora).

**Investigação:**

1. ```promql
   sum(increase(agents_audio_cost_microusd_total[1h]))
   sum by (model) (increase(agents_audio_cost_microusd_total[1h]))
   ```
2. Confirmar se o aumento é **volume** (mais áudios legítimos, esperado se o produto cresceu) ou
   **anomalia** (mesmo volume com custo por áudio muito acima do benchmark de
   `0.0001748375 USD`/áudio, `stt-lote-30-results.json`).
3. O sistema já tem dois gates de proteção: preflight (estimativa por duração × modelo antes da
   chamada) e pós-STT (`usage.cost` real comparado ao teto) — qualquer excedente já é bloqueado
   antes do dispatch (`ErrSTTCostPreflightExceeded`/`ErrSTTCostExceeded`, outcome `rejected`,
   `reason=cost_exceeded`). Este alerta é sobre o **agregado horário**, não sobre um áudio
   individual escapando do teto.
4. Reduzir `AGENT_AUDIO_MAX_COST_MICROUSD` ou desabilitar `AGENT_AUDIO_ENABLED=false`
   temporariamente se o custo agregado ultrapassar o orçamento operacional aprovado. Qualquer
   **aumento** deste threshold exige decisão operacional versionada antes do deploy (techspec).

### 3.6 Regressão golden e falso sucesso

**Sintoma:** gate de golden set (tarefa 8.0) falha localmente/CI, ou alerta `mc-audio-false-success`
(`audio_false_success > 0`) dispara em produção/canário.

**Verificação offline (golden, tolerância 0):**

```bash
go build -tags integration ./...
RUN_REAL_LLM=1 go test -tags integration -count=1 -timeout 9m \
  -run TestGoldenAudioRealLLMSuite -v ./internal/agents/application/golden/...
go test -race -count=1 ./internal/agents/application/golden/...
```

- Score por grupo (`expense`/`income`/`query`/`edit`/`confirmation`) deve ser `>= 0.90`.
- `TestInvariantNoFalseSuccessOnAudioOriginatedRun` deve permanecer verde (tolerância `0` de
  falso-sucesso mutacional em runs originados de áudio).
- Os 6 negativos obrigatórios (ruído/incoerente, fala cortada, idioma não PT-BR, formato inválido,
  timeout STT, duplicado) devem continuar com `0` tool call e `0` `HandleInbound`.

**Verificação em produção/canário (`audio_false_success`):**

Áudio só produz texto canônico aprovado e entra no **mesmo** pipeline textual/tools existente —
não há métrica de mutação dedicada por origem áudio/texto (adicionar um label de origem em métrica
de escrita financeira violaria o contrato de labels da techspec, que não lista essa dimensão para
nenhuma métrica de escrita). Por isso a detecção em produção reusa a família genérica já
provisionada:

```promql
sum(increase({__name__=~"agents_.+_false_success_total"}[5m])) > 0
```

Mesma fonte do alerta `FinancialWriteFalseSuccess`
(`docs/alerts/mecontrola-agent-gate-posdeploy.yaml`) e do procedimento de investigação já
documentado em `docs/runbooks/mecontrola-agent-gate-posdeploy.md` (seção 11 — correlacionar
`trace_id`, verificar `mecontrola.agents_write_ledger`/`mecontrola.transactions` pelo `wamid` de
origem). Durante a janela de canário do áudio (seção 5), qualquer disparo deste alerta deve ser
tratado como **crítico e imediato**, independente de a origem ser áudio ou texto — o produto ainda
não tem volume suficiente para separar as duas populações com confiança estatística.

## 4. Readiness pós-deploy: authoring-ready, merge-ready, production-ready

| Nível | Critério |
|---|---|
| **authoring-ready** | Código compila (`go build ./...`), testes unitários/integração do pacote passam (`go test -race -count=1 ./internal/agents/... ./internal/platform/... ./configs/...`), lint limpo (`golangci-lint run`), golden determinístico verde. `AGENT_AUDIO_ENABLED` ainda não precisa estar `true` em nenhum ambiente. |
| **merge-ready** | Tudo de authoring-ready **mais**: gate real por flag executado com evidência real capturada — `RUN_REAL_LLM=1 RUN_REAL_STT=1` conforme `Gates de Validacao para Merge` da techspec; score por grupo `>= 0.90`; migration `000015` validada up/down; revisão (`review` skill) com veredito `APPROVED`/`APPROVED_WITH_REMARKS` sem remark crítico; nenhuma regressão no fluxo textual existente. |
| **production-ready** | Tudo de merge-ready **mais**: deploy em produção com `AGENT_AUDIO_ENABLED=false` (canário, seção 5) e critérios de habilitação atendidos; dashboard `agent-audio-whatsapp.json` e alertas do grupo `audio` provisionados e visíveis no Grafana; ao menos uma janela de observação com `AGENT_AUDIO_ENABLED=true` sem regressão operacional (seção 6) antes de considerar o rollout definitivo. |

`production-ready` **não** é sinônimo de "código no `main`" nem de "flag ligada uma vez" — exige a
janela de observação com evidência (seção 6).

## 5. Canary: `AudioEnabled=false` por default e critérios de habilitação

- **Default de produção:** `AGENT_AUDIO_ENABLED=false`. Enquanto assim, qualquer mensagem de áudio
  recebida responde de forma determinística `"entrada por áudio ainda não está disponível por
  aqui, pode digitar sua mensagem?"` sem tocar Meta Media API nem OpenRouter — zero custo, zero
  chamada externa, zero persistência de auditoria de áudio (`internal/agents/module.go:391`,
  `whatsapp_inbound_consumer.go:251-252`). O fluxo textual existente permanece **inteiramente
  inalterado** neste estado.
- **Pré-requisitos antes de considerar habilitar** (todos obrigatórios):
  1. Tarefas 1.0–8.0 deste PRD concluídas com relatório de execução `APPROVED` em
     `.specs/prd-agente-audio-openrouter/`.
  2. Dashboard `agent-audio-whatsapp.json` e alertas do grupo `audio` provisionados e confirmados
     no Grafana (`deployment/telemetry/grafana/provisioning/alerting/rules.yaml`).
  3. Configuração de produção completa e validada no boot: `AGENT_STT_MODEL`, `AGENT_STT_TIMEOUT`,
     `AGENT_AUDIO_MAX_DURATION`, `AGENT_AUDIO_MAX_BYTES`, `AGENT_AUDIO_MAX_COST_MICROUSD`,
     `WA_MSG_AUDIO_UNCERTAIN_RETRY`, `WA_MSG_AUDIO_REJECTED_RETRY` — todos definidos (a validação de
     `configs/config.go` já impede boot com produção incompleta).
  4. `META_ACCESS_TOKEN` e `OPENROUTER_API_KEY` válidos e com crédito confirmado.
- **Habilitação controlada (recomendado):**
  1. Ligar `AGENT_AUDIO_ENABLED=true` em produção.
  2. Anotar o timestamp da habilitação no dashboard `agent-audio-whatsapp.json`.
  3. Observar os 5 alertas do grupo `audio` continuamente pela primeira janela de tráfego real (não
     há SLA de dias/N fixo definido pela techspec para esta iniciativa — usar julgamento operacional
     e o mesmo espírito de amostra mínima do runbook `mecontrola-agent-gate-posdeploy.md`).
  4. Qualquer disparo de `mc-audio-false-success` (crítico) durante a janela inicial: reverter
     imediatamente `AGENT_AUDIO_ENABLED=false` sem esperar investigação completa — falso sucesso
     financeiro é sempre reversão imediata (mesma política de `FinancialWriteFalseSuccess`).
  5. Disparo sustentado de `mc-audio-stt-error-rate` ou `mc-audio-cost-microusd-high`: avaliar causa
     (provider externo vs configuração) antes de decidir manter ou reverter — não é reversão
     automática, mas exige decisão registrada.
- **Aumento de threshold de custo:** qualquer elevação de `audio_cost_microusd_1h` (hoje `120000`,
  equivalente a 60 áudios/hora no teto de `2000` microusd/áudio) exige decisão operacional
  versionada antes do deploy (mesma exigência da techspec) — não ajustar silenciosamente em
  resposta a um alerta.

## 6. Rollback e evidências mínimas pós-deploy

### 6.1 Rollback

O áudio não introduz nenhum caminho de escrita financeira próprio — ele só produz texto canônico
que entra no pipeline textual existente. Isso dá dois níveis de rollback, do mais barato ao mais
caro:

1. **Desligar a feature (preferencial, instantâneo, sem redeploy de imagem):**
   ```bash
   # Editar deployment/config/prod.env (ou variável efetiva do serviço)
   AGENT_AUDIO_ENABLED=false
   # Reiniciar/atualizar os services que carregam a env (server + worker, ambos chamam agents.NewModule)
   docker service update --force mecontrola_server-1 mecontrola_server-2
   docker service update --force mecontrola_worker-1 mecontrola_worker-2
   ```
   Isso reverte imediatamente para o comportamento pré-áudio (resposta de indisponibilidade,
   nenhuma chamada externa) sem precisar reverter a imagem/tag. Preferir este caminho sempre que a
   regressão for isolada ao comportamento de áudio (STT, download, custo, incerteza) e o restante
   do sistema (texto, tools financeiras) estiver saudável.
2. **Rollback de deploy completo (se a regressão afetar o binário/imagem como um todo, não apenas
   o comportamento de áudio):** seguir `deployment/runbooks/rollback.md` (reimplantar tag anterior
   via Docker Swarm). A migration `000015` é aditiva (nova tabela, sem alteração de tabela
   existente) — não precisa de rollback de schema para reverter o comportamento de áudio; a tabela
   `agents_whatsapp_audio_messages` pode permanecer mesmo com `AGENT_AUDIO_ENABLED=false`.
3. Se a migration precisar ser revertida por algum motivo excepcional: `down.sql` remove a tabela e
   índices (`migrations/000015_agents_whatsapp_audio_messages.down.sql`) — confirmar que nenhuma
   auditoria em uso ainda é necessária antes de rodar.

### 6.2 Evidências mínimas pós-deploy

Antes de declarar a habilitação estável, capturar e registrar (mesmo padrão de rastreabilidade do
runbook `mecontrola-agent-gate-posdeploy.md`, seção 5.1):

1. **Volume e outcome agregado**, via Prometheus:
   ```promql
   sum by (outcome, reason) (increase(agents_audio_inbound_total[<janela-desde-habilitação>]))
   ```
2. **Nenhum disparo crítico** dos 5 alertas do grupo `audio` na janela (`mc-audio-stt-error-rate`,
   `mc-audio-transcription-uncertain-rate`, `mc-audio-transcription-latency-p95`,
   `mc-audio-cost-microusd-high`, `mc-audio-false-success`) — confirmar no painel de Alerting do
   Grafana (pasta `MeControla Alerts`, grupo `audio`).
3. **Auditoria consistente com métricas** (contagem por outcome deve bater, sem expor
   `wamid`/`peer`/`transcription` fora de investigação pontual):
   ```sql
   SELECT outcome, reason, count(*) AS total
   FROM mecontrola.agents_whatsapp_audio_messages
   WHERE created_at >= '<timestamp-da-habilitação>'
   GROUP BY outcome, reason
   ORDER BY total DESC;
   ```
4. **Custo real acumulado** dentro do orçamento esperado:
   ```promql
   sum(increase(agents_audio_cost_microusd_total[<janela-desde-habilitação>]))
   ```
5. **Nenhuma regressão no fluxo textual existente** — confirmar que
   `agents_whatsapp_inbound_total{outcome="success"}` (mensagens de texto) mantém padrão histórico
   sem queda anormal na mesma janela (áudio e texto compartilham o mesmo consumer; uma regressão de
   código no roteamento tipado do payload afetaria ambos).
6. Registrar timestamp da habilitação, `verdict` operacional (manter habilitado / reverter) e link
   para as queries acima em um registro rastreável (issue ou anexo à decisão), mesma exigência de
   evidência do runbook `mecontrola-agent-gate-posdeploy.md`.

## 7. Cardinalidade e privacidade (RF-27/RF-28)

Proibido em métricas, logs `INFO`/`WARN`, spans e auditoria de áudio:

- Áudio bruto, base64, URL temporária de download ou access token (nunca persistidos — ADR-003).
- Transcrição completa fora da coluna `transcription` da auditoria (não aparece em log nem métrica).
- `wamid`, `media_id`, `peer`/telefone, `user_id`, `resource_id`, `thread_id`, categoria, valor
  financeiro como **label de métrica** — os únicos labels permitidos pelas 6 métricas de áudio são
  `channel`, `outcome`, `reason`, `provider`, `model`, `mime_family` (ver
  `deployment/dashboards/README.md`, seção "Agente de áudio WhatsApp/STT").

Gate de verificação (deve retornar vazio antes de merge/deploy):

```bash
grep -rnE --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  '"user_id"|"resource_id"|"thread_id"|"wamid"|"media_id"' \
  internal/platform/whatsapp/media internal/platform/llm \
  internal/agents/application/usecases/process_audio_inbound.go \
  | grep -i "metric\|counter\|histogram\|label\|observability.String"
```

```bash
grep -in "wamid\|media_id\|peer\|user_id" deployment/dashboards/agent-audio-whatsapp.json \
  deployment/telemetry/grafana/provisioning/alerting/rules.yaml
```

Ambos os comandos devem retornar vazio (nenhum resultado) para o **caminho de áudio**.

Correção 2026-08-05 (review): o primeiro comando estava escrito com `grep` sem `-E` usando
alternação `|`, o que o tornava um no-op que sempre retornava vazio — ele não validava nada.
Corrigido para `grep -rnE` e com escopo restrito ao caminho de áudio. A afirmação anterior de que
"ambos retornam vazio, confirmado nesta tarefa" era falsa para o segundo comando, que retorna
ocorrências legítimas em `rules.yaml` (texto de anotação/descrição dos alertas, não labels de
métrica). Ao rodar o segundo comando, inspecionar cada ocorrência: só é bloqueante se o termo
aparecer como **label** de métrica ou campo de série, nunca em texto de anotação.

## 8. Referências cruzadas

- Auditoria e outcome terminal por WAMID: `.specs/prd-agente-audio-openrouter/adr-002-outcome-terminal-wamid-audio.md`.
- Persistência sem áudio bruto: `.specs/prd-agente-audio-openrouter/adr-003-auditoria-sem-audio-bruto.md`.
- Gate pós-deploy geral do agente (amostra mínima, margens, falso sucesso financeiro):
  `docs/runbooks/mecontrola-agent-gate-posdeploy.md`.
- Saturação de runtime/infra (goroutines, disco, pool, backup): `docs/runbooks/infra-saturation-triage.md`.
- Rollback genérico de deploy: `deployment/runbooks/rollback.md`.
- Alertas Prometheus adicionais de agente (formato Alertmanager, incluindo `FinancialWriteFalseSuccess`):
  `docs/alerts/mecontrola-agent-gate-posdeploy.yaml`.
