# Refinamento: agente Me Controla com audio via OpenRouter

Data: 2026-08-04

Status: rodada inicial de grilling respondida; pronto para conversao em PRD, ainda nao aprovado para implementacao.

## Objetivo

Permitir que o agente do Me Controla receba audio e mantenha exatamente as mesmas funcionalidades hoje disponiveis por texto, com eficiencia, economia, robustez production-ready/proof, sem inventar resposta e usando OpenRouter como provider unico.

Done esperado em uma frase: mensagens de audio devem virar entrada textual auditavel, passar pelo mesmo runtime/tool-calling/gates do agente textual e retornar resposta segura, sem perda funcional nem caminho paralelo de negocio.

## Decisoes consolidadas em 2026-08-04

| Pergunta | Decisao | Consequencia |
|---|---|---|
| Escopo de audio | Apenas entrada por audio no primeiro corte. | Resposta continua textual; TTS fica fora do PRD inicial. |
| Retencao do audio original | Descartar audio apos transcricao e guardar apenas hash/metadados. | Menor risco de privacidade e menor custo de armazenamento. |
| Idioma | PT-BR apenas. | Melhor controle de golden set, menor ambiguidade e rejeicao segura de idioma incerto. |
| Mutacoes financeiras | Podem executar sem confirmacao adicional se a transcricao passar os gates. | UX fluida, mas exige gates fortes contra falso-sucesso. |
| Custo/latencia | Fechar duracao, timeout e p95 no PRD; fechar custo financeiro na techspec por benchmark. | Implementacao fica bloqueada ate a techspec definir custo maximo por audio e modelo STT. |

## Governanca obrigatoria

O PRD, a techspec, as tasks e qualquer implementacao Go desta iniciativa devem usar obrigatoriamente:

- `domain-modeling-production`: fechar linguagem ubiqua, estados, comandos, eventos, invariantes e erros antes de backlog ou codigo.
- `design-patterns-mandatory`: executar gate de desenho antes da techspec; decisao preliminar deste refinamento e `nao aplicar padrao` no dominio e reaproveitar adapters finos/substrato existente. Se a techspec introduzir nova abstracao estrutural, o seletor deterministico da skill deve ser executado antes da implementacao.
- `mastra`: preservar o substrato real `internal/platform/{agent,llm,memory,workflow,tool,scorer}` e o consumidor `internal/agents`; consumir primitivos existentes, sem criar agente paralelo, fallback chain, workflow kernel especifico de audio ou tool com regra de negocio.
- `go-implementation`: aplicar etapas 1-6 antes de qualquer alteracao Go; classificar superficie, carregar referencias proporcionais, modelar antes de editar e validar com build/vet/test/race no escopo afetado.

Gate de arquitetura para esta iniciativa:

- Audio entra como extensao de inbound/canonical input, nao como novo agente.
- STT OpenRouter deve ser adapter fino em fronteira tecnica; regra financeira permanece no fluxo textual/tool-calling existente.
- `internal/platform/workflow` nao deve ser alterado para conhecer audio, WhatsApp, OpenRouter ou semantica financeira.
- Tools financeiras existentes continuam sendo a unica via de mutacao; audio aprovado apenas fornece a mensagem canonica.

## Evidencias confirmadas

| Fonte | Evidencia | Impacto |
|---|---|---|
| `docs/diagrams/agent/03-agent-llm-openrouter.md:5` | O substrato atual usa `internal/platform/{agent,llm,memory,tool,workflow,scorer}` e OpenRouter como provider unico. | Audio nao deve introduzir outro provider LLM nem fallback chain. |
| `docs/diagrams/agent/03-agent-llm-openrouter.md:58` | O agente executa loop de tool-calling com limite de rounds. | Audio precisa convergir para a mesma mensagem de usuario antes do loop. |
| `docs/diagrams/agent/03-agent-llm-openrouter.md:88` | Mensagens de usuario e assistente sao persistidas no `MessageStore`. | A transcricao precisa ser persistida de forma auditavel, com vinculo ao audio original. |
| `docs/diagrams/agent/02-fluxo-webhook-dispatcher.md:55` | O dispatcher extrai hoje a primeira mensagem e o texto do payload WhatsApp. | O parser inbound precisa reconhecer payload de audio alem de texto. |
| `docs/diagrams/agent/02-fluxo-webhook-dispatcher.md:66` | Dedup por WAMID ja existe antes de rotear para agente. | Audio deve reutilizar a deduplicacao pelo WAMID, nao criar idempotencia paralela. |
| `docs/diagrams/agent/02-fluxo-webhook-dispatcher.md:96` | Principal autenticado e injetado no contexto antes do agente. | A autorizacao deve permanecer identica para texto e audio. |
| `internal/platform/llm/types.go:34` | `llm.Request` hoje carrega `Messages`, `Tools`, `ToolChoice`, schema, tokens e temperatura. | O contrato atual e textual/tool-calling; audio bruto nao cabe sem extensao deliberada. |
| `internal/platform/llm/openrouter.go:22` | O provider implementa `/chat/completions` e `/embeddings`. | STT/TTS exigem novos endpoints ou novo client fino no mesmo bounded technical capability. |
| `internal/platform/llm/openrouter.go:56` | `NewOpenRouterProvider` centraliza metricas de chamada, erro, tokens e latencia. | Audio precisa metricas equivalentes para custo, erro e latencia. |

## Documentacao OpenRouter consultada

| Tema | Fonte | Implicacao |
|---|---|---|
| Audio multimodal | https://openrouter.ai/docs/guides/overview/multimodal/audio | OpenRouter aceita entrada de audio em `/api/v1/chat/completions` com `input_audio` base64 para modelos compativeis; URL direta nao e suportada para audio. |
| Speech-to-text | https://openrouter.ai/docs/guides/overview/multimodal/stt | OpenRouter oferece `/api/v1/audio/transcriptions` para transcricao dedicada, com audio base64 ou multipart OpenAI-compatible. |
| Text-to-speech | https://openrouter.ai/docs/guides/overview/multimodal/tts | OpenRouter oferece `/api/v1/audio/speech` para gerar audio a partir de texto, retornando stream de bytes. |
| Descoberta de modelos | https://openrouter.ai/docs/api/api-reference/models/list-all-models-and-their-properties | A API de modelos permite filtrar por `input_modalities` e `output_modalities`, util para escolher STT/TTS por capacidade real e custo. |

## Decisao recomendada

Usar STT dedicado como primeira etapa: `audio WhatsApp -> download seguro -> validacao -> transcricao OpenRouter -> texto normalizado -> fluxo textual existente do agente`.

Racional:

- Economia: uma transcricao curta evita mandar audio bruto para o modelo agente em todo turno e permite reutilizar prompts, tools, scorers, memoria e golden sets textuais.
- Robustez: falhas de transcricao viram erro tipado antes do agente; o agente nao precisa inferir quando o audio estiver ruim, vazio, truncado ou em idioma inesperado.
- Compatibilidade: preserva o runtime atual, o loop de tool-calling, o provider OpenRouter unico, a deduplicacao por WAMID e o `MessageStore`.
- Anti-alucinacao: o agente so recebe texto transcrito com metadados de confianca; quando a transcricao for tecnicamente incerta, a resposta correta e pedir reenvio, nao executar tool financeira.

Alternativas descartadas neste refinamento:

- Enviar audio diretamente ao modelo principal em `/chat/completions`: aumenta custo por turno, acopla o agente a modelos multimodais e dificulta manter exatamente os mesmos gates textuais.
- Criar fluxo de negocio paralelo para audio: viola o objetivo de mesmas funcionalidades e aumenta risco de drift entre texto e audio.
- TTS obrigatorio no primeiro corte: muda a experiencia de saida, aumenta custo e exige decisoes de voz/formato que nao sao necessarias para equivalencia funcional de entrada.

## Modelo de dominio proposto

### Linguagem ubiqua

| Termo | Definicao |
|---|---|
| Mensagem de audio | Mensagem inbound do canal WhatsApp cujo conteudo principal e midia de audio/voz. |
| Transcricao | Texto derivado do audio por STT, usado como entrada canonica do agente. |
| Entrada canonica | Texto final que entra no `AgentRuntime`, independentemente de ter vindo de texto digitado ou audio transcrito. |
| Audio original | Midia recebida do WhatsApp, usada temporariamente para transcricao e descartada apos gerar hash/metadados. |
| Qualidade de transcricao | Estado fechado que representa se a transcricao pode ser usada sem risco material. |
| Reenvio por baixa confianca | Resposta ao usuario pedindo novo audio quando a transcricao nao sustenta entrada canonica segura. |

Termos proibidos por enquanto:

- `audio intent`: o intent continua sendo decidido pelo agente/tool-calling a partir da entrada canonica.
- `voice agent separado`: o objetivo e o mesmo agente, nao uma capacidade concorrente.
- `fallback LLM`: o projeto declara OpenRouter como provider unico, sem cadeia de fallback.

### Estados fechados sugeridos

| Estado | Significado | Proxima acao |
|---|---|---|
| `AudioReceived` | Payload reconhecido como audio e ainda nao baixado/transcrito. | Baixar midia via adapter WhatsApp com timeout e limite de tamanho. |
| `AudioRejected` | Audio invalido por formato, tamanho, duracao, autorizacao, expiracao ou falha de download. | Responder com mensagem objetiva sem acionar o agente financeiro. |
| `TranscriptionSucceeded` | STT retornou texto utilizavel e metadados minimos. | Encaminhar texto ao fluxo atual do agente. |
| `TranscriptionUncertain` | STT retornou texto vazio, truncado, idioma nao PT-BR, transcricao incoerente/ininteligivel ou baixa confianca operacional. | Pedir reenvio antes de qualquer tool. |
| `TranscriptionFailed` | OpenRouter/STT falhou por erro tecnico, timeout, limite ou resposta invalida. | Degradar com resposta segura e metricas de erro. |
| `CanonicalInputDispatched` | Texto transcrito foi aceito como entrada canonica. | Seguir runtime existente. |

## Workflow recomendado

```text
receber webhook
  -> validar assinatura, timestamp, dedup e principal como hoje
  -> detectar tipo de mensagem
  -> se texto: seguir fluxo atual
  -> se audio: baixar midia pelo adapter WhatsApp
  -> validar formato, tamanho, duracao e content-type
  -> transcrever via OpenRouter STT
  -> classificar qualidade da transcricao
  -> se tecnicamente incerta: responder pedindo reenvio
  -> se valida: montar entrada canonica textual
  -> executar AgentRuntime existente
  -> persistir mensagem original, transcricao, run e resposta
  -> publicar resposta no WhatsApp
```

## Regras e invariantes

- O audio nunca executa tool financeira diretamente; somente a entrada canonica textual pode entrar no loop do agente.
- A mesma mensagem WhatsApp (`WAMID`) deve ser processada no maximo uma vez, inclusive quando o download ou STT falhar.
- Transcricao vazia, truncada, incoerente/ininteligivel, fora de PT-BR ou tecnicamente incerta nao pode gerar lancamento, edicao, exclusao, consulta sensivel ou confirmacao falsa.
- O agente nao deve dizer que entendeu valores, datas, categorias ou cartoes quando esses elementos nao estiverem sustentados pela transcricao.
- Quando a transcricao for tecnicamente confiavel, mas houver duvida material em valor, data, entidade financeira, comando ou consentimento, a saida deve seguir o mesmo fluxo textual existente de pergunta, confirmacao ou suspensao.
- Quando a transcricao em PT-BR passar todos os gates, mutacoes financeiras podem seguir o mesmo criterio do texto digitado, sem confirmacao adicional exclusiva por ser audio.
- OpenRouter permanece provider unico para LLM/STT/TTS; se um modelo ou endpoint nao suportar a modalidade exigida, o sistema degrada de forma tipada.
- Limites de tamanho, duracao, timeout, retry e custo devem ser configuraveis e observaveis.
- Audio original deve ser descartado apos transcricao; persistir apenas hash, metadados tecnicos e transcricao pelo mesmo ciclo das mensagens textuais.

## Fronteiras tecnicas

| Componente | Responsabilidade |
|---|---|
| `internal/platform/whatsapp/payload` | Extrair mensagem de audio sem decidir regra de negocio. |
| `internal/platform/whatsapp` adapter | Baixar midia do WhatsApp com timeout, limite e erro tipado. |
| `internal/platform/llm` | Expor cliente fino para STT OpenRouter e, futuramente, TTS, sem regra financeira. |
| `internal/agents/application` | Orquestrar `parse -> validate -> transcribe -> decide dispatch -> runtime`; sem SQL direto em handler/consumer. |
| `internal/platform/agent` | Permanecer agnostico de audio; recebe entrada canonica textual e metadados. |
| `internal/platform/scorer` | Avaliar regressao, anti-alucinacao e equivalencia texto/audio. |

## Observabilidade e operacao

Metricas minimas:

- `agents_audio_inbound_total{outcome}` com outcomes fechados: `accepted`, `rejected`, `transcription_failed`, `transcription_uncertain`, `dispatched`.
- `agents_audio_transcription_latency_seconds{model}`.
- `agents_audio_transcription_cost_total{model}` quando a resposta do provider expuser custo/uso.
- `agents_audio_bytes_total{format}` sem labels de usuario, telefone, WAMID ou conteudo.
- `agents_audio_duration_seconds{format}` se a duracao for extraida localmente com baixo custo.
- `agent_llm_provider_errors_total{reason}` deve incluir falhas STT/TTS de forma distinguivel.

Logs/auditoria:

- Registrar `run_id`, `thread_id`, `agent_id`, status, duracao, modelo STT, tamanho, formato, hash do audio e erro tipado quando houver.
- Nao logar transcricao completa em nivel `info` se ela puder conter dado financeiro sensivel.
- Guardar transcricao no mesmo mecanismo auditavel de mensagens, marcada como derivada de audio.

Alertas candidatos:

- Aumento de `transcription_failed` em janela curta.
- Aumento de `transcription_uncertain`, pois pode indicar modelo ruim, audio ruim ou mudanca no payload do WhatsApp.
- Latencia STT acima do budget operacional.
- Custo STT por mensagem acima do teto definido.

## Golden set e validacao

Antes de considerar production-ready, criar golden set pareado:

- Mesmo comando em texto e em audio claro deve acionar a mesma tool, argumentos equivalentes e mesma necessidade de confirmacao.
- Audio com ruido, fala cortada, idioma nao PT-BR, transcricao incoerente ou falha STT deve pedir reenvio e nao executar tool.
- Audio em PT-BR informal deve preservar semantica financeira sem inventar categoria, cartao ou recorrencia.
- Audio duplicado pelo mesmo WAMID nao deve gerar segunda execucao.
- Falha de download, formato nao suportado, audio grande e timeout STT devem resultar em erro seguro para usuario.

Gates sugeridos:

- Score de equivalencia texto/audio por grupo de intencao >= 0,90.
- Zero falso-sucesso em comandos financeiros mutacionais.
- Zero tool call quando `TranscriptionUncertain`.
- Teste real com OpenRouter habilitado por flag, separado dos testes unitarios.

## ADRs candidatas

### ADR-001: STT dedicado antes do AgentRuntime

Decisao consolidada: audio inbound sera transcrito por endpoint dedicado OpenRouter STT antes de entrar no agente.

Consequencias:

- Mantem o runtime textual e as tools existentes.
- Facilita budget de custo por etapa.
- Exige novo contrato para transcricao e tratamento de erros.
- Nao entrega resposta em audio automaticamente.

### ADR-002: Audio nativo em chat fica fora do primeiro corte

Decisao consolidada: nao enviar audio bruto para `/chat/completions` no primeiro corte.

Consequencias:

- Reduz custo e variabilidade.
- Evita depender de modelos multimodais no modelo principal.
- Pode perder sinais paralinguisticos, mas esses sinais nao sao necessarios para funcionalidades financeiras atuais.

### ADR-003: TTS fica fora do primeiro corte

Decisao consolidada: saida em audio nao entra na definicao de "mesmas funcionalidades" do primeiro corte.

Consequencias:

- Mantem escopo enxuto.
- Evita custo adicional por resposta.
- Permite evoluir para TTS com `/audio/speech` depois, com voz/formato/retencao definidos em refinamento proprio.

### ADR-004: Audio aprovado pode executar mutacoes sem confirmacao extra

Decisao consolidada: comandos financeiros por audio podem executar sem confirmacao adicional quando a transcricao PT-BR passar os gates de qualidade, anti-alucinacao e equivalencia texto/audio.

Consequencias:

- Mantem a mesma fluidez do texto digitado.
- Exige bloqueio absoluto de tool call em `TranscriptionUncertain`.
- Exige golden set com zero falso-sucesso em comandos mutacionais.
- Nao dispensa confirmacoes ja existentes no fluxo textual.

## Perguntas de grilling respondidas

1. A equivalencia funcional exige apenas entrada por audio ou tambem resposta em audio?
   - Resposta: A. Apenas entrada por audio; menor custo e menor risco.

2. Qual politica de retencao do audio original?
   - Resposta: A. Descartar audio apos transcricao, guardar hash/metadados; menor risco de privacidade.

3. Qual idioma suportado no primeiro corte?
   - Resposta: A. PT-BR apenas; melhor qualidade de golden set e menor ambiguidade.

4. O sistema pode executar mutacoes financeiras a partir de audio transcrito sem confirmacao adicional?
   - Resposta: A. Sim, se a transcricao passar gates; maior fluidez, maior risco.

5. Qual teto de custo/latencia por audio?
   - Resposta consolidada pelo PRD: duracao maxima, timeout e p95 foram fechados no PRD; custo financeiro maximo deve ser fechado por benchmark na techspec antes da implementacao.

## Itens em aberto

- Resolvido no PRD: canal inicial e exclusivamente WhatsApp Cloud API.
- Resolvido no PRD: duracao maxima inicial de 60 segundos, timeout STT de 20 segundos e p95 alvo de transcricao de 8 segundos.
- Gate tecnico antes da implementacao: confirmar formatos reais enviados pelo WhatsApp para mensagens de voz com payload real mascarado.
- Gate tecnico antes da implementacao: escolher modelo STT OpenRouter por benchmark de preco, latencia, PT-BR, disponibilidade e formato suportado.
- Gate tecnico antes da implementacao: definir tamanho maximo em bytes, custo maximo por audio, ambiente/carga de medicao de p95 e thresholds de pos-deploy gate, mantendo zero falso-sucesso em mutacoes.

## Proximo passo recomendado

Converter este refinamento em PRD. Depois disso, gerar techspec com foco em:

- contrato `TranscriptionClient` em `internal/platform/llm` ou pacote tecnico equivalente;
- extensao do parser WhatsApp para audio;
- use case fino em `internal/agents/application` para transformar audio em entrada canonica;
- scorers/golden set pareados texto/audio;
- runbook de custo, falha e privacidade.
