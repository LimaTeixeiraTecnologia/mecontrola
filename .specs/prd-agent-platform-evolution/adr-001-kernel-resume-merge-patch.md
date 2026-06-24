# Registro de Decisão Arquitetural (ADR-001)

## Metadados

- **Título:** Correção do resume do kernel via JSON merge-patch
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Solicitante (produto/eng) + plataforma
- **Relacionados:** `prd.md` (RF-09, RF-10), `techspec.md`, ADR-002, ADR-003,
  `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001), `prd-workflow-kernel/adr-003-suspend-resume-generalization.md`

## Contexto

`internal/platform/workflow/engine.go` (Resume, linhas ~166-177) hoje **substitui** o estado inteiro
do snapshot pelo payload de resume quando este decodifica sem erro:

```go
current, _ := e.codec.Decode(snap.State)        // estado suspenso completo
if len(resume) > 0 {
    rs, err := e.codec.Decode(resume)
    if err == nil { current = rs }              // SUBSTITUI tudo
}
```

O agent (`daily_ledger_agent.go continuePendingExpenseConfirmationKernel`) passa no resume apenas
`ExpenseState{UserID, Channel, ResumeText}`. Como esse JSON parcial decodifica com sucesso, o estado
suspenso rico (`Candidates`, `AwaitingKind`, `AmountCents`, `Merchant`, ...) seria **perdido** no
resume. Hoje o defeito está **latente**: o caminho de escrita do kernel está flag-OFF em produção
(`WORKFLOW_KERNEL_TRANSACTIONS_WRITE_ENABLED=false`, `configs/config.go:651`) e a clarificação de
categoria roda pelo caminho legacy (que reconstrói o estado a partir de `pendingexpense.Draft`). Ao
construir o HITL sobre o kernel e promovê-lo, o defeito deixaria de ser latente.

O kernel deve permanecer **genérico** (R-WF-KERNEL-001): opera sobre `S any` via codec JSON, sem
conhecer domínio.

## Decisão

Trocar a **substituição** por **JSON merge-patch (RFC 7386)** no `Engine.Resume`. O payload de resume
passa a ser um **delta** (ex.: `{"ResumeText":"sim"}`) aplicado sobre `Snapshot.State` (bytes JSON);
o resultado é decodificado em `S` e usado como estado corrente do resume.

- Novo método genérico `Codec[S].MergePatch(base, patch []byte) ([]byte, error)` que faz merge
  recursivo de objetos JSON (chave `null` remove a chave), sem conhecimento de domínio.
- `Engine.Resume` aplica `MergePatch(snap.State, resume)` e decodifica; resume vazio é **no-op**
  (mantém compatibilidade com chamadas atuais).
- O snapshot do kernel torna-se a **fonte única de verdade** no resume; consumidores não precisam de
  side-store para recuperar o estado suspenso.

Escopo: `internal/platform/workflow/codec.go` e `engine.go`. Impacta todos os consumidores do kernel
(incluindo a clarificação de categoria, que passa a recuperar estado corretamente também no caminho
kernel).

## Alternativas Consideradas

- **`ResumeApplier[S]` (hook já existente em `store.go:51`):** o consumidor implementa
  `ApplyResume(resume S) S`. *Vantagem:* tipado. *Desvantagem:* exige que o consumidor decodifique e
  saiba mesclar campo a campo; perde o benefício de "apenas delta"; empurra responsabilidade para
  cada consumidor (menos DRY). **Rejeitada** — merge genérico no kernel resolve para todos de uma vez.
- **Campo `ResumeInput []byte` reservado no estado:** engine carrega snapshot como base e injeta o
  resume num campo. *Desvantagem:* impõe convenção ao tipo `S`; vaza preocupação de mecanismo para o
  domínio. **Rejeitada.**
- **Hook de merge por `Definition[S]` (`func(base S, resume []byte) S`):** *Desvantagem:* mesma
  dispersão de responsabilidade do `ResumeApplier`. **Rejeitada.**
- **Side-store de draft (não tocar o kernel):** manter `pendingexpense.Draft`-like e reconstruir no
  resume. *Vantagem:* zero mudança no kernel. *Desvantagem:* não corrige o defeito latente para os
  demais consumidores; duplica fonte de verdade. **Rejeitada** pelo solicitante em favor de corrigir
  o kernel.

## Consequências

### Benefícios Esperados

- Corrige um defeito latente de perda de estado no resume para **todos** os consumidores do kernel.
- Resume passa a carregar apenas o delta — payloads menores e contrato mais claro.
- Elimina necessidade de side-store; snapshot é fonte única.
- Mantém o kernel genérico (R-WF-KERNEL-001) — merge opera sobre JSON, sem domínio.

### Trade-offs e Custos

- Merge via `map[string]any` tem custo de (de)serialização adicional no resume (aceitável: resume é
  evento raro, fora do hot-path de leitura).
- Semântica de `null`-remove exige documentação para evitar surpresa (delta com `null` apaga chave).

### Riscos e Mitigações

- **Risco:** regressão em fluxos atuais que dependiam (mesmo que acidentalmente) do replace.
  **Mitigação:** resume vazio é no-op; teste de regressão do defeito + `parity_test.go` verdes antes
  do merge. **Rollback:** reverter o bloco de merge para o replace anterior (mudança localizada).

## Plano de Implementação

1. Implementar `MergePatch` + testes de unidade (merge, sobrescrita, `null`-remove, vazio=no-op).
2. Trocar o bloco de replace no `Engine.Resume`; adicionar teste de regressão do defeito
   (suspende rico → resume `{"ResumeText":"x"}` → campos sobrevivem).
3. Rodar `parity_test.go` + integration de store (round-trip real).
4. Adoção concluída quando o caminho kernel de categoria recupera estado corretamente e os gates
   `R-WF-KERNEL-001` passam.

## Monitoramento e Validação

- Métrica `workflow_resume_total` por `result`; `workflow_version_conflict_total` estável.
- Critério de sucesso: 100% de resume com estado preservado em teste de durabilidade; zero efeito
  duplicado sob resume concorrente.
- Reverter se surgir regressão de paridade não coberta.

## Impacto em Documentação e Operação

- `.claude/rules/workflow-kernel.md`: nota sobre o contrato de resume (merge-patch, delta).
- Techspec e runbook do kernel atualizados.

## Revisão Futura

- Revisitar se o kernel passar a suportar estados não-JSON (codec alternativo) ou se a semântica
  `null`-remove se mostrar insuficiente para algum consumidor (ex.: precisar de patch de arrays por
  índice).
