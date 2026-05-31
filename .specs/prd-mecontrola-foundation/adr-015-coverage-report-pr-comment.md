# ADR-015 — Coverage report via `fgrosse/go-coverage-report` action (PR comment)

## Metadados

- **Título:** Reporting de cobertura como comentário automático no PR (sem gate)
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §D-28, §CS-28, §RF-18](./prd.md), [techspec §Plano de Rollout M5](./techspec.md), [D-11 (sem gate)](./prd.md), [ADR-006 (test stack)](./adr-006-test-stack-testify-mockery.md)

## Contexto

D-11 (PRD v3) define que foundation NÃO tem gate de cobertura — relatório fica como artefato anexável ao PR. Operacionalizar isso exige decidir COMO o relatório fica visível:

- **Codecov.io** / **Coveralls**: dashboard rico, badges, histórico. **Mas** repo é private (D-06); Codecov free só cobre public; pago violaria orçamento.
- **PR comment via GitHub Action**: zero infra externa, visível direto no PR, atualiza em push.
- **Artefato sem comentário**: dev precisa baixar e abrir manualmente — UX ruim, ignorado.

Production-ready argumenta por visibilidade alta com custo zero.

## Decisão

Usar **`fgrosse/go-coverage-report@v1`** (GitHub Action) que:

1. Roda após `task test:unit` no job CI.
2. Lê o `coverage.out` gerado pelo `go test -coverprofile`.
3. Posta um comentário no PR com:
   - Cobertura total do projeto.
   - Tabela por package (delta vs `main`).
   - Lista de funções com cobertura abaixo de threshold informativo (placeholder 60% no MVP; sem bloqueio — só destaque visual).
4. **Atualiza o comentário** em push subsequente (não cria novo a cada push).
5. Sem gate: CI verde mesmo com cobertura 0%.

`coverage.out` também é anexado como artefato do workflow para download manual quando necessário.

## Alternativas Consideradas

1. **Codecov.io (paid private)**: ~$10/mês por usuário. Dashboard rico mas viola orçamento (R$ 60/mês total infra inclui Codecov esticaria).
2. **Codecov.io self-hosted**: complexidade alta para foundation; manutenção extra.
3. **Coveralls**: similar a Codecov; private também é pago.
4. **Apenas artefato sem comment**: dev ignora; relatório não cumpre função.
5. **PR comment custom via gh CLI no workflow**: reinventa wheel; action `fgrosse` já está pronta.
6. **GitHub Code Scanning**: cobertura não é função primária; gambiarra.

## Consequências

### Benefícios Esperados

- **Zero custo**: action open-source, runner GitHub-hosted free para private repo dentro de quota.
- **Visibilidade alta**: comentário no PR — dev e reviewer veem sem clicar em nada.
- **Delta visível**: package que perdeu cobertura é destacado, incentiva manter.
- **Sem gate**: alinha com D-11 e R-TEST-001 (cobertura prioritária, não absoluta); evita "encher de testes triviais" só para passar gate.

### Trade-offs e Custos

- Sem histórico de cobertura ao longo do tempo (Codecov teria); aceito no MVP.
- Sem badge de cobertura no README (Codecov teria); pode adicionar shields.io estático futuro.
- Comentário polui PR conversation se houver muitos pushes (ação atualiza comentário, mas histórico de edição fica).

### Riscos e Mitigações

- **Risco**: action `fgrosse/go-coverage-report` abandonado upstream.
  - **Mitigação**: pinar versão exata (`@v1.0.x`); plano B: action `vakenbolt/go-test-report` ou rolar PR comment custom via `gh pr comment`.
- **Risco**: comentário aparece antes do CI completar e confunde reviewer.
  - **Mitigação**: action roda no final do job `test:unit`; documentar ordem dos jobs.
- **Risco**: dev usa cobertura como métrica de vaidade (>X% sem qualidade).
  - **Mitigação**: documentar no CONTRIBUTING.md/README que cobertura é "informativa, não prescritiva"; foco em testes de comportamento (R-TEST-001 §Cobertura Prioritária).
- **Risco**: `coverage.out` pesado em monorepo grande.
  - **Mitigação**: foundation é pequena; revisar quando passar 100k LoC.

## Plano de Implementação

1. `taskfiles/test.yml`: tarefa `task test:unit` adiciona flag `-coverprofile=coverage.out -covermode=atomic` (`atomic` para concorrência).
2. `.github/workflows/ci.yml`: step pós-`task test:unit`:
   ```yaml
   - uses: fgrosse/go-coverage-report@v1
     with:
       coverage-artifact-name: coverage.out
       coverage-file-name: coverage.out
   ```
3. README: link para "Como ler o comentário de cobertura" — explicar que é informativo.
4. CONTRIBUTING.md (a criar): seção "Testes" reforçando R-TEST-001 §Cobertura Prioritária.

## Monitoramento e Validação

- Step do CI sucesso/falha visível no GitHub Actions.
- Comentário visível em qualquer PR ativo.
- Métrica manual: trimestralmente, revisar média de cobertura dos 10 PRs anteriores.

## Impacto em Documentação e Operação

- README: badge informativo (estático ou via shields.io).
- Onboarding: explicar comentário automático.
- CONTRIBUTING.md (futuro): expectativa sobre cobertura.

## Revisão Futura

- Revisitar se cobertura virar bloqueador real (ex.: dev gaming a métrica).
- Revisitar adoção de Codecov se repo virar público (free tier OSS).
- Revisitar adicionar gate por package (Identity ≥70%, Conversation ≥80%) nos PRDs respectivos — gate na foundation continua proibido (D-11).
