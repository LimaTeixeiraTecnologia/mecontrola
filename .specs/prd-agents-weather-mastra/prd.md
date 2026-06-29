# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->

# `internal/agents` — Port fiel do agente Weather (Mastra) sobre `internal/platform`, validado no WhatsApp

## Visão Geral

O `mecontrola` é um monolito modular em Go. A capacidade de comportamento agentivo foi promovida a substrato genérico e reutilizável em `internal/platform` (agent, memory, scorer, tool, workflow/kernel, llm) pela iniciativa `prd-platform-mastra`. Falta agora **um consumidor de referência real, vivo e validável em produção** que prove a plataforma end-to-end no canal oficial (WhatsApp).

Este PRD define o módulo **`internal/agents`**: um **port fiel (equivalência comportamental 1:1)** do exemplo **weather** do Mastra (`/Users/jailtonjunior/Git/limateixeira-agents/agents/src/mastra`) para Go, construído **sobre** `internal/platform` (consome, não reimplementa), **persistido em Postgres** (substituindo o duckdb/libsql do exemplo TS) e **exercitado por mensagens reais de WhatsApp**. O valor é duplo: (1) provar que a plataforma reutilizável atende a um agente completo (agent síncrono + streaming, tool com I/O tipado, workflow multi-step com agent-como-step, memória thread/working/longo-prazo, scorers code-based e LLM-judged, structured output, runtime context, run auditável) num fluxo real; (2) estabelecer o **padrão canônico de como um módulo de domínio consome a plataforma**, reaproveitável por futuros agentes.

Decisão de produto fechada: o módulo legado `internal/agent` (assistente financeiro: 24 intents, onboarding conversacional, orçamento, HITL) é **eliminado 100%**, e o **fluxo de onboarding conversacional do WhatsApp é desligado**. Apenas `internal/agents` (weather) responde no WhatsApp após esta entrega. A semântica financeira não faz parte deste PRD e poderá ser reconstruída sobre a mesma plataforma numa iniciativa futura.

A referência de comportamento é o exemplo weather do Mastra, tratado como **contrato de equivalência funcional**, não inspiração opcional. Adaptações ao Go e ao Postgres são esperadas e registradas como restrições; não suavizam a paridade comportamental. Equivalência é de **comportamento funcional**, não de API/árvore de pastas do TypeScript.

### Estado atual relevante (grounding)

- `internal/platform` já oferece os primitivos (agent/memory/scorer/tool/workflow/llm) e o schema `platform_*` (migration `000003`: `platform_threads`, `platform_messages`, `platform_resources`, `platform_runs`, `platform_embeddings` com pgvector(1536)/HNSW, `platform_scorer_results`; kernel `workflow_runs`/`workflow_steps`).
- Existe um **port Go parcial** do weather em `test/conformance/weather/` (tool, workflow, scorer, conformance) — base de produção a promover/reaproveitar; hoje mocka memória/recall e não está wired ao WhatsApp.
- **Gap conhecido (B3, da review `prd-platform-mastra`)**: a **indexação assíncrona de embeddings não está conectada** — `memory.AppendMessage` não publica evento de outbox e o `EmbeddingIndexHandler` não é registrado em worker → `platform_embeddings` nunca é populada e o semantic recall retorna vazio. Sem corrigir isso, a memória de longo prazo do weather-agent não funciona de verdade.
- `internal/agent` está **vivo e wired** em `cmd/server` (`server.go`, `whatsapp_wiring.go`) e `cmd/worker` (`worker.go`), além de bindings no módulo `internal/onboarding` e e2e que importam `internal/agent/application/services`. A migration `000003` já dropa as 7 tabelas `agent_*`.
- Entrada WhatsApp: handler HTTP → verificação de assinatura → dispatcher (dedup, principal, rate limit) → callback `agentRoute(ctx,msg)`; hoje publica evento outbox `agent.whatsapp.inbound.v1`, consumido no worker e respondido via `whatsAppGateway.SendTextMessage(toE164,text)`.

## Objetivos

- Entregar `internal/agents` como **port fiel do weather Mastra** em Go, consumindo `internal/platform` sem reimplementar mecanismo.
- Provar a plataforma **end-to-end no WhatsApp real**: uma mensagem de clima resulta em resposta de clima + sugestão de atividades, com persistência completa em Postgres.
- **Eliminar 100%** `internal/agent` e **desligar o onboarding conversacional** do WhatsApp, sem deixar referência, tabela de runtime ou wiring órfão.
- **Conectar a indexação assíncrona de embeddings** (outbox→worker, idempotente por `event_id`) para que o semantic recall funcione de fato.
- Estabelecer o **padrão de consumo da plataforma** por um módulo de domínio (layout, wiring, DI, testes), aderente a go-implementation (R0–R7) e DMMF (state-as-type, smart constructors, `Decide*` puro).
- Manter **build verde, gates de governança verdes, gofmt limpo** e cobertura de testes determinística no CI, com variante real atrás de `RUN_REAL_LLM`.

## Histórias de Usuário

Atores: **Usuário final WhatsApp** (envia mensagens de clima); **Engenheiro de plataforma** (valida e mantém o substrato); **Operador** (observabilidade/auditoria).

- Como **usuário WhatsApp**, quero perguntar o clima de uma cidade ("clima em São Paulo?" / "what's the weather in Tokyo?") e receber uma resposta concisa com condições atuais e, quando eu pedir, sugestões de atividades coerentes com a previsão.
- Como **usuário WhatsApp**, quero que o agente peça a localização quando eu não informar, e que ele entenda nomes de cidade em outro idioma traduzindo para o inglês quando necessário.
- Como **engenheiro de plataforma**, quero um consumidor real (não só teste) que exercite agent síncrono+streaming, tool com I/O tipado, workflow com agent-como-step, memória (thread/working/longo prazo), scorers e structured output, para comprovar a plataforma em produção.
- Como **operador**, quero que cada interação produza um **Run auditável** (status fechado, duração, erro), com tracing correlacionável e métricas de cardinalidade controlada, reutilizando a stack de O11Y existente.
- Como **engenheiro de plataforma**, quero `internal/agent` e o onboarding conversacional **removidos sem resíduo**, para eliminar a regressão de runtime (migration dropando tabelas de um módulo vivo) e reduzir superfície morta.

## Funcionalidades Core

1. **Módulo `internal/agents` (weather) consumindo a plataforma** — agent "weather-agent" com instructions, modelo via OpenRouter, tool `get-weather`, scorers e memória, montado por DI sobre `internal/platform`. Equivalência funcional ao `weather-agent` do Mastra.
2. **Tool `get-weather`** — I/O tipado (input `location`; output temperatura/sensação/umidade/vento/rajada/condições/local); integra open-meteo (geocoding + forecast); mapeia `weather_code`→condição. Equivalência ao `weather-tool`.
3. **Workflow `weather-workflow`** — `fetch-weather` (previsão) → `plan-activities` (usa `agent.stream` via runtime context para sugerir atividades formatadas). Equivalência ao `weather-workflow`.
4. **Scorers** — `tool-call-accuracy` (code-based, tool esperada `get-weather`), `completeness` (code-based), `translation` (LLM-judged com structured output validável), com sampling configurável e resultados persistidos. Equivalência ao `weather-scorer`.
5. **Memória persistida em Postgres** — thread/resource/working memory + **semantic recall** por embeddings (pgvector), com **indexação assíncrona conectada** (outbox→worker, idempotente).
6. **Canal WhatsApp end-to-end** — inbound (texto livre vira entrada do agente) → execução via `AgentRuntime` → resposta enviada ao usuário pelo gateway WhatsApp existente; Run auditável persistido.
7. **Eliminação do legado** — remoção total de `internal/agent` e desligamento do onboarding conversacional do WhatsApp, com cutover de `cmd/server`, `cmd/worker`, bindings e configuração.

## Requisitos Funcionais

### Módulo, layout e consumo da plataforma

- RF-01: O módulo DEVE viver em `internal/agents` e **consumir** `internal/platform` (agent, memory, scorer, tool, workflow, llm) sem reimplementar mecanismo da plataforma nem importar `internal/agent`.
- RF-02: O módulo DEVE expor um construtor de módulo (DI manual via construtor, sem `init()`, sem estado global) que monte agent + tool + workflow + scorers + memória + runtime + persistência e seja consumível por `cmd/server` e `cmd/worker`.
- RF-03: O código DEVE aderir às Regras Estritas de go-implementation (R0–R7), zero comentários em produção (R-ADAPTER-001.1), DTOs de input com `Validate()` (R-DTO-VALIDATE-001) e testes testify/suite whitebox (R-TESTING-001).
- RF-04: Estados e tipos de fronteira DEVEM ser **tipos fechados** (state-as-type, DMMF), com smart constructors validando invariantes; proibido `Result[T,E]` custom, currying, DSL de pipeline e monads.

### Agent (paridade weather-agent)

- RF-05: O agente "weather-agent" DEVE ser registrado e resolvido pela plataforma, com instructions equivalentes ao Mastra (assistente de clima: pede localização se ausente; traduz nomes não-ingleses; usa a tool para buscar clima; respostas concisas; sugere atividades quando solicitado).
- RF-06: O agente DEVE usar **OpenRouter** como canal LLM único, com o modelo configurável (default equivalente ao do Mastra), via `internal/platform/llm`.
- RF-07: O agente DEVE suportar execução **síncrona** e **streaming** sob o mesmo contrato lógico (a sugestão de atividades do workflow usa streaming).
- RF-08: O agente DEVE ter a tool `get-weather` vinculada e a memória (thread/working/semantic) vinculada via plataforma.

### Tool (paridade get-weather)

- RF-09: A tool `get-weather` DEVE declarar input (`location: string`) e output (`temperature, feelsLike, humidity, windSpeed, windGust, conditions, location`) tipados/validados, e ser consumível por agent e por steps de workflow.
- RF-10: A tool DEVE obter dados de **open-meteo** (geocoding por nome → forecast por lat/long) e mapear `weather_code` para descrição textual, com tratamento explícito de erro (cidade não encontrada, falha de upstream) — IO externo do consumidor, fora da plataforma.

### Workflow (paridade weather-workflow)

- RF-11: O workflow `weather-workflow` DEVE receber `{city}` e produzir `{activities}`, encadeando determinísticamente `fetch-weather` → `plan-activities` sobre o `Engine[S]` do kernel, com estado `S` tipado preservado entre steps.
- RF-12: O step `fetch-weather` DEVE produzir a previsão (`date, maxTemp, minTemp, precipitationChance, condition, location`) a partir de geocoding+forecast.
- RF-13: O step `plan-activities` DEVE invocar o **agente como step** via `agent.stream` (runtime context), gerando sugestões de atividades formatadas a partir da previsão.

### Scorers (paridade weather-scorer)

- RF-14: DEVE existir scorer **code-based** `tool-call-accuracy` (verifica que a tool `get-weather` foi chamada) e `completeness` (campos esperados presentes na resposta).
- RF-15: DEVE existir scorer **LLM-judged** `translation` com **structured output validável** (`nonEnglish, translated, confidence, explanation`) e modelo juiz via OpenRouter, falhando explicitamente quando o contrato não é satisfeito.
- RF-16: Os scorers DEVEM ser anexados ao agente com **sampling configurável** (default equivalente ao Mastra), executados **fora do caminho crítico** (assíncrono), com resultados persistidos em `platform_scorer_results` vinculados ao Run.

### Memória e persistência (Postgres único)

- RF-17: O módulo DEVE persistir thread, mensagens/turns e working memory em Postgres (`platform_threads`/`platform_messages`/`platform_resources`) por chaves opacas (`resourceId`/`threadId`), sem semântica de domínio.
- RF-18: O módulo DEVE oferecer **semantic recall** por embeddings (pgvector) e DEVE **conectar a indexação assíncrona** de embeddings: ao persistir mensagem, emitir evento via `internal/platform/outbox`; um consumer/worker gera o embedding (OpenRouter) e grava `platform_embeddings`, **fora do caminho crítico** e **idempotente por `event_id`**. (Resolve o gap B3.)
- RF-19: O semantic recall DEVE ser **demonstrável**: após interações, `platform_embeddings` é populada e o recall retorna itens relevantes escopados por `resourceId` (sem vazamento entre resources).

### Canal WhatsApp end-to-end

- RF-20: Uma mensagem de texto recebida no WhatsApp DEVE ser roteada para `internal/agents`, ter seu texto livre usado como entrada do agente/workflow, e a resposta enviada de volta ao usuário pelo gateway WhatsApp existente.
- RF-21: O fluxo inbound→processamento→outbound DEVE seguir o padrão atual de adapters finos (handler/consumer → usecase), reutilizando dedup, verificação de assinatura, principal e rate limit já existentes em `internal/platform/whatsapp`.
- RF-22: Cada interação DEVE produzir um **Run auditável** (status fechado `running|succeeded|failed`, duração, erro quando houver) persistido em `platform_runs`, com tracing correlacionável e métricas de cardinalidade controlada (sem `resource_id`/`thread_id`/`correlation_key` como label).

### Eliminação do legado e cutover

- RF-23: `internal/agent` DEVE ser **removido 100%** do repositório; nenhuma referência de produção, teste ou wiring pode permanecer (`grep` por `internal/agent` que não seja `internal/platform/agent` retorna vazio).
- RF-24: O **fluxo de onboarding conversacional do WhatsApp DEVE ser desligado** (rota de ativação / `activation_command`), conforme decisão; bindings e e2e do módulo `internal/onboarding` que dependem de `internal/agent` DEVEM ser ajustados ou removidos sem quebrar o build/CI.
- RF-25: `cmd/server` (`server.go`, `whatsapp_wiring.go`) e `cmd/worker` (`worker.go`) DEVEM ser religados para construir e expor `internal/agents` (rota WhatsApp, consumer no worker, jobs de housekeeping), sem resíduo de `internal/agent`.
- RF-26: A configuração DEVE migrar de `AGENT_*` para a configuração do novo módulo (model ids, OpenRouter, embed model/dims), com defaults explícitos e overridáveis; sem variáveis órfãs.
- RF-27: A migration `000003` (que já dropa as 7 tabelas `agent_*`) DEVE permanecer; após o cutover NÃO pode restar dependência de runtime das tabelas `agent_*`.

### Observabilidade, testes e qualidade

- RF-28: A observabilidade DEVE reutilizar a stack existente (sem trilha paralela); métricas com cardinalidade controlada (labels enums fechados: `agent_id`, `channel`, `workflow`, `status`, `tool`, `outcome`, `scorer_id`, `kind`).
- RF-29: O CI padrão DEVE rodar testes determinísticos (provider fake + Postgres testcontainers com pgvector + migrations) sem rede LLM; uma variante E2E real DEVE ficar atrás de flag de ambiente (`RUN_REAL_LLM`), fora do gate de merge.
- RF-30: A entrega DEVE manter **build verde**, **gates de governança verdes** (kernel sem domínio/LLM, zero comentários nas camadas novas, cardinalidade, tipos fechados) e **gofmt limpo** (gate `lint:fmt:check`).

## Experiência do Usuário

Usuário final (WhatsApp):

- Envia "clima em São Paulo?" → recebe resposta concisa com condições atuais (temperatura, sensação, umidade, vento) para a cidade.
- Envia "o que fazer hoje em Lisboa com esse tempo?" → recebe sugestões de atividades coerentes com a previsão, formatadas.
- Não informa cidade → o agente pede a localização.
- Informa cidade em outro idioma/escrita → o agente normaliza/traduz e responde corretamente.

Engenheiro de plataforma: monta o módulo por DI; observa Run auditável, scorers e recall populados; usa a suite determinística no CI e a variante real sob `RUN_REAL_LLM`.

## Restrições Técnicas de Alto Nível

- **Plataforma como mecanismo**: `internal/agents` consome `internal/platform`; é proibido reimplementar agent/memory/scorer/tool/workflow/llm no módulo de domínio, e proibido importar `internal/agent`.
- **Postgres é persistência única** (threads, mensagens, working memory, runs, embeddings pgvector, scorer results, snapshots de workflow). pgvector é extensão do próprio Postgres.
- **OpenRouter** é o canal LLM e de embeddings único e obrigatório.
- **WhatsApp (Meta)** é o canal de entrada/saída; reutilizar a infraestrutura existente (`internal/platform/whatsapp`, gateway de envio).
- **Governança**: go-implementation R0–R7 (sem `init()`, sem abstração de tempo, sem `panic` em produção, `context.Context` em IO, `errors.Join`/wrapping), zero comentários (R-ADAPTER-001.1), testify/suite whitebox (R-TESTING-001), DTOs com `Validate()` (R-DTO-VALIDATE-001), DMMF state-as-type. `AGENTS.md` é a governança canônica.
- **Open-meteo** é IO externo do consumidor (geocoding + forecast), fora da plataforma; tratar timeouts/erros explicitamente.
- **Eliminação destrutiva controlada**: remover `internal/agent` e o onboarding conversacional do WhatsApp é parte do escopo; deve ser feita com cutover verificável (build/CI verdes, sem referência órfã) — operação irreversível, exige diligência.
- **Limites operacionais** (timeouts de LLM/HTTP, retenção/housekeeping de runs, sampling de scorers, janela de memória) são configuração com defaults explícitos e overridáveis.

## Fora de Escopo

- Qualquer comportamento financeiro do `internal/agent` legado (registrar despesa/receita/cartão, orçamento, HITL de deleção/edição, resumos) — descontinuado e desconsiderado.
- Onboarding conversacional via WhatsApp — explicitamente desligado nesta entrega.
- Reescrita ou alteração do substrato `internal/platform` além do estritamente necessário para consumir e para **conectar a indexação assíncrona de embeddings** (RF-18) e quaisquer correções de bug bloqueantes do recall.
- Cópia literal da API/árvore de diretórios do Mastra TypeScript.
- DDL final, nomes definitivos de colunas e plano de rollout de migração — pertencem à techspec (a migration `000003` já existe e é mantida).
- Especificação técnica de baixo nível, ADRs, decomposição em tarefas e plano de execução — pertencem à techspec/tasks.
- Métricas de produto sobre conversas de clima (analytics) e qualquer UI além do WhatsApp.

## Suposições e Questões em Aberto

Suposições:

- O substrato `internal/platform` está funcional para consumo (conforme entregue em `prd-platform-mastra`), exceto o gap B3 (indexação assíncrona), que este PRD exige conectar.
- pgvector está disponível em produção e em testcontainers; OpenRouter possui credencial (`OPENROUTER_API_KEY`) para a variante real.
- O gateway de envio WhatsApp existente (Meta) é reutilizável pelo novo módulo para a resposta outbound.
- A remoção do onboarding conversacional do WhatsApp não exige preservar a jornada de ativação atual; a decisão de produto é desligá-la.

Questões em aberto (a fechar na techspec, não bloqueiam o PRD):

- Idioma das respostas no WhatsApp (PT-BR vs. seguir idioma do usuário) e textos exatos das instruções do agente.
- Modelo OpenRouter default e modelo de embedding/dimensionalidade final (alinhar com `platform_embeddings` vector(1536)).
- Se o `weather-workflow` é o caminho primário no WhatsApp (mensagem→workflow) ou se o agente direto atende e o workflow é exercitado em paralelo/sob demanda.
- Estratégia exata de desligamento do onboarding (remover módulo `internal/onboarding` vs. apenas desconectar a rota de ativação) e tratamento dos e2e dependentes.
- Sampling default de scorers em produção vs. conformidade.
