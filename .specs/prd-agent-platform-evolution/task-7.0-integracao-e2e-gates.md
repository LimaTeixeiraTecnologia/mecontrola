# Tarefa 7.0: Integração (testcontainers) + E2E dos 4 cenários + gates R-*

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Prova de ponta a ponta production-ready do HITL: testes de integração com Postgres real
(testcontainers) para durabilidade/CAS/idempotência e E2E dos 4 cenários de confirmação, além da
verificação final dos gates `R-*` e da observabilidade.

<requirements>
- RF-09: resume idempotente sobrevive a restart/crash — efeito único.
- RF-10: resume concorrente — lock otimista por versão; a corrida perdedora não duplica efeito.
- RF-12: Run/Step auditáveis (status fechado, duration, erro, decision-id).
- RF-25: métricas com cardinalidade controlada (labels só de enums fechados).
- RF-26: gates `R-ADAPTER-001`, `R-AGENT-WF-001`, `R-WF-KERNEL-001`, `R-TESTING-001` e R0–R7 passam.
</requirements>

## Subtarefas

- [ ] 7.1 Integração (testcontainers, `//go:build integration`): suspende → recarrega (processo simulado) → resume com delta → efeito único; resume concorrente perde por `ErrVersionConflict` sem duplicar.
- [ ] 7.2 E2E dos 4 cenários por operação (delete/edit/card/budget-commit): confirmar, cancelar, ambíguo→reprompt→cancela, expirar→fall-through para nova intenção.
- [ ] 7.3 Idempotência por `messageID`: confirmação repetida não efetiva segunda vez.
- [ ] 7.4 Observabilidade: métricas com `operation`/`outcome` (enums fechados), logs `agent.hitl.*`; rodar os gates `grep` das regras e confirmar retorno vazio.

## Detalhes de Implementação

Ver `techspec.md` seções "Abordagem de Testes" e "Monitoramento e Observabilidade". Seguir
`R-TESTING-001` (testify/suite, whitebox, `fake.NewProvider()` em unit; integração usa Postgres real).
Validar que nenhum label de métrica usa `user_id`/`category_id`/`correlation_key`.

## Critérios de Sucesso

- 0 operações destrutivas efetivadas sem confirmação; 100% de resume após restart simulado.
- 0 efeito duplicado sob resume concorrente / replay por `messageID`.
- Os 4 cenários E2E passam para as 4 operações.
- Todos os gates `R-*` retornam vazio; build/test/lint verdes; cardinalidade de métrica conforme.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários — fechar lacunas de cobertura remanescentes dos passos/roteamento.
- [ ] Testes de integração — testcontainers Postgres: durabilidade, CAS de versão, idempotência; E2E dos 4 cenários.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/workflow/infrastructure/postgres/store_integration_test.go` (estendido)
- `internal/agent/application/services/kernel_e2e_test.go` (estendido — cenários HITL)
- `internal/agent/application/workflow/parity_test.go` (verde)
- `.claude/rules/*` (gates de verificação)
