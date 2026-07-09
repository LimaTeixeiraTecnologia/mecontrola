# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->
<!-- source-us: docs/us/2026-07-08-us-registro-conversacional-robustez.md -->

# Registro Conversacional de Transações Confiável, Fluido e em BRL

## Visão Geral

O registro de receitas e despesas por conversa no WhatsApp do MeControla está falhando de forma
silenciosa. No diagnóstico de produção do usuário `f56e1142` (thread `74d83407`, 2026-07-08
14:45–14:51 BRT), **nenhum lançamento tentado foi persistido**: as tabelas `mecontrola.transactions`
e `mecontrola.agents_write_ledger` ficaram vazias e as duas runs `failed/usecaseError` (`c999f675`,
`bb82fb13`) têm a coluna `error` em branco e zero linhas de log. Além da falha silenciosa, a mesma
sessão exibiu confirmação dupla, falso positivo de "múltiplos lançamentos" causado por número BRL
formatado, ausência de retry, formatação BRL inconsistente e forma de pagamento assumida sem
perguntar.

Este PRD define os requisitos de produto para tornar o registro conversacional **confiável**
(nunca engole erro; toda falha é rastreável), **fluido** (uma única confirmação por lançamento;
recuperação de falha transitória sem recomeçar) e **canônico em BRL** (valores sempre formatados
por um único helper). O alvo é o usuário do WhatsApp que registra dinheiro em linguagem natural e o
operador que precisa diagnosticar falhas por métricas, traces e logs.

O escopo cobre cada gap comprovado do diagnóstico (regras R1–R8) sem introduzir novas raízes de
categoria, reclassificação retroativa ou registro múltiplo por mensagem.

## Objetivos

Métrica de sucesso primária (rastreável em produção):

- **Zero falhas silenciosas de escrita**: nenhum lançamento tentado termina sem persistência e sem
  rastro atribuível.
- **100% de rastreabilidade de erro**: todo run de escrita que falha tem `platform_runs.error`
  preenchido, um span de erro pesquisável e um log em nível ERROR, todos correlacionados por
  `thread_id`, `run_id` e `wamid`.
- **100% de confirmação única**: todo lançamento pede confirmação exatamente uma vez (nunca duas);
  o LLM nunca emite pergunta de confirmação própria.

Objetivos de suporte:

- Salário mensal resolve para a folha income correta sem loop de clarify.
- Receita nunca é gravada em categoria de despesa (e vice-versa).
- Número BRL formatado com separador de milhar não é confundido com múltiplos lançamentos.
- Falha transitória de escrita é recuperada sem o usuário recomeçar o lançamento.
- Todo valor monetário exibido segue o formato BRL canônico a partir de uma fonte única.
- Despesa sem forma de pagamento sempre pergunta; nunca assume "dinheiro".

## Histórias de Usuário

- Como usuário do WhatsApp, quero registrar receitas e despesas por conversa de forma confiável,
  fluida e com valores sempre em BRL, para controlar meu dinheiro sem falhas silenciosas, sem
  confirmações repetidas e sem categoria errada.
- Como usuário, quero que "Recebi meu salário de R$ 13.874,40" seja registrado na categoria correta
  de salário sem ter que responder perguntas de categoria.
- Como usuário, quero confirmar cada lançamento uma única vez, para não repetir "sim" várias vezes.
- Como usuário, quero informar a forma de pagamento quando o sistema não sabe, em vez de o sistema
  inventar "dinheiro".
- Como operador, quero que toda falha de escrita apareça em `platform_runs.error`, em um span de
  erro pesquisável e em log ERROR, para diagnosticar incidentes sem depender de tabelas vazias.

## Funcionalidades Core

1. **Categoria correta de salário (R1)** — folha income de salário-base e dicionário que resolvem
   "salário" sem clarify, mantendo Décimo Terceiro separado.
2. **Guarda de kind income↔expense (R2)** — reclassificação por kind antes da escrita; defesa final
   `ErrKindMismatch` intacta.
3. **Observabilidade de escrita sem swallow (R3)** — erro real propagado ao Run, span e log; causa-
   raiz provada do incidente.
4. **Confirmação única (R4)** — gate HITL do workflow é o único emissor de confirmação; LLM nunca
   pergunta "confirma?".
5. **Detecção de múltiplos lançamentos tolerante a BRL (R5)** — separadores internos a um valor não
   disparam falso positivo.
6. **Recuperação resiliente (R6)** — retentativa idempotente automática limitada e pending entry
   retomável.
7. **BRL canônico (R7)** — `money.BRL()` como fonte única de formatação.
8. **Forma de pagamento explícita (R8)** — pergunta com exemplos; nunca assume padrão; income não
   pede.

## Requisitos Funcionais

### R1 — Categoria correta de salário mensal

- RF-01: Deve existir uma subcategoria (folha) income de salário-base "Salário > Salário" sob a raiz
  `Salário`, distinta de Décimo Terceiro, Férias, PLR e Bônus, Vale-Alimentação e Vale-Refeição.
- RF-02: O `category_dictionary` deve reconhecer "salário", "salario", "meu salário", "recebi
  salário" e "recebi meu salário" com `kind=income`, `confidence=high` e `is_ambiguous=false`,
  apontando para a folha de salário-base.
- RF-03: "Recebi meu salário de R$ X" resolve para a folha de salário-base sem loop de clarify e
  persiste com `direction=income`.
- RF-04: A folha de salário-base é global (taxonomia `categories` não é por usuário) e fica
  disponível a cada usuário após a migração de seed.
- RF-05: "13º salário" (e variantes de Décimo Terceiro no dicionário) continua apontando para
  "Salário > Décimo Terceiro", nunca para a folha de salário-base.

### R2 — Guarda de kind income↔expense (defensiva, não é causa-raiz provada)

- RF-06: Receita (income) só grava em categoria de kind income; despesa (expense) só grava em
  categoria de kind expense.
- RF-07: Ao detectar incompatibilidade de kind na resolução de categoria, o fluxo reclassifica
  usando o kind correto **antes** de iniciar o pending write, em vez de prosseguir e falhar.
- RF-08: Sem categoria compatível, o agente pede esclarecimento uma única vez, sem gerar
  `usecaseError`.
- RF-09: O gate de escrita do módulo transactions (`ErrKindMismatch`) permanece como defesa final.

### R3 — Nunca engolir erro de escrita (observabilidade) — causa-raiz provada

- RF-10: O passo de escrita do workflow (`executeWithIdempotency`, `executeDirectWrite` e a
  validação `validateCategoryForWrite`) deve propagar o erro real para `platform_runs.error` e
  marcar `StepStatusFailed`, encerrando o swallow atual que retorna `StepStatusCompleted` com `error`
  em branco.
- RF-11: A falha upstream do income (tool `register_income`/runtime, que hoje nunca chega ao passo de
  escrita) deve gravar o erro real em `platform_runs.error` e emitir um span de erro **pesquisável**
  (nome estável, `RecordError`, status error) vinculado a `thread_id`, `run_id` e `wamid`.
- RF-12: Em ambos os pontos, emitir log em nível ERROR com `thread_id`, `run_id` e `wamid`; a
  mensagem ao usuário permanece amigável, sem detalhes técnicos.
- RF-13: A métrica `agents_write_total{operation,outcome}` é mantida com `outcome` fechado
  (`created`, `reconciled`, `replay`, `usecase_error`), com cardinalidade controlada — sem `user_id`,
  `correlation_key` ou `category_id` como label.

### R4 — Confirmação única por lançamento (prioridade máxima)

- RF-14: O LLM não emite pergunta de confirmação; a confirmação é responsabilidade exclusiva do gate
  HITL do workflow.
- RF-15: O gate HITL emite um único resumo por lançamento no formato "Confirma? R$ X em Raiz > Folha
  para data no pagamento?".
- RF-16: Após um único "sim", a escrita ocorre diretamente, sem qualquer segunda pergunta de
  confirmação (o caso 2026-07-08 15:17 do livro no pix não pode se repetir).
- RF-17: O gate permanece durável e idempotente: estado de espera fechado persistido no snapshot
  antes de pedir confirmação, retomada por merge-patch.
- RF-18: Cancelamento explícito ("não") descarta o lançamento sem efeito e nenhuma transação é
  persistida.

### R5 — Não confundir número BRL com múltiplos lançamentos

- RF-19: A detecção de múltiplos lançamentos ignora separadores de milhar e decimal internos a um
  único valor monetário.
- RF-20: Uma mensagem com um único valor e um único item não dispara a mensagem de múltiplos
  lançamentos.
- RF-21: Dois ou mais valores distintos, ou dois ou mais itens separados por conectores ("e",
  "mais", "também", vírgula entre itens), continuam disparando a detecção, com o texto de orientação
  inalterado.

### R6 — Recuperação resiliente em falha transitória

- RF-22: Falha transitória de escrita gera **até 2 retentativas automáticas** idempotentes com
  backoff curto (total abaixo de ~2s, dentro do mesmo turno), pela chave (`wamid`, `itemSeq`,
  operação), antes de desistir.
- RF-23: Esgotadas as retentativas sem sucesso, o pending entry permanece retomável: a próxima
  confirmação reexecuta a escrita sem repetir a classificação de categoria.
- RF-24: Reprocessar a mesma (`wamid`, `itemSeq`, operação) resulta em replay, nunca em segundo
  lançamento.
- RF-25: Se a transação já foi criada mas o registro no ledger de idempotência falhou, o sistema não
  reexecuta a escrita em processamento posterior da mesma chave.

### R7 — Valores sempre em BRL canônico

- RF-26: Cada valor monetário exibido tem exatamente duas casas decimais, separador de milhar com
  ponto e separador decimal com vírgula, com prefixo "R$ ".
- RF-27: Um único formatador canônico (`money.BRL()`) é a fonte de formatação; os formatadores locais
  `formatBRL` (onboarding) e `formatAmountBR` (pending) são consolidados nesse helper.
- RF-28: Aplica-se a onboarding, confirmação, resumo do mês e mensagens de erro que citem valores.
  Alertas consomem o mesmo helper canônico quando forem construídos no PRD `prd-alertas-proativos`
  (as call-sites de alerta ficam fora deste escopo, mas a fonte única de formatação é compartilhada).

### R8 — Pedir forma de pagamento quando ausente

- RF-29: Em despesa sem forma de pagamento informada, o agente não assume "dinheiro" nem qualquer
  outra forma padrão.
- RF-30: O agente pergunta a forma de pagamento e apresenta exemplos ("Como você pagou? Ex.:
  dinheiro, pix, débito, crédito, boleto, vale-refeição"); o prompt do slot e o reprompt incluem
  exemplos.
- RF-31: As instruções do agente proíbem inferir ou inventar a forma de pagamento não declarada.
- RF-32: Após a resposta, o mapeamento de texto para código (`cash`, `pix`, `debit_card`,
  `debit_in_account`, `boleto`, `ted`, `credit_card`, `vale_refeicao`, `vale_alimentacao`) segue as
  regras existentes; receita (income) não pede forma de pagamento.

## Experiência do Usuário

Fluxos principais (comportamento observável para o usuário WhatsApp):

- **Salário sem clarify**: "Recebi meu salário de R$ 13.874,40" → confirmação única em "Salário >
  Salário" → "sim" → transação persistida com `direction=income` e valor 1387440 centavos.
- **Décimo terceiro preservado**: "recebi meu 13º salário de R$ 5.000,00" → "Salário > Décimo
  Terceiro", não a folha base.
- **Confirmação única (livro no pix)**: "Comprei um livro de R$ 50,00 no pix" → um único resumo →
  "sim" → gravado em "Conhecimento > Livros e E-books" com pagamento pix, sem segunda pergunta.
- **Cancelamento**: após o resumo, "não" descarta sem gravar.
- **Número BRL único**: "Recebi meu salário de R$ 13.874,40" não dispara "registre um de cada vez".
- **Dois lançamentos reais**: "gastei 30 no ônibus e 15 no café" dispara exatamente a orientação de
  registrar um de cada vez.
- **Falha transitória**: a escrita falha e é retentada automaticamente; o lançamento persiste uma
  única vez e o usuário recebe confirmação de sucesso — sem recomeçar.
- **BRL canônico**: 554976, 80000000, 5000 e 5050 centavos exibem "R$ 5.549,76", "R$ 800.000,00",
  "R$ 50,00" e "R$ 50,50".
- **Forma de pagamento**: "Gastei R$ 150,00 no supermercado ontem" (sem forma) → "Como você pagou?
  Ex.: dinheiro, pix, débito, crédito, boleto, vale-refeição"; nunca assume "dinheiro". Receita não
  pergunta forma de pagamento.

Mensagem de erro ao usuário permanece amigável em qualquer falha; detalhes técnicos vão apenas para
`platform_runs.error`, span e log.

## Restrições Técnicas de Alto Nível

- **Substrato de agent**: Run auditável e gate HITL do substrato `internal/platform/agent`, com
  estado de espera fechado (DMMF state-as-type), persistência antes da confirmação e retomada por
  merge-patch. Não reimplementar Thread/Run/WorkingMemory/PendingStep em domínio.
- **Idempotência**: ledger de idempotência com unicidade por (`wamid`, `itemSeq`, operação) via
  `ON CONFLICT`; replay nunca gera segundo lançamento; transação criada com ledger falho não é
  reexecutada.
- **Observabilidade**: métricas com cardinalidade controlada (labels fechados; sem `user_id`,
  `correlation_key`, `category_id`); spans de erro pesquisáveis no Tempo; logs ERROR no Loki.
- **Taxonomia**: `categories` e `category_dictionary` são globais (não por usuário); a folha de
  salário-base e os termos entram por migração de seed.
- **Formatação**: `money.BRL()` em `internal/platform/money/money.go` é o único formatador; helpers
  locais consolidados.
- **Estados fechados (DMMF)**: kind income/expense, estados de pending, outcome de idempotência
  (`created`, `reconciled`, `replay`, `usecase_error`) e forma de pagamento como tipos fechados com
  smart constructors e `Decide*` puro.
- **Governança Go**: R-ADAPTER-001 (zero comentários, adaptadores finos), R-AGENT-WF-001
  (roteamento por registry, tool fina, LLM só em call-sites sancionadas), R-WF-KERNEL-001 (kernel
  genérico) permanecem obrigatórias.
- **BRL único**: moeda suportada é apenas BRL; sem multi-moeda.

## Dependências

- Migração de seed em `mecontrola.categories` para a folha income de salário-base e em
  `mecontrola.category_dictionary` para os termos de salário.
- Contrato de Run auditável e gate HITL do substrato de agent (`internal/platform/agent`).
- Ledger de idempotência com unicidade por (`wamid`, `itemSeq`, operação).
- Helper canônico `money.BRL()` em `internal/platform/money/money.go`.
- Instruções do agente no arquivo `mecontrola_agent.go` (pacote de agents, camada application).

## Dados e Permissões

- Dados obrigatórios por lançamento: descrição/termo, valor em centavos, data crua (`occurredAt`),
  `direction`/kind, forma de pagamento (para despesa), `wamid`, `itemSeq`, operação, `thread_id`,
  `run_id`.
- Perfis/permissões: usuário WhatsApp autenticado com `auth.Principal` estabelecido (source
  WhatsApp); acesso de operador aos dados de observabilidade (Postgres, traces, logs, métricas).

## Métricas e Critérios de Aceite

Critérios de aceite herdados integralmente da US (Gherkin), cobrindo:

- Salário resolve para "Salário > Salário" sem clarify e persiste income com 1387440 centavos.
- Décimo terceiro permanece na subcategoria específica.
- Receita nunca gravada em categoria de despesa; reclassifica por kind income.
- Incompatibilidade de kind sem categoria compatível pede esclarecimento uma vez, sem
  `usecaseError` silencioso.
- Falha de escrita grava erro real em `platform_runs.error`, span de erro com `thread_id`/`run_id`/
  `wamid` e log ERROR; resposta ao usuário amigável.
- Métrica de outcome de escrita incrementada com label de motivo fechado, sem labels de alta
  cardinalidade.
- Run de escrita falho produz trace de erro pesquisável com o motivo no span.
- Compra de livro no pix não pede confirmação duas vezes; LLM não emite confirmação própria;
  cancelamento descarta sem gravar.
- Valor BRL único não dispara múltiplos; dois lançamentos reais continuam barrados com o texto
  inalterado.
- Falha transitória retentada persiste o lançamento uma vez; reprocessar a mesma chave resulta em
  replay sem duplicar.
- BRL canônico: "R$ 5.549,76", "R$ 800.000,00", "R$ 50,00", "R$ 50,50".
- Despesa sem forma de pagamento pergunta com exemplos e não assume "dinheiro"; receita não
  pergunta forma de pagamento.

## Fora de Escopo

- Criação de novas raízes de categoria além de `Salário` e personalização de taxonomia por usuário.
- Reclassificação retroativa de transações já registradas em categoria errada.
- Registro simultâneo de múltiplos lançamentos em uma mensagem (permanece um de cada vez).
- Fluxo de desfazer pós-registro, dashboards ou alertas específicos, forma de pagamento padrão
  configurável e suporte a moedas além de BRL.
- Construção das call-sites de alerta e das mensagens de alerta (pertencem ao PRD
  `prd-alertas-proativos`); este PRD apenas garante o helper canônico compartilhado.

## Skills Necessárias

Obrigatórias na implementação, conforme o Trio Obrigatório de Desenvolvimento Go e o padrão de agent
do repositório:

- `.claude/skills/go-implementation/` (obrigatória): toda alteração de código Go (Etapas 1 a 5;
  matriz de validação por risco).
- `.agents/skills/mastra/` (obrigatória): substrato de agent, tools, workflows, memory e instruções
  do agente (`internal/platform/{agent,workflow,memory,tool}` e `internal/agents`).
- `.agents/skills/domain-modeling-production/` (obrigatória): kind income/expense, estados de
  pending, outcome de idempotência e forma de pagamento como tipos fechados (DMMF state-as-type,
  smart constructors, `Decide*` puro).
- `.agents/skills/design-patterns-mandatory/` (gate): decisão `aplicar` vs `não aplicar padrão`
  antes de qualquer mudança estrutural (provável Retry/Strategy leve em R6; `não aplicar padrão` nas
  demais).

Economia de contexto: carregar apenas as referências exigidas pelos gatilhos de cada tarefa; nunca
carregar `patterns-structural.md` para Factory, Functional Options, Adapter, Decorator ou Facade.

## Notas de Validação

- Testes real-LLM (política do repositório): salário resolve sem clarify; salário nunca em categoria
  expense; um único turno de confirmação (incluindo o livro no pix); número BRL formatado como
  lançamento único; despesa sem forma de pagamento pergunta com exemplos; receita não pergunta forma
  de pagamento.
- Testes de unidade/integração: propagação de erro para Run/span/log em falha de escrita; até 2
  retentativas idempotentes com backoff curto e ausência de duplicação; pending retomável após
  retentativas esgotadas; formatação BRL de milhar, milhão e valores pequenos.

## Suposições e Questões em Aberto

Decisões fechadas nesta versão (sem questões em aberto):

- **Retry (R6)**: até 2 retentativas automáticas idempotentes com backoff curto (< ~2s no mesmo
  turno) antes de deixar o pending entry retomável.
- **BRL/alertas (R7)**: este PRD consolida `money.BRL()` como fonte única e migra os sites
  existentes (onboarding, confirmação, resumo do mês, erros); alertas adotam o helper no
  `prd-alertas-proativos`.
- **Métrica de sucesso primária**: zero falhas silenciosas + 100% dos runs de escrita falhos
  rastreáveis (error + span + log) + 100% de confirmação única.

Suposições:

- O nome exato da folha é "Salário > Salário" (confirmado nos critérios de aceite da US).
- A confirmação de receita omite o trecho "no pagamento" do resumo, pois income não tem forma de
  pagamento (detalhe de redação a ser fechado na especificação técnica).
- O incidente diagnosticado tem R3 (observabilidade) como bloqueador raiz; R2 é mantida como defesa,
  não como mecanismo comprovado do incidente.
