<!-- spec-hash-prd: fcb9286b6b71310a28ab180a856e778dc3243d91dca5ac2118fd524e4d62572d -->

# Especificacao Tecnica: agente com audio WhatsApp via OpenRouter

Status: payload WhatsApp real, Media API e lote STT minimo de 30 audios sanitizados validados;
pronta para decomposicao em tasks de codigo produtivo.

## Resumo Executivo

A funcionalidade sera implementada como extensao do inbound WhatsApp antes do `HandleInbound`.
O audio sera validado, baixado da Media API da Meta, transcrito no endpoint STT dedicado do
OpenRouter e convertido em texto canonico somente quando passar por gates tecnicos fechados. O
runtime agentivo, as tools financeiras, workflows de confirmacao, memory, scorers e write-tool guard
existentes serao reaproveitados sem novo agente e sem workflow kernel especifico de audio.

A decisao tecnica principal e falhar fechado: `TranscriptionUncertain`, audio invalido, duracao
indeterminavel, falha de download, falha STT, custo pre-STT estimado acima do teto, custo pos-STT
medido acima do teto ou modelo indisponivel geram outcome terminal seguro para o WAMID e resposta
textual de reenvio, sem chamada a `HandleInbound` e sem tool financeira. Para atender auditoria e
privacidade, o audio bruto nunca sera persistido; a implementacao devera criar persistencia minima
para hash, metadados tecnicos, outcome tipado, modelo STT e transcricao aprovada ou rejeicao tecnica.

## Entradas e Evidencias

### PRD Consumido

- PRD: `.specs/prd-agente-audio-openrouter/prd.md`
- Hash: `fcb9286b6b71310a28ab180a856e778dc3243d91dca5ac2118fd524e4d62572d`
- Requisitos: `RF-01` a `RF-46`

### Evidencias do Codebase

| Evidencia | Conclusao tecnica |
|---|---|
| `internal/platform/whatsapp/payload/types.go:29` | O payload interno atual tem `Type`, mas o `Message` publico so expoe texto. |
| `internal/platform/whatsapp/payload/types.go:41` | O contrato publico `payload.Message` nao carrega media id, MIME, hash ou duracao. |
| `internal/platform/whatsapp/payload/parser.go:19` | O parser atual so extrai `msg.Text.Body`; audio vira texto vazio. |
| `internal/platform/whatsapp/dispatcher/dispatcher.go:127` | Timestamp e validado antes de rotear. |
| `internal/platform/whatsapp/dispatcher/dispatcher.go:132` | Dedup WAMID ocorre antes da resolucao principal/rota. |
| `internal/platform/whatsapp/dispatcher/dispatcher.go:141` | Principal WhatsApp e resolvido antes da rota do agente. |
| `internal/platform/whatsapp/dispatcher/dispatcher.go:154` | Rate limit autenticado ja existe antes da rota. |
| `internal/agents/module.go:474` | A rota do agente publica outbox `agents.whatsapp.inbound.v1`. |
| `internal/agents/module.go:505` | Payload publicado hoje contem `user_id`, `peer`, `text`, `message_id`. |
| `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:154` | Consumer rejeita payload sem texto. |
| `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:159` | Consumer tem segunda dedup por message id. |
| `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:256` | `HandleInbound` recebe somente texto aprovado. |
| `internal/agents/application/usecases/handle_inbound.go:22` | `HandleInbound` valida input e delega para `AgentRuntime`. |
| `internal/platform/agent/runtime.go:116` | Runtime resolve Thread antes de abrir Run. |
| `internal/platform/agent/runtime.go:122` | Runtime abre Run auditavel. |
| `internal/platform/agent/runtime.go:129` | WAMID ja entra como correlation key. |
| `internal/platform/agent/runtime.go:213` | Runtime persiste mensagem do usuario e resposta do assistant. |
| `internal/platform/llm/provider.go:5` | `llm.Provider` atual cobre `Complete`, `Stream` e `Embed`, nao STT. |
| `internal/platform/llm/openrouter.go:22` | OpenRouter atual tem endpoints de chat e embeddings, nao transcricao. |
| `internal/onboarding/infrastructure/http/client/meta/client.go:96` | Client Meta atual envia texto; nao baixa midia. |
| `configs/config.go:180` | Config de agente nao possui modelo STT, timeout STT, limite de bytes ou custo. |
| `deployment/compose/compose.yml:11` | PostgreSQL local usa `postgres:16-alpine`; migrations devem mirar PostgreSQL 16. |
| `internal/agents/application/golden/harness_realllm_test.go:25` | Golden real-LLM ja usa threshold `0.90`. |
| `internal/agents/application/golden/harness_realllm_test.go:31` | Golden real depende de `RUN_REAL_LLM=1` e `OPENROUTER_API_KEY`. |
| Produção `mecontrola-vps`, `docker service ls`, 2026-08-04 17:40 UTC | Server/worker rodam com 2 replicas; Postgres, PgBouncer, otel-lgtm, exporters e Caddy estao healthy. |
| Produção `mecontrola-vps`, `SELECT version()`, 2026-08-04 | Banco real e PostgreSQL 16.14. |
| Produção `mecontrola-vps`, `schema_migrations`, 2026-08-04 | Migration aplicada atual e `14`, `dirty=false`; a proxima migration deve ser `000015`. |
| Produção `mecontrola-vps`, contagens DB, 2026-08-04 | `platform_runs=181`, `platform_messages=398`, `agents_write_ledger=46`, `consumer_processed_messages=198`, `outbox_events=902`. |
| Produção `mecontrola-vps`, Prometheus 24h, 2026-08-04 | `whatsapp_dispatcher_route_total`: ~13 agent, ~47 invalid; `agents_whatsapp_inbound_total`: ~11 success; sem 5xx HTTP e sem erros OpenRouter nas series consultadas. |
| Produção `mecontrola-vps`, Grafana datasources, 2026-08-04 | Datasources reais: Prometheus uid `prometheus`, Loki uid `loki`, Tempo uid `tempo`. |
| Produção `mecontrola-vps`, Tempo search, 2026-08-04 | Tempo possui traces recentes de `mecontrola-api`, principalmente `GET /healthz` e spans de onboarding. |
| Produção `mecontrola-vps`, Loki labels, 2026-08-04 | Loki usa labels `deployment_environment`, `service_instance_id`, `service_name`; logs podem conter `trace_id`/`span_id`. |
| Produção `mecontrola-vps`, docker logs 6h, 2026-08-04 | Ha erro recorrente fora do escopo: `billing-reconciliation` com Kiwify 401; nao e bloqueio de audio, mas deve entrar no runbook de triagem geral. |
| Produção `mecontrola-vps`, audio real controlado, 2026-08-04 | Evento inbound falhou com `text=""`; payload outbox tinha `message_id`, `peer`, `text`, `user_id`, sem `audio` e sem `media_id`; nenhum `platform_run` foi criado. |
| Produção `mecontrola-vps` + Loki/Prometheus, audio real controlado, 2026-08-04 18:19 UTC | Payload bruto sanitizado confirmado: `message_type=audio`, `audio_id_present=true`, `mime_type=audio/ogg; codecs=opus`, `media_sha256` presente, `voice=true`, `has_text=false`, `text_empty=true`. |
| Meta Media API, audio id real, 2026-08-04 | `GET /v18.0/{audio_id}` retornou URL temporaria, `mime_type=audio/ogg`, `file_size=11044`; download autenticado retornou `11044` bytes e SHA-256 igual ao informado pela Meta. |
| Lote STT real sanitizado, 30 audios WhatsApp, 2026-08-04 | `openai/whisper-large-v3` transcreveu 30/30 audios `audio/ogg` sem conversao; erro `0`, p50 `529ms`, p95 `775ms`, custo total `0.003295125 USD`, custo maximo por audio `0.0001748375 USD`. |

Evidencia operacional sanitizada: `.specs/prd-agente-audio-openrouter/prod-evidence-2026-08-04.md`.
Evidencia de audio real sanitizada: `.specs/prd-agente-audio-openrouter/whatsapp-audio-payload-evidence-2026-08-04.md`.
Smoke benchmark STT sanitizado: `.specs/prd-agente-audio-openrouter/benchmark-stt.md`.

### Documentacao Oficial Consultada

- OpenRouter STT: `https://openrouter.ai/docs/guides/overview/multimodal/stt`
- OpenRouter audio multimodal: `https://openrouter.ai/docs/guides/overview/multimodal/audio`
- Meta WhatsApp Media API: `https://developers.facebook.com/documentation/business-messaging/whatsapp/reference/media/media-api`
- PostgreSQL 16 tipos: `https://www.postgresql.org/docs/16/datatype.html`
- PostgreSQL 16 constraints: `https://www.postgresql.org/docs/16/ddl-constraints.html`
- PostgreSQL 16 defaults: `https://www.postgresql.org/docs/16/ddl-default.html`
- PostgreSQL 16 `ALTER TABLE`: `https://www.postgresql.org/docs/16/sql-altertable.html`

## Arquitetura do Sistema

### Componentes Novos ou Modificados

| Componente | Tipo | Responsabilidade |
|---|---|---|
| `internal/platform/whatsapp/payload` | modificar | Expor mensagem tipada com `MessageTypeText` e `MessageTypeAudio`, mantendo texto atual intacto. |
| `internal/platform/whatsapp/media` | novo | Cliente fino para resolver URL de midia e baixar bytes via Meta Media API com timeout, auth e limite. |
| `internal/platform/llm` | modificar | Adicionar porta STT e implementacao OpenRouter para `/api/v1/audio/transcriptions`. |
| `internal/agents/application/dtos/input` | novo/modificar | Adicionar DTO de audio inbound e tipos fechados de status/outcome. |
| `internal/agents/application/usecases` | novo | Orquestrar `validate -> download -> hash -> transcribe -> classify -> persist audit -> dispatch`. |
| `internal/agents/infrastructure/persistence` | novo | Repositorio Postgres de auditoria de audio WhatsApp. |
| `internal/agents/infrastructure/messaging/database/consumers` | modificar | Processar payload tipado e bloquear `HandleInbound` quando audio nao gerar texto canonico aprovado. |
| `internal/agents/module.go` | modificar | Fazer DI manual do downloader, transcriber, audit repo e use case de audio. |
| `configs/config.go` | modificar | Adicionar config tipada para STT/audio com defaults seguros e validacao de producao. |
| `internal/agents/application/golden` | modificar | Adicionar golden pareado texto/audio e gate real STT por flag. |
| `deployment/runbooks` | novo/modificar | Runbook de falha download/STT/incerteza/custo/latencia/regressao golden. |

### Fluxo de Dados

```text
WhatsApp webhook
  -> signature/rate limit/timestamp/dedup/principal existentes
  -> payload.ExtractMessages retorna Message text ou audio
  -> agents WhatsApp route publica outbox com inbound tipado
  -> WhatsAppInboundConsumer
     -> se text: fluxo atual sem mudanca
     -> se audio:
        -> validar campos, MIME, bytes, duracao quando disponivel
        -> Meta Media API: GET /{media-id} para URL, download com bearer token
        -> SHA-256 dos bytes e descarte do audio bruto apos STT/rejeicao
        -> OpenRouter STT /api/v1/audio/transcriptions
        -> classificar outcome tecnico
        -> persistir auditoria minima
        -> se Approved: chamar resume/onboarding/HandleInbound com texto canonico
        -> se Uncertain/Rejected/Failed: enviar resposta textual segura sem HandleInbound
```

### Fronteiras

- `internal/platform/whatsapp/media` pode conhecer Meta/Graph API e `internal/platform/httpclient`.
- `internal/platform/llm` pode conhecer OpenRouter STT, mantendo provider unico e sem fallback chain.
- `internal/agents` consome as duas fronteiras para decidir se existe texto canonico seguro.
- `internal/platform/workflow` nao sera alterado e nao deve importar audio, WhatsApp, LLM ou dominio financeiro.
- Tools financeiras nao serao alteradas para aceitar audio; elas continuam recebendo decisao via texto canonico.

## Design de Implementacao

### Estados Fechados

Estados de audio devem ser tipos fechados, sem `string` livre em assinatura publica.

```go
type WhatsAppMessageType string

const (
    WhatsAppMessageTypeText  WhatsAppMessageType = "text"
    WhatsAppMessageTypeAudio WhatsAppMessageType = "audio"
)

type AudioOutcome string

const (
    AudioOutcomeApproved               AudioOutcome = "approved"
    AudioOutcomeRejected               AudioOutcome = "rejected"
    AudioOutcomeTranscriptionUncertain AudioOutcome = "transcription_uncertain"
    AudioOutcomeTranscriptionFailed    AudioOutcome = "transcription_failed"
    AudioOutcomeDispatched             AudioOutcome = "dispatched"
)
```

Regra DMMF: `AudioOutcomeApproved` e o unico estado que permite `HandleInbound`. Qualquer outro
estado deve retornar antes de `tryResume`, `tryResolveOnboarding` e `handleAgentInbound`.

### Payload WhatsApp

`payload.Message` deve passar a carregar modalidade sem quebrar texto:

```go
type Message struct {
    From      string
    WAMID     string
    Timestamp string
    Type      MessageType
    Text      string
    Audio     *Audio
}

type Audio struct {
    MediaID  string
    MimeType string
    SHA256   string
    Voice    bool
}
```

Campos definitivos devem ser fechados pelo discovery com payload real sanitizado antes da primeira
task de codigo. Se o payload real divergir, a task deve ajustar a techspec ou bloquear por drift.

### Contrato de Media Download

```go
type MediaClient interface {
    Resolve(ctx context.Context, mediaID string) (MediaDescriptor, error)
    Download(ctx context.Context, url string, maxBytes int64) (DownloadedMedia, error)
}

type MediaDescriptor struct {
    URL      string
    MimeType string
    SHA256   string
    Size     int64
}

type DownloadedMedia struct {
    Bytes    []byte
    MimeType string
    SHA256   string
    Size     int64
}
```

Regras:

- Interface deve ficar no consumidor que orquestra audio; implementacao concreta em
  `internal/platform/whatsapp/media`.
- `Resolve` usa a Media API oficial da Meta para obter URL a partir de media id.
- `Download` usa a URL retornada com bearer token e `io.LimitReader` ou equivalente para impor
  `maxBytes + 1`.
- `context.Context` e primeiro parametro em todas as fronteiras de IO.
- Nenhum audio bruto pode ir para log, span attribute, metric label, outbox permanente ou mensagem do agent.

### Contrato STT OpenRouter

Adicionar uma porta pequena em `internal/platform/llm`:

```go
type Transcriber interface {
    Transcribe(ctx context.Context, req TranscriptionRequest) (TranscriptionResponse, error)
}

type TranscriptionRequest struct {
    Model       string
    Audio       []byte
    Format      AudioFormat
    Language    string
    Temperature float64
}

type TranscriptionResponse struct {
    Text         string
    Language     string
    DurationMs   *int
    Model        string
    Provider     string
    Usage        TranscriptionUsage
    Truncated    bool
    Confidence   *float64
    RawPreview   []byte
}
```

Regras:

- OpenRouter deve usar `/api/v1/audio/transcriptions`.
- Formato inicial da chamada: JSON com `input_audio.data` base64 sem data URI e
  `input_audio.format`, porque isso reduz multipart parsing interno e e suportado pela doc oficial.
- `language` deve ser `pt` no primeiro corte.
- `temperature` deve ser `0` para reduzir variabilidade.
- Timeout STT default: `20s`, configuravel.
- Modelo STT nao pode ser hardcoded em codigo; vem de config `AGENT_STT_MODEL`.
- `Language` deve vir do retorno do provider quando disponivel; se indisponivel, a implementacao deve
  aplicar classificador deterministico leve de idioma ou tratar como `TranscriptionUncertain`.
- `DurationMs` deve ser determinado antes do STT. A implementacao deve extrair duracao por payload
  WhatsApp quando a Meta passar a enviar esse campo ou por componente local deterministico para os
  formatos aceitos no primeiro corte (`audio/ogg`/Opus e M4A/AAC). Como o codebase atual nao possui
  biblioteca de duracao de audio comprovada no `go.mod`, a task deve implementar ou introduzir a menor
  dependencia auditavel com testes de fixture; se a duracao nao puder ser determinada, o outcome deve
  ser rejeicao tecnica antes de chamar OpenRouter.
- `DurationMs > AGENT_AUDIO_MAX_DURATION` deve ser rejeitado antes do STT.
- O custo deve ter dois gates. Pre-STT: estimar upper bound por duracao, modelo escolhido e teto
  configurado; se a estimativa exceder `AGENT_AUDIO_MAX_COST_MICROUSD`, rejeitar antes da chamada.
  Pos-STT: comparar `usage.cost` quando retornado; se exceder o teto, registrar outcome terminal de
  custo excedido, nao chamar `HandleInbound` e emitir metrica de custo.
- `Confidence` so e usado quando o provider retornar esse campo; ausencia nao aprova nem reprova sozinha.
- `Truncated` deve ser `true` quando o provider sinalizar corte, quando o texto exceder limite interno
  definido em config ou quando a resposta vier incompleta por erro de decodificacao.
- Se OpenRouter retornar texto vazio, erro, resposta invalida, length/truncation ou modelo indisponivel,
  o outcome e `TranscriptionUncertain` ou `TranscriptionFailed`, nunca dispatch.
- Nao introduzir fallback chain, provider paralelo ou envio do audio bruto para chat completions.

### Classificacao Tecnica de Transcricao

Criar decisor puro no pacote de aplicacao de audio:

```go
type AudioDecisionInput struct {
    Text       string
    Language   string
    Truncated  bool
    Confidence *float64
    STTError   error
}

type AudioDecision struct {
    Outcome       AudioOutcome
    CanonicalText string
    Reason        AudioReason
}
```

Regras:

- `DecideAudioTranscription` nao faz IO e nao recebe `context.Context`.
- `CanonicalText` so e preenchido quando `Outcome == AudioOutcomeApproved`.
- `Language != "pt"` ou idioma ausente quando nao houver detector confiavel deve produzir
  `AudioOutcomeTranscriptionUncertain`.
- `Confidence < AGENT_AUDIO_MIN_CONFIDENCE` deve produzir `AudioOutcomeTranscriptionUncertain`;
  quando o provider nao retornar confianca, o gate usa os demais sinais tecnicos.
- Texto vazio, so pontuacao, repeticao ininteligivel ou menos de 3 caracteres alfanumericos deve produzir
  `AudioOutcomeTranscriptionUncertain`.
- `Truncated=true` sempre produz `AudioOutcomeTranscriptionUncertain`.
- Normalizacao permitida: trim, normalizacao de espacos e remocao de caracteres de controle.
- Normalizacao proibida: corrigir valor, inferir categoria, completar data, trocar meio de pagamento ou
  enriquecer semantica financeira.
- Ambiguidade financeira com texto tecnicamente confiavel nao e `TranscriptionUncertain`; deve seguir o
  fluxo textual existente.

### Persistencia de Auditoria

Criar migration `000015_agents_whatsapp_audio_messages` com tabela proposta:

```sql
CREATE TABLE mecontrola.agents_whatsapp_audio_messages (
    wamid text PRIMARY KEY,
    user_id uuid NOT NULL,
    peer text NOT NULL,
    media_id text NOT NULL,
    media_sha256 text NOT NULL,
    mime_type text NOT NULL,
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
    duration_ms integer CHECK (duration_ms IS NULL OR duration_ms >= 0),
    stt_model text,
    outcome text NOT NULL,
    reason text NOT NULL,
    transcription text,
    transcription_sha256 text,
    cost_microusd bigint CHECK (cost_microusd IS NULL OR cost_microusd >= 0),
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
```

Constraints obrigatorias na implementation:

- `PRIMARY KEY (wamid)` para outcome terminal unico por mensagem WhatsApp.
- `CHECK outcome IN (...)` com todos os valores fechados de `AudioOutcome`.
- `CHECK reason IN (...)` com razoes tecnicas fechadas.
- `transcription` pode ser `NULL` para rejeicao/falha/incerteza.
- `media_id` e `peer` nao devem virar metric labels.
- `audio` bruto nao pode ser coluna, arquivo ou objeto persistido.
- Down migration deve remover tabela e indices criados.

Justificativa PostgreSQL: a tabela tem chave primaria, nulabilidade explicita, `CHECK` para
integridade de estados fechados, tipos nativos coerentes e default de timestamp conforme regras
oficiais de PostgreSQL 16 para data types, constraints e defaults.

### Idempotencia e Outcome Terminal

O dispatcher ja grava WAMID antes de publicar para o agente. A implementacao nao deve compensar essa
dedup do dispatcher quando download/STT falhar. Para audio, o consumer deve:

- tentar inserir auditoria por `wamid` antes do download;
- se a linha ja existir com outcome terminal, enviar a mesma resposta segura ou ignorar duplicado;
- atualizar `completed_at`, `outcome`, `reason` e metadados em transacao curta;
- nao chamar `dedup.Delete` para falha de download, validacao ou STT;
- manter a compensacao atual apenas para falhas antes de qualquer outcome terminal no consumer textual.

Essa decisao evita falso positivo de robustez em retries: falha STT nao pode reabrir o mesmo WAMID e
causar mutacao futura por replay automatico.

### Configuracao

Adicionar em `AgentConfig`:

| Campo | Env | Default | Validacao |
|---|---|---|---|
| `STTModel` | `AGENT_STT_MODEL` | vazio fora de audio habilitado | obrigatorio quando audio habilitado |
| `STTTimeout` | `AGENT_STT_TIMEOUT` | `20s` | `1s..20s` |
| `AudioEnabled` | `AGENT_AUDIO_ENABLED` | `false` | booleano |
| `AudioMaxDuration` | `AGENT_AUDIO_MAX_DURATION` | `60s` | `1s..60s` |
| `AudioMaxBytes` | `AGENT_AUDIO_MAX_BYTES` | sem default de producao | obrigatorio quando audio habilitado |
| `AudioMaxCostMicrousd` | `AGENT_AUDIO_MAX_COST_MICROUSD` | sem default de producao | obrigatorio quando audio habilitado |
| `AudioMinConfidence` | `AGENT_AUDIO_MIN_CONFIDENCE` | `0.80` | `0.50..1.00`; aplicado somente quando provider retornar confianca |
| `AudioUncertainReply` | `WA_MSG_AUDIO_UNCERTAIN_RETRY` | mensagem curta PT-BR | nao vazio |
| `AudioRejectedReply` | `WA_MSG_AUDIO_REJECTED_RETRY` | mensagem curta PT-BR | nao vazio |

`AudioEnabled=false` deve manter comportamento atual. Em `production`, se `AudioEnabled=true`, os
campos obrigatorios nao podem estar vazios ou zero.

Validacao adicional de producao:

- `AGENT_AUDIO_MAX_COST_MICROUSD=2000` e `AGENT_AUDIO_MAX_DURATION=60s` definem o teto inicial de
  aproximadamente `34` microusd por segundo para preflight conservador.
- Se a estimativa pre-STT para a duracao extraida exceder o teto configurado, o audio deve ser
  rejeitado antes de envio ao OpenRouter.
- Se a duracao nao for extraida, o audio deve ser rejeitado antes de envio ao OpenRouter.
- Se `usage.cost` vier ausente, a auditoria deve registrar custo desconhecido, mas o dispatch so pode
  ocorrer quando o preflight ja tiver aprovado a estimativa conservadora.

### Benchmark Obrigatorio Antes das Tasks de Codigo

Smoke benchmark real executado e versionado em
`.specs/prd-agente-audio-openrouter/benchmark-stt.md`. O modelo preferencial inicial e
`openai/whisper-large-v3`, com `AGENT_AUDIO_MAX_BYTES=2000000` e
`AGENT_AUDIO_MAX_COST_MICROUSD=2000`, baseado em uma amostra M4A/AAC PT-BR de `4.693333s`.

O gate final foi executado em 2026-08-04 com lote minimo de 30 audios reais WhatsApp sanitizados.
Resultado: 30/30 STT HTTP 200, erro 0, p50 `529ms`, p95 `775ms`, max `940ms`, custo total
`0.003295125 USD`, custo maximo por audio `0.0001748375 USD`, formato real `audio/ogg` aceito sem
conversao.

Gate minimo:

- listar modelos via OpenRouter Models API filtrando `output_modalities=transcription`;
- testar pelo menos 2 modelos STT disponiveis ou registrar que apenas 1 atende os criterios no momento;
- medir latencia p50/p95 em lote minimo de 30 audios curtos sanitizados;
- medir custo por audio quando usage/custo estiver disponivel;
- validar formatos reais baixados do WhatsApp;
- escolher um unico `AGENT_STT_MODEL`;
- definir `AGENT_AUDIO_MAX_BYTES` e `AGENT_AUDIO_MAX_COST_MICROUSD`;
- salvar resultado em `.specs/prd-agente-audio-openrouter/benchmark-stt.md`.

Todos os itens acima estao fechados para iniciar `create-tasks`. A implementacao ainda deve manter
suites reais por flag, golden set e feature flag `AudioEnabled=false` por default para evitar falso
positivo de readiness operacional.

## Endpoints de API

Nao ha endpoint publico novo.

Modificacao de comportamento:

- `POST /api/v1/whatsapp/inbound` passa a aceitar payload WhatsApp com mensagem `type=audio` apos
  discovery tecnico.
- Respostas HTTP do webhook continuam seguindo o handler atual; a resposta ao usuario continua sendo
  enviada de forma assincrona pelo gateway WhatsApp.

## Pontos de Integracao

### Meta WhatsApp Media API

- Resolver URL: `GET /{media-id}` com bearer token.
- Baixar midia: usar URL retornada com bearer token.
- Reusar `META_ACCESS_TOKEN`, a base URL Meta/Graph existente do client Meta e `internal/platform/httpclient`.
- Nunca aceitar URL de media diretamente do modelo ou do usuario.

### OpenRouter STT

- Endpoint: `/api/v1/audio/transcriptions`.
- Auth: `Authorization: Bearer <OPENROUTER_API_KEY>`.
- Payload: JSON base64 com `model`, `input_audio.data`, `input_audio.format`, `language=pt` e
  `temperature=0`.
- Erros 401/403, 402/no credit, 429, 5xx e timeout devem virar erro tipado e metricas por reason.

## Abordagem de Testes

### Testes Unitarios

- `internal/platform/whatsapp/payload`: parser deve cobrir texto, audio valido, payload sem media id,
  payload misto e payload real sanitizado.
- `internal/platform/whatsapp/media`: client fake HTTP para resolver URL, baixar bytes, impor
  `maxBytes`, bearer token e timeout.
- `internal/platform/llm`: transcriber OpenRouter com server fake para sucesso, 4xx, 5xx, timeout,
  JSON invalido, texto vazio, usage ausente e formato nao suportado.
- `internal/agents/application/usecases`: decisor puro para `Approved`, `Rejected`,
  `TranscriptionUncertain`, `TranscriptionFailed` e bloqueio de `HandleInbound`.
- `internal/agents/infrastructure/messaging/database/consumers`: garantir que audio incerto nao chama
  `tryResume`, `tryResolveOnboarding` nem `handleAgentInbound`.
- `configs`: defaults e validacao de producao para audio habilitado.

### Testes de Integracao

Obrigatorios porque ha persistencia de auditoria e outbox/consumer:

- migration `000015` up/down;
- repository de auditoria com insert terminal unico por `wamid`;
- consumer com Postgres real/testcontainers garantindo que falha STT registra terminal e nao reprocessa;
- compatibilidade do fluxo textual existente apos schema novo.

### Testes Reais OpenRouter e WhatsApp

- `RUN_REAL_STT=1` e `OPENROUTER_API_KEY` habilitam suite real STT; sem flag, skip deterministico.
- `RUN_REAL_WHATSAPP_MEDIA=1` e credenciais Meta habilitam teste de download com media id sanitizado
  de ambiente controlado; sem flag, skip deterministico.
- Nenhum RF que exige provider real pode ser marcado como atendido apenas por mock.

### Golden Set Audio

Adicionar camada pareada sobre golden textual:

- pelo menos 3 casos positivos por grupo: despesas, receitas, consultas, edicoes, confirmacoes;
- negativos: ruido, fala cortada, idioma nao PT-BR, formato invalido, timeout STT e duplicado;
- score por grupo `>= 0.90`;
- falso-sucesso em mutacao: `0`;
- tool call e `HandleInbound` em `TranscriptionUncertain`: `0`.

## Monitoramento e Observabilidade

Metricas novas com labels de baixa cardinalidade:

| Metrica | Tipo | Labels permitidos |
|---|---|---|
| `agents_audio_inbound_total` | counter | `channel`, `outcome`, `reason` |
| `agents_audio_transcription_latency_seconds` | histogram | `provider`, `model`, `outcome` |
| `agents_audio_download_latency_seconds` | histogram | `provider`, `outcome` |
| `agents_audio_size_bytes` | histogram | `mime_family`, `outcome` |
| `agents_audio_duration_seconds` | histogram | `outcome` |
| `agents_audio_cost_microusd_total` | counter | `provider`, `model` |

Proibido em metric labels: `user_id`, `resource_id`, `thread_id`, `wamid`, telefone, media id,
categoria, valor financeiro ou conteudo transcrito.

Logs:

- `INFO`: outcome agregado, reason, duracao, tamanho, modelo, sem texto completo.
- `WARN`: rejected/uncertain/timeout/rate/cost.
- `ERROR`: falha inesperada de IO/persistencia.
- Nao logar audio bruto, base64, access token, URL de download, transcricao completa ou dados financeiros
  extraidos.

Alertas iniciais:

- `stt_error_rate_15m > 5%`;
- `transcription_uncertain_rate_15m > 20%`;
- `transcription_p95_15m > 8s`;
- `audio_cost_microusd_1h > 120000`;
- `audio_false_success > 0` em golden ou canary.

O threshold `audio_cost_microusd_1h > 120000` equivale a 60 audios no teto inicial de `2000`
microusd por audio em uma hora. Esse valor deve ser reduzido em canary se o volume liberado for menor;
qualquer aumento exige decisao operacional versionada antes do deploy.

## Sequenciamento de Desenvolvimento

1. Implementar payload WhatsApp tipado e testes de parser.
2. Implementar Media API client com testes.
3. Implementar `llm.Transcriber` OpenRouter com testes e metricas.
4. Implementar tipos fechados e decisor puro de audio.
5. Implementar migration/repository de auditoria com testes de integracao.
6. Integrar consumer, route e module wiring com `AudioEnabled` default false.
7. Adicionar golden pareado audio/texto e suites reais por flag.
8. Atualizar dashboards/runbook e executar gates globais.

## Decisoes Chave

- ADR-001: STT dedicado OpenRouter antes do runtime agentivo.
- ADR-002: Outcome terminal por WAMID para audio.
- ADR-003: Persistencia minima de auditoria sem audio bruto.
- ADR-004: Nao aplicar pattern formal novo.

## Mapeamento RF -> Decisao -> Gate

| RFs | Decisao tecnica | Gate |
|---|---|---|
| RF-01, RF-02, RF-03, RF-04, RF-05, RF-40, RF-41 | Parser/payload WhatsApp audio apos discovery real, preservando texto atual. | Teste com payload real sanitizado, parser unitario e regressao textual. |
| RF-06, RF-07, RF-08 | Media client Meta com timeout, auth, tamanho e duracao. | Unit fake HTTP, real media flag, maxBytes e limite de 60s quando duracao existir. |
| RF-09, RF-10, RF-11, RF-12, RF-45 | `llm.Transcriber` OpenRouter, benchmark, config STT, timeout 20s e fail-closed por modelo/custo/formato. | `benchmark-stt.md`, real STT flag, timeout 20s, p95 <= 8s. |
| RF-13, RF-14, RF-15, RF-16, RF-17, RF-33, RF-35 | Decisor tecnico fechado, resposta de reenvio e bloqueio de dispatch em incerteza tecnica. | Unit garante 0 `HandleInbound` e 0 tool call em `TranscriptionUncertain`. |
| RF-18, RF-19, RF-20, RF-42, RF-43 | Texto canonico aprovado entra no fluxo textual existente sem enriquecimento semantico e mantendo confirmacoes existentes. | Golden pareado e regressao textual. |
| RF-21, RF-22, RF-23 | WAMID terminal unico e sem compensacao em STT/download failure. | Integracao consumer + auditoria + dedup. |
| RF-24, RF-25, RF-26, RF-27 | Auditoria sem audio bruto, vinculada a run/thread/mensagem por WAMID e sem transcricao completa em logs. | Migration/repository e grep/log tests. |
| RF-28, RF-29 | Metricas/alerts de baixa cardinalidade com thresholds iniciais. | Gate de labels e dashboard/runbook. |
| RF-30, RF-31, RF-32, RF-34, RF-36, RF-44 | Golden pareado, negativos obrigatorios, real provider por flag e score por grupo. | Score >= 0.90, falso-sucesso 0. |
| RF-37, RF-38, RF-39 | Skills obrigatorias, Mastra e decisao de `nao aplicar padrao` sem novo agente/provider/workflow de audio. | Review final e ADR-004. |
| RF-46 | Runbook operacional. | Arquivo em `deployment/runbooks` validado em task. |

## Conformidade com Skills e Regras

- `go-implementation`: task type `cross-cutting`, validation profile `global`.
- `domain-modeling-production`: estados de audio e decisoes modelados como tipos fechados; regra financeira
  permanece no dominio existente.
- `design-patterns-mandatory`: seletor executado; resultado `reject`; decisao `nao aplicar padrao`.
  Evidencias persistidas em `.specs/prd-agente-audio-openrouter/pattern-selector-input.json` e
  `.specs/prd-agente-audio-openrouter/pattern-selector-output.json`.
- `mastra`: comportamento novo entra antes do `HandleInbound`, consumindo substrato e consumidor real.
- `postgresql-production-standards`: PostgreSQL 16 confirmado por `deployment/compose/compose.yml:11`; migration
  proposta usa PK, NOT NULL, CHECK, defaults e rollback.

## Gates de Validacao para Merge

Comandos minimos esperados apos implementacao:

```bash
gofmt -w <arquivos-go-alterados>
go build ./internal/platform/... ./internal/agents/... ./configs/... ./cmd/server/... ./cmd/worker/...
go vet ./internal/platform/... ./internal/agents/... ./configs/... ./cmd/server/... ./cmd/worker/...
go test -race -count=1 ./internal/platform/... ./internal/agents/... ./configs/...
RUN_REAL_LLM=1 RUN_REAL_STT=1 go test -count=1 ./internal/agents/application/golden/...
golangci-lint run ./internal/platform/... ./internal/agents/... ./configs/... ./cmd/server/... ./cmd/worker/...
```

Gates heurisiticos obrigatorios:

```bash
grep -rn "^func init()" --include="*.go" internal cmd configs
grep -rnE "\\b_[a-zA-Z][a-zA-Z0-9]*" --include="*.go" internal cmd configs
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" "^[[:space:]]*//" internal/platform internal/agents | grep -Ev "(//go:|//nolint:|// Code generated)"
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" '"user_id"|"resource_id"|"thread_id"|"wamid"|"media_id"' internal/platform internal/agents | grep -i "metric\\|counter\\|histogram\\|label\\|observability.String"
```

## Riscos e Mitigacoes

| Risco | Mitigacao |
|---|---|
| Payload WhatsApp real divergir do modelo assumido. | Payload real sanitizado ja validado; parser tera fixture real sanitizada e deve bloquear drift. |
| Duracao de audio nao ser determinavel antes do STT. | Falhar fechado antes de OpenRouter; task deve implementar extractor deterministico para OGG/Opus e M4A/AAC com fixtures. |
| STT custar ou ter latencia acima do budget. | Benchmark de 30 audios definiu modelo, bytes, custo e timeout antes de codigo; runtime tera preflight de custo por duracao e gate pos-STT por usage. |
| Reprocessamento do mesmo WAMID apos falha STT. | Auditoria com PK WAMID e outcome terminal. |
| Vazamento de transcricao/audio em logs. | Regras de logging e testes grep especificos. |
| Duplicacao de regra financeira em audio. | Audio so gera texto canonico; tools/workflows existentes decidem negocio. |
| Migration mal dimensionada. | Tabela append/update por PK, sem backfill; integration up/down. |

## Itens Bloqueantes Antes de Codar

Nenhum bloqueio de discovery permanece aberto para iniciar decomposicao em tasks. Os bloqueios agora
sao os gates normais da implementacao: codigo, testes, migrations, golden set, validacao real por flag
e deploy controlado com `AudioEnabled=false` por default ate aprovacao operacional.
