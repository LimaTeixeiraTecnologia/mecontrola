# Review PRD - Registro Conversacional Transacoes Dia a Dia

## Metadados

- Data: `2026-07-07`
- PRD alvo: `.specs/prd-registro-conversacional-transacoes-dia-a-dia/`
- Arquivo principal: `.specs/prd-registro-conversacional-transacoes-dia-a-dia/prd.md`
- Saida deste documento: prompt enriquecido para execucao posterior

## Prompt original

```text
Use @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-registro-conversacional-transacoes-dia-a-dia
Critérios obrigatórios:
* Todos os critérios de aceite atendidos (implementados).
* DoD 100% atendido (implementados).
* 0 gaps.
* 0 lacunas.
* 0 falsos positivos.
* 0 ressalvas
* Todas Regras de negócio atendidos (implementados)
Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review → bugfix → review até obter APPROVED, sem falsos positivos e em conformidade total com a especificação.
Dispare subagentes especializados quando agregarem qualidade à revisão.
Não implemente nada. Apenas crie/enriqueça o prompt e salve o arquivo em docs/reviews/. faça quantas rodadas forem necessárias
```

## Ambiguidades resolvidas no enriquecimento

- `DoD` nao aparece como secao unica canonica na PRD. Neste prompt, `DoD` significa a soma verificavel de:
  - `tasks.md`
  - todos os `task-*.md` em `.specs/prd-registro-conversacional-transacoes-dia-a-dia/`
  - todos os `*_execution_report.md` relevantes, incluindo secoes `Critérios de Aceite` e `DoD` quando existirem
  - `prd.md` e `techspec.md`
- `Criterios de aceite` devem ser confrontados tanto no nivel da PRD quanto nos artefatos derivados da execucao das tarefas.
- `Nao implemente nada` aplica-se a esta solicitacao de enriquecimento. O prompt abaixo foi desenhado para ser executado depois, inclusive com ciclo `review -> bugfix -> review`.

## Prompt enriquecido

```xml
<goal>
Executar uma revisao estrita e deterministica da implementacao da PRD `registro-conversacional-transacoes-dia-a-dia` contra o workspace atual, repetindo o ciclo `review -> bugfix -> review` ate obter veredito final `APPROVED`, com 0 gaps, 0 lacunas, 0 falsos positivos, 0 ressalvas e conformidade integral com a especificacao.
</goal>

<context>
  <repository>/Users/jailtonjunior/Git/mecontrola</repository>
  <language>pt-BR</language>
  <primary_spec_dir>.specs/prd-registro-conversacional-transacoes-dia-a-dia</primary_spec_dir>
  <prd>.specs/prd-registro-conversacional-transacoes-dia-a-dia/prd.md</prd>
  <techspec>.specs/prd-registro-conversacional-transacoes-dia-a-dia/techspec.md</techspec>
  <tasks>.specs/prd-registro-conversacional-transacoes-dia-a-dia/tasks.md</tasks>
  <task_files_glob>.specs/prd-registro-conversacional-transacoes-dia-a-dia/task-*.md</task_files_glob>
  <execution_reports_glob>.specs/prd-registro-conversacional-transacoes-dia-a-dia/*_execution_report.md</execution_reports_glob>
  <adrs_glob>.specs/prd-registro-conversacional-transacoes-dia-a-dia/adr-*.md</adrs_glob>
  <review_skill>@.claude/skills/review/</review_skill>
  <bugfix_skill>@.claude/skills/bugfix/</bugfix_skill>
  <project_rules>
    Seguir estritamente `AGENTS.md`, incluindo anti-alucinacao, governanca, validacao, uso de subagentes, carga base obrigatoria e regras de output.
  </project_rules>
  <hard_scope_note>
    A implementacao ja deve existir no workspace. O trabalho e verificar e, se houver defeitos reais, remediar via skill `bugfix` antes de nova rodada de review.
  </hard_scope_note>
</context>

<source_of_truth>
  <item>`prd.md` e a fonte primaria de requisitos, RFs, metricas, cenarios, regras de negocio, restricoes e criterios de validacao.</item>
  <item>`techspec.md` e a fonte primaria de desenho tecnico, superficie de mudanca, abordagem de testes e riscos conhecidos.</item>
  <item>`tasks.md` e todos os `task-*.md` definem cobertura esperada, decomposicao da entrega e criterios de sucesso por tarefa.</item>
  <item>Todos os `*_execution_report.md` relevantes definem evidencias declaradas de execucao, criterios de aceite e `DoD` onde houver.</item>
  <item>Os ADRs locais definem decisoes travadas que tambem devem ser verificadas contra o codigo real.</item>
  <item>Se houver conflito entre artefatos, priorize o workspace atual e a regra mais segura, mas registre o drift explicitamente e trate como nao conformidade ate haver evidencia objetiva.</item>
</source_of_truth>

<mandatory_inputs>
  <item>Leia obrigatoriamente `prd.md`, `techspec.md` e `tasks.md` antes do primeiro veredito.</item>
  <item>Leia todas as task files `task-*.md` da PRD alvo para confrontar cada `Critério de Sucesso` contra a implementacao.</item>
  <item>Leia todos os `*_execution_report.md` da PRD alvo para confrontar cada `Critério de Aceite`, `DoD` e evidencia declarada contra o codigo e as validacoes reais.</item>
  <item>Leia ADRs da PRD alvo quando eles forem referenciados por PRD, techspec, tasks ou diff.</item>
  <item>Leia o diff apropriado conforme a skill `review` determinar, mas nao limite a analise ao diff quando um requisito precisar de verificacao no estado atual do codigo.</item>
</mandatory_inputs>

<operating_rules>
  <item>Use `@.claude/skills/review/` de forma criteriosa e sem flexibilizacao.</item>
  <item>Se houver qualquer finding real, converta-o para o formato canonico de bugs e acione `@.claude/skills/bugfix/`.</item>
  <item>Repita o ciclo `review -> bugfix -> review` quantas rodadas forem necessarias ate zerar findings e obter `APPROVED` puro.</item>
  <item>Dispare subagentes especializados quando aumentarem qualidade, cobertura ou precisao, especialmente para spec-vs-code, testes, harness real-LLM, fluxos agentivos, idempotencia, workflow, regras de dominio e leitura extensa de arquivos.</item>
  <item>Se estiver incerto sobre um criterio, trate como `nao atendido` ate obter evidencia objetiva.</item>
  <item>Nao aceite `APPROVED_WITH_REMARKS` como estado final.</item>
  <item>Nao encerre com risco residual, evidencia incompleta, `nao verificavel pelo diff` ou follow-up pendente.</item>
  <item>Nao gere falsos positivos: todo finding precisa de evidencia concreta, localizacao e impacto.</item>
  <item>Nao gere falsos negativos: todo requisito, decisao travada e criterio derivado deve ser confrontado com o codigo real.</item>
</operating_rules>

<review_protocol>
  <step>Executar a carga base obrigatoria do repositorio e da skill `review`.</step>
  <step>Montar uma matriz de conformidade completa cobrindo PRD, techspec, tasks, task files, execution reports e ADRs.</step>
  <step>Confrontar o codigo real contra cada item da matriz, sem inferencia benevolente.</step>
  <step>Executar ou consultar validacoes suficientes para sustentar cada conclusao.</step>
  <step>Se aparecer qualquer falha, lacuna de implementacao, falta de evidencia, drift ou regressao, registrar finding e entrar em remediacao via `bugfix`.</step>
  <step>Apos cada bugfix, revisar novamente o delta e, se necessario, a feature inteira, ate que todos os itens estejam implementados e comprovados.</step>
</review_protocol>

<mandatory_checklist>
  <group name="PRD">
    <item>Validar todos os RFs `RF-01` ate `RF-23`.</item>
    <item>Validar todas as metricas `M-01` ate `M-05`.</item>
    <item>Validar os cenarios `R1` ate `R8`, respeitando `R8` como fronteira de escopo.</item>
    <item>Validar as decisoes `D-01` ate `D-06`.</item>
    <item>Validar as restricoes tecnicas de alto nivel e o comportamento esperado da experiencia do usuario.</item>
    <item>Validar os criterios de validacao definidos na propria PRD.</item>
  </group>
  <group name="Techspec">
    <item>Validar as quatro entregas declaradas no resumo executivo.</item>
    <item>Validar a arquitetura prevista, os pontos de integracao e as superficies de mudanca listadas.</item>
    <item>Validar a abordagem de testes, incluindo harness e real-LLM quando exigidos.</item>
    <item>Validar conformidade com os padroes obrigatorios citados na techspec.</item>
  </group>
  <group name="Tasks">
    <item>Validar a cobertura de requisitos por tarefa descrita em `tasks.md`.</item>
    <item>Para cada `task-*.md`, confrontar todos os itens de `Critérios de Sucesso` com a implementacao real.</item>
    <item>Se um criterio de sucesso de tarefa nao estiver implementado ou nao tiver evidencia suficiente, emitir finding bloqueante.</item>
  </group>
  <group name="Execution Reports">
    <item>Para cada `*_execution_report.md`, confrontar todos os `Critérios de Aceite` com o codigo e as validacoes reais.</item>
    <item>Quando houver secao `DoD` ou `Definition of Done (DoD)`, confrontar item por item e exigir 100% atendido.</item>
    <item>Se o report afirmar algo nao sustentado pelo codigo ou pelos testes, tratar como drift e nao conformidade.</item>
  </group>
  <group name="Business Rules">
    <item>Confirmacao humana universal antes de persistencia.</item>
    <item>Zero alucinacao de campos obrigatorios.</item>
    <item>Zero duplicidade por idempotencia `(wamid, itemSeq, operation)`.</item>
    <item>Datas em `America/Sao_Paulo`, incluindo dias da semana e rejeicao de baixa precisao.</item>
    <item>Categorizacao deterministica com thresholds e sem inventar categoria.</item>
    <item>Resolucao de cartao e parcelas conforme PRD.</item>
    <item>Rejeicao correta de multiplas transacoes em uma unica mensagem.</item>
    <item>Formato e idioma das respostas em pt-BR conforme PRD.</item>
  </group>
  <group name="Acceptance Gates">
    <item>Todos os criterios de aceite atendidos e implementados.</item>
    <item>DoD 100% atendido e implementado.</item>
    <item>0 gaps.</item>
    <item>0 lacunas.</item>
    <item>0 falsos positivos.</item>
    <item>0 ressalvas.</item>
    <item>Todas as regras de negocio atendidas e implementadas.</item>
  </group>
</mandatory_checklist>

<validation_rules>
  <item>Executar as validacoes proporcionais exigidas por `AGENTS.md`, pela skill `review` e pelos artefatos da PRD.</item>
  <item>Para esta feature agentiva, exigir evidencia compativel com o risco, inclusive harness real-LLM quando a PRD ou task 8.0 o exigirem.</item>
  <item>Se a baseline estiver quebrada, separar claramente falha preexistente de falha introduzida, mas nao aprovar sem demonstrar conformidade integral da feature.</item>
  <item>Se um criterio depender de teste existente, evidenciar que o teste realmente cobre o comportamento alegado; nao aceitar cobertura nominal.</item>
</validation_rules>

<output_format>
  <section>`verdict`: `BLOCKED`, `REJECTED`, `APPROVED_WITH_REMARKS` ou `APPROVED` na rodada atual.</section>
  <section>`cycle_status`: `REVIEWING`, `BUGFIXING`, `REVIEW_RETRY` ou `DONE`.</section>
  <section>`files_reviewed`: arquivos efetivamente lidos.</section>
  <section>`refs_loaded`: referencias e skills efetivamente carregadas.</section>
  <section>`compliance_matrix`: lista completa de todos os itens verificados, cada um marcado como `atendido com evidencia` ou `nao atendido`, com prova objetiva.</section>
  <section>`findings`: lista estruturada de achados com `severity`, `file`, `line`, `impact`, `fix_hint`, `origin` e bug canonico correspondente quando houver.</section>
  <section>`validations_run`: comandos executados, harnesses consultados e evidencias analisadas.</section>
  <section>`residual_risks`: deve estar vazia no encerramento final.</section>
  <section>`final_assertion`: somente quando concluir com `APPROVED`, declarar explicitamente que ha `0 gaps`, `0 lacunas`, `0 falsos positivos`, `0 ressalvas` e conformidade total com `.specs/prd-registro-conversacional-transacoes-dia-a-dia`.</section>
</output_format>

<termination_rule>
  <allowed_final_state>Somente `APPROVED`.</allowed_final_state>
  <definition_of_done>
    O trabalho so termina quando todos os RFs, metricas, cenarios, decisoes travadas, criterios de sucesso das tarefas, criterios de aceite, itens de DoD, regras de negocio e validacoes obrigatorias estiverem implementados e comprovados por evidencia objetiva no workspace atual, sem findings restantes e sem riscos residuais.
  </definition_of_done>
</termination_rule>

<example>
  <input>Existe um execution report afirmando cobertura completa de idempotencia, mas o harness real nao comprova replay por `wamid` original.</input>
  <expected_behavior>Registrar finding bloqueante com evidencia, acionar `bugfix`, adicionar ou corrigir a cobertura necessaria, revalidar e revisar novamente. Nao aprovar enquanto o item nao estiver comprovado.</expected_behavior>
</example>
```

## Justificativas das adicoes

| Adicao | Justificativa curta |
|---|---|
| Estrutura em XML com `goal` primeiro | Atende a regra dura do repositorio para prompts multi-parte e reduz ambiguidade. |
| Fonte da verdade explicitada | Evita review parcial baseado so na PRD ou so no diff. |
| Definicao objetiva de `DoD` | Remove o risco de inventar um DoD inexistente na PRD principal. |
| Inclusao obrigatoria de `task-*.md` e `*_execution_report.md` | Garante confronto real com criterios de sucesso, aceite e DoD por fatia implementada. |
| Checklist nominal de RFs, metricas, cenarios e decisoes | Forca cobertura completa da especificacao. |
| Regras para tratar incerteza como nao conformidade | Evita aprovacao por inferencia benevolente. |
| Estado final permitido apenas `APPROVED` | Traduz seu objetivo em condicao de parada inequívoca. |
| Protocolo explicito de review -> bugfix -> review | Remove margem para o agente parar cedo com `REJECTED` ou `APPROVED_WITH_REMARKS`. |
| Exigencia de subagentes especializados | Aumenta qualidade em investigacoes extensas e reduz risco de falso positivo. |

## Variante recomendada

Use o prompt enriquecido acima sem simplificacao.
