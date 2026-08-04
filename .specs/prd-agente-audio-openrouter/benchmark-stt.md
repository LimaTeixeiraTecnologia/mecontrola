# Benchmark STT OpenRouter: audio PT-BR WhatsApp

Data: 2026-08-04
Status: smoke benchmark e lote minimo de 30 audios reais concluidos

## Objetivo

Validar, com audio real sanitizado, se o endpoint STT do OpenRouter transcreve PT-BR em formato util
ao fluxo financeiro existente, sem acionar o `AgentRuntime` e sem persistir audio bruto.

## Amostra

| Campo | Valor |
|---|---|
| Arquivo original | `/Users/jailtonjunior/Downloads/Rua Jose Pontes.m4a` |
| Copia de trabalho | `tmp/audio-benchmark/audio-ptbr-001.m4a` |
| SHA-256 | `927a443e2cd0e07e2cbf6186fc11576737761bf78e6a4a0853165e395e1f3532` |
| Tamanho | `76550` bytes de audio; arquivo `78K` |
| Formato | M4A/AAC, stereo, 48000 Hz |
| Duracao estimada | `4.693333s` |
| Bitrate | `128727 bps` |

O conteudo textual do audio nao deve ser publicado em logs, docs de refinamento ou respostas do
agente. Para comparar estabilidade entre modelos, este documento registra apenas tamanho, hash e
caracteristica funcional da transcricao.

## Modelos Listados na Conta OpenRouter

Consulta executada em 2026-08-04:

```bash
curl -fsS --max-time 30 \
  -H "Authorization: Bearer <redacted>" \
  "$OPENROUTER_BASE_URL/api/v1/models?output_modalities=transcription"
```

Modelos retornados:

| Modelo | Pricing prompt | Pricing completion |
|---|---:|---:|
| `fish-audio/transcribe-1` | `0.0001` | `0` |
| `x-ai/grok-stt-1.0` | `0.1` | `0` |
| `deepgram/nova-3` | `0.0043` | `0` |
| `microsoft/mai-transcribe-1.5` | `0.36` | `0` |
| `nvidia/parakeet-tdt-0.6b-v3` | `0.0015` | `0` |
| `mistralai/voxtral-mini-transcribe` | `0.003` | `0` |
| `qwen/qwen3-asr-flash-2026-02-10` | `0.000035` | `0` |
| `google/chirp-3` | `0.016` | `0` |
| `openai/gpt-4o-mini-transcribe` | `0.00000125` | `0.000005` |
| `openai/whisper-large-v3` | `0.0015` | `0` |
| `openai/whisper-large-v3-turbo` | `0.04` | `0` |
| `openai/whisper-1` | `0.006` | `0` |
| `openai/gpt-4o-transcribe` | `0.0000025` | `0.00001` |

## Chamada STT Usada

Endpoint:

```text
POST /api/v1/audio/transcriptions
```

Payload:

```json
{
  "model": "<modelo>",
  "input_audio": {
    "data": "<base64-redacted>",
    "format": "m4a"
  },
  "language": "pt",
  "temperature": 0
}
```

## Resultado Sanitizado

| Modelo | HTTP | Latencia ms | Texto bytes | SHA-256 texto | Uso/custo retornado | Resultado funcional |
|---|---:|---:|---:|---|---|---|
| `openai/gpt-4o-mini-transcribe` | 200 | 686 | 43 | `336f663b2db938e6106a545e3642cc887a6804d1892b8f54659aa8208f08bcbe` | `{"total_tokens":60,"input_tokens":47,"output_tokens":13,"cost":0.00012375}` | Transcreveu PT-BR, mas normalizou o valor monetario em palavras. |
| `openai/whisper-large-v3` | 200 | 478 | 32 | `e98ddb3f657a9fb595d4b66da81581b883c372836715bc6e3dbc71f44ce5e374` | `{"seconds":4.713375,"cost":0.000117834375}` | Transcreveu PT-BR e preservou valor monetario em formato financeiro. |
| `qwen/qwen3-asr-flash-2026-02-10` | 400 | 2679 | 0 | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | vazio | Provider retornou 400 para esta entrada M4A. |
| `openai/gpt-4o-transcribe` | 200 | 712 | 43 | `336f663b2db938e6106a545e3642cc887a6804d1892b8f54659aa8208f08bcbe` | `{"total_tokens":60,"input_tokens":47,"output_tokens":13,"cost":0.0002475}` | Transcreveu PT-BR, mas normalizou o valor monetario em palavras. |
| `mistralai/voxtral-mini-transcribe` | 200 | 1014 | 32 | `e98ddb3f657a9fb595d4b66da81581b883c372836715bc6e3dbc71f44ce5e374` | `{"seconds":4,"total_tokens":396,"input_tokens":5,"output_tokens":16,"cost":0.0002}` | Transcreveu PT-BR e preservou valor monetario em formato financeiro. |
| `deepgram/nova-3` | 200 | 1095 | 43 | `336f663b2db938e6106a545e3642cc887a6804d1892b8f54659aa8208f08bcbe` | `{"seconds":4.649375,"cost":0.0003332052083333333}` | Transcreveu PT-BR, mas normalizou o valor monetario em palavras. |

Arquivos brutos locais do smoke benchmark foram tratados como artefatos temporarios e nao devem ser
versionados. A evidencia versionada deste benchmark se limita a metadados, hashes, latencia, custo e
resultado funcional sanitizado.

## Lote Minimo de 30 Audios WhatsApp

Coleta executada em 2026-08-04 com 30 audios reais enviados diretamente pelo WhatsApp para o numero
de producao. A instrumentacao temporaria registrou apenas metadados allowlisted em Loki; audio bruto,
base64 e transcricoes completas nao foram persistidos em log.

Arquivo sanitizado do lote: `.specs/prd-agente-audio-openrouter/stt-lote-30-results.json`.

| Campo | Valor |
|---|---:|
| Eventos `type=audio` capturados | 30 |
| Downloads Meta Media API OK | 30 |
| SHA-256 do download igual ao SHA-256 da Meta | 30 |
| MIME retornado pela Meta | `audio/ogg` |
| `voice=true` no payload WhatsApp | 30 |
| Modelo STT | `openai/whisper-large-v3` |
| Formato enviado ao OpenRouter | `ogg` |
| STT HTTP 200 | 30 |
| STT erros | 0 |
| Latencia STT minima | `320ms` |
| Latencia STT p50 | `529ms` |
| Latencia STT p95 | `775ms` |
| Latencia STT maxima | `940ms` |
| Custo total do lote | `0.003295125 USD` |
| Custo medio por audio | `0.0001098375 USD` |
| Custo maximo por audio | `0.0001748375 USD` |
| Tamanho minimo do audio | `7520` bytes |
| Tamanho maximo do audio | `15717` bytes |
| Texto transcrito minimo | `25` caracteres |
| Texto transcrito maximo | `48` caracteres |

Conclusoes do lote:

- o formato real `audio/ogg` baixado da Media API foi aceito pelo `openai/whisper-large-v3` sem conversao;
- o caminho Meta `GET /{audio_id}` + download autenticado ficou validado em 30/30 casos;
- o budget inicial `AGENT_AUDIO_MAX_COST_MICROUSD=2000` por audio permanece conservador frente ao maior
  custo medido de aproximadamente `175` microusd;
- o timeout inicial `AGENT_STT_TIMEOUT=20s` permanece conservador frente ao maximo medido de `940ms`;
- o limite inicial `AGENT_AUDIO_MAX_BYTES=2000000` permanece conservador frente ao maior audio real
  medido de `15717` bytes.

## Decisao Tecnica

Modelo preferencial para a implementacao: `openai/whisper-large-v3`.

Justificativa baseada nesta amostra:

- menor latencia medida entre os modelos que retornaram HTTP 200;
- menor custo medido entre os modelos que preservaram valor monetario em formato financeiro;
- custo retornado por usage foi `0.000117834375 USD` para `4.713375s`;
- extrapolacao linear para `60s`: aproximadamente `1500` microusd por audio.

Configuracao inicial recomendada para ambiente com audio habilitado:

| Env | Valor |
|---|---|
| `AGENT_STT_MODEL` | `openai/whisper-large-v3` |
| `AGENT_AUDIO_MAX_BYTES` | `2000000` |
| `AGENT_AUDIO_MAX_COST_MICROUSD` | `2000` |
| `AGENT_STT_TIMEOUT` | `20s` |
| `AGENT_AUDIO_MAX_DURATION` | `60s` |
| `AGENT_AUDIO_MIN_CONFIDENCE` | `0.80` |

`AGENT_AUDIO_MAX_BYTES=2000000` cobre audio AAC de 60s no bitrate observado com folga operacional,
sem aceitar payloads grandes desnecessarios. `AGENT_AUDIO_MAX_COST_MICROUSD=2000` cobre o custo
extrapolado do modelo escolhido para 60s com margem sobre o valor medido nesta conta.

## Gates de Discovery Fechados

Este arquivo fecha os gates de discovery para criar tasks de codigo produtivo:

- smoke benchmark comparativo de modelos STT com audio PT-BR;
- payload WhatsApp real sanitizado;
- Meta Media API real com download autenticado e SHA-256 conferido;
- lote minimo de 30 audios reais WhatsApp;
- confirmacao de `audio/ogg` no OpenRouter sem conversao.

Readiness aqui significa pronto para decompor e iniciar implementacao controlada. Nao significa que
a feature ja esteja pronta para producao sem codigo, testes, migrations, golden set e deploy.
