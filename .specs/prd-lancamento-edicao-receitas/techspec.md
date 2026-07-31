<!-- spec-hash-prd: 67567b60f7514c1b2c7f9e59399d98775b767be1fdc4b09e1777cd17f77da09b -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Lançamento e Edição de Receitas em Linguagem Natural

## Resumo Executivo

A capacidade de registrar e editar receitas já existe no consumidor `internal/agents`: a ferramenta `register_income` inicia o `transaction_write_workflow`, que resolve categoria/subcategoria (inclusive clarify), aplica o gate de confirmação HITL e persiste de forma idempotente; a ferramenta `edit_entry` cobre edição com busca por valor/termo e confirmação. A compreensão semântica ampla (as nove famílias de intenção de receita) e o valor por-extenso são responsabilidade do LLM (arquitetura LLM-first já vigente), provados por golden real-LLM.

O trabalho **novo** é cirúrgico e concentrado em três frentes: (1) estender a função pura `ParseInputDate` para resolver deterministicamente `semana passada`, `mês passado` e `dia N`, eliminando o fallback silencioso para o dia corrente (ADR-001); (2) exibir condicionalmente a linha `📅 Data` no bloco de confirmação de receita (ADR-002); (3) ampliar a suíte golden com cobertura por grupo de intenção mais casos-armadilha e formas de valor/data (ADR-003). O contrato de data entre LLM e tool passa a ser explícito: o LLM envia a expressão verbatim do usuário em `occurredAt` e a função pura (que conhece `now`) faz a aritmética de calendário — nunca o LLM.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes **modificados**:

- `internal/agents/application/workflows/write_shared.go` — função pura `ParseInputDate(text, now)` estendida com `semana passada`, `mês passado`, `dia N`; novos helpers puros `resolveSameDayPreviousMonth` e `resolveMostRecentDayOfMonth`. Consumida por `resolveEntryDate` (register_entry.go:111) no caminho de escrita e edição.
- `internal/agents/application/messages/catalog.go` — `IncomeConfirmationBlock` passa a renderizar a linha `📅 Data` quando `ConfirmationView.DateFormatted` não é vazio.
- `internal/agents/application/workflows/transaction_write_workflow.go` — `confirmSummaryIncome` passa a preencher `DateFormatted` via `formatConfirmDate(state.OccurredAt, now)` (espelha `confirmSummaryExpense`, :1060).
- `internal/agents/application/tools/register_income.go` e `internal/agents/application/tools/edit_entry.go` — a propriedade `occurredAt` do schema ganha `description` instruindo o LLM a enviar a expressão de data verbatim do usuário, sem computar data.

Componentes **novos**:

- `internal/agents/application/golden/cases_income_natural_language.go` — casos golden por grupo de intenção (nove grupos), por-extenso (simples e composto), gíria e datas novas, além de casos de edição de receita; registrados no `registry.go`.

Componentes **confirmados sem alteração** (comportamento já existente, coberto por golden):

- Máquina de slots do `transaction_write_workflow` (`TransactionAwaitingCategory`/`TransactionAwaitingConfirmation`/`TransactionAwaitingDate`, :1084-1106) — clarify de categoria de receita e gate de confirmação já operam para income.
- `register_income`/`RegisterIncomeCommand` e `transaction_write_starter` — idempotência por `wamid`/`itemSeq` já vigente.
- Parser de valor `reMoney` (write_shared.go:97) — numérico, separador BR e gíria (`contos?|pilas?|mangos?`) já suportados; por-extenso permanece resolvido pelo LLM.

### Fluxo de Dados (lançamento)

```
mensagem WhatsApp
  → AgentRuntime.Execute (Thread → Run)
  → LLM decide register_income(amountCents, description, occurredAt=<expressão verbatim>, ...)
  → register_income.exec → RegisterIncomeCommand
  → transaction_write_starter → Engine.Start(transaction_write_workflow)
      → resolução de categoria/subcategoria (clarify se necessário)
      → resolveEntryDate(occurredAt) → ParseInputDate(occurredAt, now)   [NOVO: semana passada / mês passado / dia N]
      → slot de confirmação → IncomeConfirmationBlock(Valor/Origem[/Data])  [NOVO: linha Data condicional]
      → "sim" → persistência idempotente → WriteSuccess(income)
```

## Design de Implementação

### Interfaces Chave

`ParseInputDate` mantém a assinatura pura atual (sem `context.Context`, recebe `now` — sem abstração de tempo):

```go
func ParseInputDate(text string, now time.Time) string
```

Novos helpers puros no mesmo pacote `workflows` (zero comentários, receiver-less, determinísticos):

```go
func resolveSameDayPreviousMonth(now time.Time) string
func resolveMostRecentDayOfMonth(n int, now time.Time) (string, bool)
func daysInMonth(year int, month time.Month) int
```

Semântica (ADR-001):

- `semana passada` → `now.AddDate(0, 0, -7)`.
- `mês passado` / `mes passado` → `resolveSameDayPreviousMonth`: mesmo dia-do-mês do mês anterior; se o dia não existir no mês anterior (ex.: 31/03 → fevereiro), usa o último dia válido do mês anterior.
- `dia N` (regex `^\s*dia\s+([0-9]{1,2})\s*$`, N em 1..31) → `resolveMostRecentDayOfMonth`: varre até 12 meses a partir do mês corrente e retorna a data mais recente com dia-do-mês N que não seja futura (`!candidate.After(now)`); `dia 31` num mês de 30 dias cai no mês anterior com 31 dias.

Ordem de avaliação em `ParseInputDate`: manter os casos atuais (`hoje`/`ontem`/`anteontem`, `parseWeekday`, `DD/MM`, ISO) e inserir os novos ramos antes do `return ""` final, garantindo que `parseWeekday` continue tratando `sexta passada` (weekday + sufixo `passada`) e que `semana passada`/`mes passado` — que hoje retornam `false` de `parseWeekday` — sejam capturados pelos novos ramos.

### Modelos de Dados

Sem novo schema de banco. `ConfirmationView` (catalog.go:104) já possui o campo `DateFormatted`; nenhuma coluna nova. `TransactionWriteState.OccurredAt` já carrega a data resolvida (string `AAAA-MM-DD`).

Alteração de mensagem (catalog.go):

```go
func IncomeConfirmationBlock(v ConfirmationView) string {
    if v.DateFormatted != "" {
        return fmt.Sprintf("💰 Valor: %s\n📥 Origem: %s\n📅 Data: %s\n\nPosso registrar?",
            v.AmountFormatted, v.Origin, v.DateFormatted)
    }
    return fmt.Sprintf("💰 Valor: %s\n📥 Origem: %s\n\nPosso registrar?",
        v.AmountFormatted, v.Origin)
}
```

`formatConfirmDate` (transaction_write_workflow.go:1066) já retorna `""` quando a data é vazia ou igual ao dia corrente, satisfazendo D1 ("linha Data só quando informada/derivada e diferente de hoje").

### Endpoints de API

Não se aplica. A superfície é o inbound WhatsApp já existente; nenhum endpoint HTTP novo.

## Pontos de Integração

- LLM via OpenRouter (provider único) para compreensão de intenção, valor por-extenso e extração da expressão de data verbatim. Sem fallback chain; comportamento já vigente.
- WhatsApp (Meta) como canal inbound; sem mudança de contrato.

## Abordagem de Testes

### Testes Unitários

- `ParseInputDate` (novo/estendido): tabela cobrindo `semana passada`, `mês passado` (incluindo clamp 31/03→28-29/02), `dia N` (caso comum, `dia 31` em mês de 30 dias, N futuro caindo no mês anterior, N inválido >31), e não-regressão das formas atuais (`hoje`/`ontem`/`anteontem`/`sexta`/`DD/MM`/ISO). Função pura, sem mock, `now` fixo injetado.
- `IncomeConfirmationBlock`: com e sem `DateFormatted` (linha `📅 Data` presente só quando informado).
- `resolveMostRecentDayOfMonth`/`resolveSameDayPreviousMonth`/`daysInMonth`: casos de fronteira de calendário (ano bissexto, viradas de mês/ano).

### Testes de Integração

Não são necessários testes de integração novos. Critérios avaliados: (a) as fronteiras de IO (persistência do workflow, resume por snapshot) já têm cobertura de integração em `onboarding_workflow_postgres_resume_integration_test.go` e afins; (b) a lógica nova é pura (datas) ou de formatação (mensagem), sem IO; (c) não há nova fronteira de banco/fila. Portanto o risco incremental é coberto por unit + golden real-LLM.

### Testes E2E

Gate golden real-LLM (RF-25, ADR-003), executado com `RUN_REAL_LLM=1` e credenciais OpenRouter:

- ≥ 1 caso por grupo de intenção de receita (nove grupos): recebimentos, trabalho, autônomos, comércio, produção artesanal, comissões/bônus, salário, aluguéis, investimentos.
- Formas de valor: por-extenso simples (`cem`, `dois mil`) e composto (`dois mil e quinhentos`); gíria (`100 pila`, `500 conto`).
- Datas novas: `semana passada`, `mês passado`, `dia 10` — assertando data resolvida ≠ dia corrente e coerente com a semântica.
- Edição de receita: `corrige aquela receita`, `era 150`, `na verdade foi 180`, `muda a data / foi semana passada`.
- Casos-armadilha derivados de produção (falso múltiplo, paráfrase de origem) preservados.
- Limiar: taxa ≥ 0,90 por grupo, 0 falso-sucesso (nenhum verde sem a tool/args corretos).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. `ParseInputDate` + helpers puros + unit tests (fundação determinística; sem dependências). Motivo: base de correção de data que edição e lançamento compartilham.
2. `IncomeConfirmationBlock` (linha Data condicional) + `confirmSummaryIncome` (wiring `DateFormatted`) + unit tests. Depende só de (1) para exibir a data resolvida.
3. Descrições de `occurredAt` em `register_income` e `edit_entry` (contrato verbatim). Depende de (1) para o token verbatim ter resolvedor.
4. `cases_income_natural_language.go` + registro no `registry.go` + gate real-LLM. Depende de (1)-(3) para provar o comportamento fim-a-fim.

### Dependências Técnicas

- Nenhuma infraestrutura nova. Execução do gate real-LLM exige credenciais OpenRouter em ambiente de teste (`RUN_REAL_LLM=1`).

## Monitoramento e Observabilidade

- Reuso das métricas de Run/tool já existentes (RunStatus, tool outcome). Nenhum label novo; cardinalidade controlada preservada (RF-27): proibido `user_id`/`category_id` como label.
- Sem novo dashboard; a saúde do fluxo de receita já é observável pelo Run auditável (thread_id/run_id/status/duration).

## Considerações Técnicas

### Decisões Chave

- ADR-001 — Resolução determinística de data por extensão da função pura `ParseInputDate` + contrato verbatim LLM→tool (`.specs/prd-lancamento-edicao-receitas/adr-001-resolucao-deterministica-data.md`).
- ADR-002 — Linha `📅 Data` condicional no bloco de confirmação de receita (`.specs/prd-lancamento-edicao-receitas/adr-002-confirmacao-receita-data-condicional.md`).
- ADR-003 — Gate de aceite golden por grupo de intenção mais armadilhas (`.specs/prd-lancamento-edicao-receitas/adr-003-gate-golden-por-grupo.md`).

Decisão de escopo (sem ADR próprio, já fixada no PRD): receita **não** captura forma de pagamento (RF-15); nenhuma mudança em `register_income` (sem campo `paymentMethod`) nem em `IncomeConfirmationBlock` (sem linha de pagamento).

### Riscos Conhecidos

- Risco: o LLM computar a data em ISO em vez de enviar a expressão verbatim, contornando `ParseInputDate`. Mitigação: descrição explícita da propriedade `occurredAt` + casos golden que asseguram a resolução correta das datas relativas.
- Risco: colisão de normalização entre `parseWeekday` (sufixo `passada`/`passado`) e os novos ramos `semana passada`/`mês passado`. Mitigação: `parseWeekday` retorna `false` para `semana`/`mes` (não são weekdays), e os novos ramos usam correspondência exata da expressão normalizada; unit tests cobrem ambos.
- Risco: `dia N` ambíguo com valores crus. Mitigação: o ramo `dia N` exige o prefixo literal `dia` (regex ancorada), não captura número solto.

### Conformidade com Padrões

- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001): roteamento por registry/LLM; tools finas sem regra de domínio; estados fechados; LLM só nas call-sites sancionadas; Run auditável.
- `.claude/rules/go-adapters.md` (R-ADAPTER-001): zero comentários em Go de produção; adapters finos; sem SQL direto.
- `.claude/rules/transactions-workflows.md` (R-TXN-004): cardinalidade de métricas controlada.
- `.claude/rules/go-testing.md` (R-TESTING-001): suíte testify quando houver teste de use case; datas são função pura (testes de domínio puro, fora do escopo de suite).
- DMMF (`domain-modeling-production`): `ParseInputDate` e helpers são puros, determinísticos, recebem `now` (sem abstração de tempo, conforme decisão do projeto).

### Arquivos Relevantes e Dependentes

- `internal/agents/application/workflows/write_shared.go` (ParseInputDate, parseWeekday, reMoney)
- `internal/agents/application/usecases/register_entry.go` (resolveEntryDate:111; RegisterIncomeCommand)
- `internal/agents/application/workflows/transaction_write_workflow.go` (confirmSummaryIncome:1023; formatConfirmDate:1066; slots:1084)
- `internal/agents/application/messages/catalog.go` (IncomeConfirmationBlock:251; ConfirmationView:104; WriteSuccess:258)
- `internal/agents/application/tools/register_income.go` (schema occurredAt:18)
- `internal/agents/application/tools/edit_entry.go` (schema occurredAt:45)
- `internal/agents/application/golden/registry.go`, `cases_expense_income.go`, `cases_income_natural_language.go` (novo)
- `internal/agents/application/golden/harness_realllm_test.go` (gate real-LLM)
