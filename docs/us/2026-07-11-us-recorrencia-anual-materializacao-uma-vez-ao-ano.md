# Recorrência Anual Materializa Uma Vez por Ano — User Story

> Objetivo: honrar a frequência `yearly` já aceita e persistida pelos templates recorrentes, de modo que uma recorrência anual gere o lançamento apenas uma vez ao ano (no mês/dia de início), e não a cada mês.
> Escopo confirmado com o solicitante: **corrigir a materialização da frequência anual**; a limpeza de lançamentos duplicados já gerados em produção fica fora de escopo.
> Âncora anual confirmada com o solicitante: **mês + dia de `started_at`** (reaproveita campos já persistidos, sem novo campo nem migração de schema).
> Data de geração: 2026-07-11
> Nome do arquivo: `2026-07-11-us-recorrencia-anual-materializacao-uma-vez-ao-ano.md`
> Base: `internal/transactions` (domínio de recorrência e materialização), consumida por REST (`/api/v1/recurring-templates`) e pelo agente WhatsApp (`internal/agents/application/tools/create_recurrence.go`).

---

## Declaração
Como pessoa que controla suas finanças pelo MeControla e cadastra uma despesa recorrente anual (por exemplo IPVA, IPTU, seguro ou anuidade), quero que a recorrência anual gere o lançamento apenas uma vez ao ano no mês em que começou, para que meu resumo mensal, meu orçamento e minha fatura reflitam o valor real sem lançamentos-fantasma repetidos a cada mês.

## Contexto
- Problema: a frequência de um template recorrente é um estado fechado com dois valores — `FrequencyMonthly` e `FrequencyYearly` (`internal/transactions/domain/valueobjects/frequency.go:12-15`). O valor `yearly` é aceito na entrada, validado e persistido, porém a decisão de materialização nunca lê a frequência. Resultado: um template anual materializa em cada mês cujo dia coincide com o `day_of_month`, gerando aproximadamente doze lançamentos por ano em vez de um.
- Resultado esperado: um template com `frequency = yearly` materializa somente quando o mês corrente (em `America/Sao_Paulo`) é igual ao mês de `started_at` e o dia é igual ao `day_of_month`; nos demais meses, não materializa. Templates `monthly` mantêm o comportamento atual, materializando mensalmente no `day_of_month`.
- Fonte: análise do módulo `internal/transactions` solicitada pelo usuário em 2026-07-11, com foco em lacunas de produção usando as skills `go-implementation`, `domain-modeling-production`, `design-patterns-mandatory` e `user-stories`.

## Regras de Negócio
- RN-01: Para `frequency = yearly`, a materialização ocorre apenas quando `mês corrente (America/Sao_Paulo) == started_at.Month()` E `dia corrente == day_of_month`. O ano corrente sempre satisfaz a âncora anual, respeitada a janela `started_at`/`ended_at`.
- RN-02: Para `frequency = monthly`, o comportamento permanece inalterado: materializa em cada mês, no dia igual ao `day_of_month`, dentro da janela `started_at`/`ended_at` e enquanto `deleted_at` for nulo.
- RN-03: A âncora anual deriva exclusivamente de `started_at` (mês) e `day_of_month`, campos já presentes no agregado (`internal/transactions/domain/entities/recurring_template.go:29,31`). Nenhum campo novo, nenhuma migração de schema e nenhuma alteração na consulta `FindActiveByDayOfMonth` são necessários.
- RN-04: A decisão continua sendo uma função pura de domínio, sem IO, sem `context.Context` e determinística, recebendo `now`/`loc` como parâmetros (R-TXN-001). A frequência passa a ser lida de `template.Frequency()` dentro do `Decide*`.
- RN-05: A garantia de exatamente-uma-materialização por par `(template, ref_month)`, hoje sustentada por advisory lock e `InsertIfAbsent` (`internal/transactions/application/usecases/materialize_recurring_for_day.go:110-161`), não pode regredir.
- RN-06: A janela temporal existente permanece respeitada: não materializa antes de `started_at` nem depois de `ended_at`, nem para template com `deleted_at` preenchido.
- RN-07: Após a correção, `FrequencyYearly` deixa de ser um estado representável sem comportamento; o modelo passa a tornar o estado ilegal (anual tratado como mensal) irrepresentável no fluxo de materialização.

## Critérios de Aceite
```gherkin
Cenário: Template anual materializa uma vez no mês-âncora
  Dado um template recorrente ativo com frequency igual a yearly, started_at em 10 de janeiro e day_of_month igual a 10
  Quando o job de materialização executa em 10 de janeiro do ano corrente no fuso America/Sao_Paulo
  Então um único lançamento é criado para aquele template naquele mês de referência

Cenário: Template anual não materializa em meses fora da âncora
  Dado o mesmo template anual com started_at em janeiro e day_of_month igual a 10
  Quando o job de materialização executa em 10 de fevereiro, 10 de março e 10 de qualquer mês diferente de janeiro
  Então nenhum lançamento é criado para aquele template nesses meses

Cenário: Template mensal preserva a materialização a cada mês
  Dado um template recorrente ativo com frequency igual a monthly e day_of_month igual a 10
  Quando o job de materialização executa em 10 de janeiro e 10 de fevereiro
  Então um lançamento é criado em cada um desses meses, mantendo o comportamento atual

Cenário: Template anual no mês-âncora, porém em dia diferente do day_of_month
  Dado um template anual com started_at em janeiro e day_of_month igual a 10
  Quando o job de materialização executa em 11 de janeiro
  Então nenhum lançamento é criado, porque o dia corrente difere do day_of_month

Cenário: Reexecução idempotente no mesmo dia-âncora não duplica
  Dado um template anual que já materializou em 10 de janeiro do ano corrente
  Quando o job de materialização executa novamente em 10 de janeiro do mesmo ano
  Então nenhum lançamento adicional é criado para aquele par de template e mês de referência
```

## Dados e Permissões
- Dados obrigatórios (já persistidos no agregado, sem novos campos): `frequency`, `started_at`, `day_of_month`, `ended_at`, `deleted_at` (`internal/transactions/domain/entities/recurring_template.go:28-34`).
- Fonte de verdade da frequência na entrada: `ParseFrequency` aceitando os literais `monthly` e `yearly` (`internal/transactions/domain/valueobjects/frequency.go:17-26`), acionada tanto pela criação REST quanto pela ferramenta do agente `create_recurrence` (`internal/agents/application/tools/create_recurrence.go:130`).
- Perfis/permissões: operação executada pelo job de plataforma sob a identidade do dono do template, via `auth.WithPrincipal(ctx, auth.Principal{UserID: template.UserID().UUID()})` (`internal/transactions/application/usecases/materialize_recurring_for_day.go:106`). O isolamento por usuário existente é preservado; nenhuma nova permissão é introduzida.

## Dependências
- Sem dependência bloqueante externa: a correção vive na função pura `DecideMaterializeForDay` (`internal/transactions/domain/services/recurring_workflow.go:23-61`), que já recebe o `template` completo, o `today` e o `loc`. A consulta `FindActiveByDayOfMonth` (`internal/transactions/infrastructure/repositories/postgres/recurring_template_repository.go:284-322`) permanece como está.
- Governança de domínio aplicável e já existente no repositório: R-TXN-001 e R-TXN-WORKFLOWS-001 (`.claude/rules/transactions-workflows.md`) exigem que a regra de negócio permaneça exclusivamente no `Decide*`.

## Fora de Escopo
- Remediação, reversão ou soft-delete dos lançamentos mensais já gerados indevidamente por templates anuais existentes em produção (registrada como risco/dependência separada abaixo, em Notas de Validação).
- Introdução de novas frequências além de `monthly` e `yearly` (por exemplo semanal, quinzenal, trimestral).
- Criação de um novo campo `month_of_year` no agregado, DTOs, comando e schema — descartado em favor da âncora `started_at`.
- Pausa, retomada ou pulo de uma única ocorrência de template recorrente.
- Alterações no fluxo de fatura de cartão além do reflexo natural de deixar de criar parcelas mensais fantasmas.

## Evidências
- Entrada: pedido do usuário em 2026-07-11 para analisar `internal/transactions`, identificar uma lacuna real de produção e produzir uma única história de usuário; decisões de âncora anual e de exclusão da remediação confirmadas por múltipla escolha na mesma data.
- Base de código:
  - `internal/transactions/domain/valueobjects/frequency.go:12-26` — enum fechado com `FrequencyMonthly` e `FrequencyYearly`; `ParseFrequency` aceita `monthly` e `yearly`.
  - `internal/transactions/domain/services/recurring_workflow.go:23-61` — `DecideMaterializeForDay` decide a materialização checando apenas `deleted_at`, janela `started_at`/`ended_at` e igualdade de dia (`dayInLoc != template.DayOfMonth().Value()`); nunca referencia `template.Frequency()`.
  - `internal/transactions/infrastructure/repositories/postgres/recurring_template_repository.go:298-301` — a consulta filtra por `day_of_month`, `deleted_at IS NULL` e janela de datas; não filtra por frequência nem por mês, portanto não compensa a omissão do `Decide*`.
  - `internal/transactions/domain/commands/create_recurring_template.go:73-76,134` — a frequência é parseada e gravada no comando de criação, confirmando que `yearly` é um estado alcançável e persistido.
  - `internal/transactions/application/dtos/output/recurring_template.go:23,44` — a frequência é devolvida na saída, confirmando que o valor `yearly` é visível ao cliente.
  - `internal/agents/application/tools/create_recurrence.go:25,52,130` — a ferramenta do agente WhatsApp repassa `frequency` como string livre para a criação, tornando o cenário anual acessível ao usuário final por conversa.
  - `internal/transactions/application/usecases/materialize_recurring_for_day.go:110-161` — a garantia de exatamente-uma-materialização por `(template, ref_month)` via advisory lock e `InsertIfAbsent` que a correção precisa preservar.
- Inferências: como `NewDayOfMonth` limita o dia a 1..28 (`internal/transactions/domain/valueobjects/day_of_month.go:15-20`), a âncora `started_at.Month()` combinada com `day_of_month` sempre encontra um dia válido em janeiro (mês de exemplo), então a correção puramente no `Decide*` é suficiente e não exige alteração de consulta nem migração.
- Não evidenciado: nenhum teste de materialização cobre o caso anual — a busca por `yearly`/`Yearly`/`FrequencyYearly` em `internal/transactions/domain/services/recurring_workflow_test.go` e `internal/transactions/application/usecases/materialize_recurring_for_day_test.go` não retornou ocorrências, confirmando a ausência de cobertura para o comportamento anual.

## Notas de Validação
- Cobertura de cenários: a história inclui fluxo feliz (materializa uma vez no mês-âncora), fluxos alternativos válidos (não materializa fora da âncora; mensal preservado) e fluxos de borda/erro (dia diferente do `day_of_month` no mês-âncora; reexecução idempotente sem duplicar).
- Risco residual e dependência de dados: templates anuais já existentes em produção podem ter gerado lançamentos mensais indevidos antes desta correção. A limpeza histórica é explicitamente fora de escopo desta história e deve ser tratada como item separado, pois envolve operação potencialmente destrutiva sobre dados existentes.
- Não regressão obrigatória: a correção não pode alterar o comportamento de templates mensais nem enfraquecer a idempotência por `(template, ref_month)`; ambos precisam de asserção nos testes.
- Conformidade de governança: a regra de negócio adicionada permanece dentro de `DecideMaterializeForDay` como função pura, aderente a R-TXN-001 e R-TXN-WORKFLOWS-001; nenhuma lógica de frequência deve vazar para use case, handler, consumer ou producer.
- Validação automatizada: `python3 .agents/skills/user-stories/scripts/validar-historias-usuario.py docs/us/2026-07-11-us-recorrencia-anual-materializacao-uma-vez-ao-ano.md` executado com resultado de sucesso.
</content>
</invoke>
