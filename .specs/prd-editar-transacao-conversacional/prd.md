# PRD — Editar Transação pela Conversa (paridade total de campos)

- spec-version: 1
- Data: 2026-07-10
- Status: aprovado para create-technical-specification
- Fonte (User Story): `docs/us/2026-07-10-us-editar-transacao-conversacional.md`
- Módulos impactados: `internal/agents`, `internal/transactions`, `internal/budgets`, substrato `internal/platform/{agent,workflow,tool,memory,scorer}`
- Questões em aberto: nenhuma (15 decisões materiais resolvidas em 4 rodadas de múltipla escolha; ver seção 5)

---

## 1. Objetivo

Permitir que o usuário do MeControla edite pela conversa no WhatsApp uma transação já registrada, ajustando qualquer um dos campos suportados pelo domínio (valor, descrição, data, categoria, forma de pagamento, cartão e parcelas, direção), com confirmação humana explícita antes de gravar, controle otimista de concorrência e reflexo correto no consumo do orçamento por categoria — sem que o usuário precise excluir e recadastrar o lançamento, e sem qualquer afirmação de sucesso sem gravação real.

## 2. Contexto e Problema

- A edição conversacional existe apenas parcialmente: a tool `edit_entry` aceita somente `amountCents`, `description` e `occurredAt` (`internal/agents/application/tools/edit_entry.go:30-43`), enquanto o caso de uso de domínio `UpdateTransaction` já suporta editar direção, forma de pagamento, categoria/subcategoria, cartão, parcelas, valor, descrição e data com `version` de controle otimista (`internal/transactions/application/usecases/update_transaction.go:53-160`). Há uma lacuna funcional entre o que o agente oferece e o que o domínio sabe fazer.
- O reflexo no orçamento está quebrado para edição: `internal/budgets` consome apenas `transactions.transaction.created.v1` e `transactions.transaction.deleted.v1` (`internal/budgets/module.go:144-145`); não existe consumidor para `transactions.transaction.updated.v1`, apesar de o `transactions` publicá-lo. Além disso, o evento `TransactionUpdated` não carrega `CategoryID`/`SubcategoryID` (`internal/transactions/domain/entities/events.go:26-35`), diferente do `TransactionCreated`. Consequência: editar valor, data ou categoria não atualiza `budgets_expenses`, deixando resumo mensal e alertas de limite defasados.
- Criar transação por conversa já é robusto (workflow `pending_entry` + `register_expense`/`register_income` + 22 cenários golden C1–C22) e serve de molde de reuso, não de entrega.

## 3. Usuários-alvo

- Persona primária: usuário final do MeControla que registra e corrige seus lançamentos financeiros por mensagens de texto no WhatsApp.
- Ator de sistema: agente conversacional `mecontrola` (runtime de tool-calling) operando sob a identidade inbound do usuário (principal resolvido de `resourceId`).

## 4. Escopo

### 4.1 Em escopo

- Edição conversacional de uma transação existente do próprio usuário, com paridade total dos campos suportados pelo domínio.
- Resolução do alvo por "último lançamento" e por atributos (valor/descrição/data/categoria), com desambiguação quando houver múltiplas correspondências.
- Confirmação humana explícita (HITL) antes de gravar, com resumo de impacto.
- Gravação via `UpdateTransaction` com controle otimista (`version`) e tratamento de conflito.
- Idempotência de escrita por `(wamid, itemSeq, operation)`.
- Recomposição de parcelas e fatura para transações de cartão de crédito (edição no nível da compra inteira).
- Reflexo no orçamento: novo consumidor de `transactions.transaction.updated.v1` em `internal/budgets` e enriquecimento do evento `TransactionUpdated` com categoria, recomputando todas as competências afetadas.
- Cobertura integral das possibilidades de conversação já existentes (perguntas de esclarecimento, reprompts, confirmações, cancelamentos, expiração, erros) reusando os textos verbatim atuais.

### 4.2 Fora de escopo

- Criar transação por conversa (já implementado; tratado como pré-condição e molde de reuso).
- Excluir transação por conversa (coberto por `delete_entry` + `destructive_confirm`).
- Edição via API HTTP (`UpdateTransactionHandler` já existe e não é alvo desta história conversacional).
- Edição de templates de recorrência (`update_recurrence`/`delete_recurrence`); editar uma ocorrência materializada afeta apenas aquela transação.
- Edição de parcela individual de uma compra de cartão (o domínio opera na compra inteira).
- Edição em lote de múltiplas transações numa única mensagem.

## 5. Decisões de Produto (resolvidas)

Todas as decisões abaixo foram confirmadas com o solicitante (recomendação aceita), fechando 0 questões em aberto.

- D-01: O reflexo no orçamento entra no escopo deste PRD (novo consumidor de `updated.v1` + enriquecimento do evento com `CategoryID`/`SubcategoryID`). Justificativa: sem isso, editar valor/categoria/data é falso sucesso para o orçamento.
- D-02: A transação alvo é resolvida por "último lançamento" e por atributos, com lista de desambiguação quando houver mais de uma correspondência.
- D-03: É permitido editar transação de competência (mês) passada, respeitando o comportamento de cutoff já existente no consumo do orçamento.
- D-04: Paridade total inclui migração de forma de pagamento, inclusive para/de cartão de crédito (com recomposição de parcelas e fatura), respeitando os guards de domínio.
- D-05: Gate de aceite = suíte golden real-LLM com razão de acerto ≥ 0,90 por categoria mais verificação de consistência transação↔orçamento.
- D-06: Rollout = liberar para todos após o gate de aceite (sem feature flag por usuário).
- D-07: Conflito de `version` (controle otimista) → re-ler o estado atual e re-apresentar confirmação fresca; sem sobrescrita silenciosa e sem last-writer-wins.
- D-08: Granularidade de edição de cartão = compra inteira; parcelas e fatura recompostas pelo domínio (`DecideUpdate`).
- D-09: Edição no-op (valores idênticos aos atuais) não grava, não incrementa `version` e informa que nada mudou.
- D-10: Alvo inexistente e alvo já excluído (soft-deleted) produzem mensagens distintas.
- D-11: Editar múltiplos campos do mesmo alvo numa única mensagem é suportado; a guarda de "um lançamento por vez" continua valendo apenas para múltiplas transações.
- D-12: Editar uma transação materializada de recorrência afeta apenas aquela ocorrência; o template não muda.
- D-13: Campos editáveis de receita = valor, descrição, data, categoria e direção (receita não possui forma de pagamento nem cartão).
- D-14: Mudar a direção exige re-resolução de categoria compatível com o novo kind (expense/income).
- D-15: Edição que afeta múltiplos meses recomputa todas as competências afetadas (antigas + novas), via `RefMonthsAffected`.

## 6. Requisitos Funcionais

### 6.1 Resolução do alvo

- RF-01: O sistema deve iniciar uma edição a partir de linguagem natural que referencie o "último lançamento" ou atributos do lançamento (valor, descrição, data, categoria).
- RF-02: Quando a referência do usuário corresponder a mais de uma transação, o sistema deve apresentar uma lista numerada de candidatas e aguardar a escolha antes de prosseguir.
- RF-03: O alvo deve pertencer ao usuário autenticado; transação de outro usuário não é localizável.
- RF-04: Alvo inexistente ou não pertencente ao usuário deve gerar mensagem de "não localizado"; alvo existente porém soft-deleted deve gerar mensagem distinta de "já excluído / não editável" (D-10).

### 6.2 Campos editáveis e paridade

- RF-05: Para transação de despesa, devem ser editáveis: valor, descrição, data, categoria/subcategoria, forma de pagamento, cartão, parcelas e direção.
- RF-06: Para transação de receita, devem ser editáveis: valor, descrição, data, categoria/subcategoria e direção; forma de pagamento e cartão não são oferecidos (D-13).
- RF-07: O sistema deve suportar editar mais de um campo do mesmo alvo numa única mensagem, resolvendo tudo em uma única confirmação (D-11).
- RF-08: A guarda de múltiplas transações por mensagem permanece: ao detectar mais de uma intenção de lançamento/edição distinta, o agente responde a frase verbatim de múltiplos lançamentos e não chama ferramenta de escrita.

### 6.3 Coleta conversacional (slot-filling)

- RF-09: Quando faltar um dado para completar a edição (categoria, forma de pagamento, cartão ou data), o sistema deve perguntar um campo por vez usando os textos verbatim existentes ("Qual é a categoria deste lançamento?", "Como você pagou? Ex.: dinheiro, pix, débito, crédito, boleto, vale-refeição", "Qual cartão foi utilizado?", "Qual foi a data do lançamento?").
- RF-10: Cada slot admite um único reprompt; após a segunda resposta não reconhecida, a edição é cancelada com a mensagem correspondente de não identificação.
- RF-11: A descrição não pode ser parafraseada; deve refletir o termo literal do usuário.
- RF-12: Editar a categoria deve disparar re-resolução via `classify_category`; múltiplos candidatos devem ser apresentados como lista numerada ("Qual se encaixa melhor? 1. ... 2. ...").
- RF-13: Editar para forma de pagamento cartão de crédito deve exigir resolução do cartão por apelido via `resolve_card` antes de confirmar; cartão não encontrado deve levar a listar cartões e pedir escolha; o `cardId` nunca é inventado.
- RF-14: Mudar a direção deve exigir re-resolução de categoria compatível com o novo kind (expense/income), pedindo escolha se ambíguo (D-14).
- RF-15: Migração para fora de cartão de crédito deve ser bloqueada quando houver parcelas em aberto, conforme guard de domínio.

### 6.4 Confirmação (HITL)

- RF-16: Toda edição exige confirmação humana explícita antes de gravar; o sistema deve apresentar um resumo de impacto contendo descrição, valor, categoria, data e segmento de pagamento (crédito à vista / crédito em Nx / forma de pagamento; receita omite pagamento).
- RF-17: Aceite reconhecido (`sim`, `confirmar`, `confirma`, `ok`, `pode`) grava a edição; cancelamento reconhecido (`não`, `nao`, `cancela`, `cancelar`, `deixa pra lá`, `não registra`) descarta com "Tudo certo, o registro foi cancelado.".
- RF-18: Resposta ambígua na confirmação gera um único reprompt; a segunda resposta ambígua cancela a operação sem efeito.
- RF-19: O estado de espera deve ser persistido antes de o sistema perguntar; a retomada aplica merge-patch sobre o snapshot antes de qualquer parse; a coleta expira em 35 minutos e o gate de confirmação em até 5 minutos; expirado, a operação é cancelada sem efeito com a mensagem "O registro expirou. Para registrar, envie a informação completa novamente." e o run é concluído sem estado órfão.

### 6.5 Gravação, concorrência e idempotência

- RF-20: A gravação deve ocorrer via `UpdateTransaction` usando `version` (controle otimista).
- RF-21: Em conflito de `version` (a transação mudou entre a leitura e a confirmação), o sistema deve re-ler o estado atual e re-apresentar uma confirmação fresca com os valores correntes, sem sobrescrita silenciosa (D-07).
- RF-22: Edição no-op (valores confirmados idênticos aos atuais) não deve gravar, não deve incrementar `version` e deve informar que nada foi alterado (D-09).
- RF-23: A operação deve ser idempotente por `(wamid, itemSeq, operation)`; o reenvio da mesma mensagem não gera segunda mutação e devolve desfecho de replay.

### 6.6 Cartão de crédito e parcelas

- RF-24: Editar uma compra parcelada deve recompor todas as parcelas e os deltas de fatura via domínio (`DecideUpdate`); a edição é no nível da compra inteira, não da parcela individual (D-08).

### 6.7 Anti-simulação e formatação

- RF-25: O sistema nunca deve afirmar que a edição foi salva sem o retorno efetivo da gravação; falha de infraestrutura deve responder exatamente "Não consegui registrar. Tente novamente em breve." e marcar o run como falho.
- RF-26: As respostas devem usar negrito de WhatsApp com asterisco único (`*texto*`), sem asterisco duplo, e não podem vazar termos internos (`workflow`, `run`, `thread`, `correlation`, `usecase`, `sistema interno`).

### 6.8 Reflexo no orçamento

- RF-27: Ao gravar a edição, o módulo `transactions` deve publicar `transactions.transaction.updated.v1` enriquecido com `CategoryID`/`SubcategoryID` e com `RefMonthsAffected` (competências antigas e novas).
- RF-28: O módulo `budgets` deve consumir `transactions.transaction.updated.v1` e atualizar `budgets_expenses` pelo caminho de atualização (`ExpectedVersion` / `MutationKindUpdate`), recomputando cada competência afetada.
- RF-29: A troca de categoria deve mover o consumo da categoria antiga para a nova; a troca de valor ou data deve ajustar o consumo e/ou a competência; o resumo mensal e os alertas de limite devem refletir o novo estado.
- RF-30: Editar transação de competência passada deve ser permitido, respeitando o cutoff existente do consumo do orçamento (D-03).

### 6.9 Roteamento, auditoria e observabilidade

- RF-31: A capacidade de edição deve ser resolvida por registry (agente/tool/workflow), sem `switch case intent.Kind`.
- RF-32: Cada edição deve ser um Run auditável contendo, no mínimo, `thread_id`, `run_id`, `workflow`, `tool`, `status`, `duration_ms` e `error` quando houver, referenciando o `decision_id` do audit trail; métricas com cardinalidade controlada (labels permitidos como enums fechados; proibido `user_id` e `category_id` como label).

## 7. Requisitos Não-Funcionais

- RNF-01: O LLM só pode aparecer nas call-sites sancionadas (loop de tool-calling do agente, step que chama `Stream`, scorer LLM-judged); nenhum LLM no kernel de workflow nem dentro de tool determinística; OpenRouter permanece como único provider, sem fallback chain nem circuit breaker.
- RNF-02: Adapters finos e zero comentários em código Go de produção; estados de fronteira como tipos fechados (DMMF state-as-type); regra de negócio exclusivamente em `Decide*`; validação em smart constructors; input DTOs com `Validate()`.
- RNF-03: Idioma PT-BR e formatação WhatsApp em todas as respostas do agente.
- RNF-04: Sem regressão nos fluxos existentes de criar, consultar e excluir transações e nos consumidores de orçamento já registrados.
- RNF-05: A edição não pode introduzir nova chamada de LLM inline fora das call-sites sancionadas; a latência deve permanecer dentro do orçamento de turno atual do agente.
- RNF-06: Persistência durável do estado do workflow via `workflow.Store`; retomada por merge-patch (RFC 7386); nenhum side-store de rascunho fora do snapshot do kernel.

## 8. Métricas de Sucesso e Gate de Aceite

- Gate de aceite (D-05): suíte golden real-LLM cobrindo os cenários de edição com razão de acerto ≥ 0,90 por categoria; verificação de consistência transação↔orçamento (após editar valor/categoria/data, `budgets_expenses`, resumo mensal e alertas refletem o novo estado); scorer `write_persistence_accuracy` verde (desfechos de escrita `Routed`/`Reconciled`); build, vet, test race e lint do projeto verdes.
- Métrica de produto: proporção de correções de lançamento feitas por edição conversacional em vez de exclusão + recadastro; ausência de inconsistência conhecida entre transação e orçamento após edição.

## 9. Dependências e Mudanças em Outros Módulos

- `internal/transactions`: enriquecer o struct `TransactionUpdated` e o produtor com `CategoryID`/`SubcategoryID` (`internal/transactions/domain/entities/events.go:26-35`; `transaction_event_publisher.go`); reusar `UpdateTransaction` e `TransactionWorkflow.DecideUpdate` sem alterar as invariantes.
- `internal/budgets`: novo consumidor para `transactions.transaction.updated.v1` registrado em `internal/budgets/module.go` (hoje só `created`/`deleted` em `module.go:144-145`), reusando `UpsertExpense` (`upsert_expense.go:150-218`) e o molde de `transaction_created_consumer.go`.
- `internal/agents`: capacidade de edição com paridade reusando o molde do `pending_entry_workflow` (slot-filling) e o gate do `destructive_confirm_workflow` (`OpEditEntry`), o binding `transactionsLedgerAdapter.UpdateTransaction`, a idempotência `IdempotentWrite`, e as tools `classify_category`/`resolve_card`.
- Governança obrigatória: R-AGENT-WF-001, R-ADAPTER-001, R-TXN-WORKFLOWS-001, R-WF-KERNEL-001, R-DTO-VALIDATE-001, R-TESTING-001.

## 10. Riscos e Mitigações

- R-01: Enriquecer `TransactionUpdated` altera contrato de evento consumido por outros. Mitigação: campo aditivo compatível; verificar consumidores atuais antes do merge.
- R-02: Recomputo de orçamento em múltiplas competências (parcelas) pode divergir se `RefMonthsAffected` não cobrir meses que saíram. Mitigação: RF-15/RF-28 exigem cobrir competências antigas e novas.
- R-03: Conflito de `version` mal tratado poderia sobrescrever silenciosamente. Mitigação: RF-21 exige re-leitura e re-confirmação, nunca last-writer-wins.
- R-04: Brittleness de teste real-LLM pode mascarar defeito de produto. Mitigação: harness dirige até o estado/invariante semântico, sem baixar a régua de 0,90 por categoria.
- R-05: Migração de forma de pagamento envolvendo cartão pode gerar faturas inconsistentes. Mitigação: reuso de `DecideUpdate` (recomposição atômica) e guard de parcelas em aberto (RF-15).

## 11. Rastreabilidade (US → RF)

- US "Editar Transação pela Conversa" (paridade total) → RF-01..RF-32.
  - Declaração e RN-01..RN-12 da US mapeiam integralmente para os RFs correspondentes: alvo (RF-01..04), paridade (RF-05..08), coleta (RF-09..15), confirmação (RF-16..19), gravação/concorrência/idempotência (RF-20..23), cartão (RF-24), anti-simulação/formatação (RF-25..26), orçamento (RF-27..30), roteamento/auditoria (RF-31..32).

## Apêndice A — User Story de origem

- Arquivo: `docs/us/2026-07-10-us-editar-transacao-conversacional.md` (validada por `scripts/validar-historias-usuario.py`, exit=0).
- A US contém a matriz completa de possibilidades de conversação com textos verbatim e citações `arquivo:linha`, base de rastreabilidade deste PRD.

## Apêndice B — Suposições registradas na conversão

- As mensagens de confirmação e de esclarecimento reusam os textos verbatim já existentes no fluxo de criação/confirmação; nenhum texto de usuário foi inventado. Onde a mensagem específica de edição ainda não existe, o alvo é reusar o texto equivalente já implementado, a ser fixado na especificação técnica.
- O comportamento de cutoff do orçamento em competências passadas segue o já implementado em `UpsertExpense`; este PRD não redefine a política de fechamento, apenas exige respeitá-la.
