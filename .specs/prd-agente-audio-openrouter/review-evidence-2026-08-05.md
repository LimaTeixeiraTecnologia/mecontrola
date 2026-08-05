# Evidencia de Review e Remediacao — 2026-08-05

Ciclo `review -> bugfix -> review` executado sobre `.specs/prd-agente-audio-openrouter/`.
Este arquivo registra **execucoes reais**, nao claims. Todo item abaixo foi rodado nesta maquina.

## 1. Achado critico: provider nao retorna `language`

Verificacao empirica com audio PT-BR real (`.m4a`, fixture fornecida pelo dono do produto) contra o
endpoint STT real do OpenRouter, via `TestRealSTT_Transcribe` com `STT_REAL_MODEL` sobrescrito:

| Modelo | `language` retornado | texto | resultado |
|---|---|---|---|
| `openai/whisper-large-v3` | `""` | 32 bytes | transcreveu corretamente |
| `openai/gpt-4o-transcribe` | `""` | 43 bytes | transcreveu corretamente |
| `openai/gpt-4o-mini-transcribe` | `""` | 43 bytes | transcreveu corretamente |
| `mistralai/voxtral-mini-transcribe` | `""` | 32 bytes | transcreveu corretamente |
| `deepgram/nova-3` | `""` | 43 bytes | transcreveu corretamente |

Saida bruta do modelo escolhido antes da correcao:

```text
STT real: provider="openrouter" language="" truncated=false seconds=4.713375 text_len=32
--- FAIL: TestRealSTT_Transcribe
    provider deve retornar o campo language; o gate de idioma de producao depende dele
```

`seconds=4.713375` e `text_len=32` batem exatamente com a linha de `openai/whisper-large-v3` em
`benchmark-stt.md:82`, confirmando consistencia com o benchmark original.

**Impacto antes da correcao:** `decide_audio_transcription.go` avaliava
`isPortugueseAudioLanguage("")` como `false`, classificando **100% dos audios** como
`TranscriptionUncertain / language_unsupported`. Com `AGENT_AUDIO_ENABLED=true` a feature estaria
completamente inoperante em producao. O defeito era invisivel porque o gate real nunca havia sido
executado (ver secao 3).

**Correcao aplicada:** fallback para o idioma requisitado em `buildTranscriptionResponse`
(`internal/platform/llm/openrouter_stt.go`). Emendas registradas em `prd.md` (RF-13/RF-14),
`techspec.md` (contrato STT) e `deployment/runbooks/audio-whatsapp-stt.md` (secao 2.1).

Apos a correcao:

```text
STT real: provider="openrouter" language="pt" truncated=false seconds=4.713375 text_len=32
--- PASS: TestRealSTT_Transcribe (1.43s)
```

## 2. Gate golden real (RF-30/RF-32/RF-34)

`RUN_REAL_LLM=1 go test -tags=integration -run GoldenAudioRealLLM ./internal/agents/application/golden`

```text
audio_group=expense      hits=9 total=9 ratio=1.0000
audio_group=income       hits=9 total=9 ratio=1.0000
audio_group=query        hits=9 total=9 ratio=1.0000
audio_group=edit         hits=9 total=9 ratio=1.0000
audio_group=confirmation hits=9 total=9 ratio=1.0000
--- PASS: TestGoldenAudioRealLLMSuite/TestGoldenAudioPairedGate (60.39s)
```

45 execucoes reais contra o LLM (15 casos x 3 repeticoes), todas as 5 categorias em `1.0000`,
acima do gate `0.90`.

Sobre "0 falso-sucesso" (RF-34): `EvaluateCaseWithCapture` reprova o caso quando a tool esperada nao
e chamada (`checkExpectedTools`, `harness.go:42`). Portanto uma resposta que **afirme** ter
registrado sem executar a tool conta como falha de caso e derruba o ratio do grupo. O resultado
`45/45` significa zero ocorrencias desse padrao nesta execucao.

## 3. Gate real de STT estava inexecutavel — falso-verde eliminado

Estado anterior: os comandos registrados no DoD das tasks 3.0, 8.0 e 10.0 nao tinham
`-tags=integration`. Como os arquivos reais sao `//go:build integration`, o comando compilava zero
testes e reportava sucesso:

```text
$ go test -count=1 ./internal/platform/llm -run Real
testing: warning: no tests to run
PASS
ok  ... [no tests to run]
```

Alem disso, mesmo com a tag e credencial validas, a suite fazia `t.Skip` por ausencia de
`STT_REAL_AUDIO_FIXTURE` — e nenhuma fixture de audio era versionada. Ou seja: o gate real nunca
rodou, em nenhum momento, apesar dos checkboxes `[x]`.

Correcoes:

1. `t.Skip` por fixture ausente virou `t.Fatal` em `realstt_test.go` e
   `harness_audio_realllm_test.go` — com `RUN_REAL_STT=1`, ausencia de fixture agora **falha**.
2. Comandos corrigidos em `task-3.0`, `task-8.0`, `task-10.0` e `techspec.md` para incluir
   `-tags=integration` e `STT_REAL_AUDIO_FIXTURE`.
3. Formato do audio passou a ser derivado da extensao do arquivo, em vez de `AudioFormatOGG`
   hardcoded.

## 4. Teto de custo rejeitava audio de 59-60s (RF-07)

Taxa de preflight era `34` microusd/s. Com `AGENT_AUDIO_MAX_DURATION=60s` e
`AGENT_AUDIO_MAX_COST_MICROUSD=2000` (ambos prescritos por `techspec.md` e `benchmark-stt.md`):
`60 x 34 = 2040 > 2000` ⇒ todo audio de 59-60s era rejeitado como `cost_exceeded`. Teto efetivo
real era `58s`, violando RF-07 e o objetivo de "ate 60 segundos". Custo real medido a 60s ≈ `1500`.

Taxa corrigida para `25` microusd/s, derivada da medicao real do benchmark
(`0.000117834375 USD / 4.713375 s = 25` exatos). Regressao coberta por
`TestDecideSTTPreflightCost/approves_max_duration_audio_under_techspec_budget`.

## 5. Audio quebrado com a flag desligada (default de producao)

Com `AGENT_AUDIO_ENABLED=false`, `buildWhatsAppAgentRoute` nao marcava `message_type=audio`; o
consumer entao tratava a mensagem como texto vazio, retornava "payload incompleto" e o evento ia
para retry/dead-letter — **sem nenhuma resposta ao usuario**. A resposta amigavel
`defaultAudioDisabledReply` era inalcancavel, porque rota e processor derivavam da mesma flag.
O runbook afirmava o comportamento oposto.

Correcao: audio passa a ser sempre tipado no payload; a decisao de processar ou responder
"indisponivel" fica no consumer, que e onde o runbook sempre disse que estava. Com isso o texto do
runbook passa a ser verdadeiro. Regressao coberta por
`TestBuildWhatsAppAgentRoute_AudioAlwaysTypedSoConsumerCanReplyWhenDisabled`.

## 6. Demais correcoes aplicadas

- `MarkDispatched` passou a emitir metrica; a dimensao `outcome="dispatched"` de RF-28 nao existia.
- Formato nao suportado passou a reportar `invalid_payload` em vez de `duration_unavailable`,
  permitindo triagem correta pelo runbook (a checagem de formato foi movida para antes da extracao
  de duracao).
- `RawPreview` removido: campo morto que carregava 256 bytes do corpo bruto (contendo a
  transcricao) — vetor de vazamento pronto para violar RF-27.
- `dispatchSpy` removido do pacote golden: reimplementava a condicao de producao e por isso nunca
  poderia falhar; o runbook o citava como "garantia auditavel" de RF-35. A prova real e
  `TestHandleAudio_UncertainNeverCallsHandleInboundOrTools`, que espiona o consumer de producao.
- `AudioConfig.MetaBaseURL` e `MediaTimeout` removidos: nunca eram preenchidos por nenhuma
  composition root e nao tinham env correspondente.
- `.env.example` passou a documentar as 9 variaveis de audio/STT, que estavam ausentes.
- Gate de cardinalidade do runbook corrigido: usava `grep` sem `-E` com alternacao `|`, o que o
  tornava um no-op que sempre passava. A afirmacao "ambos os comandos retornam vazio — confirmado"
  era falsa para o segundo comando.

## 7. Validacoes executadas

```text
go build ./...                                              OK
go vet ./...                                                OK
go vet -tags integration ./...                              OK
go test -race -count=1 ./...                                OK  149 pacotes, zero falhas
go test -tags integration -count=1 ./...                    OK  zero falhas
go test -tags integration ./migrations/...                  OK  Postgres 16 real (testcontainers)
go test -tags integration ./internal/agents/.../persistence OK  auditoria de audio, Postgres real
```

### Prova de zero regressao no fluxo textual

O gate golden textual **pre-existente** foi executado por inteiro contra o LLM real:

```text
RUN_REAL_LLM=1 go test -tags=integration -run TestGoldenRealLLM ./internal/agents/application/golden

categoria=expense_income   hits=159 total=162 ratio=0.9815
categoria=query            hits=33  total=33  ratio=1.0000
categoria=card             hits=24  total=24  ratio=1.0000
categoria=budget           hits=21  total=21  ratio=1.0000
categoria=recurrence       hits=18  total=18  ratio=1.0000
categoria=treatment_name   hits=18  total=18  ratio=1.0000
categoria=onboarding       hits=12  total=12  ratio=1.0000
... (todas as demais 14 categorias em 1.0000)
--- PASS: TestGoldenRealLLMSuite (551.66s)
```

Todas as 21 categorias acima do gate `0.90`; suite **PASS**.

Sobre as 3 falhas em `expense_income` (caso `receita edicao mudanca de data semana passada`,
`edit_entry` nao chamada): **nao e regressao deste changeset**, por duas evidencias independentes:

1. Nenhum arquivo do caminho do agente textual foi modificado — nem por esta remediacao nem pela
   feature de audio. Verificado: zero alteracoes em `application/tools`, `application/agents`,
   `application/workflows`, `application/messages` e `handle_inbound.go`.
2. O caso e **nao-deterministico**, nao um erro fixo: reexecutado isoladamente, passou em 1 de 3
   tentativas (`hits=1 total=3`), contra 0 de 3 na execucao completa. E variabilidade do LLM no
   caso textual, nao comportamento quebrado.

Margem do gate: `expense_income` tem 162 execucoes; perder mais 3 ainda deixaria `0.963`,
confortavelmente acima de `0.90`.

### Gate golden de audio (real LLM)

```text
audio_group=expense/income/query/edit/confirmation  ->  9/9 cada, ratio=1.0000
--- PASS  (45 execucoes reais)
```

Gates hard de governanca (todos vazios):

```text
R-ADAPTER-001.1  zero comentarios em .go de producao        OK
R-ADAPTER-001.2  sem SQL direto em adapters                 OK
R0               sem init()                                 OK
R5.12            sem panic em producao                      OK
R5.26            sem identificador com prefixo _            OK
RF-28            sem label de alta cardinalidade em metrica OK
R-AGENT-WF-001   sem switch por intent.Kind                 OK
```

`golangci-lint` **nao executado**: o binario local e v1 e a configuracao do repo e v2
(`you are using a configuration file for golangci-lint v2 with golangci-lint v1`). Limitacao
ambiental desta maquina, nao achado de codigo. Deve ser executado no CI antes do merge.

## 8. Gaps levantados no review e seu desfecho

Todos os itens abaixo foram fechados: corrigidos, refutados como falso positivo, ou aceitos por emenda
formal de produto. Nenhum permanece em aberto.

| # | Sev | Item | Situacao |
|---|---|---|---|
| 1 | high | ADR-002: "inserir linha antes do download" | **CORRIGIDO** — ver secao 9. |
| 2 | high | RF-30: pareamento texto/audio auto-referente | **CORRIGIDO** — ver secao 10. |
| 3 | medium | RF-13/RF-14 emendados | **FECHADO por emenda aceita** — ver secao 12. A redacao original nao e exequivel com STT do OpenRouter (provado por tres angulos); RF-13/RF-14 foram reescritos e aceitos no PRD como decisao de produto em 2026-08-05. A garantia de PT-BR vem do hint fixo `language=pt`, travado por teste. |
| 4 | low | Download bufferiza ate `maxBytes` em RAM | **CORRIGIDO** — preflight de `Content-Length` rejeita antes de ler o corpo, e o buffer e pre-alocado pelo tamanho anunciado. Teste: `rejeita por content-length antes de bufferizar o corpo`. |
| 5 | low | `size_bytes` gravado como `0` quando desconhecido | **CORRIGIDO** — migration `000018` torna a coluna nullable; `AudioAuditRecord.SizeBytes` virou `*int64`. "Desconhecido" (NULL) agora e distinguivel de "zero bytes". |
| 6 | low | ADTS MPEG-2 (`0xFFF9`) nao sincroniza | **FALSO POSITIVO — nenhuma mudanca necessaria.** `0xFFF9 & 0xFFF0 == 0xFFF0 == adtsSyncValue`: a mascara isola apenas os 12 bits do syncword e ja ignora o bit de versao MPEG. Provado por `TestDetermineDuration_ADTS_MPEG2_TambemSincroniza`, que agora guarda o comportamento. |

## 8.1 Nota sobre o achado ADTS

O achado original ("ADTS MPEG-2 nunca sincroniza") nao se confirmou na verificacao. A mascara
`adtsSyncMask = 0xFFF0` aplicada a `0xFFF9` resulta em `0xFFF0`, igual a `adtsSyncValue` — ou seja,
MPEG-2 sincroniza normalmente. Nenhuma correcao foi aplicada; foi adicionado teste de regressao para
fixar o comportamento. Registrado aqui porque relatorio de subagente nao substitui verificacao.

## 9. ADR-002 implementado — auditoria aberta antes do download

Migration `000017` adiciona o estado nao-terminal `processing` e a reason `interrupted` aos CHECKs,
mais o indice `(user_id, created_at DESC)` que faltava.

Contrato do repositorio passou a ser de duas fases:

- `InsertProcessing` — abre a linha do WAMID **antes** de resolver/baixar a midia
  (`process_audio_inbound.go`, imediatamente apos o `checkExisting`).
- `FinalizeTerminal` — transiciona para o outcome terminal com `WHERE wamid = $1 AND outcome = 'processing'`,
  de modo que uma linha ja terminal **nunca** e sobrescrita (retorna `ErrAudioAuditNotFound`).

Recuperacao de execucao interrompida: ao reencontrar um WAMID em `processing`, o use case finaliza a
linha como `transcription_failed / interrupted` e responde ao usuario pedindo reenvio, **sem**
reprocessar midia nem STT — preservando a regra do ADR-002 de que so um novo WAMID e processado.
Antes desta correcao o audio sumia em silencio nessa janela.

Cobertura:

- unit: `deve finalizar wamid interrompido sem reprocessar media nem stt`,
  `nao deve baixar midia quando abrir auditoria falhar`, e todos os cenarios passaram a exigir
  `InsertProcessing` com `outcome=processing`.
- integracao (Postgres real): `TestInsertProcessingBeforeDownloadThenFinalize_ADR002`,
  `TestFinalizeTerminalIsNotIdempotentlyReappliable_RF22`,
  `TestInterruptedProcessingCanBeFinalizedWithoutReprocessing_ADR002`.
- migration up/down/up revalidada; versao fixada nos testes atualizada para `17`.

## 10. RF-30 — pareamento agora referencia o golden textual real

`AudioPairedCase` deixou de embutir um `Case` proprio e passou a referenciar o caso textual
pre-existente por nome (`TextCaseName` + `ResolveTextCase()`). Os 15 casos de audio agora apontam
para casos que ja existiam no golden textual, e o harness resolve o caso real — herdando dele
`ExpectedTool`, `ExpectedArgs`, `ToolSubset`, `PriorTurns` e `ResponseProperty`.

O grupo `confirmation` passou a usar os tres unicos fluxos multi-turno reais do golden
(`confirmacao de escrita apos pergunta pendente`, `jornada hoje completa data da pendencia`,
`follow up reinvoca tool de fatura`), eliminando os casos que so aparentavam ser confirmacao.

`SpokenText` deixou de ser copia do input e passou a ser a variante falada (sem pontuacao, caixa
baixa, espacamento duplo tipico de STT). Novos testes:

- `TestAudioPairedCasesReferenceExistingTextualCases` — falha se um caso de audio inventar caso proprio.
- `TestSpokenTextIsAnAudioVariantNotACopy` — impede que o pareamento vire tautologia.

Gate real apos a mudanca (`RUN_REAL_LLM=1`): `expense`, `income`, `query`, `edit` e `confirmation`
todos em `9/9 ratio=1.0000`, agora exercitando os casos textuais reais.

Efeito colateral util: ao resolver os casos textuais reais, o teste de anonimizacao expos **5
telefones de producao sem mascara** em `Origin` de `cases_expense_income.go` (dado pessoal
versionado no repo). Foram mascarados para `+55****NNNN` e o teste passou a barrar telefone sem
mascara por regex, em vez do literal `+55`.

## 11. Validacao em producao — deploy e Fases 1 a 3 (2026-08-05)

Deploy via CD (`main`), commits `f487baf`, `bdb67a5`, `0957237`, `c1a95cb`, `2744f5c`, `a907612`.
Todos os gates da esteira verdes, incluindo `Quality` (golangci-lint v2), `Golden Gate (real-LLM
pre-deploy)`, `Integration Tests`, `Governance Gates`, `Scan & Sign Image`, `Deploy Swarm Stack` e
`Healthcheck`.

Estado de producao verificado por `ssh` na VPS:

```text
migration       v18, dirty=false   (era v14 antes do deploy)
tabela          mecontrola.agents_whatsapp_audio_messages criada
colunas         size_bytes/duration_ms/transcription/cost_microusd nullable
CHECK outcome   inclui 'processing' (ADR-002)
indices         pkey + created_at + user_id_created_at
midia bruta     0 colunas proibidas (RF-24)
servicos        caddy, server-1/2, worker-1/2 -> todos 1/1
AGENT_AUDIO_ENABLED=true em server e worker (lido de /proc/<pid>/environ)
```

### Fase 1 — regressao do fluxo textual

3 eventos `agents.whatsapp.inbound.v1`, todos `status=3` / `attempts=0`. Conversa completa:
lancamento -> pergunta de forma de pagamento -> confirmacao -> registro. `platform_runs` 181 -> 184.
Nenhuma regressao.

### Fase 2 — audio com a flag desligada

O defeito corrigido, comprovado em producao:

| Sinal | Antes da correcao | Depois |
|---|---|---|
| `message_type` no payload | ausente | `audio` |
| `status` do evento | 4 (dead-letter) | **3 (processado)** |
| erro | `payload incompleto` | **sem erro** |
| resposta ao usuario | silencio | mensagem de indisponivel |
| linhas de auditoria | — | **0** (nao tocou Meta nem OpenRouter) |

### Fase 3 — audio habilitado

| Cenario | Resultado |
|---|---|
| 3.1 caminho feliz | `"Gastei R$ 50,00 no mercado hoje no débito."` -> `dispatched/approved`, 5,2 s, 130 microusd |
| 3.2 incerteza tecnica | silencio de 31 s -> provider devolveu texto vazio -> `transcription_failed / stt_empty_text`, **runs=0** |
| 3.3 idempotencia | 0 WAMID duplicado |
| 3.4 audio longo | **59.340 ms**, custo real **1.483 microusd** vs estimativa 1.500 do preflight (teto 2.000) |
| 3.5 privacidade | 0 logs com transcricao, base64 ou URL da Meta |
| 3.6 ADR-002 | linha abriu em `processing` antes do download e finalizou; **0** presas |

O cenario 3.4 e a prova direta da correcao da taxa de preflight: com a taxa antiga de 34 microusd/s
a estimativa seria 2.040 > 2.000 e o audio de 59 s teria sido **rejeitado** como `cost_exceeded`. A
taxa corrigida de 25 microusd/s estimou 1.500 contra 1.483 reais — erro de 1%.

Invariantes finais: `wamid_duplicado=0`, `preso_em_processing=0`, `outbox_dead_letter=0` (2 h).
Total: 7 audios processados, custo acumulado ~1.9 mUSD.

Metricas confirmadas no Prometheus: `agents_audio_inbound_total` por outcome, mais os histogramas
`agents_audio_transcription_latency_seconds`, `agents_audio_download_latency_seconds`,
`agents_audio_size_bytes`, `agents_audio_duration_seconds` e o contador
`agents_audio_cost_microusd_total`. Alertas `mc-audio-*` (5) provisionados; dashboard
`agent-audio-whatsapp.json` presente.

### Nota sobre falhas transitorias da esteira

O job `Deploy Swarm Stack` falhou uma vez com `ssh: connect to host *** port 22: Connection timed
out`, e o `Healthcheck` falhou duas vezes com `curl (28)`. Nenhuma das duas foi indisponibilidade
real: TCP/22 respondia por IPv4, IPv6 e localhost, e `/healthz` e `/readyz` retornaram 200 da VPS, do
usuario `github-runner` e de maquina externa durante toda a investigacao. Causa do healthcheck: o
deploy envia `SIGTERM` ao Caddy (registrado no log as 16:04:21) e o job, que roda em runner hospedado
no GitHub, corria contra a convergencia do proxy; cada `gh run rerun --failed` reexecutava o deploy
junto, reproduzindo a corrida. Re-executando **apenas** o job de healthcheck, passou em 5 s.

## 12. Achado: `language` e `confidence` do provider

Verificacao com audio PT-BR real contra o OpenRouter, tres angulos:

1. Formato padrao: os 5 modelos do benchmark devolvem `language=""`.
2. `response_format=verbose_json`: `language` passa a vir, mas e **eco do parametro enviado**
   (`pt` -> `pt`, `en` -> `en`), nao deteccao.
3. **Sem** enviar `language`: o provider classificou audio PT-BR como `en` e **traduziu** o texto
   (`"Gastei R$ 25,00 no mercado hoje."` -> `"I spent R$ 25 on the market today"`).

`confidence`, `avg_logprob` e `no_speech_prob` nao vem em nenhum formato; os segmentos do
`verbose_json` trazem apenas `id`, `start`, `end`, `text`.

Conclusao: a deteccao de idioma nao e exequivel com este provider, e a garantia de PT-BR vem do
**hint fixo** `language=pt`. Remover o hint faz o provider traduzir para ingles e quebrar o parsing
financeiro — risco travado por `TestTranscribe_ProviderOmitsLanguageFallsBackToRequested`.
RF-13/RF-14 foram formalmente **emendados e aceitos** no PRD com base nesta evidencia.
