# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 3 -->

# Plataforma Reutilizável de Agentes, Memória e Workflows em `internal/platform` (paridade Mastra)

## Visão Geral

O `mecontrola` é um monolito modular em Go com bounded contexts em `internal/`. O módulo `internal/agent` **será descontinuado** e é desconsiderado totalmente por este PRD: nenhum requisito, métrica ou critério depende dele. Em seu lugar, **toda a capacidade de comportamento agentivo passa a viver em `internal/platform` como substrato genérico e reutilizável** — agentes, memória, workflows, threads, runs, working memory, structured output, streaming, suspend/resume e observabilidade. A semântica de domínio (transações, billing, identity e quaisquer regras de negócio) permanece exclusivamente nos módulos consumidores; a plataforma fornece apenas os mecanismos genéricos.

Este PRD define o produto interno **Plataforma de Agentes, Memória e Workflows**: um substrato compartilhado, genérico e production-ready em `internal/platform`, com **equivalência de comportamento funcional ao Mastra** como referência primária e inegociável. O valor é eliminar reimplementação acoplada por módulo, padronizar o ciclo de execução auditável e oferecer ao time uma base única, confiável e reutilizável para construir comportamento agentivo em qualquer bounded context.

A referência de comportamento é o Mastra, tratado como **contrato de equivalência funcional, não como inspiração opcional**: a documentação oficial (https://mastra.ai/docs, https://mastra.ai/docs/agents/overview, https://mastra.ai/docs/memory/overview, https://mastra.ai/docs/workflows/overview), o código (https://github.com/mastra-ai/mastra) e os pacotes `@mastra/core/agent`, `@mastra/core/tools` e `@mastra/core/workflows` são a base conceitual obrigatória dos requisitos de agents, tools e workflows. O repositório `/Users/jailtonjunior/Git/limateixeira-agents` é a referência comparativa local obrigatória e `/Users/jailtonjunior/Git/mecontrola` é o repositório alvo. Nenhum comportamento pode ser inventado fora dessas três fontes.

Distinção mandatória: **equivalência de comportamento funcional ≠ cópia da API ou da árvore de pastas do TypeScript**. Adaptações ao ecossistema Go são esperadas e registradas como restrições de implementação futura, sem suavizar a exigência de paridade. Este é um documento de produto: não contém especificação técnica de baixo nível, código, ADR, diff, pseudo-código, scaffolding nem plano de execução. SQL concreto, DDL e nomes finais de coluna pertencem à techspec — aqui o modelo de dados aparece apenas como capacidade e restrição de alto nível.

### Estado atual relevante (grounding)

- O kernel genérico de workflow já existe em `internal/platform/workflow` com `Engine[S any]`, `Snapshot`, `Step[S]` e `MergePatch` (resume por JSON merge-patch). Persistido em `workflow_runs` e `workflow_steps`. **Este kernel é aproveitado como base evolutiva da inspiração Mastra — preservado, não descartado.**
- **Não existem Scorers/Evals** hoje. O exemplo de referência (`limateixeira-agents`, weather) usa scorers code-based (`toolCallAccuracy`, `completeness`) e LLM-judged com structured output (`translationScorer`, judge via OpenRouter) com sampling.
- O **exemplo weather** do `limateixeira-agents` (`weather-agent`, `weather-tool`, `weather-workflow`, `weather-scorer`) é o consumidor de referência canônico: agent com instructions+model+tools+scorers+memory, tool com schema de I/O, workflow com step que usa `agent.stream()` (streaming + agent-como-step).
- O outbox transacional já existe em `internal/platform` (`outbox_events`).
- **Não existe streaming** hoje; LLM via OpenRouter é request/response. Structured output já é `json_schema` com `strict: true`.
- **Não existe pgvector/embeddings**; working memory atual é texto puro.
- As 7 tabelas `agent_*` (`agent_sessions`, `agent_decisions`, `agent_threads`, `agent_runs`, `agent_working_memory`, `agent_observations`, `agent_processed_events`) pertencem ao `internal/agent` descontinuado e serão removidas.
- Migrations são sequenciais golang-migrate (`000NNN_*.{up,down}.sql`), com par up/down simétrico.

## Objetivos

- Oferecer em `internal/platform` um substrato reutilizável que cubra, com paridade comportamental ao Mastra, o ciclo de vida de **agentes**, a execução de **workflows por steps** e a **memória** conversacional, de trabalho e de longo prazo.
- Tornar os primitivos **Thread, Run, WorkingMemory e PendingStep genéricos da plataforma** (chaves opacas `resourceId`/`threadId`/`correlationKey`), sem semântica de domínio.
- Garantir que `internal/platform` permaneça **livre de regra de negócio e semântica de domínio**, operando sobre estado genérico e chaves de correlação opacas.
- Entregar **structured output validável na fronteira** como capacidade obrigatória e não negociável, conciliada com **streaming**.
- Entregar **memória de longo prazo com recuperação semântica** (pgvector) sobre Postgres como persistência única.
- Tornar toda execução **auditável e observável** reutilizando a stack de observabilidade já existente, sem trilha paralela.
- Provar reuso real por meio de um **consumidor de referência** e de uma **suite de conformidade** da plataforma.

## Histórias de Usuário

Atores: **Time de engenharia da plataforma** (mantenedores do substrato); **Engenheiro de módulo consumidor** (constrói comportamento agentivo em um bounded context sobre a plataforma); **Operador** (observabilidade, auditoria e suporte em produção).

- Como **engenheiro consumidor**, quero registrar e resolver um agente pela plataforma para não reimplementar ciclo de vida, roteamento e contrato de execução no meu módulo.
- Como **engenheiro consumidor**, quero executar um agente de forma síncrona **ou** em streaming sob o mesmo contrato lógico, para atender respostas completas e incrementais.
- Como **engenheiro consumidor**, quero declarar um contrato de saída estruturada e recebê-lo validado na fronteira, para consumir dados confiáveis sem parsing frágil — inclusive quando a resposta é entregue em streaming.
- Como **engenheiro consumidor**, quero compor workflows com steps tipados, encadeamento determinístico e estado compartilhado, para que fluxos multi-step sejam previsíveis e testáveis.
- Como **engenheiro consumidor**, quero usar agentes e tools como steps **dentro** de workflows, para compor comportamento agentivo em fluxos maiores.
- Como **engenheiro consumidor**, quero suspender e retomar uma execução interrompida, para que esperas por input humano ou eventos externos não percam estado já computado.
- Como **engenheiro consumidor**, quero memória conversacional por thread e por resource, working memory estruturada e recuperação semântica de longo prazo, para que o agente mantenha contexto entre mensagens, canais e ao longo do tempo.
- Como **engenheiro consumidor**, quero injetar dependências e valores efêmeros de request (runtime context) acessíveis a steps, agents e tools, sem persistir esses valores no estado durável.
- Como **operador**, quero que toda execução produza um run auditável e tracing correlacionável, para investigar incidentes e comprovar o que foi executado.
- Como **time de plataforma**, quero impedir que `internal/platform` absorva regra de domínio, para manter a fronteira arquitetural íntegra à medida que novos consumidores adotam a base.

## Funcionalidades Core

1. **Primitivo Agent genérico (completo)** — registro/resolução, execução síncrona e streaming, instruções/system prompt, binding de tools, binding de memory, structured output e **hooks de ciclo de vida**. Equivalência funcional a `@mastra/core/agent`.
2. **Tools e contrato de execução** — invocação e resultado tipado, consumíveis por agents e por steps de workflow. Equivalência funcional a `@mastra/core/tools`.
3. **Workflows por steps** — steps tipados, encadeamento determinístico, estado compartilhado e composição (agents/tools como steps). Equivalência funcional a `@mastra/core/workflows`. Evolui o kernel existente.
4. **Structured output validável conciliado com streaming** — contrato de saída validado na fronteira; em streaming, validação aplicada na conclusão do stream, com falha explícita quando o contrato não é satisfeito.
5. **Memória (thread, resource, working e longo prazo)** — histórico por thread, working memory estruturada por resource e **recuperação semântica de longo prazo via pgvector**, com sumarização/compressão. Equivalência funcional à camada memory do Mastra.
6. **Persistência de execução e suspend/resume** — estado durável em Postgres com retomada idempotente a partir do ponto suspenso (resume por merge-patch sobre o snapshot).
7. **Runtime context (DI)** — contexto tipado de injeção de dependências e valores efêmeros, acessível a steps/agents/tools, **não persistido** no estado durável. Equivalência funcional ao runtimeContext do Mastra.
8. **Auditoria de runs e observabilidade** — todo run auditável e observável reutilizando a stack de O11Y existente, com tracing correlacionável fim a fim e cardinalidade controlada.
9. **Scorers/Evals** — avaliação de runs do agente com scorers **code-based** (determinísticos, ex.: acurácia de tool-call, completude) e **LLM-judged** (com structured output via OpenRouter), com **sampling** configurável e resultados persistidos em Postgres. Equivalência funcional a `@mastra/core/evals`.
10. **Storage genérico inspirado no Mastra** — modelo de dados de threads, mensagens, resources/working memory, runs, snapshots, vetores e resultados de scorer, com chaves opacas e sem semântica de domínio.

## Requisitos Funcionais

### Primitivo Agent e ciclo de vida

- RF-01: A plataforma DEVE permitir registrar agentes e resolvê-los por identificador estável, com ciclo de vida explícito (criação, configuração, execução, encerramento), funcionalmente equivalente a `@mastra/core/agent`.
- RF-02: A plataforma DEVE expor **hooks de ciclo de vida** do agente (no mínimo pré/pós execução e em torno de invocação de tool), suficientes para o consumidor observar e estender o fluxo sem alterar o mecanismo.
- RF-03: A plataforma DEVE expor execução **síncrona** de um agente, retornando o resultado completo ao chamador.
- RF-04: A plataforma DEVE expor execução em **streaming** de um agente, entregando resultado incremental sob o mesmo contrato lógico da execução síncrona.
- RF-05: A plataforma DEVE comunicar-se com modelos LLM exclusivamente através do **OpenRouter** como canal oficial.

### Structured output × streaming

- RF-06: A plataforma DEVE permitir declarar um contrato de **saída estruturada** para uma execução e DEVE validar a conformidade do resultado na fronteira antes de devolvê-lo.
- RF-07: Em execução com streaming, a validação do contrato de structured output DEVE ocorrer **na conclusão do stream**; tokens incrementais são entregues durante o stream e o resultado estruturado validado é disponibilizado ao final.
- RF-08: Quando a saída não satisfizer o contrato declarado (síncrono ou ao final do stream), a plataforma DEVE falhar de forma **explícita e auditável**, nunca devolver resultado não conforme silenciosamente.

### Tools

- RF-09: A plataforma DEVE oferecer um contrato de **tool** (invocação e resultado tipado) funcionalmente equivalente a `@mastra/core/tools`.
- RF-10: A plataforma DEVE permitir que tools sejam consumidas tanto por agents quanto por steps de workflow.

### Workflows

- RF-11: A plataforma DEVE executar **workflows por steps tipados** com encadeamento determinístico, funcionalmente equivalente a `@mastra/core/workflows`, evoluindo o `Engine[S any]` existente.
- RF-12: A plataforma DEVE manter **estado compartilhado de workflow** (`S`) acessível e atualizável entre steps de uma mesma execução.
- RF-13: A plataforma DEVE permitir o uso de **agents e tools como steps componíveis** dentro de workflows.
- RF-14: A plataforma DEVE oferecer combinadores de fluxo (sequência, ramificação, paralelismo) preservando o estado `S` ao longo dos steps.

### Persistência, suspensão e retomada

- RF-15: A plataforma DEVE persistir o estado de execução em **Postgres** (estado, threads, runs, mensagens, memória, working memory, snapshots, checkpoints, vetores e demais artefatos), sendo o Postgres a persistência oficial e única.
- RF-16: A plataforma DEVE suportar **suspend** de uma execução, gravando estado durável suficiente para retomada antes de devolver o controle ao chamador.
- RF-17: A plataforma DEVE suportar **resume** de uma execução interrompida a partir do ponto suspenso, aplicando o payload de resume como **delta merge-patch sobre o snapshot** (preservando o estado rico já computado), nunca substituindo o estado inteiro.
- RF-18: A retomada DEVE ser **idempotente** quanto a sinais repetidos (replay): o mesmo sinal já processado não produz efeito duplicado.
- RF-19: A plataforma DEVE garantir que runs concluídos ou expirados sejam **purgados/encerrados** deterministicamente (housekeeping), sem run preso indefinidamente em estado suspenso.

### Memória (thread, resource, working, longo prazo)

- RF-20: A plataforma DEVE oferecer **Thread genérico** identificado por chaves opacas (`resourceId`, `threadId`), sem semântica de domínio, funcionalmente equivalente ao thread do Mastra.
- RF-21: A plataforma DEVE persistir **mensagens/turns** por thread, recuperáveis como histórico conversacional com limite de janela configurável.
- RF-22: A plataforma DEVE oferecer **working memory estruturada** escopada por resource, recuperável e atualizável, disponibilizável ao contexto de execução do agente.
- RF-23: A plataforma DEVE oferecer **memória de longo prazo com recuperação semântica** via armazenamento vetorial (pgvector) sobre Postgres, com geração de embeddings via OpenRouter.
- RF-24: A plataforma DEVE oferecer **sumarização/compressão** de histórico para conversas que excedam a janela imediata, preservando recuperabilidade.
- RF-25: O **compartilhamento de contexto** de memória DEVE ser controlado e explícito por `resourceId`/`threadId`, sem vazamento entre resources/threads não relacionados.

### Runtime context (DI)

- RF-26: A plataforma DEVE oferecer um **runtime context tipado** (injeção de dependências e valores efêmeros de request) acessível a steps, agents e tools durante uma execução.
- RF-27: O runtime context **NÃO DEVE** ser persistido no estado durável (snapshot); apenas o estado `S` e artefatos declarados são duráveis.

### Auditoria e observabilidade

- RF-28: Toda execução DEVE ser registrada como um **Run auditável** contendo, no mínimo, identificação de correlação, status fechado (`running`/`suspended`/`succeeded`/`failed`), duração e erro quando houver.
- RF-29: A plataforma DEVE emitir **tracing correlacionável** fim a fim, reutilizando obrigatoriamente a stack de observabilidade já existente no projeto, sem introduzir trilha paralela.
- RF-30: Métricas DEVEM ter **cardinalidade controlada**: labels restritos a enums fechados (ex.: `workflow`, `step`, `status`, `outcome`, `agent_id`, `channel`), proibido `resource_id`/`correlation_key`/identificadores de alta cardinalidade como label.

### Fronteira de domínio e reuso

- RF-31: `internal/platform` NÃO DEVE conter regra de negócio nem semântica de domínio; opera sobre **estado genérico** e **chaves de correlação opacas**.
- RF-32: A plataforma DEVE ser **consumível por múltiplos módulos** simultaneamente, permitindo composição e reuso sem acoplamento a um bounded context.
- RF-33: Estados expostos pela plataforma (`RunStatus`, `StepStatus`, `SuspendReason`, outcome, awaiting kinds) DEVEM ser **tipos fechados**, nunca strings livres na fronteira.
- RF-34: O **kernel de workflow** (`internal/platform/workflow`) DEVE permanecer livre de LLM; a capacidade LLM (OpenRouter) reside no primitivo Agent da plataforma, em pacote distinto do kernel, preservando a pureza do kernel.

### Migrations e modelo de dados (capacidade/restrição de alto nível)

- RF-35: A plataforma DEVE descontinuar e **remover as 7 tabelas `agent_*`** (`agent_sessions`, `agent_decisions`, `agent_threads`, `agent_runs`, `agent_working_memory`, `agent_observations`, `agent_processed_events`) via nova migration sequencial, com `down` simétrico.
- RF-36: A plataforma DEVE introduzir um **modelo de storage genérico inspirado no Mastra**, cobrindo no mínimo: threads (por `resourceId`/`threadId`), mensagens/turns, resources + working memory, runs auditáveis, snapshots de execução (evoluindo `workflow_runs`/`workflow_steps`), armazenamento vetorial para recuperação semântica e **resultados de scorer**.
- RF-37: O modelo de dados da plataforma DEVE usar **chaves opacas** e NÃO DEVE conter colunas com semântica de domínio (ex.: `intent_kind`); a deduplicação/idempotência DEVE ser genérica (por chave opaca), não específica de domínio.
- RF-38: A migration DEVE **habilitar a extensão vetorial** (pgvector) no Postgres e prover a estrutura para embeddings e índice de similaridade, mantendo o Postgres como persistência única.
- RF-39: As migrations DEVEM seguir o padrão sequencial golang-migrate existente (`000NNN_*.{up,down}.sql`) com par up/down reversível.

### Scorers / Evals

- RF-40: A plataforma DEVE oferecer um primitivo **Scorer** genérico com duas modalidades: **code-based** (determinístico, ex.: acurácia de tool-call, completude) e **LLM-judged** (avaliação por modelo via OpenRouter), funcionalmente equivalente a `@mastra/core/evals`.
- RF-41: Scorers DEVEM ser anexáveis a um agente com **sampling configurável** (ex.: razão/ratio), sem alterar o caminho de execução principal do agente.
- RF-42: O scorer **LLM-judged** DEVE produzir **structured output validável** (contrato declarado) na avaliação, falhando explicitamente quando o contrato não é satisfeito.
- RF-43: Os **resultados de scorer** (score, razão, metadados) DEVEM ser persistidos em Postgres, vinculados ao Run avaliado, com chaves opacas e sem semântica de domínio.

### Consumidor de referência e conformidade

- RF-44: A plataforma DEVE ser validada por um **consumidor de referência** — o exemplo weather (`weather-agent`, `weather-tool`, `weather-workflow`, `weather-scorer`) portado para Go — exercitando end-to-end todas as capacidades nucleares (agent síncrono+streaming, tool com I/O estruturado, workflow com agent-como-step, memória thread/working/longo prazo, scorers, structured output, suspend/resume, runtime context).
- RF-45: A **suite de conformidade E2E** DEVE exercitar **OpenRouter real** e **Postgres real**, executável atrás de **flag de ambiente** (padrão do projeto, ex.: `RUN_REAL_LLM`); o CI padrão DEVE rodar testes unitários/integração determinísticos sem dependência de rede LLM no gate de merge.
- RF-46: Os testes de **persistência** DEVEM rodar contra **Postgres real** (testcontainers/compose) com as migrations aplicadas, **incluindo a extensão pgvector**, validando persistência, suspend/resume e recuperação semântica de verdade.

## Experiência do Usuário

Aplicável apenas a desenvolvedores (produto interno, sem UI de usuário final). Experiência-alvo do engenheiro consumidor:

- Resolver um agente registrado e executá-lo (síncrono ou streaming) com poucas chamadas, declarando opcionalmente um contrato de structured output.
- Compor um workflow declarando steps tipados, com estado compartilhado e agents/tools como steps.
- Suspender e retomar execuções sem gerenciar manualmente persistência de estado.
- Acessar memória (thread, working, longo prazo) e injetar dependências via runtime context sem reimplementar persistência.
- Obter automaticamente run auditável e tracing sem instrumentar manualmente cada caminho.

Princípio transversal: o consumidor expressa **semântica de domínio no seu módulo**; a plataforma expõe apenas mecanismo genérico. A fronteira deve ser óbvia e difícil de violar acidentalmente.

## Restrições Técnicas de Alto Nível

- Postgres é a persistência **oficial e única** (estado, threads, mensagens, runs, memória, working memory, snapshots, checkpoints, vetores). pgvector é extensão do próprio Postgres — não introduz segundo armazenamento.
- OpenRouter é o **canal oficial e obrigatório** para LLM e para geração de embeddings.
- Structured output validável na fronteira é capacidade **obrigatória e não negociável**, conciliada com streaming via validação na conclusão do stream.
- A observabilidade DEVE reutilizar a stack de O11Y já existente; é proibido criar trilha paralela. Métricas com cardinalidade controlada.
- `internal/platform` é proibido de absorver regra de negócio ou semântica de domínio; estados na fronteira DEVEM ser tipos fechados.
- O **kernel de workflow** (`internal/platform/workflow`) é **aproveitado como base evolutiva** (inspiração Mastra) e permanece livre de LLM e de domínio; os primitivos Agent/Memory/Scorer (com LLM via OpenRouter) são pacotes distintos dentro de `internal/platform` que **consomem** o kernel, nunca o contrário (layering preservado).
- **Estratégia de testes (obrigatória):** suite de conformidade E2E exercita OpenRouter real e Postgres real atrás de flag de ambiente (`RUN_REAL_LLM` ou equivalente); CI padrão roda unit/integração determinísticos sem rede LLM. Testes de persistência rodam contra Postgres real (testcontainers/compose) com migrations aplicadas, incluindo pgvector. O consumidor de referência é o exemplo weather portado para Go.
- **Impacto de governança (já tratado):** as regras `R-WF-KERNEL-001` e `R-AGENT-WF-001` foram **alteradas em 2026-06-29** para permitir Thread/Run/WorkingMemory/PendingStep como primitivos genéricos em `internal/platform` (revogando a exclusividade ao `internal/agent`, descontinuado), preservando a pureza do kernel `internal/platform/workflow`. A pré-condição de governança está resolvida; a techspec deve reemitir os gates de verificação apontando para os arquivos genéricos da plataforma.
- Limites operacionais (TTL, retry/backoff, circuit breaker, request timeout, concorrência, retenção/housekeeping) são **configuração de plataforma com defaults explícitos e overridáveis** pelo consumidor.
- Paridade comportamental com o Mastra é a referência principal; adaptações ao Go são restrições de implementação futura e não justificam desvio conceitual. Equivalência é de **comportamento funcional**, não de API ou árvore de pastas do TypeScript.

Critérios objetivos de "production-ready" (não-claims vagos): limites declarados com defaults (concorrência, tamanho de payload/estado, janela de memória, TTL de suspensão, política de retry); invariantes explícitas (run nunca preso suspenso além do TTL; estado de domínio nunca vaza para a plataforma; runtime context nunca persistido); falhas esperadas e recuperação documentadas (timeout de LLM, falha de persistência, contrato de output não satisfeito, falha de embedding); idempotência de retomada e de sinais; auditabilidade completa de cada run; operação coberta pela observabilidade existente.

## Métricas de Sucesso

- **Reuso comprovado**: o **consumidor de referência weather** (portado para Go) exercita todas as capacidades nucleares end-to-end (agent síncrono+streaming, structured output, tools, workflow multi-step com agent-como-step, suspend/resume, memória thread/working/longo prazo, scorers, runtime context) com sucesso.
- **Conformidade de plataforma**: **suite de conformidade** verde cobrindo structured output (síncrono e fim-de-stream), suspend/resume idempotente, memória e recuperação semântica (pgvector), scorers (code-based e LLM-judged), auditoria de runs e observabilidade — com variante E2E contra OpenRouter real e Postgres real atrás de flag.
- **Confiabilidade de execução**: taxa de sucesso de suspend/resume e de retomada após interrupção em meta declarada; zero run preso suspenso além do TTL.
- **Idempotência**: zero efeito duplicado comprovado para sinais/replays repetidos em verificação.
- **Auditoria**: 100% das execuções com Run auditável completo (correlação, status, duração, erro quando houver).
- **Observabilidade**: 100% dos caminhos de execução cobertos pela stack de O11Y existente, sem trilha paralela; métricas com cardinalidade controlada.
- **Structured output**: 100% das saídas estruturadas validadas na fronteira; falha explícita quando o contrato não é satisfeito.
- **Migração de schema**: remoção das 7 tabelas `agent_*` e criação do storage genérico aplicadas com `up`/`down` reversíveis e sem resíduo de semântica de domínio na plataforma.

## Fora de Escopo

- Qualquer dependência, requisito, métrica ou compatibilidade com o módulo `internal/agent` (descontinuado e desconsiderado totalmente).
- Regra de negócio ou semântica de domínio de qualquer bounded context dentro de `internal/platform`.
- Reemissão dos gates de verificação das regras de governança apontando para os arquivos genéricos finais da plataforma (as regras já foram alteradas; os caminhos concretos pertencem à techspec).
- Criação de uma trilha de observabilidade paralela à existente.
- Cópia literal da API pública ou da árvore de diretórios do Mastra em TypeScript.
- DDL final, nomes definitivos de tabelas/colunas, índices e estratégia de rollout de migração — pertencem à techspec.
- Especificação técnica de baixo nível, ADRs, decomposição em tarefas, plano de execução, diffs ou decisões de implementação.

## Suposições e Questões em Aberto

Suposições:

- Existe stack de observabilidade única já adotada no projeto que a plataforma reutilizará sem trilha paralela.
- O kernel genérico em `internal/platform/workflow` (`Engine[S any]`, `Snapshot`, `MergePatch`) é base evolutiva da plataforma, preservando suas invariantes (sem import de domínio, estados como tipos fechados, kernel sem LLM).
- pgvector pode ser habilitado no Postgres do projeto (extensão disponível no ambiente de produção).
- As regras de governança `R-WF-KERNEL-001` e `R-AGENT-WF-001` já foram alteradas (2026-06-29) para permitir Thread/Run/WorkingMemory/PendingStep como primitivos genéricos da plataforma; a techspec apenas reemitirá os gates de verificação para os caminhos finais.
- O padrão de testes do projeto suporta flag de execução de LLM real (ex.: `RUN_REAL_LLM`) e integração contra Postgres real (testcontainers/compose); pgvector disponível nesse ambiente de teste.

Questões em aberto: **nenhuma pendente de produto.** Todas as decisões de fronteira de escopo foram fechadas:

- Streaming + structured output: ambos no MVP; contrato validado na **conclusão do stream** (RF-06, RF-07).
- Memória de longo prazo: **inclusa** com sumarização + recuperação semântica via pgvector (RF-23, RF-24, RF-38).
- Primitivo Agent: escopo **completo** (lifecycle + tools + memory binding + hooks) (RF-01, RF-02).
- Thread/Run/WorkingMemory/PendingStep: **promovidos a primitivos genéricos** da plataforma (RF-20..RF-22, RF-31..RF-33).
- Migrations: **dropar** as 7 tabelas `agent_*` e **criar** storage genérico inspirado no Mastra + pgvector (RF-35..RF-39).
- Limites operacionais: **config de plataforma com defaults overridáveis** (Restrições Técnicas).
- Runtime context: **adicionado** (S durável + runtime context tipado não persistido) (RF-26, RF-27).
- Production-ready: **consumidor de referência + suite de conformidade** (Métricas de Sucesso).
- Scorers/Evals: **inclusos** como primitivo completo (code-based + LLM-judged com structured output + sampling + resultados em Postgres) (RF-40..RF-43).
- Consumidor de referência: **exemplo weather portado para Go** (agent+tool+workflow+scorer) (RF-44).
- Testes: **OpenRouter real + Postgres real atrás de flag**, CI padrão determinístico; integração contra Postgres real com pgvector (RF-45, RF-46).

Governança já tratada: as regras `R-WF-KERNEL-001` e `R-AGENT-WF-001` foram **alteradas** (2026-06-29) para permitir Thread/Run/WorkingMemory/PendingStep como primitivos genéricos em `internal/platform`, preservando a pureza do kernel `internal/platform/workflow` (aproveitado como base evolutiva). A pré-condição de governança deixa de ser bloqueio.

Itens remanescentes são de **engenharia/techspec**, não de produto (não bloqueiam o PRD): formato exato de validação parcial vs. fim-de-stream no transporte; modelo de embeddings e dimensionalidade do vetor; nomes/índices finais das tabelas (incl. tabela de resultados de scorer); valores numéricos default de cada limite operacional; e o desenho dos scorers code-based portados.
