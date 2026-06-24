# Documento de Requisitos do Produto (PRD) — Refatoração Canônica do `internal/agent` e do Canal WhatsApp Oficial

<!-- spec-version: 2 -->

> Fonte oficial da verdade do produto: `docs/oficial/2026_06_24_mecontrola_oficial.md` (v1.0).
> Este PRD **consolida e supera** os PRDs anteriores `prd-onboarding-v2`, `prd-agent-platform-evolution`,
> `prd-workflow-kernel` e `prd-activation-ux`: passa a ser a referência única da arquitetura-alvo do
> `internal/agent`, preservando o que segue válido e marcando explicitamente o que é removido/substituído.

## Visão Geral

O MeControla é um agente financeiro conversacional no WhatsApp que interpreta linguagem natural
(`Mercado 120 pix`, `Recebi salário 4000`, `Quanto ainda posso gastar?`, `Apaga aquele Uber`) e
executa a ação correspondente, com tom simples, motivador e foco em **realização de objetivos**.

Hoje a plataforma já tem base sólida: webhook oficial da Meta Cloud API (verificação, assinatura
HMAC-SHA256, deduplicação por `wamid`, Graph API v18 para envio), parser LLM na fronteira via
OpenRouter, roteamento `IntentWorkflow → Tool → binding → usecase`, kernel genérico de workflow
(suspend/resume durável), Thread/Run auditável e estados como tipos fechados. O `internal/agent`
**já consome outros módulos apenas por porta de entrada** (binding→usecase), sem SQL direto nem
import de repositório de outro bounded context.

Porém o sistema acumulou complexidade e código que não serve mais ao produto-alvo: um segundo canal
(Telegram) fora de escopo, caminhos *legacy* coexistindo com o kernel, fallback determinístico
inalcançável, eventos órfãos e configuração de modelo única (sem aproveitar modelos melhores por
tipo de tarefa). O contrato entre LLM e os módulos de domínio também não está formalizado como
**Structured Output tipado**, o que abre espaço para alucinação e mapeamento frágil.

Este PRD define uma **refatoração completa e production-ready** com quatro eixos:

1. **Canal único WhatsApp Oficial da Meta** — eliminar Telegram por inteiro (código, wiring, config,
   colunas/CHECK de banco), deixando o pipeline `WhatsApp (Meta Cloud API) → API MeControla →
   internal/agent → OpenRouter` como o único caminho.
2. **Interação com LLM via Structured Outputs** — o LLM produz **saída estruturada e tipada** (schema
   estrito) que mapeia de forma determinística para as **portas de entrada** dos módulos
   (`internal/transactions`, `internal/onboarding`, `internal/categories`, `internal/card`,
   `internal/budgets`), eliminando texto livre como contrato e reduzindo alucinação/fluxos indesejados.
3. **Roteamento de modelo por classe de tarefa** — selecionar, no OpenRouter, modelos com bom score
   por **classe** (parse de intenção, onboarding, resposta conversacional), cada classe com primário
   + fallback + circuit breaker.
4. **Eliminação do que não faz sentido** — remover *legacy*, código morto, eventos órfãos e tabelas/
   colunas que não serão mais usadas, com fronteira de dados blindada por gate verificável.

## Objetivos

- **Canal único e robusto**: 100% do tráfego conversacional via WhatsApp Oficial da Meta, sem
  qualquer caminho Telegram remanescente em código, config, env ou schema.
- **Contrato LLM↔módulos à prova de alucinação**: toda ação de domínio nasce de um **Structured
  Output tipado** validado contra schema, mapeado 1:1 para a porta de entrada do módulo dono — sem
  o agente inventar regra de negócio.
- **Fronteira de dados inviolável**: `internal/agent` lê/escreve **apenas** tabelas próprias; todo
  acesso a outro bounded context passa pela porta de entrada apropriada (usecase, http handler,
  producer, consumer ou job), com gate de CI bloqueando regressão.
- **Eficiência de modelo**: usar o melhor modelo por classe de tarefa, reduzindo custo/latência sem
  perder qualidade de parse; fallback e circuit breaker garantem disponibilidade.
- **Cobertura funcional do Documento Oficial (escopo MVP)**: onboarding (8 etapas), operação diária
  (receita, despesa, cartão, parcelamento, consultas, alteração/exclusão com confirmação),
  recorrência de orçamento, edição de valores de categoria pós-onboarding, casos especiais e matriz
  de decisão — todos atendidos sem lacuna. **Alertas (80/90/100%) e objetivo no resumo diário ficam
  fora deste MVP** (PRDs dedicados — ver Fora de Escopo).
- **Base limpa e production-ready**: zero código morto, zero evento órfão, zero tabela órfã, suíte
  verde, gates `R-*` passando, observabilidade com cardinalidade controlada.

### Métricas-chave a acompanhar

- Referências a Telegram em código/config/env/schema após a refatoração: **0**.
- Acessos do `internal/agent` a tabela de outro bounded context (SQL direto ou import de repo): **0**
  (gate de CI).
- % de ações de domínio originadas de Structured Output validado contra schema (vs. texto livre):
  **100%**.
- Operações destrutivas/sensíveis efetivadas sem confirmação humana explícita: **0**.
- Caminhos *legacy* coexistindo com o kernel após a refatoração: **0** (flag/branch/teste de paridade
  removidos).
- Eventos producer-sem-consumer e consumer-sem-producer após a limpeza: **0** (confirmados por string
  literal).
- Cobertura dos fluxos do Documento Oficial por cenário de aceite: **100%**.
- Diferença de comportamento observável nos fluxos válidos antes vs. depois (não regressão de UX):
  **0** nos cenários cobertos.
- Cardinalidade de métrica: **sem** `user_id`/`category_id`/`correlation_key`/`message_id` como label.

## Histórias de Usuário

- Como **usuário do MeControla**, quero mandar `Mercado 120 pix` e receber a confirmação do registro
  com categoria, valor e disponível, sem precisar aprender comandos.
- Como **usuário**, quero que, quando faltar informação (`Mercado 120`), o assistente pergunte
  **apenas o que falta** (`Como foi o pagamento?`) sem reiniciar o fluxo.
- Como **usuário**, quero que ao pedir `Apaga o Uber` o assistente **localize, exiba e peça
  confirmação** antes de remover, para eu não perder dados por engano.
- Como **usuário novo**, quero ser conduzido pelo onboarding (objetivo → orçamento → cartões →
  categorias → valores → resumo → conclusão) de forma natural e sem loops.
- Como **usuário**, quero registrar movimentações **mesmo sem orçamento configurado**, sem ser
  bloqueado.
- Como **engenheiro do `internal/agent`**, quero que o LLM devolva uma **estrutura tipada** que eu
  mapeio direto para a porta de entrada do módulo dono, sem inventar regra nem tocar banco de outro
  contexto.
- Como **operador/SRE**, quero observar parse, roteamento de modelo, gates de confirmação e chamadas
  às portas de entrada com cardinalidade controlada, para diagnosticar sem explodir métricas.
- Como **mantenedor**, quero uma base sem Telegram, sem *legacy* e sem código morto, para reduzir
  superfície de manutenção e risco.
- Como **revisor**, quero que a fronteira "agent só acessa porta de entrada de outro módulo" seja
  garantida por gate automatizado, não por convenção.

## Funcionalidades Core

### A. Canal único — WhatsApp Oficial da Meta

- **O que faz**: consolida o pipeline `WhatsApp (Meta Cloud API) → API MeControla → internal/agent →
  OpenRouter` como único caminho conversacional; remove integralmente o canal Telegram.
- **Por que importa**: foca o produto, reduz superfície de ataque/manutenção e elimina ramos de
  código condicionais por canal.
- **Como funciona em alto nível**: o webhook oficial da Meta (verificação, assinatura HMAC-SHA256,
  deduplicação por `wamid`, janela de timestamp) recebe a mensagem; a identidade do usuário é
  resolvida por E164; a mensagem entra no `internal/agent`; a resposta sai pela Graph API. Telegram
  some de código, wiring, config, env e schema (migration ALTER reversível).

### B. Interação com LLM via Structured Outputs (contrato anti-alucinação)

- **O que faz**: o LLM, no único ponto de parse, produz uma **saída estruturada e tipada** (schema
  estrito) representando a intenção, os slots e a confiança; o agente valida e mapeia essa estrutura
  para a **porta de entrada** do módulo dono.
- **Por que importa**: texto livre como contrato gera alucinação e fluxos indesejados; estrutura
  tipada + validação torna o comportamento previsível e o mapeamento determinístico, atendendo aos
  guardrails do Documento Oficial.
- **Como funciona em alto nível**: o parse extrai uma intenção tipada (e, quando aplicável, um plano
  determinístico de 1..N ações) com slots normalizados; campos ausentes disparam clarificação
  (uma pergunta por vez); a execução é 100% determinística via `Workflow → Tool → binding → usecase`,
  **sem** LLM no meio; regra de negócio vive nos módulos donos, nunca no agente.

### C. Roteamento de modelo por classe de tarefa

- **O que faz**: seleciona o modelo do OpenRouter por **classe de tarefa** — (1) parse de intenção,
  (2) onboarding, (3) resposta conversacional/fallback — cada classe com primário + fallback +
  circuit breaker.
- **Por que importa**: classes diferentes têm requisitos diferentes de custo, latência e qualidade;
  usar o modelo com melhor score por classe melhora eficiência sem sacrificar robustez.
- **Como funciona em alto nível**: configuração declarativa por classe (modelo primário e lista de
  fallback); o runtime resolve a chain pela classe da tarefa; métricas por classe/modelo/outcome com
  cardinalidade controlada.

### D. Fronteira de dados blindada e portas de entrada canônicas

- **O que faz**: garante que o `internal/agent` acesse **somente** suas próprias tabelas e consuma
  outros módulos **apenas** pela porta de entrada mais adequada (usecase, http handler, producer,
  consumer ou job).
- **Por que importa**: preserva os limites de bounded context, evita acoplamento de banco e mantém o
  agente como orquestrador fino sem regra de negócio.
- **Como funciona em alto nível**: cada operação do MeControla é roteada à porta de entrada do módulo
  dono (transactions, card, budgets, categories, onboarding); bindings finos traduzem a intenção
  tipada para o DTO/command do módulo; um gate automatizado bloqueia SQL direto e import de
  repositório/infra de outro contexto.

### E. Eliminação e limpeza (production-ready)

- **O que faz**: remove código *legacy*, código morto, eventos órfãos e tabelas/colunas que não
  serão mais usadas.
- **Por que importa**: "0 gaps, 0 lacunas, 0 falso positivo" exige base enxuta e verificável.
- **Como funciona em alto nível**: o kernel passa a ser o caminho único (remover flag/branch/teste de
  paridade *legacy*); fallback determinístico inalcançável é removido; eventos sem par são removidos
  após confirmação por string literal; migrations ALTER tratam colunas/constraints de Telegram.

## Requisitos Funcionais

### A. Canal único WhatsApp Oficial da Meta

- RF-01: O sistema DEVE manter o **WhatsApp Oficial da Meta como único canal conversacional**:
  ingress por webhook Meta (verificação `hub.challenge`, assinatura `X-Hub-Signature-256`,
  deduplicação por `wamid`, janela de timestamp) e egress pela Graph API.
- RF-02: O sistema DEVE **eliminar integralmente o canal Telegram**: árvore `internal/platform/telegram`,
  consumer inbound do agent, wiring de servidor, ativação/serviços de onboarding Telegram, adapter de
  notificação Telegram e roteamento de webhook Telegram.
- RF-03: O sistema DEVE remover toda **configuração, variáveis de ambiente e defaults** de Telegram
  (`TELEGRAM_*`, validações de produção, `ONBOARDING_TELEGRAM_*`).
- RF-04: O sistema DEVE remover a representação de Telegram nos **tipos fechados e value objects de
  canal** (Channel VO, `SourceTelegram`, resolução de canal preferido), mantendo apenas WhatsApp.
- RF-05: O sistema DEVE tratar o **schema de banco** removendo coluna/índice/CHECK específicos de
  Telegram (`telegram_external_id`, índice associado, `channel IN ('whatsapp','telegram')` →
  `'whatsapp'`) via **nova migration ALTER** — sem editar baseline já aplicada e com rota de rollback.
- RF-06: A resolução de identidade DEVE continuar derivando `user_id` a partir do **E164** do WhatsApp;
  usuário não encontrado DEVE rotear para o fluxo de onboarding/ativação (sem erro ao usuário).

### B. Interação com LLM via Structured Outputs

- RF-07: Toda extração de intenção DEVE produzir um **Structured Output tipado** com **provider
  `Strict=true`** (JSON schema estrito no OpenRouter) — intenção, slots normalizados e confiança — no
  único ponto de parse. `Strict=true` aplica-se a **todas as classes que produzem dados
  estruturados** (parse de intenção/plano e captura estruturada do onboarding). A resposta
  conversacional de `KindUnknown` é texto livre por natureza (não alimenta porta de módulo nem decisão
  de fluxo) e, portanto, não é alvo de validação de schema; se envelopada, usa schema mínimo.
- RF-08: A saída estruturada inválida (schema não satisfeito) NÃO DEVE virar ação de domínio: DEVE
  resultar em clarificação ou fallback conversacional determinístico, nunca em execução adivinhada.
- RF-09: O agente DEVE **mapear a saída estruturada 1:1 para a porta de entrada** do módulo dono, sem
  reinterpretar, recalcular ou inventar regra de negócio (regra vive no módulo dono).
- RF-10: A execução das ações DEVE ser **determinística** — proibido invocar LLM, prompt rendering ou
  fallback chain durante a execução (LLM apenas no parse; exceções sancionadas: resposta
  conversacional de `KindUnknown` e onboarding).
- RF-11: Quando faltar informação para completar uma ação (valor, meio de pagamento, categoria,
  cartão), o sistema DEVE **perguntar apenas o que falta, uma pergunta por vez**, sem reiniciar o
  fluxo, persistindo o estado de espera de forma durável (pending step).
- RF-12: O sistema DEVE suportar **plano determinístico de 1..N ações** a partir de uma única
  mensagem (ex.: registrar + consultar), com short-circuit em falha dura e agregação determinística
  da resposta; plano de 1 ação é o comportamento padrão.
- RF-13: Intenções, slots, outcomes e estados de espera DEVEM ser **tipos fechados** (state-as-type);
  proibido `string` livre em assinatura pública.

### C. Roteamento de modelo por classe de tarefa

- RF-14: O sistema DEVE permitir configurar o modelo do OpenRouter por **classe de tarefa** — parse
  de intenção, onboarding e resposta conversacional/fallback — cada classe com **modelo primário e
  lista de fallback**.
- RF-15: Cada classe DEVE ter **fallback chain + circuit breaker** independentes, garantindo
  disponibilidade quando o primário falhar ou estiver indisponível.
- RF-16: A seleção de modelo por classe DEVE ser **observável** (métrica por classe/modelo/outcome)
  com cardinalidade controlada; proibido expor `user_id`/`message_id` como label.
- RF-17: O parse DEVE manter a **política de confiança**: ações de escrita abaixo do limiar mínimo de
  confiança DEVEM ser bloqueadas/clarificadas (preserva a política vigente de confidence).
- RF-18: Como `Strict=true` é exigido nas classes estruturadas, os **modelos elegíveis** para parse e
  onboarding DEVEM ser **apenas os que suportam structured outputs estritos de forma confiável**,
  comprovado por **guard de teste com LLM real** antes de configurar como primário/fallback. Modelos
  que quebram com schema estrito (ex.: variantes que falham tool-calling/json_schema) são inelegíveis.
- RF-19: O **modelo de onboarding DEVE ser revalidado** sob `Strict=true`: o modelo atualmente usado
  (haiku) só permanece se passar no guard real-LLM com schema estrito; caso contrário DEVE ser
  substituído por um modelo elegível.

### D. Fronteira de dados e portas de entrada canônicas

- RF-20: O `internal/agent` DEVE acessar **apenas suas próprias tabelas**; é **proibido** SQL direto a
  tabela de outro bounded context e import de repositório/infra de outro contexto. Um **gate de CI**
  DEVE falhar o build em caso de violação.
- RF-21: Todo consumo de outro módulo DEVE ocorrer pela **porta de entrada mais adequada**: usecase,
  http handler, producer, consumer ou job do módulo dono — escolhida conforme a natureza da operação
  (síncrona de leitura/escrita vs. assíncrona por evento).
- RF-22: As **tools do MeControla DEVEM residir em `internal/agent`** e permanecer **adapters finos**:
  mapeiam a intenção tipada para o DTO/command do módulo, invocam o binding e formatam a resposta —
  sem regra de negócio, sem branching de domínio.
- RF-23: O mapeamento operação → porta de entrada DEVE cobrir, no mínimo: registrar despesa, registrar
  receita, compra de cartão (à vista e parcelada), consultar resumo/orçamento, editar/apagar
  lançamento, cadastrar/listar/editar/excluir cartão, configurar orçamento, definir valores de
  categorias no onboarding, **editar valor/percentual de categoria pós-onboarding** e **recorrência de
  orçamento (repetir planejamento por N meses)** — cada um pela porta do módulo dono (`transactions`,
  `card`, `budgets`, `categories`, `onboarding`).
- RF-24: O agente NÃO DEVE replicar regra de negócio dos módulos (cálculo de competência/fatura,
  parcelamento, percentuais de categoria): essas regras permanecem nos módulos donos e são apenas
  **acionadas** via porta de entrada.

### E. Cobertura funcional do Documento Oficial (escopo MVP)

- RF-25: O **onboarding** DEVE conduzir o usuário pelas etapas oficiais (boas-vindas → objetivo →
  orçamento → cartões → apresentação de categorias → valores das categorias → resumo final →
  conclusão) sem loops, persistindo progresso e acionando os módulos donos via porta de entrada. A
  captura do objetivo permanece como hoje (em `onboarding_sessions`); sua promoção a `budgets` e
  exibição no resumo diário ficam **fora deste MVP** (PRD futuro — ver Fora de Escopo).
- RF-26: O sistema DEVE permitir **registrar movimentações mesmo sem orçamento configurado**
  (continuidade sem orçamento), criando internamente a estrutura necessária via porta de entrada, sem
  bloquear o usuário.
- RF-27: **Compras parceladas** DEVEM ser criadas com parcelas e competências futuras pela porta de
  entrada do módulo dono; o usuário não gerencia parcelas manualmente.
- RF-28: O sistema DEVE responder às **consultas** oficiais (resumo do mês, "como estou", orçamento
  detalhado por categoria, cartões) compondo dados via portas de entrada de `transactions`,
  `budgets`, `card` e `categories`. O resumo do MVP **não** inclui o objetivo (deferido).
- RF-29: O sistema DEVE suportar **recorrência de orçamento** ("repetir meu orçamento por N meses")
  via a porta de entrada do módulo `budgets` (limite de meses conforme regra do módulo dono); o agente
  apenas aciona, sem recalcular.
- RF-30: O sistema DEVE suportar **edição de valor/percentual de categoria pós-onboarding** via a
  porta de entrada do módulo `budgets`; o agente não recalcula percentuais (regra no módulo dono).
- RF-31: Operações de **alteração e exclusão** DEVEM seguir o fluxo oficial **Localizar → Exibir →
  Confirmar → Executar → Confirmar sucesso**, com **confirmação humana explícita** (HITL) para
  deletar/editar lançamento (último **ou localizado por referência** — ver RF-36/RF-37), deletar
  cartão e reconfigurar orçamento.
- RF-32: O gate HITL DEVE **suspender** a execução com estado durável (reusando suspend/resume do
  kernel), retomar de forma **idempotente** após restart, e **limpar** o estado após
  efetivação/cancelamento/expiração — sem efeito em cancelamento/expiração e sem draft órfão.
- RF-33: O sistema DEVE tratar os **casos especiais** da matriz de decisão oficial: falta valor,
  falta pagamento, falta categoria, cartão não encontrado (oferecer cadastro), múltiplos resultados
  (solicitar escolha), categoria ambígua, e falha de integração (informar erro com clareza).
- RF-34: Para o pedido de **desfazer ("Desfaz isso")** — fora do MVP — o agente DEVE **redirecionar**
  para o fluxo suportado de apagar/editar o último lançamento (com confirmação HITL), mantendo a
  continuidade da conversa; nunca executar reversão automática.
- RF-35: As **respostas** DEVEM seguir o tom de voz, os emojis oficiais e a hierarquia visual do
  Documento Oficial (mensagens curtas, claras, motivadoras, com quebras de linha).
- RF-36: O sistema DEVE suportar **editar/apagar lançamento por referência** (ex.: "Apaga o Uber",
  "O Uber foi 42 e não 35"): **localizar** o lançamento por descrição via porta de **leitura** do
  `transactions` (sem SQL no agent), **exibir** o item encontrado e seguir o fluxo HITL antes de
  efetivar.
- RF-37: Em **múltiplos resultados** ("Apaga o mercado" → vários), o sistema DEVE apresentar a lista
  e pedir **escolha** (pending step de desambiguação com **tipo fechado** de `AwaitingKind`) antes da
  confirmação HITL; nenhuma efetivação ocorre sem item único selecionado e confirmado.
- RF-38: O gate HITL DEVE seguir o **contrato ADR-003 as-is**: confirmação explícita (sim/confirmar)
  executa; cancelamento explícito (não/cancelar) descarta sem efeito; resposta ambígua **re-pergunta
  uma vez** e, na segunda ambiguidade, cancela sem efeito; **TTL expirado** cancela sem efeito (texto
  segue para parse); **replay de `messageID`/`wamid`** já processado não duplica a mutação. Tudo
  durável via suspend/resume do kernel.

### F. Eliminação, limpeza e governança (transversal)

- RF-39: O **kernel** DEVE ser o caminho único de execução: remover flag/branch/método e teste de
  paridade *legacy* (`kernelEnabled`, `EnableKernel`, `continuePendingExpenseConfirmationLegacy`,
  `parity_test`), migrando previamente qualquer caso ainda atendido pelo *legacy*.
- RF-40: O **fallback determinístico inalcançável** de configuração de orçamento (branch morto) DEVE
  ser removido após confirmação de inacessibilidade.
- RF-41: **Eventos órfãos DEVEM ser removidos neste PRD, inclusive cross-module** (agent +
  transactions + budgets): candidatos confirmados — `agent.intent.rejected`,
  `budgets.budget_activated`, `transactions.recurring_template.{created,updated,deleted}` e
  `transactions.card_purchase.deleted`. A remoção DEVE ser precedida de **guarda anti-falso-positivo**:
  confirmar ausência de par pela **constante de event-type** (não apenas pelo nome do arquivo
  `producer/consumer`), pois eventos são publicados via constante da entidade — qualquer evento com
  par (ex.: `onboarding.splits_calculated`, consumido por `budgets`) é **mantido**. Toda remoção
  cross-module respeita as regras do módulo dono.
- RF-42: **Tabelas e colunas** que não serão mais usadas DEVEM ser removidas via migration reversível;
  nenhuma tabela do agent atualmente em uso (sessions, decisions, working_memory, observations,
  threads, runs) DEVE ser removida sem evidência de não-uso.
- RF-43: Toda alteração de Go DEVE passar nos gates `R-ADAPTER-001`, `R-AGENT-WF-001`,
  `R-WF-KERNEL-001`, `R-TESTING-001`, `R-DTO-VALIDATE-001` e `R-TXN-WORKFLOWS-001` (quando tocar
  transactions), além dos checklists R0–R7 da skill `go-implementation`.
- RF-44: Toda implementação Go em produção DEVE ter **zero comentários** (R-ADAPTER-001.1), sem
  `init()` (R0), sem `panic` em produção (R5.12), `context.Context` em toda fronteira de IO (R6) e
  agregação/wrapping de erro com `errors.Join`/`fmt.Errorf %w` (R7).
- RF-45: Toda métrica nova DEVE ter **cardinalidade controlada** (labels apenas de enums fechados);
  proibido `user_id`/`category_id`/`correlation_key`/`message_id` como label.

## Experiência do Usuário

- **Linguagem natural**: o usuário conversa normalmente; o sistema interpreta e age, perguntando
  apenas o que falta, uma pergunta por vez, sem reiniciar o fluxo.
- **Onboarding guiado**: jornada fluida das boas-vindas à conclusão, com confirmações curtas e
  resumo final claro; sem loops nem perguntas repetidas.
- **Operação diária**: registros de receita/despesa/cartão/parcelamento e consultas (resumo do mês,
  "como estou", orçamento por categoria, cartões) respondidos com tom motivador, emojis oficiais e
  hierarquia visual; continuidade garantida mesmo sem orçamento. Recorrência de orçamento e edição de
  valores de categoria também são suportadas.
- **Ações sensíveis**: alteração/exclusão e reconfiguração de orçamento sempre passam por
  Localizar → Exibir → Confirmar → Executar → Confirmar sucesso; cancelar/expirar não produz efeito.
  Pedido de "desfazer" é redirecionado para apagar/editar com confirmação.
- **Casos especiais**: cartão não cadastrado oferece cadastro; múltiplos resultados pedem escolha;
  categoria ambígua oferece as 5 categorias oficiais; falha de integração informa erro com clareza.
- **Canal**: experiência exclusiva no WhatsApp Oficial da Meta, com respostas determinísticas e
  alinhadas ao runbook conversacional.

## Restrições Técnicas de Alto Nível

- **Stack**: Go (versão conforme `go.mod`); toda implementação segue a skill `go-implementation`
  (Etapas 1–5 + checklist R0–R7), obrigatória e inegociável.
- **Localização**: capacidades vivem em `internal/agent`; o kernel genérico em
  `internal/platform/workflow` é **consumido**, não estendido com domínio. O webhook/ingress/egress
  WhatsApp permanece em `internal/platform/whatsapp` e na porta de entrada de onboarding/identity.
- **Contrato LLM**: Structured Outputs com provider `Strict=true` como contrato único das classes
  estruturadas (parse de intenção/plano e captura do onboarding); LLM exclusivo do parse (mais
  exceções sancionadas: conversacional de `KindUnknown` e onboarding). Determinismo na execução é
  não-negociável (anti-alucinação por design). **Consequência:** modelos elegíveis limitam-se aos que
  suportam structured outputs estritos de forma confiável (validados por guard real-LLM); haiku/
  gpt-5-nano são inelegíveis enquanto quebrarem com strict, e o modelo de onboarding será revalidado/
  substituído.
- **Fronteira de dados (hard)**: `internal/agent` acessa apenas tabelas próprias; consumo de outro
  módulo só por porta de entrada; gate de CI bloqueia SQL direto e import de repo/infra de outro BC.
- **Provedor de modelo**: OpenRouter; roteamento por classe de tarefa com primário + fallback +
  circuit breaker; chaves/limites/timeout configuráveis por ambiente.
- **Canal**: WhatsApp Oficial da Meta (Graph API; assinatura HMAC-SHA256; dedup por `wamid`);
  remoção total de Telegram inclusive em schema (migration ALTER reversível, sem editar baseline).
- **DMMF / state-as-type**: estados/outcomes/intenções/slots como tipos fechados; proibidos
  `Result[T,E]` customizado, currying, DSL de pipeline e monads (anti-padrões hard).
- **Durabilidade/idempotência**: gates HITL e pending steps reusam suspend/resume durável do kernel +
  idempotência por `event_id`/`wamid` + lock otimista; sem goroutine leak; shutdown cooperativo.
- **Privacidade/segurança**: dados e memória são escopo por usuário; recuperação não cruza dados
  entre usuários; sanitização/redação do audit trail preservada; cartões nunca solicitam limite,
  banco, bandeira ou dados sensíveis (apelido + dia de vencimento apenas).
- **Observabilidade**: stack `otel-lgtm`; cada execução é auditável como Run; métricas com
  cardinalidade controlada nos três sinais.
- **Governança (gate)**: aderência integral às regras hard vigentes; qualquer nova regra/addendum é
  gate na techspec/ADR **antes** do código.

## Fora de Escopo

- **Alertas de orçamento (⚠️ 80% / ⚠️ 90% / 🚨 100%)**: explicitamente **fora deste MVP** e tratados
  em **PRD dedicado** (mandatório). Motivo de produto: a entrega proativa no WhatsApp depende da
  janela de 24h da Meta e de **templates aprovados** para mensagens fora dessa janela — decisão e
  custo operacional próprios, que merecem PRD separado.
- **Objetivo/meta na operação diária**: a promoção do objetivo para o módulo `budgets` e sua
  exibição no resumo diário ficam **fora deste MVP** (PRD futuro). A captura do objetivo no
  onboarding permanece como hoje.
- **Desfazer automático ("undo")**: reversão automática da última ação fica fora do MVP; o agente
  redireciona para apagar/editar com confirmação (RF-34).
- **Canal Telegram** (e qualquer outro canal além do WhatsApp Oficial da Meta): explicitamente
  removido; não há suporte multicanal neste produto-alvo.
- **Loop de agente autônomo dirigido por LLM** (plan→act→observe→iterate com tool-calling iterativo):
  excluído; execução permanece determinística com LLM apenas no parse (anti-alucinação).
- **RAG semântico/vetorial** (pgvector, embeddings, store de vetor, busca semântica de base de
  conhecimento): fora de escopo; recuperação contextual, quando houver, é query estruturada no
  Postgres existente.
- **Reescrita do kernel de workflow** ou novos primitivos de control-flow (loops `foreach`/`dountil`,
  `map`, workflows aninhados, streaming por step, scorers, `.sleep()`/`.waitForEvent()`): fora deste
  escopo (já listados como futuros em `prd-workflow-kernel`).
- **Novas categorias** além das 5 oficiais (Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade
  Financeira): proibido pelo Documento Oficial.
- **Funcionalidades bancárias/contábeis/investimento/ERP**: o MeControla não é nenhuma dessas coisas.
- **Detalhes de desenho** (assinaturas, schema de colunas, nomes de modelos por classe, parâmetros de
  retenção/timeout, ADRs, plano de migração passo-a-passo): pertencem à Especificação Técnica.

## Decisões Resolvidas

Resolvidas com o solicitante (rodada de múltipla escolha + diretriz textual):

- **Canal**: eliminar Telegram 100% (código, wiring, config, env, schema via migration ALTER);
  WhatsApp Oficial da Meta como canal único.
- **Roteamento de modelo**: por **classe de tarefa** (parse / onboarding / conversacional), cada
  classe com primário + fallback + circuit breaker.
- **Structured Outputs com `Strict=true` em todas as classes estruturadas**: contrato tipado
  validado pelo provider; **consequência aceita** — modelos que quebram com strict (haiku/gpt-5-nano)
  ficam inelegíveis e o modelo de onboarding será revalidado/substituído (guard real-LLM).
- **Paradigma/foco**: **interação com LLM via Structured Outputs** — saída tipada e validada pronta
  para conversar com as portas de entrada de `internal/transactions`, `internal/onboarding`,
  `internal/categories`, `internal/card`, `internal/budgets`; execução determinística, sem loop LLM.
- **Escopo MVP incluído**: plano determinístico multi-tool (1..N); recorrência de orçamento e edição
  de valor/percentual de categoria pós-onboarding (portas já existentes em `budgets`); **editar/apagar
  por referência com desambiguação de múltiplos resultados** (fiel ao Documento Oficial, Cap 11);
  contrato HITL **ADR-003 as-is** (re-prompt único, TTL, replay sem duplicar).
- **Eventos órfãos**: removidos **neste PRD, inclusive cross-module** (agent + transactions +
  budgets), com **guarda anti-falso-positivo** por constante de event-type (mantém o que tiver par,
  ex.: `onboarding.splits_calculated`).
- **Portas de entrada (verificado, não suposto)**: todas as operações do MVP mapeiam para usecases
  existentes (`CreateTransaction`, `UpdateTransaction`, `DeleteTransaction`, `CreateCardPurchase`,
  `GetMonthlySummary`, `ListTransactions`, `CreateRecurrence`, `EditCategoryPercentage`,
  `CreateBudget`/`ActivateBudget`, `CreateCard`/`ListCards`/`UpdateCard`/`SoftDeleteCard`/`CountCards`).
- **Escopo MVP excluído (deferido a PRDs próprios)**: alertas 80/90/100% (mandatório, PRD dedicado),
  objetivo na operação diária (objetivo-em-budgets + resumo), desfazer automático.
- **Relação com PRDs anteriores**: este PRD **consolida e supera** `prd-onboarding-v2`,
  `prd-agent-platform-evolution`, `prd-workflow-kernel` e `prd-activation-ux`, tornando-se a fonte
  única da arquitetura-alvo do `internal/agent`.
- **Fronteira de dados**: já satisfeita no código atual (sem SQL direto a outro BC, sem import de repo
  de outro contexto); este PRD a **blinda com gate de CI** e a torna requisito hard.

## Suposições e Questões em Aberto

**Nenhuma questão de produto em aberto.** Todas as ambiguidades materiais foram resolvidas com o
solicitante em 4 rodadas de múltipla escolha (canal, modelo, paradigma, relação com PRDs, objetivo,
alertas, strict mode, escopo MVP, undo, editar/apagar por referência, eventos órfãos cross-module,
contrato HITL). As suposições anteriores foram **verificadas e convertidas em fatos**:

- **Portas de entrada (VERIFICADO)**: 100% das operações do MVP têm usecase existente (lista na seção
  "Decisões Resolvidas"). Não é mais suposição. Lacuna pontual de porta (se surgir na techspec) vira
  sub-tarefa **no módulo dono**, **nunca** SQL direto no agent.
- **Eventos órfãos (VERIFICADO)**: confirmados por contagem producer×consumer e por constante de
  event-type; os reais (agent/transactions/budgets) entram na limpeza; falsos positivos
  (`onboarding.splits_calculated`, consumido por budgets) são mantidos. Guarda anti-falso-positivo é
  requisito (RF-41).
- **Interpretação registrada (sem divergência prática)**: `Strict=true` aplica-se às saídas
  estruturadas (parse/plano/onboarding); a resposta conversacional de `KindUnknown` é texto livre e
  não é validada por schema (não há estrutura a validar).

**Restam apenas parâmetros da Especificação Técnica** (não são questões de produto): nomes/score
específicos de modelo elegível por classe, schema exato do Structured Output, limites de passos do
plano, valores de TTL/expiração e re-prompt do gate HITL (semântica já fixada por ADR-003), limite de
meses da recorrência (regra do `budgets`) e o plano de migração ALTER do Telegram (ordem, rollback).
