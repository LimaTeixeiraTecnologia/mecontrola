# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Guarda de kind income↔expense com reclassificação antes do write
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Time de plataforma MeControla
- **Relacionados:** PRD (RF-06..RF-09), techspec.md, ADR-002 (propagação de erro)

## Contexto

O gate de escrita do módulo transactions rejeita incompatibilidade de kind via `ErrKindMismatch`
(`internal/categories/application/usecases/resolve_category_for_write.go`), invocado pelo pending
workflow em `validateCategoryForWrite` (`pending_entry_workflow.go:413-442`, chamando `ResolveForWrite`).
Hoje, se um income for resolvido para uma categoria expense (ex.: salário parqueado em "Metas", kind
expense), a validação falha e — pelo swallow (ADR-002) — o registro é cancelado silenciosamente.

Natureza (fiel à US): esta guarda é **defensiva**, não a causa-raiz provada do incidente (o income
sequer chegou ao passo de escrita — não há série `agents_write_total{register_income}`). A causa-raiz
é a observabilidade (ADR-002). Ainda assim, a guarda reduz uma classe de falha e evita `usecaseError`
silencioso por kind incompatível.

`Kind` já é tipo fechado (`valueobjects.Kind`: `KindIncome=1`, `KindExpense=2`). A resolução por
dicionário/candidatos vive em `category_resolution.go` e `candidate_resolver.go`; o pending state
guarda `state.Kind` e `state.Candidates`.

## Decisão

Ao detectar incompatibilidade de kind na resolução de categoria (candidato com kind ≠ `state.Kind`),
o fluxo **reclassifica usando o kind correto antes de iniciar o pending write**, em vez de prosseguir
e falhar:

1. Na resolução de candidatos, filtrar/priorizar candidatos cujo kind == `state.Kind` (o kind vem da
   `direction` do lançamento: income/expense).
2. Se após o filtro houver candidato compatível, seguir com ele para o pending write.
3. Se **não** houver categoria compatível de kind, o agente pede esclarecimento de categoria **uma
   única vez** (slot `AwaitingSlotCategory`), sem gerar `usecaseError` — o run não termina como erro
   silencioso (integra ADR-002: pedir clarify é `Completed`, não `Failed`).
4. O gate `ErrKindMismatch` do módulo transactions permanece como **defesa final** — se algo escapar,
   ele barra e agora o erro é propagado (ADR-002), não engolido.

A regra de negócio (comparação de kind) permanece na camada de resolução/decisão do consumidor, não
em adapter nem no kernel (R-ADAPTER-001, R-WF-KERNEL-001). Nenhum `switch case intent.Kind` de
roteamento é introduzido (R-AGENT-WF-001.1).

## Alternativas Consideradas

1. **Só confiar no `ErrKindMismatch` e propagar o erro** — Parcialmente adotada como defesa final,
   mas insuficiente sozinha: produziria clarify/erro onde uma reclassificação por kind resolveria
   automaticamente (ex.: salário → folha income correta), piorando a UX.
2. **Reclassificar no gate de transactions** — Descartada: o gate é defesa final e não deve conter
   lógica de recuperação; a reclassificação pertence à resolução de categoria do consumidor.
3. **Ignorar kind na resolução** — Descartada: permitiria income em categoria expense, corrompendo
   relatórios (o problema original).

## Consequências

### Benefícios Esperados

- Receita nunca gravada em categoria de despesa e vice-versa (RF-06); reclassificação automática
  evita clarify desnecessário (RF-07).
- Sem `usecaseError` silencioso por kind (RF-08), agora também coberto por ADR-002.
- Defesa em profundidade: `ErrKindMismatch` como última barreira (RF-09).

### Trade-offs e Custos

- A resolução de candidatos passa a considerar kind explicitamente; leve aumento de complexidade na
  seleção de candidato.

### Riscos e Mitigações

- **Risco:** filtro por kind eliminar todos os candidatos e cair em clarify com frequência.
  **Mitigação:** o seed de salário (ADR-001) cobre o caso mais comum; clarify único é aceitável e
  não é erro.

## Plano de Implementação

1. Ajustar a resolução de candidatos (`category_resolution.go`) para priorizar/filtrar por
   `state.Kind`.
2. Em `validateCategoryForWrite`, tratar ausência de candidato compatível como clarify único
   (`AwaitingSlotCategory`), não como erro.
3. Manter `ResolveForWrite`/`ErrKindMismatch` como defesa final, com erro propagado (ADR-002).
4. Testes: income com candidato expense ⇒ reclassifica para income; sem candidato income ⇒ clarify
   único, run não-failed.

## Monitoramento e Validação

- Critérios de aceite: "Receita nunca é gravada em categoria de despesa" e "Incompatibilidade de kind
  sem categoria compatível não gera erro silencioso".

## Impacto em Documentação e Operação

- Runbook de agents: nota sobre reclassificação por kind e clarify único.

## Revisão Futura

- Revisitar se novos kinds além de income/expense forem introduzidos (hoje fechado em dois).
