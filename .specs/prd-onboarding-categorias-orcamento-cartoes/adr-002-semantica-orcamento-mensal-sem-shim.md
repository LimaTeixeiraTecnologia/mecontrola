# Registro de Decisão Arquitetural (ADR-002)

## Metadados

- **Título:** Renomeação semântica renda líquida → orçamento mensal no estado, sem shim de compatibilidade
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Autor da feature, usuário (decisões D-04, D-06)
- **Relacionados:** PRD (RF-13, RF-14, RF-35, RF-43), techspec.md, ADR-001

## Contexto

O onboarding pergunta "renda mensal líquida" (`onboarding_workflow.go:464` `incomePrompt`;
`:466` `incomeReprompt`) e persiste o valor no campo `IncomeCents` (`:156`), usado como total
planejado do orçamento (`:779` `TotalCents: state.IncomeCents`) e exibido no resumo como "Renda
mensal" (`:524`). A US exige que o modelo de estado, prompts, erros, resumo e WorkingMemory usem a
semântica de **orçamento mensal**, nunca renda líquida.

O estado é serializado em JSON no snapshot do kernel; o resume aplica merge-patch com
`{"resumeText": msg}` (`resolve_onboarding_or_agent.go:143`; `engine.go:249`). Renomear a chave JSON
`incomeCents`→`monthlyBudgetCents` faz um snapshot antigo (onboarding em andamento no deploy) decodar
sem o valor no campo novo.

## Decisão

Renomear o campo de estado `IncomeCents`→`MonthlyBudgetCents` (tag JSON `monthlyBudgetCents`), a
função pura `DecideIncomeCents`→`DecideMonthlyBudgetCents` (mesma regra `amountBRL > 0`), o schema
`income_extract`→`monthly_budget_extract`, e todos os textos (prompts, reprompt, resumo, WorkingMemory)
para orçamento mensal. **Não** implementar shim de compatibilidade para snapshots antigos (D-06):
aceita-se que um onboarding em andamento que já passou da etapa de valor re-pergunte o orçamento uma
vez. O campo `ResumeText` (tag `resumeText`) é preservado — o rename não o afeta.

## Alternativas Consideradas

- **Shim de compatibilidade (ler `incomeCents` uma vez).** Vantagem: zero re-pergunta para in-flight.
  Desvantagem: código de migração para um caso de volume desprezível. Rejeitada (D-06).
- **Manter tag JSON antiga, renomear só o campo Go e os textos.** Vantagem: zero risco de snapshot.
  Desvantagem: campo Go `MonthlyBudgetCents` com tag `incomeCents` gera incoerência semântica
  permanente no estado persistido. Rejeitada: a US exige semântica de estado completa (RF-35).

## Consequências

### Benefícios Esperados

- Semântica de orçamento mensal coerente em estado, contratos internos e mensagens (M-02, RF-35).
- Sem dívida de nomenclatura no snapshot.

### Trade-offs e Custos

- Onboardings raros em andamento no deploy podem re-perguntar o orçamento uma vez (RF-43).

### Riscos e Mitigações

- **Risco:** decode de snapshot antigo falhar. **Mitigação:** `encoding/json` ignora chaves
  desconhecidas e deixa o campo novo em zero value; o step de orçamento então re-pergunta (fluxo
  normal de `MonthlyBudgetCents==0`). Nenhum orçamento ativado é afetado.
- **Rollback:** reverter os identificadores e as strings.

## Plano de Implementação

1. Renomear campo, função pura, schema e strings.
2. Atualizar `TotalCents` no draft (`:779`) para `state.MonthlyBudgetCents`.
3. Atualizar testes (`DecideIncomeCents`→`DecideMonthlyBudgetCents`, `:242`) e o gate M-02.

Concluído quando: nenhum símbolo/String referencia renda; grep por "renda líquida"/"renda mensal"
retorna vazio nos prompts/erros/resumo/WM.

## Monitoramento e Validação

- Teste M-02 (varredura de termos) verde.
- Ausência de regressão em `workflow_runs_total{status="failed"}` do onboarding pós-deploy.

## Impacto em Documentação e Operação

- Atualizar exemplos de diálogo do onboarding.
- Sem migration.

## Revisão Futura

Revisar se o produto reintroduzir coleta de renda como dado separado (fora de escopo atual).
