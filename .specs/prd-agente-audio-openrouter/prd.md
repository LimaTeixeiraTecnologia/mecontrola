# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->

Status: completo para handoff de especificacao tecnica; nao autoriza implementacao sem techspec, ADRs e tasks aprovadas.

## Visao Geral

O Me Controla deve permitir que usuarios enviem mensagens de audio pelo WhatsApp Cloud API para executar as mesmas funcionalidades hoje disponiveis por texto no agente financeiro. A equivalencia funcional minima cobre despesas, receitas, consultas, edicoes e fluxos de confirmacao existentes. O audio sera convertido em uma entrada textual canonica por transcricao STT via OpenRouter e, quando aprovado pelos gates tecnicos de qualidade, seguira o mesmo fluxo do agente textual, com as mesmas tools, memoria, confirmacoes ja existentes, idempotencia e resposta textual.

O primeiro corte nao inclui resposta em audio. O objetivo e remover atrito de digitacao sem criar um agente paralelo, sem duplicar regras financeiras, sem inventar resposta quando a transcricao for incerta e sem ampliar desnecessariamente custo ou superficie operacional.

## Decisoes Fechadas

| Tema | Decisao |
|---|---|
| Canal inicial | Exclusivamente WhatsApp Cloud API. |
| Direcao de audio | Entrada por audio apenas; resposta permanece textual. |
| Provider | OpenRouter como provider unico para LLM e STT. |
| Estrategia de processamento | STT dedicado antes do `AgentRuntime`; audio bruto nao entra no modelo principal. |
| Idioma | PT-BR apenas no primeiro corte. |
| Retencao | Descartar audio original apos transcricao/rejeicao; reter hash, metadados e transcricao quando aplicavel. |
| Mutacoes financeiras | Permitidas sem confirmacao adicional exclusiva por audio quando a transcricao passar gates e o fluxo textual ja permitir. |
| Discovery tecnico | Payload real de audio WhatsApp e benchmark STT sao gates obrigatorios antes de codar. |
| Pattern | `nao aplicar padrao` por padrao; reaproveitar adapters finos e substrato Mastra Go existente. |

## Atores

- Usuario final autenticado no WhatsApp: envia audio e recebe resposta textual.
- Operador/engenharia: acompanha custo, latencia, falhas, incerteza e regressao por golden set.
- WhatsApp Cloud API: canal externo de recebimento da midia e envio da resposta textual.
- OpenRouter: provider externo unico para STT e LLM.
- Agente `mecontrola`: consumidor agentivo existente que decide e executa tools financeiras a partir da entrada canonica textual.

## Objetivos

- Permitir entrada por audio no WhatsApp mantendo equivalencia funcional com mensagens textuais.
- Preservar OpenRouter como provider unico para LLM/STT, sem fallback chain.
- Garantir que audio incerto, invalido, fora de PT-BR ou tecnicamente falho nao execute mutacoes financeiras.
- Manter duracao e latencia sob budget inicial explicito: audio de ate 60 segundos, timeout STT de 20 segundos e p95 alvo de ate 8 segundos para a etapa de transcricao.
- Reduzir risco de privacidade descartando o audio original apos transcricao e mantendo apenas hash, metadados tecnicos e transcricao auditavel.
- Validar a entrega com golden set pareado texto/audio, exigindo score por grupo de intencao >= 0,90 e 0 falso-sucesso em mutacoes.

## Metricas de Produto e Operacao

| Metrica | Meta inicial |
|---|---|
| Sucesso de transcricao em audios validos PT-BR | Medir em beta; nao pode mascarar falso-sucesso. |
| Score golden texto/audio por grupo de intencao | >= 0,90. |
| Falso-sucesso em mutacoes financeiras por audio | 0. |
| Tool call em `TranscriptionUncertain` | 0. |
| Duplicidade por mesmo WAMID | 0. |
| p95 de transcricao | <= 8 segundos. |
| Timeout STT | 20 segundos, configuravel. |
| Custo maximo por audio | Deve ser fechado na techspec por benchmark antes da implementacao. |
| Retencao de audio original apos processamento | 0 arquivos/objetos persistidos. |

## Historias de Usuario

- Como usuario do Me Controla no WhatsApp, quero enviar um audio dizendo uma despesa, receita, consulta ou pedido financeiro para nao precisar digitar no celular.
- Como usuario, quero receber uma resposta textual clara quando o audio for entendido para confirmar que o agente fez a mesma acao que faria por texto.
- Como usuario, quero que o agente peca reenvio ou confirmacao quando o audio estiver ruim, incompleto, fora de PT-BR ou ambiguo para evitar lancamentos errados.
- Como usuario, quero que audios duplicados ou reenviados pelo WhatsApp nao gerem lancamentos duplicados.
- Como usuario, quero editar ou confirmar uma acao financeira por audio quando esse fluxo ja existir por texto para manter a mesma experiencia sem aprender um caminho novo.
- Como operador do sistema, quero observar falhas, latencia, custo e incerteza de transcricao para operar a funcionalidade com seguranca.

## Funcionalidades Core

### Entrada por audio no WhatsApp

O sistema deve reconhecer mensagens inbound de audio enviadas pelo WhatsApp Cloud API, mantendo as validacoes atuais de assinatura, timestamp, deduplicacao, principal autenticado e rate limit. Mensagens textuais devem continuar usando o fluxo atual.

### Download e validacao de midia

O sistema deve baixar a midia de audio do WhatsApp apenas quando o usuario estiver autorizado e o WAMID ainda nao tiver sido processado. O PRD exige discovery tecnico com payload real do WhatsApp antes da implementacao para confirmar campos, formatos e limites reais; sem esse discovery, a implementacao nao deve assumir contrato de payload.

### Transcricao STT via OpenRouter

O sistema deve transcrever audio por endpoint STT do OpenRouter. O PRD nao fixa o modelo: a techspec deve escolher o modelo por benchmark documentado de custo, latencia, disponibilidade e qualidade em PT-BR.

### Entrada canonica textual

Quando a transcricao for aprovada, o sistema deve transformar o resultado em entrada textual canonica e encaminhar ao fluxo existente do agente, sem criar agente separado e sem alterar as tools financeiras como consequencia direta do audio.

### Tratamento de incerteza

Quando a transcricao for tecnicamente incerta, o sistema deve responder de forma segura pedindo reenvio, sem tool call financeira e sem criar estado pendente de confirmacao. A incerteza tecnica inclui texto vazio, erro STT, idioma nao PT-BR, truncamento, baixa confianca quando disponivel ou transcricao incoerente/ininteligivel. Quando a transcricao for tecnicamente confiavel, mas faltar dado financeiro exigido, o sistema deve seguir o mesmo fluxo textual existente de pergunta, confirmacao ou suspensao, sem criar regra especial por audio.

### Auditoria, privacidade e operacao

O sistema deve registrar metadados suficientes para auditoria e troubleshooting sem reter o audio original. Devem ser observaveis outcomes de audio aceito, rejeitado, falha de transcricao, transcricao incerta e dispatch para o agente.

## Requisitos Funcionais

- RF-01: O sistema deve aceitar mensagens de audio exclusivamente pelo WhatsApp Cloud API no primeiro corte.
- RF-02: O sistema deve preservar o fluxo atual para mensagens textuais sem mudanca funcional.
- RF-03: O sistema deve executar discovery tecnico com payload real de audio WhatsApp antes de qualquer implementacao, confirmando campos de media id, tipo, MIME, hash, timestamp e metadados disponiveis.
- RF-04: O discovery tecnico deve produzir uma lista versionada de campos obrigatorios, campos opcionais, formatos suportados, limites observados e exemplos sanitizados de payload aceito e rejeitado.
- RF-05: O sistema deve rejeitar de forma segura payload de audio que nao contenha os campos obrigatorios confirmados no discovery tecnico.
- RF-06: O sistema deve baixar a midia de audio do WhatsApp usando fronteira tecnica com timeout explicito, autenticacao apropriada e limite de tamanho configuravel.
- RF-07: O sistema deve rejeitar audio que exceda 60 segundos quando a duracao estiver disponivel antes da transcricao.
- RF-08: O sistema deve rejeitar audio acima do tamanho maximo configurado; o valor inicial em bytes deve ser fechado na techspec antes da implementacao e coberto por teste de aceite.
- RF-09: O sistema deve enviar audio para transcricao STT via OpenRouter, mantendo OpenRouter como provider unico.
- RF-10: A techspec deve selecionar o modelo STT OpenRouter por benchmark documentado de PT-BR, custo, latencia, disponibilidade e formato suportado.
- RF-11: O sistema deve usar timeout STT inicial de 20 segundos por audio, configuravel por ambiente.
- RF-12: O sistema deve atingir p95 alvo de ate 8 segundos na etapa de transcricao no ambiente e carga de medicao definidos pela techspec.
- RF-13: O sistema deve suportar apenas PT-BR no primeiro corte e tratar idioma incerto ou diferente de PT-BR como `TranscriptionUncertain`. **[EMENDADO 2026-08-05 — NAO EXEQUIVEL NESTA FASE]** ver nota abaixo.
- RF-14: O sistema deve classificar a transcricao como tecnicamente incerta quando houver texto vazio, erro STT, idioma nao PT-BR, truncamento, baixa confianca quando disponivel ou transcricao incoerente/ininteligivel. **[PARCIALMENTE EMENDADO 2026-08-05]** — os criterios de texto vazio, erro STT, truncamento e incoerencia permanecem integralmente exigiveis e implementados; os criterios de "idioma nao PT-BR" e "baixa confianca" nao sao exequiveis, ver nota abaixo.

> **Nota de emenda RF-13/RF-14 (2026-08-05, review com evidencia empirica).**
> Verificacao real contra o OpenRouter com audio PT-BR real demonstrou que **nenhum** dos 5 modelos
> STT do benchmark (`openai/whisper-large-v3`, `openai/gpt-4o-transcribe`,
> `openai/gpt-4o-mini-transcribe`, `mistralai/voxtral-mini-transcribe`, `deepgram/nova-3`) retorna os
> campos `language` ou `confidence` na resposta — todos transcrevem corretamente e devolvem
> `language=""`. Trocar de modelo nao resolve.
> Manter o gate original tornaria a feature 100% inoperante (todo audio classificado como
> `language_unsupported`). Decisao adotada: assumir o idioma enviado no request (`pt`) quando o
> provider omitir o campo. Em consequencia, **a deteccao de idioma e o piso de confianca nao sao
> controles ativos nesta fase** e nao podem ser declarados atendidos; a protecao contra transcricao
> ruim recai sobre os gates de texto vazio, incoerencia e truncamento, esses sim ativos e testados.
> Detalhamento tecnico e regressoes em `techspec.md`, secao de contrato STT.
- RF-15: O sistema nao deve acionar tool financeira nem `HandleInbound` quando a transcricao estiver tecnicamente incerta.
- RF-16: Quando a transcricao estiver tecnicamente incerta, o sistema deve responder ao usuario pedindo reenvio, sem afirmar que entendeu dados nao sustentados e sem abrir pending step de confirmacao.
- RF-17: Quando a transcricao estiver tecnicamente confiavel, mas faltar dado financeiro exigido, o sistema deve seguir o mesmo fluxo textual existente de pergunta, confirmacao ou suspensao.
- RF-18: Quando a transcricao passar os gates tecnicos, o sistema deve montar uma entrada textual canonica e encaminhar ao mesmo fluxo do agente textual.
- RF-19: A entrada canonica deve preservar o texto transcrito usado para decisao, sem resumir, corrigir semanticamente ou enriquecer dados financeiros ausentes.
- RF-20: Mutacoes financeiras por audio podem executar sem confirmacao adicional exclusiva por serem audio, desde que a entrada canonica passe os gates e respeite as confirmacoes ja existentes do fluxo textual.
- RF-21: O sistema deve reutilizar a idempotencia por WAMID para impedir processamento duplicado de audio.
- RF-22: O mesmo WAMID deve ter outcome terminal unico, inclusive quando download, validacao ou STT falhar; reenvio pelo usuario so deve ser processado quando chegar como nova mensagem WhatsApp.
- RF-23: Falhas de download, validacao, STT ou dispatch devem degradar com resposta segura e erro tipado, sem duplicar lancamentos em retries.
- RF-24: O audio original deve ser descartado apos a transcricao ou rejeicao, mantendo apenas hash, metadados tecnicos e transcricao quando aplicavel.
- RF-25: A transcricao aprovada deve ser auditavel e vinculada ao run/thread/mensagem inbound sem expor audio bruto.
- RF-26: Outcomes rejeitados, incertos ou falhos devem registrar status, duracao quando disponivel, modelo STT quando aplicavel, tamanho, formato, hash e erro tipado sem audio bruto.
- RF-27: Logs de nivel informativo nao devem conter audio bruto nem transcricao completa quando ela puder conter dado financeiro sensivel.
- RF-28: O sistema deve emitir metricas com labels de baixa cardinalidade para audio aceito, rejeitado, falha STT, incerteza tecnica, dispatch, latencia, tamanho, duracao e custo quando disponivel.
- RF-29: A techspec deve definir nomes, janelas e thresholds iniciais para falha STT, incerteza tecnica, latencia e custo; sem esses thresholds, a funcionalidade nao atende readiness operacional.
- RF-30: O golden set deve incluir casos pareados texto/audio para despesas, receitas, consultas, edicoes e fluxos de confirmacao ja suportados por texto.
- RF-31: A matriz minima de golden set deve conter ao menos 3 casos positivos por categoria de capacidade textual: despesas, receitas, consultas, edicoes e confirmacoes existentes.
- RF-32: O golden set deve incluir casos negativos de audio com ruido, fala cortada, idioma nao PT-BR, audio duplicado, formato invalido e timeout STT.
- RF-33: Ambiguidade financeira com transcricao tecnicamente confiavel deve ser validada contra o mesmo comportamento textual existente, nao contra `TranscriptionUncertain`.
- RF-34: O gate de aceite deve exigir score por grupo de intencao >= 0,90 e 0 falso-sucesso em comandos financeiros mutacionais.
- RF-35: O gate de aceite deve exigir 0 tool call e 0 chamada a `HandleInbound` quando a classificacao for `TranscriptionUncertain`.
- RF-36: Testes reais com OpenRouter devem ser executaveis por flag e credencial, sem substituir testes unitarios por mocks.
- RF-37: O PRD, techspec, tasks e implementacao devem seguir obrigatoriamente as skills `domain-modeling-production`, `design-patterns-mandatory`, `mastra` e `go-implementation`.
- RF-38: A techspec deve registrar explicitamente a decisao de pattern: `nao aplicar padrao` quando adapters finos e fluxo direto resolverem; se introduzir nova abstracao estrutural, deve executar o seletor deterministico de `design-patterns-mandatory`.
- RF-39: O sistema nao deve criar agente paralelo, workflow kernel especifico de audio, fallback chain de LLM, provider STT fora do OpenRouter ou tool financeira com regra de negocio duplicada.
- RF-40: O sistema deve manter os gates existentes de assinatura, timestamp, deduplicacao, principal autenticado e rate limit antes de qualquer download ou transcricao de audio.
- RF-41: O sistema deve registrar como evidencia de validacao o payload real de audio WhatsApp usado no discovery, com dados sensiveis mascarados.
- RF-42: O sistema deve impedir que o texto transcrito seja semanticamente corrigido por heuristica propria antes do agente; normalizacoes permitidas devem ser tecnicas e auditaveis.
- RF-43: O sistema deve manter as confirmacoes ja obrigatorias para fluxos sensiveis ou destrutivos existentes, mesmo quando a origem for audio aprovado.
- RF-44: O sistema deve expor um modo de teste que injete transcricoes simuladas para validar fluxo sem chamada real ao OpenRouter e um modo real-LLM/STT por flag para aceite.
- RF-45: O sistema deve falhar fechado se o modelo STT escolhido deixar de estar disponivel, exceder o budget financeiro definido na techspec, nao suportar formato confirmado ou nao atender PT-BR no benchmark.
- RF-46: O sistema deve produzir runbook operacional para triagem de falha de download, falha STT, incerteza tecnica alta, latencia alta, custo alto e regressao golden.

## Experiencia do Usuario

Fluxo principal:

1. Usuario envia audio em PT-BR pelo WhatsApp.
2. Sistema valida canal, autenticidade, usuario, rate limit e duplicidade.
3. Sistema baixa e transcreve o audio.
4. Se a transcricao for confiavel, o agente processa como se o usuario tivesse digitado a mensagem.
5. Usuario recebe resposta textual no WhatsApp.

Fluxo de incerteza:

1. Usuario envia audio com ruido, fala cortada, idioma diferente de PT-BR ou dados insuficientes.
2. Sistema nao executa tool financeira.
3. Usuario recebe resposta textual pedindo reenvio quando a transcricao for tecnicamente incerta.

Fluxo de ambiguidade financeira com transcricao confiavel:

1. Usuario envia audio PT-BR tecnicamente transcrito, mas com dado financeiro incompleto.
2. Sistema encaminha a entrada canonica ao mesmo fluxo textual existente.
3. Usuario recebe a mesma pergunta, confirmacao ou suspensao que receberia se tivesse digitado a mensagem.

Fluxo de rejeicao tecnica:

1. Usuario envia audio invalido, grande demais, expirado ou com formato nao suportado.
2. Sistema rejeita a mensagem de forma segura.
3. Usuario recebe resposta textual explicando que nao foi possivel processar aquele audio e orientando nova tentativa.

## Restricoes Tecnicas de Alto Nivel

- Canal inicial: WhatsApp Cloud API exclusivamente.
- Provider obrigatorio: OpenRouter para LLM e STT, sem fallback chain.
- Saida em audio/TTS: fora do primeiro corte.
- Idioma: PT-BR apenas no primeiro corte.
- Budget inicial: audio ate 60 segundos, timeout STT de 20 segundos e p95 alvo de transcricao de ate 8 segundos.
- Budget financeiro: custo maximo por audio e limite de tamanho em bytes devem ser fechados na techspec por benchmark antes da implementacao.
- Descoberta obrigatoria: payload real de audio WhatsApp deve ser capturado em ambiente controlado antes da implementacao.
- Privacidade: audio original deve ser descartado; persistencia deve se limitar a hash, metadados tecnicos e transcricao auditavel quando aplicavel.
- Arquitetura agentiva: comportamento novo deve consumir `internal/platform/{agent,llm,memory,workflow,tool,scorer}` e o consumidor `internal/agents`, sem reimplementar primitivos.
- Domain modeling: estados de transcricao e outcomes devem ser tipos fechados, nao strings livres em contratos publicos.
- Observabilidade: metricas nao podem usar labels de alta cardinalidade como usuario, telefone, WAMID, thread ou conteudo.
- Governanca inegociavel: `domain-modeling-production`, `design-patterns-mandatory`, `mastra` e `go-implementation` devem ser usadas nas proximas fases.
- Implementacao Go so pode iniciar apos techspec com `spec-hash-prd`, ADRs materiais e tasks rastreaveis por RF.

## Fora de Escopo

- Resposta em audio via TTS.
- Entrada de audio por canais diferentes de WhatsApp Cloud API.
- Envio de audio bruto para o modelo principal em `/chat/completions`.
- Criacao de um novo agente de voz separado do agente `mecontrola`.
- Mudanca nas regras financeiras existentes de despesas, receitas, consultas, edicoes, cartoes, categorias ou orcamentos.
- Retencao do audio original para auditoria longa.
- Suporte a idioma diferente de PT-BR.
- Transcricao offline/local ou provider STT diferente de OpenRouter.
- Redesenho do kernel `internal/platform/workflow`.
- Alteracao de prompts ou tools para aceitar dados inferidos que nao estejam na transcricao.
- Mudanca de politica de retencao para guardar audio original.
- Definicao de preco final ao usuario ou mudanca comercial de plano/entitlement.
- Suporte a comandos por audio que ainda nao existam por texto.
- Uso de sinais paralinguisticos, emocao, identidade vocal ou biometria de voz para decisao financeira.
- Reprocessamento automatico do mesmo WAMID apos falha terminal.

## Criterios de Sucesso

- 100% dos fluxos textuais existentes continuam verdes nos testes ja existentes.
- Golden set pareado texto/audio atinge score por grupo de intencao >= 0,90.
- 0 falso-sucesso em comandos financeiros mutacionais por audio.
- 0 tool call financeira quando a transcricao for `TranscriptionUncertain`.
- 0 duplicidade de processamento para mesmo WAMID.
- Audio original nao permanece armazenado apos processamento.
- p95 de transcricao <= 8 segundos no ambiente definido pela techspec.
- Timeout STT de 20 segundos respeitado e configuravel.
- Custo maximo por audio fechado na techspec antes da implementacao e respeitado nos gates.
- Observabilidade cobre volume, erro, incerteza, latencia e custo com labels de baixa cardinalidade.
- Techspec demonstra, com evidencia do codebase, que audio nao cria agente paralelo nem duplica regra financeira.
- Review final da entrega deve confrontar cada RF com evidencia real; reports autodeclarados nao substituem build/test/golden.
- Readiness pos-deploy exige monitoramento ativo de falha STT, incerteza tecnica, p95 e custo no periodo inicial definido pela techspec.

## Criterios de Aceite por Categoria

| Categoria | Aceite bloqueante |
|---|---|
| Produto | Usuario autenticado envia audio PT-BR e recebe resposta textual equivalente ao texto digitado. |
| Seguranca financeira | Nenhuma mutacao ocorre quando a transcricao e tecnicamente incerta. |
| Idempotencia | Mesmo WAMID nao gera segunda execucao nem segunda mutacao. |
| Privacidade | Audio original nao fica persistido apos processamento. |
| Operacao | Latencia, custo, falha, incerteza e dispatch sao observaveis com labels de baixa cardinalidade. |
| Testes | Golden pareado texto/audio passa com score >= 0,90 e 0 falso-sucesso. |
| Governanca | Techspec/tasks/implementacao citam e aplicam as quatro skills obrigatorias. |

## Gates de Nao-Falso-Positivo

- Nenhum RF pode ser marcado como atendido apenas por mock quando o aceite exigir comportamento com OpenRouter real.
- Nenhum RF pode ser marcado como atendido sem evidencia `path:line`, teste executado ou payload real mascarado quando aplicavel.
- Nenhum comportamento de audio pode ser declarado equivalente ao texto sem caso pareado no golden set.
- Nenhuma mutacao por audio pode ser considerada segura se houver qualquer tool call ou chamada a `HandleInbound` em `TranscriptionUncertain`.
- Nenhum contrato de payload WhatsApp pode ser assumido sem discovery tecnico com amostra real.
- Nenhuma metrica pode ser aprovada se usar labels de usuario, telefone, WAMID, thread, resource id ou conteudo.

## Suposicoes e Questoes em Aberto

Nao ha questoes de produto pendentes para iniciar a techspec. Restam gates tecnicos obrigatorios, que nao autorizam suposicao nem implementacao direta:

- O formato exato do payload de audio WhatsApp deve ser confirmado por discovery tecnico com amostra real; esta e uma atividade obrigatoria antes da implementacao.
- O limite de tamanho maximo em bytes deve ser definido na techspec apos confirmar formato real e custo do modelo STT escolhido.
- O modelo STT OpenRouter deve ser escolhido na techspec por benchmark; o PRD define criterios, nao o slug do modelo.
- O custo maximo por audio deve ser numericamente fechado na techspec apos benchmark do modelo STT.
- A techspec deve definir ambiente/carga de medicao do p95 de transcricao.
- A techspec deve definir janelas e thresholds do pos-deploy gate.

## Evidencias do Codebase Usadas no PRD

| Evidencia | Interpretacao de produto |
|---|---|
| `internal/platform/whatsapp/payload/types.go:29` | Payload atual modela mensagem com `Type` e `Text`, sem contrato de audio confirmado. |
| `internal/platform/whatsapp/payload/parser.go:19` | Parser atual extrai texto quando `msg.Text != nil`; audio precisa de extensao explicita. |
| `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:154` | Consumer atual exige `Text`; audio sem entrada canonica seria rejeitado. |
| `internal/onboarding/infrastructure/http/client/meta/client.go:96` | Client Meta atual envia texto; download de midia nao esta comprovado nesse client. |
| `internal/platform/llm/openrouter.go:22` | OpenRouter atual cobre chat completions e embeddings; STT precisa contrato novo. |
| `docs/diagrams/agent/03-agent-llm-openrouter.md:5` | OpenRouter e provider unico; PRD proibe fallback/provider paralelo. |
| `docs/diagrams/agent/02-fluxo-webhook-dispatcher.md:66` | Deduplicacao por WAMID ja existe no fluxo WhatsApp; audio deve reaproveitar. |

## Referencias

- Refinamento: `docs/refin/2026-08-04-refin-agente-audio-openrouter.md`
- Fluxo agentivo atual: `docs/diagrams/agent/03-agent-llm-openrouter.md`
- Fluxo WhatsApp atual: `docs/diagrams/agent/02-fluxo-webhook-dispatcher.md`
- OpenRouter Audio: https://openrouter.ai/docs/guides/overview/multimodal/audio
- OpenRouter STT: https://openrouter.ai/docs/guides/overview/multimodal/stt
- OpenRouter TTS: https://openrouter.ai/docs/guides/overview/multimodal/tts
