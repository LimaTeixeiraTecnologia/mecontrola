# Evidencia WhatsApp Audio: producao

Data da coleta: 2026-08-04
Host: `mecontrola-vps`
Modo: somente leitura
Janela: envios de audio reais controlados entre 17:49 UTC e 18:19 UTC

## Resultado

O audio real enviado ao WhatsApp chegou ao pipeline autenticado com `message_type=audio`,
`audio_id_present=true`, `mime_type=audio/ogg; codecs=opus` e `voice=true`. O contrato outbox atual
perde a midia antes do consumer agentivo porque publica apenas `message_id`, `peer`, `text` e
`user_id`.

## Evidencias

### Outbox Recente

Consulta agregada em `mecontrola.outbox_events` apos novo envio em `2026-08-04 18:19:15 UTC`:

- `outbox_events` em `status=3`: `905`
- `outbox_events` em `status=4`: `3`
- `agents.whatsapp.inbound.v1` mais recente: `status=4`, `attempts=3`

O erro do evento `agents.whatsapp.inbound.v1` foi sanitizado. A causa tecnica foi:

```text
agents.consumer.whatsapp_inbound: payload incompleto: user_id=<uuid> peer=<phone> text=""
```

### Payload Bruto Sanitizado Confirmado por Telemetria

Fonte: Loki/Prometheus no container `mecontrola_otel-lgtm`, consulta por
`whatsapp.handler.inbound.message_evidence_detected` e metricas
`whatsapp_message_evidence_detected_total` / `whatsapp_audio_payload_detected_total`.

Campos confirmados no envio real de `2026-08-04 18:19:15 UTC`:

| Campo | Valor observado |
|---|---|
| `message_type` | `audio` |
| `audio_id_present` | `true` |
| `audio_id` | presente; valor completo mantido apenas na telemetria operacional |
| `mime_type` | `audio/ogg; codecs=opus` |
| `media_sha256` | presente |
| `voice` | `true` |
| `has_text` | `false` |
| `text_empty` | `true` |
| `wamid` | presente |
| `trace_id` | presente |
| `span_id` | presente |

Metricas confirmadas:

| Metrica | Resultado |
|---|---|
| `whatsapp_message_evidence_detected_total` | serie presente com valor `1` para o novo envio |
| `whatsapp_audio_payload_detected_total` | series presentes para os envios instrumentados |

### Shape do Payload Persistido

Chaves presentes no payload do evento `agents.whatsapp.inbound.v1`:

```text
message_id
peer
text
user_id
```

Verificacao booleana do payload:

| Campo | Resultado |
|---|---|
| `text` | presente |
| tamanho de `text` | `0` |
| `audio` | ausente |
| `media_id` | ausente |

### Media API

Validacao de download via Media API concluida com o `audio_id` real do envio de
`2026-08-04 18:19:15 UTC`.

Fonte do token: `.env` local, carregado em memoria sem imprimir valor.

Chamada de metadados:

```text
GET https://graph.facebook.com/v18.0/{audio_id}
```

Resultado:

```text
metadata_exit=0
metadata_error=false
media_url_present=true
mime_type=audio/ogg
sha256=a3a064090479e03e4e84a372ecc07c500312530b0a051a57186725a5a69d165c
file_size=11044
id=1081986107489646
```

Download da URL temporaria da Meta:

```text
download_exit=0
download_stderr_present=false
download_bytes=11044
download_sha256_file=a3a064090479e03e4e84a372ecc07c500312530b0a051a57186725a5a69d165c
```

Conclusao: o `audio_id` real resolve metadados na Graph API, a URL temporaria permite download
autenticado e o SHA-256 do arquivo baixado bate com o SHA-256 informado pela Meta. O audio bruto nao
foi persistido.

### Efeito no Runtime Agentivo

Consultas na mesma janela:

- `platform_runs`: `0`
- `platform_messages`: `0`

Conclusao: o audio nao chegou ao `AgentRuntime`, nao criou `Run`, nao persistiu mensagem de usuario e
nao teve chance de acionar tool financeira.

## Conclusao Para SDD

- O parser/rota atual devem ser alterados antes do consumer para preservar modalidade `audio`.
- O payload outbox `agents.whatsapp.inbound.v1` deve virar contrato tipado multimodal ou ganhar campos
  de audio confirmados por payload real: `audio_id`, `mime_type`, `media_sha256`, `voice` e `message_type`.
- O consumer atual esta correto em falhar fechado para `text=""`, mas essa falha vai para dead-letter e
  nao responde ao usuario com orientacao especifica.
- O SDD deve tratar esse caso como `AudioReceivedWithoutCanonicalText` no desenho, ate existir download
  de midia e STT.
- A implementacao deve garantir que audio incerto continue sem `platform_runs`, sem `platform_messages`,
  sem `HandleInbound` e sem tool call.
- O SDD pode considerar fechado o gate de Media API para o formato observado `audio/ogg`.
