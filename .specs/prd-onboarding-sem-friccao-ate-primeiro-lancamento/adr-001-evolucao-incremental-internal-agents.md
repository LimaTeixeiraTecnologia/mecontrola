# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Evolução incremental em `internal/agents`
- **Data:** 2026-07-11
- **Status:** Aceita
- **Decisores:** Requester, Codex
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O PRD corrige uma jornada real de onboarding e primeiro lançamento financeiro no WhatsApp. O codebase já possui runtime agentivo, workflow durável, stores, tools financeiras, pending-entry e adapters de transação. Criar novo workflow, novo bounded context, novo endpoint ou novo kernel aumentaria risco sem resolver a causa raiz observada.

As regras do repositório exigem consumir `internal/platform/{agent,llm,memory,workflow,tool,scorer}` e o consumidor real `internal/agents`, sem reimplementar Thread, Run, WorkingMemory ou PendingStep.

## Decisão

A implementação será incremental em `internal/agents`, modificando prompts, decisões e guards existentes. A definição do onboarding continuará usando workflow durável. O passo `welcome` será preservado para compatibilidade estrutural, mas deixará de suspender com saudação isolada; o passo `goal` será a primeira suspensão real com a mensagem combinada.

Não serão criados novo bounded context, novo endpoint HTTP público, novo kernel de workflow, novo design pattern estrutural ou migration PostgreSQL.

## Alternativas Consideradas

- Criar novo workflow de onboarding v2: permitiria isolamento, mas duplicaria estado, testes, reaper e retomada sem necessidade.
- Criar novo bounded context de onboarding: separaria responsabilidades, mas violaria o escopo incremental e exigiria contratos novos para capacidades já presentes.
- Resolver só via prompt do agente geral: reduziria código, mas não corrigiria o workflow durável nem a trilha auditável.

## Consequências

### Benefícios Esperados

- Menor raio de mudança.
- Compatibilidade com stores e auditoria existentes.
- Menos risco operacional em produção.
- Testes conseguem confrontar diretamente o comportamento atual.

### Trade-offs e Custos

- Runs legados suspensos no passo `welcome` precisam ser considerados em teste de retomada.
- A definição mantém um passo `welcome` que passa a ser compatibilidade, não experiência de usuário.

### Riscos e Mitigações

- Risco: resposta de objetivo ser consumida pelo passo errado.
- Mitigação: `welcome` completa sem consumir `ResumeText`; `goal` suspende e processa a próxima resposta.

- Risco: regressão em mensagem registrada.
- Mitigação: teste de `platform_messages` garantindo uma única mensagem assistente inicial.

## Plano de Implementação

1. Alterar `BuildWelcomeStep` para completar sem suspensão em início novo.
2. Alterar `BuildGoalStep` para suspender com a copy combinada do PRD.
3. Atualizar testes existentes que esperam saudação e objetivo separados.
4. Validar retomada de run legado ou registrar bloqueio antes do deploy.

## Monitoramento e Validação

- Ativação nova deve gerar uma única mensagem assistente inicial.
- `workflow_runs` e `workflow_steps` continuam auditáveis.
- Nenhuma mensagem artificial `"Oi"` é necessária para avançar.

## Impacto em Documentação e Operação

- Atualizar testes e runbook de pós-deploy.
- Registrar no checklist de deploy que a primeira mensagem esperada é combinada.

## Revisão Futura

Revisar se uma mudança futura exigir onboarding versionado ou migração formal de runs suspensos.
