# Prompt de Revisao: Jornada WhatsApp financeira sem falso sucesso

- **Data:** 2026-07-10
- **PRD:** `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/prd.md`
- **Artefatos relacionados:** `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/techspec.md`, `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/tasks.md`, ADRs `adr-001` a `adr-006`, `*_execution_report.md` e `bugfix_report.md` quando existirem
- **Escopo deste arquivo:** prompt enriquecido para execucao posterior. Nenhum review ou bugfix foi executado na criacao deste artefato.

---

## Analise do Prompt Original

| Dimensao | Prompt original | Enriquecimento aplicado |
|---|---|---|
| Objetivo | Revisar estritamente contra `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso`. | Define criterio de encerramento operacional: somente `APPROVED` real encerra; qualquer outro veredito exige novo ciclo. |
| Fonte da verdade | PRD informado por diretorio. | Explicita leitura obrigatoria de `prd.md`, `techspec.md`, `tasks.md`, ADRs e evidencias de execucao do pacote. |
| Criterios | Todos os criterios de aceite, DoD e regras de negocio implementados; zero gaps/lacunas/falsos positivos/ressalvas. | Converte isso em matriz de rastreabilidade obrigatoria por requisito funcional, objetivo, historia, funcionalidade core, task e ADR. |
| Ciclo de correcao | Usar `bugfix` e repetir o ciclo ate `APPROVED`. | Formaliza quando gerar bugs canonicos, quando chamar `bugfix` e quando uma nova revisao deve recortar apenas o delta da remediacao. |
| Subagentes | Disparar quando agregarem qualidade. | Define subagentes especializados por area para reduzir falso positivo e aumentar cobertura tecnica. |
| Saida | Nao especificada alem do resultado esperado. | Exige relatorio markdown com veredito, rastreabilidade, achados, bugs canonicos, validacoes, riscos residuais e proxima acao. |

### Ambiguidades tratadas

- O pedido atual proibe implementacao; portanto este arquivo apenas cria o prompt. O prompt abaixo, quando executado depois, pode acionar `review` e `bugfix`.
- "DoD 100%" foi ancorado nos artefatos efetivos do pacote do PRD: `tasks.md`, `techspec.md`, ADRs, reports de execucao e gates obrigatorios do repositorio.
- "0 falsos positivos" exige duas coisas ao mesmo tempo: nao aprovar sem evidencia e nao apontar falha sem referenciar especificacao, codigo, teste, fixture, diff ou validacao concreta.
- Como o pacote contem tarefas e relatórios de execucao, o revisor nao pode se limitar ao diff; precisa confrontar implementacao, evidencias e contratos declarados.

---

## Prompt Original

> Use `@.claude/skills/review/` para criar um review criterioso e sem flexibilizacao, validando estritamente contra `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso`
>
> Critérios obrigatórios:
> - Todos os critérios de aceite atendidos (implementados).
> - DoD 100% atendido (implementados).
> - 0 gaps.
> - 0 lacunas.
> - 0 falsos positivos.
> - 0 ressalvas.
> - Todas Regras de negócio atendidos (implementados)
>
> Caso encontre qualquer problema, utilize `@.claude/skills/bugfix/` e repita o ciclo `review -> bugfix -> review` até obter `APPROVED`, sem falsos positivos e em conformidade total com a especificação.
>
> Dispare subagentes especializados quando agregarem qualidade à revisão.

---

## Prompt Enriquecido

<goal>
Executar uma revisao estrita, criteriosa e sem flexibilizacao da implementacao do PRD "Jornada WhatsApp financeira sem falso sucesso", usando `@.claude/skills/review/`, ate obter um veredito final `APPROVED` real.

Done significa: todos os objetivos, funcionalidades core, historias de usuario, requisitos funcionais, criterios de aceite, DoD, tarefas implementadas, ADRs e restricoes do pacote `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso` estao atendidos com evidencia concreta; existem 0 gaps, 0 lacunas, 0 falsos positivos e 0 ressalvas.
</goal>

<context>
Voce esta no repositorio MeControla e deve seguir `AGENTS.md` como fonte canonica.

A fonte da verdade desta revisao e o pacote completo:
- `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/prd.md`
- `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/techspec.md`
- `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/tasks.md`
- `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/adr-001-escrita-aceita-sem-recurso-duravel.md`
- `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/adr-002-idempotencia-por-pendencia-e-retry-controlado.md`
- `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/adr-003-validacao-simetrica-distribuicao-orcamento.md`
- `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/adr-004-scorer-persistencia-per-run.md`
- `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/adr-005-correlacao-wamid-e-run-update-observavel.md`
- `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/adr-006-identidade-canonica-resolve-path.md`
- `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/*_execution_report.md`
- `.specs/prd-jornada-whatsapp-financeira-sem-falso-sucesso/bugfix_report.md` quando existir

O dominio revisado cobre a jornada financeira via WhatsApp com foco em:
- personalizacao de orcamento sem perda nem sobrescrita silenciosa;
- pendencia de despesa deterministica;
- confirmacao que so responde sucesso com efeito duravel;
- idempotencia por pendencia/operacao;
- correlacao por WAMID e reconciliacao de run;
- scorer honesto baseado em persistencia real;
- identidade canonica com trilha;
- golden set, gates Go e real-LLM.
</context>

<rules>
- Carregue e siga `@.claude/skills/review/` integralmente.
- Se houver achado acionavel, converta-o para o formato canonico exigido por `@.claude/skills/bugfix/`.
- Nao implemente nada diretamente fora do fluxo definido pelas skills referenciadas.
- Leia integralmente `prd.md`, `techspec.md`, `tasks.md` e todos os ADRs antes do veredito.
- Confronte implementacao, diff, testes, fixtures, migrations, harness, relatorios e evidencias persistidas; nao aceite conformidade por nome de arquivo ou por intencao declarada.
- Toda afirmacao de atendimento deve ter evidencia concreta em codigo, teste, fixture, validacao executada ou artefato persistido.
- Todo achado deve citar a fonte de verdade correspondente: objetivo, funcionalidade core, historia, RF, tarefa, ADR, DoD ou regra de governanca.
- `APPROVED_WITH_REMARKS` nao satisfaz este pedido. Qualquer ressalva, lacuna, evidencia ausente, risco residual acionavel ou validacao faltante impede encerramento.
- Nao gere falso positivo: nao reporte defeito sem base objetiva.
- Nao gere falso negativo: se um requisito nao tiver evidencia suficiente, trate como `nao atendido` ou `nao verificavel`, nunca como atendido por inferencia.
- Nao reverta mudancas do usuario nem mascare drift do workspace.
- Se o escopo revisavel estiver difuso, determine-o com diff, stat, tasks e reports; se ainda faltar contexto minimo, retorne `BLOCKED`.
</rules>

<mandatory_checks>
Construa uma matriz de rastreabilidade cobrindo, no minimo:

- Objetivos do PRD:
  - zero falso sucesso;
  - zero cancelamento indevido;
  - zero personalizacao perdida;
  - correlacao completa;
  - concordancia de estado;
  - scorer honesto;
  - regressao barrada.
- Todas as historias de usuario.
- Todas as funcionalidades core 1..9.
- Todos os requisitos funcionais RF-01..RF-30.
- Todas as tarefas declaradas em `tasks.md` e sua cobertura de requisitos.
- Todas as decisoes arquiteturais ADR-001..ADR-006.
- Todo DoD inferivel de `tasks.md`, `techspec.md`, execution reports e gates obrigatorios do repositorio.

Para cada item, marque obrigatoriamente um status:
- `atendido`
- `nao atendido`
- `nao verificavel`

`nao atendido` e `nao verificavel` impedem `APPROVED`.
</mandatory_checks>

<technical_focus>
Revise com profundidade especial os pontos que o PRD declara como falhas reais da jornada:

- perda de personalizacao de orcamento;
- validacao simetrica de distribuicao;
- pendencia de despesa confundida com multiplos lancamentos;
- confirmacao que nao gera ledger/transacao;
- idempotencia por pendencia com replay e retry controlado;
- proibicao de `PendingStatusCancelled` representar falha de escrita;
- correlacao por WAMID em `platform_runs` e `workflow_runs`;
- observabilidade de falha em `run.Update`;
- concordancia entre `platform_runs`, `workflow_runs`, `agents_write_ledger`, `transactions` e `platform_scorer_results`;
- scorer de persistencia por run;
- status outbound do WhatsApp distinguindo "nao recebido" de outras falhas;
- identidade canonica com backfill idempotente e trilha em `auth_events`;
- reconstructibilidade da jornada por DB/traces/logs;
- golden set derivado da jornada real;
- gates `go build`, `go vet`, `go test -race -count=1`, `golangci-lint run` e gate real-LLM `>= 0,90`.
</technical_focus>

<architecture_and_governance>
Verifique explicitamente:

- handlers, consumers, jobs e tools permanecem adapters finos;
- regras de negocio e decisoes vivem em use cases/domain, nao nos adapters;
- `internal/platform/workflow` permanece generico, sem vazamento de dominio consumidor;
- `internal/platform/agent`, `memory`, `tool`, `scorer` e `workflow` preservam contratos e estados fechados;
- nenhum estado ilegal e representado por `string` livre quando o contrato exige tipo fechado;
- nenhum fluxo financeiro declara sucesso sem efeito duravel correspondente;
- logs e metricas respeitam privacidade e cardinalidade agregada;
- o diff e o estado atual obedecem as restricoes Go do repositorio, inclusive R5.26 e ausencia de comentarios proibidos em Go de producao.
</architecture_and_governance>

<validations>
Registre e confronte, no minimo, as seguintes validacoes:

- `go build` no escopo proporcional;
- `go vet` no escopo proporcional;
- `go test -race -count=1` nos pacotes alterados e nos pacotes que cobrem a jornada;
- `golangci-lint run` no escopo proporcional, quando disponivel;
- testes de regressao para budget customization, pending-entry, confirmacao, replay, retry, scorer, correlacao e golden set;
- gate real-LLM e thresholds por categoria;
- verificacao de reports e evidencias persistidas geradas pelas tasks do PRD.

Se alguma validacao obrigatoria nao puder ser executada ou nao existir evidencia equivalente prevista na especificacao, isso impede `APPROVED`.
</validations>

<subagents>
Dispare subagentes especializados quando agregarem qualidade ou reduzirem risco de falso positivo. Sugestao minima:

- `review-budget-customization`: orcamento, validacao simetrica, alocacoes e onboarding.
- `review-pending-entry-idempotency`: pendencias, confirmacao, replay, retry, ledger/transacao.
- `review-runs-observability`: WAMID, correlacao, reconciliacao, logs, metricas e status.
- `review-scorers-golden`: scorer per-run, golden set, harness e gate real-LLM.
- `review-governance-tests`: governanca Go, arquitetura, testes, lint, build e reports.

Cada subagente deve devolver apenas:
- arquivos lidos;
- comandos executados;
- achados com evidencia;
- riscos residuais.

A decisao final continua centralizada no agente principal.
</subagents>

<bugfix_cycle>
Se houver qualquer problema:
1. Gere bugs canonicos no formato `{ id, severity, file, line, reproduction, expected, actual }`.
2. Inclua `Origem` apontando para RF, tarefa, ADR, objetivo, funcionalidade core ou item do DoD.
3. Acione `@.claude/skills/bugfix/` para remediacao pela causa raiz com teste de regressao obrigatorio.
4. Revise novamente usando o delta da remediacao quando aplicavel.
5. Repita `review -> bugfix -> review` ate obter `APPROVED`.

Nao encerre com `APPROVED` se restar qualquer item `blocked`, `failed`, `skipped`, `nao verificavel`, ressalva, risco residual acionavel ou validacao ausente.
</bugfix_cycle>

<output>
Produza um relatorio markdown em pt-BR com as secoes obrigatorias abaixo:

1. `Veredito Final`
2. `Arquivos e Referencias Lidos`
3. `Matriz de Rastreabilidade`
4. `Achados`
5. `Bugs Canonicos para Bugfix`
6. `Validacoes Executadas`
7. `Subagentes`
8. `Riscos Residuais e Ressalvas`
9. `Proxima Acao`

Regras do output:
- `Veredito Final` deve ser apenas `APPROVED`, `REJECTED` ou `BLOCKED`.
- A matriz deve listar item por item com `Fonte`, `Status`, `Evidencia`, `Validacao` e `Observacao`.
- Se nao houver achados, escreva exatamente: `Nenhum achado encontrado com base nas evidencias revisadas.`
- Se nao houver bugs canonicos, escreva exatamente: `Nenhum bug canonico gerado.`
- Para `APPROVED`, a secao `Riscos Residuais e Ressalvas` deve conter exatamente: `Nenhuma ressalva, gap ou lacuna identificada.`
</output>

<approval_gate>
So emita `APPROVED` se todas as afirmacoes abaixo forem verdadeiras e comprovadas:

- todos os objetivos do PRD foram atendidos;
- todas as funcionalidades core foram implementadas;
- todos os RF-01..RF-30 foram implementados;
- todos os criterios de aceite e DoD estao 100% atendidos;
- todas as regras de negocio estao implementadas;
- todas as tarefas relevantes em `tasks.md` estao efetivamente refletidas na implementacao;
- todas as ADRs foram respeitadas;
- todas as validacoes proporcionais passaram ou ha evidencia formal equivalente aceita pela especificacao;
- nao existem achados, bugs canonicos, gaps, lacunas, riscos residuais acionaveis, falsos positivos ou ressalvas.

Se qualquer item for falso, desconhecido ou nao verificavel, o veredito deve ser `REJECTED` ou `BLOCKED`, nunca `APPROVED`.
</approval_gate>

---

## Justificativas das Adicoes

| Adicao | Justificativa |
|---|---|
| Tags XML (`<goal>`, `<context>`, `<rules>`, etc.) | Atende a regra local para prompts multi-parte e reduz ambiguidade operacional. |
| Pacote completo do PRD como fonte da verdade | Evita revisao parcial baseada so no `prd.md`. |
| Matriz de rastreabilidade obrigatoria | Traduz "0 gaps" e "0 lacunas" em verificacao objetiva e auditavel. |
| Cobertura explicita de RF-01..RF-30 | Impede aprovacao por amostragem informal. |
| Acoplamento com `tasks.md`, ADRs e execution reports | Forca confronto entre especificacao, plano e evidencia de execucao. |
| Subagentes especializados por area | Aumenta profundidade tecnica sem degradar foco nem gerar conclusoes rasas. |
| Gate de `APPROVED` sem remarks | Alinha a skill `review` ao requisito do usuario de zero ressalvas. |
| Ciclo canonico `review -> bugfix -> review` | Materializa o fluxo exigido para qualquer achado corrigivel. |
