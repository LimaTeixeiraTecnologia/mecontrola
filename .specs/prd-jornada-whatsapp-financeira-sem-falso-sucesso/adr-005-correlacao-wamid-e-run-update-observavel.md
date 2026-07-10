# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Correlação por WAMID na fronteira do runtime e erro de run.Update observável
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** time de plataforma / observabilidade
- **Relacionados:** PRD `prd-jornada-whatsapp-financeira-sem-falso-sucesso` (RF-13, RF-14, RF-15, RF-16, RF-22, RF-23, RF-24), `techspec.md`, US-001

## Contexto

Foram observados 4 registros em `platform_runs` do agente com `status = succeeded`, `outcome = routed` e `correlation_key` VAZIO. A hipótese inicial era perda do WAMID no caminho de roteamento.

A investigação refuta o bug vivo:

- `buildWhatsAppAgentRoute` (`module.go` L378-383) propaga `msg.WAMID` para o evento `agents.whatsapp.inbound.v1`.
- O consumer (`whatsapp_inbound_consumer.go`) exige `MessageID` não-vazio (L172) antes de `HandleInbound`.
- `HandleInbound` mapeia `MessageID` para `InboundRequest.MessageID`, e o runtime (`runtime.go` L114) define `CorrelationKey: in.MessageID`.
- O campo `CorrelationKey = in.MessageID` foi introduzido no commit `6539cfd`.

Portanto, os 4 runs vazios são DADOS LEGADOS pré-regressão, não defeito ativo no caminho routed atual. O `closeRun` central do runtime (`runtime.go` L336-346) JÁ trata o erro de `Update` (métrica `agent_run_update_errors_total` + log).

Lacunas reais remanescentes:

1. `InboundRequest.Validate()` (`ports.go` L73-88) NÃO valida `MessageID`. Qualquer novo caller com `MessageID = ""` cria um run com `correlation_key` vazio silenciosamente, antes de qualquer gate.
2. Os 3 continuers engolem o erro de `Update` no fechamento do run: `pending_entry_continuer.go` (`closeRun` L297-307, `_ = c.runs.Update(...)`), `card_create_confirm_continuer.go` (L167), `budget_creation_continuer.go` (L171). Uma falha de persistência do fechamento fica invisível.
3. DDL: `correlation_key TEXT NOT NULL DEFAULT ''` — sem `CHECK` de comprimento, o vazio é representável no banco.

Restrições: o run auditável é telemetria, não contrato de negócio; o resultado de negócio já foi entregue ao usuário antes do fechamento. Cardinalidade de métrica é controlada (R-TXN-004 / R-AGENT-WF-001.5): proibido `user_id`/`wamid`/`correlation_key` como label. Zero comentários em Go de produção (R-ADAPTER-001.1).

## Decisão

Tornar o run sem WAMID irrepresentável na fronteira única, tornar o erro de fechamento observável em todos os caminhos, e provar a concordância de estado por invariante testável.

1. **Defesa na fronteira única (DMMF — estado ilegal irrepresentável).** Adicionar a `InboundRequest.Validate()` (`internal/platform/agent`) a branch:

   ```go
   if i.MessageID == "" {
       errs = append(errs, fmt.Errorf("message_id: %w", ErrEmptyMessageID))
   }
   ```

   com sentinela nova `ErrEmptyMessageID` em `internal/platform/agent`. A validação falha ANTES de `runs.Insert`, logo nenhum run pode nascer com `correlation_key` vazio. Um único ponto de defesa cobre todos os callers presentes e futuros.

2. **Fechamento observável nos 3 continuers.** Substituir `_ = c.runs.Update(...)` por tratamento ÚNICO do erro (regra go-implementation: tratar o erro uma vez): log estruturado com `run_id`, `wamid`, `workflow`, `stage` + nova métrica `agents_run_update_errors_total` com labels FECHADOS `workflow`, `stage`, `status` (NUNCA `user_id`/`wamid`/`correlation_key` como label). O erro NÃO é propagado ao usuário — o run é telemetria e o resultado de negócio já foi entregue. Espelha o `closeRun` central do runtime.

3. **Concordância de estado (RF-16) por invariante testável.** Query de reconciliação read-only (adapter Postgres novo, ex. `audit_reconciliation.go`, ou usecase `ReconcileRunConsistency`) cruzando `platform_runs` × `workflow_runs` × `agents_write_ledger` × `transactions` × `platform_scorer_results` por `correlation_key`/`wamid`, verificando as invariantes:
   - `correlation_key != ''` para todo run;
   - run `succeeded`/`routed` ⇒ efeito no ledger OU workflow `succeeded`;
   - run `failed` ⇒ nenhum write órfão;
   - `wf_status` (kernel) e `run_status` (agent) não divergem.

   Materializar como TESTE de invariante de integração (0 violações), NÃO como dashboard novo.

4. **Constraint de comprimento (decisão firme, confirmada 2026-07-10).** Defesa em profundidade obrigatória: a mesma migration 000008 executa o backfill idempotente dos 4 runs legados `UPDATE mecontrola.platform_runs SET correlation_key = 'legacy:' || id::text WHERE correlation_key = ''` e, em seguida, `ALTER TABLE mecontrola.platform_runs ADD CONSTRAINT platform_runs_correlation_len_chk CHECK (char_length(correlation_key) BETWEEN 1 AND 256)` (validado, não `NOT VALID`). Assim a invariante "correlation_key nunca vazio" passa a ser garantida pelo banco para sempre, além da validação em Go — nenhum caminho futuro que insira `Run` fora do runtime pode reintroduzir o vazio.

5. **Padronização de campos (RF-22/23/24).** Padronizar campos de log/span `run_id`/`wamid`/`workflow`/`stage`/`status` no runtime central e nos 3 continuers; auditar por gate `grep` que nenhum label de métrica carrega dado sensível.

Escopo: `internal/platform/agent` (validação + sentinela), continuers de `internal/agents`, adapter/usecase de reconciliação, migration de `platform_runs`. Sem novo GoF pattern: validação de fronteira + tratamento de erro + query de auditoria.

## Alternativas Consideradas

- **(a) Só limpar os dados legados.** Descrição: `UPDATE`/`DELETE` dos 4 runs vazios e encerrar. Vantagem: mínimo esforço. Desvantagem: não previne recorrência — qualquer novo caller com `MessageID = ""` reintroduz o defeito e o erro de fechamento permanece invisível. Rejeitada por não fechar a causa estrutural.
- **(b) Propagar o erro de `Update` ao usuário.** Descrição: retornar erro de fechamento na resposta ao usuário. Vantagem: erro sempre visível. Desvantagem: o run auditável é telemetria; o resultado de negócio já foi entregue antes do fechamento, então propagar contaminaria o contrato de negócio com falha de observabilidade. Rejeitada: tratar uma vez com log + métrica preserva a separação telemetria vs. negócio.

## Consequências

### Benefícios Esperados

- Run sem WAMID torna-se irrepresentável: a falha ocorre na validação de fronteira, antes de `runs.Insert`, com sentinela tipada.
- Falha de fechamento deixa de ser silenciosa em todos os caminhos (runtime central + 3 continuers), com log estruturado e métrica de cardinalidade controlada.
- Concordância de estado (RF-16) passa a ser garantida por invariante testável, sem custo de dashboard novo.
- Constraint de banco fecha a última superfície onde o vazio era representável.

### Trade-offs e Custos

- Nova sentinela e branch de validação na fronteira do runtime.
- Nova métrica `agents_run_update_errors_total` a manter com labels fechados.
- Query de reconciliação com joins entre 5 tabelas, com colunas de join confirmadas no schema real (ver R3).
- Migration com backfill obrigatório antes do `CHECK`.

### Riscos e Mitigações

- **R1 — Migration com `CHECK` falha por dados legados.** Impacto: migration aborta em produção. Mitigação: executar o backfill (`SET correlation_key = 'legacy:' || id WHERE correlation_key = ''`) ANTES de adicionar o constraint, na mesma migration ou em migration anterior.
- **R2 — Validar `MessageID` quebra fixtures de teste.** Impacto: testes que constroem `InboundRequest` sem `MessageID` passam a falhar. Mitigação: varrer `*_test.go` e corrigir fixtures antes do merge.
- **R3 — Colunas de join (CONFIRMADO 2026-07-10, risco resolvido).** Verificado no `000001_initial_schema.up.sql`: `workflow_runs(correlation_key TEXT, status TEXT [running|suspended|succeeded|failed], state JSONB → state->>'status')`; `agents_write_ledger(wamid TEXT, item_seq, operation, resource_id UUID)` join `wamid = platform_runs.correlation_key`; `platform_scorer_results(run_id UUID FK → platform_runs.id, scorer_id, score)`; `transactions` via `agents_write_ledger.resource_id = transactions.id`; `platform_runs(correlation_key TEXT DEFAULT '', status [running|succeeded|failed], outcome TEXT)`. A query de reconciliação usa esses nomes; sem suposição pendente.
- **R4 — Conjunto de workflow IDs não fechado.** Impacto: cardinalidade de label `workflow` cresce sem controle. Mitigação: manter os workflow IDs como conjunto fechado (tipo/enumeração), rejeitando string livre no label.
- **R5 — Status do kernel vs. status do agent são tipos distintos.** Impacto: comparação de `wf_status` × `run_status` na reconciliação pode divergir por incompatibilidade de representação. Mitigação: mapear `String()` de cada estado de forma consistente e comparar por mapeamento explícito, nunca por igualdade crua de tipos distintos.

## Plano de Implementação

1. Adicionar sentinela `ErrEmptyMessageID` e a branch de `MessageID == ""` em `InboundRequest.Validate()` (`internal/platform/agent/ports.go`); varrer e corrigir fixtures `*_test.go` (R2).
2. Substituir `_ = c.runs.Update(...)` por tratamento único (log estruturado + `agents_run_update_errors_total`) nos 3 continuers (`pending_entry_continuer.go`, `card_create_confirm_continuer.go`, `budget_creation_continuer.go`), espelhando o `closeRun` central.
3. Confirmar colunas de join (R3) e implementar a query/usecase de reconciliação read-only; materializar como teste de invariante de integração (0 violações).
4. Escrever migration: backfill dos 4 runs legados seguido de `ADD CONSTRAINT correlation_len_chk` (R1).
5. Padronizar campos de log/span `run_id`/`wamid`/`workflow`/`stage`/`status` (RF-22/23/24) e rodar o gate `grep` de labels sensíveis.

Dependências: passo 4 depende do backfill; passo 3 depende da confirmação de esquema (R3). Sequência recomendada: 1 → 2 → 5 → 3 → 4.

Adoção concluída quando: `Validate()` rejeita `MessageID` vazio; os 3 continuers emitem log + métrica no erro de fechamento; teste de invariante de integração passa com 0 violações; migration aplicada com constraint ativo e 0 runs com `correlation_key` vazio.

## Monitoramento e Validação

- Métrica `agents_run_update_errors_total` (labels `workflow`, `stage`, `status`) acompanhada em produção; qualquer incremento sustentado indica falha de persistência de fechamento.
- Teste: run com `outcome = routed` DEVE ter `correlation_key` não-vazio.
- Teste: falha simulada de `Update` incrementa a métrica e emite o log estruturado, sem silenciar e sem propagar ao usuário.
- Teste de invariante de reconciliação: 0 violações das 4 invariantes de RF-16.
- Gate `grep`: nenhum label de métrica no runtime/continuers usa `user_id`/`wamid`/`correlation_key`.

Critérios de sucesso: 0 novos runs com `correlation_key` vazio; métrica em 0 em regime normal; teste de invariante verde em CI. Critério para revisar: incremento recorrente de `agents_run_update_errors_total` ou aparecimento de nova violação de invariante.

Rollback: reverter a migration com `DROP CONSTRAINT correlation_len_chk` (o backfill é idempotente e não precisa ser desfeito); reverter as mudanças de código por revert dos commits — a validação e o tratamento de erro são aditivos e não alteram contrato de negócio.

## Impacto em Documentação e Operação

- Documentação técnica: `techspec.md` referenciando RF-13..RF-16 e RF-22..RF-24.
- Runbook do agente: acrescentar `agents_run_update_errors_total` como sinal de falha de fechamento.
- Observabilidade: registrar os campos padronizados `run_id`/`wamid`/`workflow`/`stage`/`status` e o gate de labels sensíveis.
- Migrations de `platform_runs`: documentar o backfill obrigatório antes do `CHECK`.

## Revisão Futura

Revisitar quando: um consumidor reintroduzir HITL/estado de espera que exija novo caminho de fechamento; o conjunto de workflow IDs mudar (revalidar R4); ou o teste de invariante de reconciliação passar a exigir novas tabelas/joins (revalidar R3). Sem data fixa — dirigida por evento.
