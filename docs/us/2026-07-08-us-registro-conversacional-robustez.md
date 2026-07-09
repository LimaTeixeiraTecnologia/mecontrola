# US-01: Registro conversacional de transações confiável, fluido e em BRL

## Declaração
Como usuário do MeControla no WhatsApp, quero registrar minhas receitas e despesas por conversa de forma confiável, fluida e com valores sempre em BRL, para controlar meu dinheiro sem falhas silenciosas, sem confirmações repetidas e sem categoria errada.

## Contexto
- Problema: no diagnóstico de produção do usuário `f56e1142` (thread `74d83407`, 2026-07-08 14:45–14:51 BRT, investigado via SSH `root@187.77.45.48`), nenhum lançamento tentado foi persistido. As tabelas `mecontrola.transactions` e `mecontrola.agents_write_ledger` estão vazias e as duas runs `failed/usecaseError` (`c999f675`, `bb82fb13`) têm a coluna `error` em branco e zero linhas de log. A cadeia da falha: salário sem categoria-folha correta → parqueado em "Metas" (kind expense) → escrita income em categoria expense rejeitada (`ErrKindMismatch`) → erro engolido no passo de escrita → nada persistido, sem rastro. Somam-se confirmação dupla, falso positivo de múltiplos lançamentos no número BRL, ausência de retry, formatação BRL inconsistente e forma de pagamento assumida sem perguntar.
- Resultado esperado: o registro conversacional resolve a categoria correta, bloqueia incompatibilidade de kind antes da escrita, nunca engole erro, confirma uma única vez, tolera número BRL formatado, recupera falha transitória sem recomeçar, exibe valores em BRL canônico e pede a forma de pagamento quando ausente.
- Fonte: diagnóstico de produção e plano `/Users/jailtonjunior/.claude/plans/analise-criteriosamente-via-ssh-crispy-milner.md`.

Persona base: **Usuário do MeControla no WhatsApp** que registra receitas e despesas em linguagem natural.

Decisões confirmadas (não assumidas): confirmação única no workflow (o LLM nunca pergunta "confirma?"); criar folha income `Salário > Salário` e termos no `category_dictionary`; escopo cobre cada gap do diagnóstico.

## Status de Prova (confronto produção 2026-07-08)

Confronto executado contra código, DB (`mecontrola.*`), métricas (Prometheus via otel-lgtm), traces (Tempo) e logs (Loki/docker). Classificação por regra:

- R1 (salário) — Provado: árvore income real sem folha de salário-base e `category_dictionary` só com aliases de Décimo Terceiro; `agent_tool_invocations_total{tool="register_income"}=2` e nenhuma transação persistida.
- R2 (guarda de kind) — Refutado como causa-raiz; mantido como defesa: não há série `agents_write_total` para `register_income`, logo o salário NUNCA chegou ao passo de escrita — não houve tentativa de gravar em "Metas". O "Metas" às 17:49:39 é texto de confirmação do próprio LLM num run `succeeded/routed`. A causa do `usecaseError` do salário é upstream (tool `register_income`/runtime) e indeterminável hoje (consequência de R3). Regra mantida como defesa, não como mecanismo do incidente.
- R3 (observabilidade) — Provado e é o bloqueador raiz, com dois loci: (1) o passo do workflow engole o erro — `agents_write_total{operation="register_expense",outcome="usecase_error"}=1` incrementou mas o run ficou com `error` em branco; (2) a falha upstream do income não tem sinal atribuível — sem série `agents_write_total` para `register_income`, única pista `agent_runs_total{status="failed"}=4` sem motivo, e sem trace de erro pesquisável do run do agente no Tempo.
- R4 (confirmação única) — Provado: transcript 15:17 (livro no pix) e 17:51; `workflow_runs` pending-entry `succeeded/awaiting_input` (2ª confirmação = gate do workflow) e `agents_pending_entry_total`.
- R5 (número BRL) — Provado: transcript 17:50:43 e heurística no prompt do agente.
- R6 (retry) — Provado: cancelamento sem retry no código e `agents_pending_entry_total{outcome="cancelled"}=1`.
- R7 (BRL canônico) — Provado: transcript de onboarding "R$ 5549,76" e formatadores locais divergentes.
- R8 (forma de pagamento) — Provado: transcript 15:17/17:51 e schema com `paymentMethod` obrigatório mais prompt seco do slot.

## Regras de Negócio

### R1 — Categoria correta de salário mensal
- Deve existir uma subcategoria (folha) income de salário-base sob a raiz `Salário`, distinta de Décimo Terceiro, Férias, PLR e Bônus, Vale-Alimentação e Vale-Refeição.
- O `category_dictionary` deve reconhecer "salário", "salario", "meu salário", "recebi salário" e "recebi meu salário" com `kind=income`, `confidence=high` e `is_ambiguous=false`, apontando para a folha de salário-base.
- "Recebi meu salário de R$ X" resolve para a folha de salário-base sem loop de clarify e persiste com `direction=income`.
- A folha é global (taxonomia `categories` não é por usuário) e disponível a cada usuário após a migração de seed.

### R2 — Guarda de kind income↔expense (defensiva, não é causa-raiz provada)
- Natureza: guarda defensiva. A causa exata do `usecaseError` do salário na sessão diagnosticada NÃO é provável de determinar por telemetria, porque o erro é engolido (ver R3); esta regra reduz uma classe de falha, mas não deve ser tratada como o único mecanismo do incidente.
- Receita (income) só grava em categoria de kind income; despesa (expense) só grava em categoria de kind expense.
- Ao detectar incompatibilidade de kind na resolução de categoria, o fluxo reclassifica usando o kind correto antes de iniciar o pending write, em vez de prosseguir e falhar.
- Sem categoria compatível, o agente pede esclarecimento uma única vez, sem gerar `usecaseError`.
- O gate de escrita do módulo transactions (`ErrKindMismatch`) permanece como defesa final.

### R3 — Nunca engolir erro de escrita (observabilidade) — causa-raiz provada
Contexto de precisão: a métrica de outcome de escrita já existe e é adequada — `agents_write_total{operation,outcome}` com `outcome` fechado (`created`, `reconciled`, `replay`, `usecase_error`), e o `idempotent_write` já faz `span.RecordError` na falha e log no insert. O gap está em dois pontos, não na ausência de métrica:

- Ponto 1 — swallow no passo do workflow: `executeWithIdempotency`/`executeDirectWrite` recebem `idemErr`/`writeErr`, descartam o erro, retornam `StepStatusCompleted` e não gravam `platform_runs.error`. Correção: propagar o erro para `platform_runs.error`, marcar `StepStatusFailed` e manter `agents_write_total`. Provado: `agents_write_total{operation="register_expense",outcome="usecase_error"}=1` incrementou, mas o run correspondente ficou com `error` em branco.
- Ponto 2 — falha upstream do income sem sinal atribuível: o `usecaseError` do salário nunca chegou ao passo de escrita (não há série `agents_write_total` para `register_income`); a falha ocorreu na tool `register_income`/runtime. Correção: `register_attempt`/runtime devem gravar o erro real em `platform_runs.error` e emitir um span de erro pesquisável (nome estável, `RecordError`, status error) vinculado a `thread_id`, `run_id` e `wamid` — hoje o run do agente no worker não aparece como trace de erro no Tempo (apenas o ingest `agents.route.whatsapp_inbound` da API é observável).
- Em ambos os pontos: log em nível ERROR com `thread_id`, `run_id` e `wamid`; mensagem ao usuário permanece amigável; cardinalidade controlada (sem `user_id`, `correlation_key` ou `category_id`).

### R4 — Confirmação única por lançamento (prioridade máxima)
- O LLM não emite pergunta de confirmação; a confirmação é responsabilidade exclusiva do gate HITL do workflow.
- O gate HITL emite um único resumo por lançamento no formato "Confirma? R$ X em Raiz > Folha para data no pagamento?".
- Após um único "sim", a escrita ocorre diretamente, sem qualquer segunda pergunta de confirmação.
- Exemplo real que não pode se repetir (2026-07-08 15:17): "Comprei um livro de R$ 50,00 no pix" → "Confirma? R$ 50,00 em Conhecimento para a compra de um livro no pix? ✅" → usuário "sim" → segunda pergunta indevida "Confirma? compra de um livro R$ 50,00 em Conhecimento > Livros e E-books para hoje (08/07/2026) no pix? ✅".
- O gate permanece durável e idempotente (estado de espera fechado persistido no snapshot antes de pedir confirmação, retomada por merge-patch).

### R5 — Não confundir número BRL com múltiplos lançamentos
- A detecção de múltiplos lançamentos ignora separadores de milhar e decimal internos a um único valor monetário.
- Uma mensagem com um único valor e um único item não dispara a mensagem de múltiplos lançamentos.
- Dois ou mais valores distintos ou dois ou mais itens separados por conectores ("e", "mais", "também", vírgula entre itens) continuam disparando a detecção, com o texto de orientação inalterado.

### R6 — Recuperação resiliente em falha transitória
- Falha transitória de escrita gera retentativa automática limitada, idempotente pela chave (`wamid`, `itemSeq`, operação), antes de desistir.
- Sem recuperação na hora, o pending entry permanece retomável: a próxima confirmação reexecuta a escrita sem repetir a classificação de categoria.
- Reprocessar a mesma (`wamid`, `itemSeq`, operação) resulta em replay, nunca em segundo lançamento.
- Se a transação já foi criada mas o registro no ledger de idempotência falhou, o sistema não reexecuta a escrita em processamento posterior da mesma chave.

### R7 — Valores sempre em BRL canônico
- Cada valor monetário exibido tem exatamente duas casas decimais, separador de milhar com ponto e separador decimal com vírgula, com prefixo "R$ ".
- Um único formatador canônico (`money.BRL()`) é a fonte de formatação; os formatadores locais `formatBRL` (onboarding) e `formatAmountBR` (pending) são consolidados nesse helper.
- Aplica-se a onboarding, confirmação, resumo do mês, alertas e mensagens de erro que citem valores.

### R8 — Pedir forma de pagamento quando ausente
- Em despesa sem forma de pagamento informada, o agente não assume "dinheiro" nem qualquer outra forma padrão.
- O agente pergunta a forma de pagamento e apresenta exemplos ("Como você pagou? Ex.: dinheiro, pix, débito, crédito, boleto, vale-refeição"); o prompt do slot e o reprompt incluem exemplos.
- As instruções do agente proíbem inferir ou inventar a forma de pagamento não declarada.
- Após a resposta, o mapeamento de texto para código (`cash`, `pix`, `debit_card`, `debit_in_account`, `boleto`, `ted`, `credit_card`, `vale_refeicao`, `vale_alimentacao`) segue as regras existentes; receita (income) não pede forma de pagamento.

## Critérios de Aceite
```gherkin
Cenário: Salário mensal resolve para a folha correta sem clarify
  Dado que existe a folha income "Salário > Salário" e o dicionário reconhece "salário" como income de alta confiança
  Quando o usuário envia "Recebi meu salário de R$ 13.874,40"
  Então o lançamento é resolvido para "Salário > Salário" sem pergunta de categoria
  E a transação é persistida com direction igual a income e valor 1387440 centavos

Cenário: Décimo terceiro continua na subcategoria específica
  Dado que o dicionário mantém "13º salário" apontando para "Salário > Décimo Terceiro"
  Quando o usuário envia "recebi meu 13º salário de R$ 5.000,00"
  Então o lançamento é resolvido para "Salário > Décimo Terceiro" e não para a folha de salário-base

Cenário: Receita nunca é gravada em categoria de despesa
  Dado que "Metas" é uma categoria de kind expense
  Quando o usuário envia "Recebi meu salário de R$ 13.874,40"
  Então o fluxo reclassifica usando kind income
  E não inicia escrita apontando para a categoria "Metas"

Cenário: Incompatibilidade de kind sem categoria compatível não gera erro silencioso
  Dado que o lançamento é income e nenhuma categoria income compatível foi encontrada
  Quando a guarda de kind bloqueia a escrita
  Então o agente pede esclarecimento de categoria uma única vez
  E a run não termina como usecaseError silencioso

Cenário: Falha de escrita grava erro no Run, span e log
  Dado que a escrita do lançamento falha por erro de domínio ou infraestrutura
  Quando o passo de escrita conclui a execução
  Então o erro real é gravado em platform_runs.error
  E o span é marcado como erro com thread_id, run_id e wamid
  E um log em nível ERROR é emitido com thread_id, run_id e wamid
  E a resposta ao usuário permanece amigável sem detalhes técnicos

Cenário: Métrica de outcome de escrita registra o motivo
  Dado que uma escrita de lançamento termina com sucesso ou com falha
  Quando o resultado é contabilizado
  Então a métrica de outcome de escrita é incrementada com label de motivo fechado
  E os labels não incluem user_id, correlation_key nem category_id

Cenário: Run de escrita falha produz trace de erro pesquisável
  Dado que a escrita do lançamento falha no worker
  Quando o run é finalizado
  Então existe um span de erro pesquisável no trace vinculado a thread_id, run_id e wamid
  E o motivo do erro está registrado no span

Cenário: Compra de livro no pix não pede confirmação duas vezes
  Dado que o usuário envia "Comprei um livro de R$ 50,00 no pix"
  E o agente apresentou um único resumo de confirmação do lançamento
  Quando o usuário responde "sim"
  Então o lançamento é gravado em Conhecimento > Livros e E-books com forma de pagamento pix
  E o agente não envia uma segunda pergunta de confirmação

Cenário: LLM não emite confirmação própria
  Dado que o usuário envia "Gastei R$ 150,00 no supermercado ontem"
  Quando o agente processa o lançamento
  Então somente o gate HITL do workflow emite um resumo de confirmação
  E o LLM não emite pergunta de confirmação adicional

Cenário: Cancelamento explícito descarta o lançamento sem gravar
  Dado que o gate HITL apresentou o resumo de confirmação
  Quando o usuário responde "não"
  Então o lançamento é descartado sem efeito
  E nenhuma transação é persistida

Cenário: Valor BRL único não dispara múltiplos lançamentos
  Dado que o usuário envia "Recebi meu salário de R$ 13.874,40"
  Quando o agente avalia a mensagem
  Então a detecção de múltiplos lançamentos não dispara
  E o lançamento único segue para registro

Cenário: Dois lançamentos reais continuam sendo barrados
  Dado que o usuário envia "gastei 30 no ônibus e 15 no café"
  Quando o agente avalia a mensagem
  Então a detecção de múltiplos lançamentos dispara
  E o agente responde exatamente a orientação de registrar um de cada vez

Cenário: Falha transitória é retentada e o lançamento persiste uma vez
  Dado que a primeira tentativa de escrita falha por erro transitório
  Quando a retentativa idempotente automática ocorre
  Então o lançamento é persistido uma única vez
  E o usuário recebe confirmação de sucesso

Cenário: Reprocessamento da mesma chave não duplica lançamento
  Dado que um lançamento com uma dada wamid, itemSeq e operação já foi persistido
  Quando a mesma chave é reprocessada
  Então o resultado é replay
  E nenhum segundo lançamento é criado

Cenário: Valores em BRL canônico com separador de milhar
  Dado que os valores a exibir são 554976, 80000000, 5000 e 5050 centavos
  Quando o agente formata os valores para o usuário
  Então as saídas são "R$ 5.549,76", "R$ 800.000,00", "R$ 50,00" e "R$ 50,50"

Cenário: Despesa sem forma de pagamento pede orientação com exemplo
  Dado que o usuário envia "Gastei R$ 150,00 no supermercado ontem" sem informar como pagou
  Quando o agente processa a despesa
  Então o agente pergunta a forma de pagamento e apresenta exemplos como dinheiro, pix, débito, crédito, boleto e vale-refeição
  E não assume "dinheiro" automaticamente

Cenário: Receita não pede forma de pagamento
  Dado que o usuário envia "Recebi meu salário de R$ 13.874,40"
  Quando o agente processa a receita
  Então o agente não pergunta a forma de pagamento
```

## Dados e Permissões
- Dados obrigatórios: descrição/termo do lançamento, valor em centavos, data crua (`occurredAt`), `direction`/kind, forma de pagamento (para despesa), `wamid`, `itemSeq`, operação, `thread_id`, `run_id`.
- Perfis/permissões: usuário WhatsApp autenticado com `auth.Principal` estabelecido (source WhatsApp); acesso de operador aos dados de observabilidade (Postgres, traces, logs, métricas).

## Dependências
- Migração de seed em `mecontrola.categories` para a folha income de salário-base e em `mecontrola.category_dictionary` para os termos de salário.
- Contrato de Run auditável e gate HITL do substrato de agent (`internal/platform/agent`), com estado de espera fechado, persistência antes da confirmação e retomada por merge-patch.
- Ledger de idempotência com unicidade por (`wamid`, `itemSeq`, operação).
- Helper canônico `money.BRL()` em `internal/platform/money/money.go`.
- Instruções do agente no arquivo `mecontrola_agent.go` (pacote de agents, camada application).

## Fora de Escopo
- Criação de novas raízes de categoria além de `Salário` e personalização de taxonomia por usuário.
- Reclassificação retroativa de transações já registradas em categoria errada.
- Registro simultâneo de múltiplos lançamentos em uma mensagem (permanece um de cada vez).
- Fluxo de desfazer pós-registro, dashboards ou alertas específicos, forma de pagamento padrão configurável e suporte a moedas além de BRL.

## Skills Necessárias
O uso das quatro skills abaixo é obrigatório na implementação, conforme o Trio Obrigatório de Desenvolvimento Go e o padrão de agent do repositório:
- `.claude/skills/go-implementation/` (obrigatória): toda alteração de código Go (Etapas 1 a 5; matriz de validação por risco).
- `.agents/skills/mastra/` (obrigatória): substrato de agent, tools, workflows, memory e instruções do agente (`internal/platform/{agent,workflow,memory,tool}` e `internal/agents`).
- `.agents/skills/domain-modeling-production/` (obrigatória): kind income/expense, estados de pending, outcome de idempotência e forma de pagamento como tipos fechados (DMMF state-as-type, smart constructors, `Decide*` puro).
- `.agents/skills/design-patterns-mandatory/` (gate): decisão `aplicar` vs `não aplicar padrão` antes de qualquer mudança estrutural (provável Retry/Strategy leve em R6; `não aplicar padrão` nas demais).

Economia de contexto: carregar apenas as referências exigidas pelos gatilhos de cada tarefa; nunca carregar `patterns-structural.md` para Factory, Functional Options, Adapter, Decorator ou Facade.

## Evidências
- Entrada: pedido do usuário, requisito explícito de valores em BRL, prioridade máxima na confirmação única e plano aprovado.
- Base de código:
  - Taxonomia income real em `mecontrola.categories` sem folha de salário-base; `category_dictionary` só com aliases de Décimo Terceiro.
  - `ErrKindMismatch` em `internal/categories/application/usecases/resolve_category_for_write.go`; `ResolveForWrite` chamado em `internal/agents/application/workflows/pending_entry_workflow.go:430`; busca de candidatos em `internal/agents/application/workflows/category_resolution.go`.
  - Swallow do erro em `internal/agents/application/workflows/pending_entry_workflow.go:453-468` (executeWithIdempotency e executeDirectWrite) e `:436-439` (validateCategoryForWrite); fechamento do Run em `internal/platform/agent/runtime.go` (`finishRun`, `closeRun`).
  - Formato de confirmação nas instruções em `mecontrola_agent.go:161` (pacote de agents, camada application); resumo do workflow em `buildConfirmSummary` e slot `AwaitingSlotConfirmation` de `pending_entry_workflow.go`.
  - Heurística de múltiplos lançamentos em `mecontrola_agent.go:16` e `:55` (pacote de agents, camada application).
  - Cancelamento sem retry em `pending_entry_workflow.go:454-456`; risco de escrita duplicada em `internal/agents/application/usecases/idempotent_write.go:100-122`; unicidade por chave via `ON CONFLICT` em `internal/agents/infrastructure/persistence/write_ledger_repository.go`.
  - Helper `money.BRL()` em `internal/platform/money/money.go:60`; formatadores locais `formatBRL` em `onboarding_workflow.go:300` (`:311`) e `formatAmountBR` em `pending_entry_workflow.go:570` (`:576`).
  - Prompt seco do slot em `pending_entry_workflow.go:680`; `paymentMethod` obrigatório no schema da ferramenta em `internal/agents/application/tools/register_expense.go:52` com enum em `:44`; mapeamento em `mecontrola_agent.go:76`.
- Telemetria (produção 2026-07-08, otel-lgtm): `agent_runs_total` succeeded=10, failed=4 (sem label de motivo); `agent_tool_invocations_total{tool="register_income"}=2`, `{tool="register_expense"}=2`, `{tool="classify_category"}=1`; `agents_write_total{operation="register_expense",outcome="usecase_error"}=1` e nenhuma série para `register_income` (prova de que o income não chegou à escrita); `agents_pending_entry_slot_total{outcome="cancelled",slot="confirmation"}=1` e `{outcome="replied",slot="confirmation"}=1`; `money.BRL()` em `money.go` usa `groupThousands` (produz "R$ 5.549,76"), enquanto `formatAmountBR`/`formatBRL` locais não; `buildConfirmSummary` (pending_entry_workflow.go:712) é o 2º resumo de confirmação e usa `formatAmountBR`; Tempo com traces de `mecontrola-api` (ingest `agents.route.whatsapp_inbound`) e de `mecontrola-worker` (reaper, embedding), mas sem span de erro do run de registro; Loki e docker logs sem linha de erro na janela do incidente. Ressalva de coleta: a primeira rodada de queries falhou por o container otel-lgtm ter apenas `curl` (sem `wget`); refeitas com `curl`.
- Inferências: as tabelas vazias (`transactions`, `agents_write_ledger`) e as colunas `error` vazias confirmam falha silenciosa; a causa exata do `usecaseError` do salário é indeterminável por telemetria (consequência de R3), portanto NÃO se afirma que "Metas" foi o mecanismo — o "Metas" às 17:49:39 é texto de confirmação do próprio LLM; o exemplo de 2026-07-08 15:17 confirma a confirmação dupla; "R$ 5549,76" no onboarding confirma o formatador local sem separador de milhar; o valor "13.874,40" com ponto e vírgula gerou o falso positivo de múltiplos lançamentos.
- Não evidenciado: nenhum item pendente de evidência para esta história.

## Notas de Validação
- Cobrir com testes real-LLM (conforme política do repositório): salário resolve sem clarify; salário nunca em categoria expense; um único turno de confirmação (incluindo o caso do livro no pix); número BRL formatado como lançamento único; despesa sem forma de pagamento pergunta com exemplos; receita não pergunta forma de pagamento.
- Cobrir com testes de unidade/integração: propagação de erro para Run/span/log em falha de escrita; retentativa idempotente e ausência de duplicação; formatação BRL de milhar, milhão e valores pequenos.
