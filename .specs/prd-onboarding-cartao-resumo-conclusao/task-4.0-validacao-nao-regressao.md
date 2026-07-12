# Tarefa 4.0: Validação production-ready e não-regressão (gates + R0-R7)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a entrega com a matriz de validação proporcional ao risco e provar zero regressão nos fluxos que já funcionam em produção (objetivo, orçamento, distribuição, recorrência, criação de cartão, idempotência). Executar build/vet/lint/test -race, os gates de governança e o gate real-LLM, e confirmar que nada fora do escopo foi tocado.

<requirements>
- RF-17: nenhuma regressão — mudanças restritas a copy/montagem; ordem de etapas, suspend/resume, criação de cartão, ativação, recorrência e idempotência inalterados.
- RF-18: gate real-LLM verde + testes determinísticos verdes; nenhum comportamento declarado pronto apenas com mock.
</requirements>

## Subtarefas

- [x] 4.1 `gofmt -l` limpo; `go build ./...`; `go vet ./...`; `golangci-lint run` no pacote alterado.
- [x] 4.2 `go test ./internal/agents/... -count=1 -race` verde (unit + suites determinísticas).
- [x] 4.3 Rodar o pacote de integração COMPLETO (não apenas o `-run` estreito) para pegar o assert stale de `TestCardFlow_Integration:657` e evitar `-run` vazio: `go test -tags integration ./internal/agents/application/workflows/... -v` (compilação + testes mock-based) e, com credenciais, `RUN_REAL_LLM=1 OPENROUTER_API_KEY=... go test -tags integration -run 'TestOnboardingWorkflowRealLLMSuite/TestCardExtractionRealLLMGate' ./internal/agents/application/workflows -v` verde; gate golden do agente (`TestGoldenSetGate`) ≥ 0.90.
- [x] 4.4 Gates de governança (devem retornar vazio): zero comentários no Go alterado (R-ADAPTER-001.1); sem `switch case intent.Kind` (R-AGENT-WF-001); sem SQL direto em adapter; `internal/platform/workflow` não modificado (R-WF-KERNEL-001); estrutura de teste testify/suite (R-TESTING-001).
- [x] 4.5 Checklist R0-R7 (go-implementation): sem `init()`, sem `panic`/`os.Exit`/`log.Fatal` fora de `main`, sem `interface{}`, sem prefixo `_` em identificador.
- [x] 4.6 Não-regressão explícita: `git diff --stat` confirma que só `onboarding_workflow.go`, `onboarding_workflow_test.go` e `onboarding_workflow_integration_test.go` mudaram; `module.go`, kernel, schemas de extração e estado de domínio intactos.

## Detalhes de Implementação

Ver `techspec.md` seções "Conformidade com Padrões" e "Riscos Conhecidos", e a matriz de validação de `AGENTS.md`/`CLAUDE.md` (mudança em `application/` de módulo → build/vet/test race/lint no módulo alterado + gates de governança). Coletar evidência de cada gate.

## Critérios de Sucesso

- Todos os gates verdes com evidência anexada (comando + resultado).
- `git diff --stat` restrito aos três arquivos previstos (RF-17).
- Gate real-LLM e testes determinísticos verdes (RF-18).
- Nenhum gate de governança retorna violação.

## Skills Necessárias

<!-- MANDATÓRIO: go-implementation é auto-carregada por detecção de diff (category: language). -->

- `mastra` — aplicar `rules-checklist` do substrato (Run auditável, LLM só nas call-sites sancionadas, kernel intacto).
- `domain-modeling-production` — confirmar que nenhum estado/semântica de domínio novo foi introduzido.
- `design-patterns-mandatory` — confirmar o verdict "não aplicar padrão" no diff final (sem abstração desnecessária).

## Testes da Tarefa

- [x] Testes unitários (reexecução das suites determinísticas com `-race`)
- [x] Testes de integração (gate real-LLM + gate golden do agente)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go`, `onboarding_workflow_test.go`, `onboarding_workflow_integration_test.go` — alvos do diff.
- `.claude/rules/*.md` — gates de governança aplicáveis.
- `internal/platform/workflow/`, `internal/agents/module.go` — devem permanecer inalterados (verificação de não-regressão).
</content>
