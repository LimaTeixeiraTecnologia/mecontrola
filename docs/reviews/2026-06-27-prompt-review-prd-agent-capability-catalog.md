# Prompt Pronto para Uso

Execute `@.claude/skills/review/` para revisar de forma estrita todo o trabalho relacionado a `.specs/prd-agent-capability-catalog`, sem flexibilizacao, sem inferencias benevolentes e sem aceitar conformidade parcial.

Objetivo:
- validar estritamente a implementacao, os testes, os artefatos e a documentacao contra `.specs/prd-agent-capability-catalog`;
- obter somente `APPROVED` quando houver conformidade total, sem gaps, sem lacunas e sem falsos positivos;
- se houver qualquer defeito real, acionar `@.claude/skills/bugfix/` e repetir o ciclo `review -> bugfix -> review` ate atingir conformidade total.

Fonte de verdade obrigatoria:
- `AGENTS.md`
- `.specs/prd-agent-capability-catalog/prd.md`
- `.specs/prd-agent-capability-catalog/techspec.md`
- `.specs/prd-agent-capability-catalog/tasks.md`

Artefatos adicionais que devem ser consultados quando relevantes para verificar aderencia, aceite, execucao e nao-regressao:
- `.specs/prd-agent-capability-catalog/task-*.md`
- `.specs/prd-agent-capability-catalog/adr-*.md`
- `.specs/prd-agent-capability-catalog/*_execution_report.md`
- `.specs/prd-agent-capability-catalog/_orchestration_report.md`
- `.claude/skills/review/SKILL.md`
- `.claude/skills/bugfix/SKILL.md`

Modo de execucao:
1. Carregue e siga `@.claude/skills/review/` integralmente.
2. Confronte a entrega com a especificacao, nao apenas com o diff.
3. Considere como valido apenas o que puder ser comprovado por codigo, testes, artefatos e documentacao local.
4. Nao gere falso positivo: todo achado precisa ter evidencia objetiva, impacto concreto e referencia precisa.
5. Nao gere falso negativo por omissao: verifique explicitamente requisitos funcionais, criterios de aceite, criterios de sucesso, DoD, restricoes arquiteturais e nao-regressao.
6. Dispare subagentes especializados quando isso aumentar a qualidade da revisao, especialmente para:
   - aderencia Go e governanca arquitetural;
   - cobertura de testes, nao-regressao e evidencias de validacao;
   - consistencia entre PRD, techspec, tasks, ADRs e reports.
7. Nao implemente nada durante a revisao. Primeiro revise, produza achados e determine o veredito.

Checklist obrigatorio:
- todos os RFs aplicaveis atendidos;
- todos os criterios de aceite atendidos;
- todos os criterios de sucesso atendidos;
- DoD 100% atendido;
- 0 gaps;
- 0 lacunas;
- 0 falsos positivos;
- 0 regressao em comportamento, observabilidade, governanca e rastreabilidade;
- conformidade integral com `AGENTS.md`;
- conformidade integral com o PRD e a techspec;
- aderencia total ao catalogo canonico e aos seams reais documentados.

Confronto minimo obrigatorio:
- verificar cobertura integral de `RF-01` a `RF-17`, marcando cada item como `atendido`, `nao_atendido` ou `nao_verificavel`, sempre com evidencia;
- verificar se tudo que esta marcado como `done` em `tasks.md` possui implementacao, teste e validacao correspondentes;
- verificar aderencia da implementacao aos ADRs e decisoes travadas;
- verificar especialmente:
  - fonte unica de verdade do catalogo de capabilities;
  - derivacao de runtime, auditoria e metricas a partir do catalogo;
  - ausencia de drift estrutural;
  - preservacao ou correcao intencional dos labels conforme especificado;
  - migracao de `isDestructiveKind` conforme a especificacao;
  - atualizacao fiel da skill `mastra` e do checklist de extensao;
  - nao-regressao da suite de `internal/agent` e `internal/platform/workflow`.

Regras de veredito:
- use `BLOCKED` se faltar diff, contexto essencial, evidencias ou validacoes suficientes;
- use `REJECTED` se houver qualquer defeito real de severidade `critical` ou `high`;
- use `APPROVED_WITH_REMARKS` apenas se nao houver falha de criterio obrigatorio;
- use `APPROVED` somente quando todos os criterios obrigatorios estiverem comprovadamente satisfeitos, sem achados e sem incerteza material restante.

Regras duras:
- se qualquer criterio de aceite falhar, o resultado nao pode ser `APPROVED`;
- se qualquer RF falhar, o resultado nao pode ser `APPROVED`;
- se o DoD nao estiver 100% atendido, o resultado nao pode ser `APPROVED`;
- se houver gap, lacuna, evidencia insuficiente ou falso positivo, o resultado nao pode ser `APPROVED`;
- nao trate ausencia de evidencia como detalhe menor;
- nao suavize defeito real para observacao branda;
- nao invente conformidade.

Fluxo de remediacao:
1. Execute `@.claude/skills/review/`.
2. Se houver achados reais e acionaveis, converta-os para o formato canonico exigido pela skill `bugfix`.
3. Execute `@.claude/skills/bugfix/` apenas sobre bugs confirmados, exigindo correcao de causa raiz e testes de regressao obrigatorios.
4. Rode nova revisao usando `AI_REVIEW_PRIOR_SHA` ou mecanismo equivalente previsto na skill, para revisar o delta da remediacao quando aplicavel.
5. Repita o ciclo `review -> bugfix -> review` ate obter `APPROVED`.
6. Se qualquer rodada final ainda contiver gap, lacuna, falso positivo, criterio nao atendido ou evidencia insuficiente, o resultado final nao pode ser `APPROVED`.

Formato obrigatorio da saida final:
- `verdict`: `APPROVED`, `APPROVED_WITH_REMARKS`, `REJECTED` ou `BLOCKED`
- `spec_scope_reviewed`: lista dos arquivos lidos em `.specs/prd-agent-capability-catalog/`
- `files_reviewed`: arquivos de codigo, diff e artefatos efetivamente revisados
- `refs_loaded`: referencias e skills carregadas
- `acceptance_matrix`: matriz cobrindo RFs, criterios de aceite, criterios de sucesso e DoD, item a item, com status e evidencia
- `findings`: lista de achados com `severity`, `file`, `line`, `impact`, `evidence`, `fix_hint`
- `false_positive_check`: confirmacao explicita de ausencia de falsos positivos ou lista do que foi descartado por falta de evidencia
- `residual_risks`: vazia para `APPROVED`; se nao estiver vazia, justificar por que o veredito nao e `APPROVED`
- `validations_run`: comandos, suites, relatórios e artefatos usados como evidencia
- `final_decision_rationale`: justificativa curta, objetiva e rastreavel para o veredito

Condicao de encerramento:
- encerre apenas com `APPROVED` quando houver conformidade total com a especificacao e nenhuma incerteza material restante;
- se houver contradicao nao resolvida entre especificacao, codigo, testes ou artefatos, registre explicitamente e use `BLOCKED` ou `REJECTED`, conforme o caso.
