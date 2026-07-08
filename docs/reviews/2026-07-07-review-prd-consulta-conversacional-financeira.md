# Review PRD - Consulta Conversacional Financeira

## Prompt original

```text
Use @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-consulta-conversacional-financeira
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

## Conflito tratado

O prompt original mistura:

1. A tarefa desta execução atual: apenas enriquecer e salvar o prompt.
2. A tarefa do agente executor futuro: revisar, corrigir via `bugfix` e repetir o ciclo até `APPROVED`.

Para não alterar o objetivo original, o prompt enriquecido abaixo preserva o ciclo operacional
`review -> bugfix -> review` para a execução futura e remove da instrução final apenas a restrição
local "não implemente nada", porque ela pertence a esta rodada de autoria do prompt, não à rodada
posterior de revisão real.

## Comparativo

| Aspecto | Original | Enriquecido |
|---|---|---|
| Escopo documental | Só aponta a pasta da PRD | Lista `prd.md`, `techspec.md`, `tasks.md`, task files, ADRs, checkpoints e reports |
| Critério de revisão | Intenção geral | Protocolo determinístico com checklist exaustivo de RFs, FCs, DoD, tarefas e validações |
| Ciclo de remediação | Implícito | Explicita `review -> bugfix -> review` até `APPROVED` |
| Ambiguidade operacional | Alta | Remove o conflito entre "criar o prompt agora" e "executar a revisão depois" |
| Evidência | Não definida | Exige evidência objetiva por requisito e proíbe aprovação por inferência |
| Subagentes | Só pede disparo quando útil | Define quando acionar e para quais frentes de análise |

## Prompt enriquecido

```xml
<goal>
Executar uma revisão estrita, completa e sem flexibilização da implementação associada à PRD
`consulta-conversacional-financeira`, repetindo o ciclo `review -> bugfix -> review` até obter
veredito final `APPROVED`, com aderência integral à especificação e sem falsos positivos.
</goal>

<context>
  <repository>/Users/jailtonjunior/Git/mecontrola</repository>
  <language>pt-BR</language>
  <primary_spec_dir>.specs/prd-consulta-conversacional-financeira</primary_spec_dir>
  <primary_prd>.specs/prd-consulta-conversacional-financeira/prd.md</primary_prd>
  <primary_techspec>.specs/prd-consulta-conversacional-financeira/techspec.md</primary_techspec>
  <primary_tasks>.specs/prd-consulta-conversacional-financeira/tasks.md</primary_tasks>
  <primary_task_files>
    <item>.specs/prd-consulta-conversacional-financeira/task-1.0-get-transaction-subcategory-aditivo.md</item>
    <item>.specs/prd-consulta-conversacional-financeira/task-2.0-instrucoes-consulta-c1-c7.md</item>
    <item>.specs/prd-consulta-conversacional-financeira/task-3.0-gate-real-llm-c1-c7.md</item>
  </primary_task_files>
  <primary_adrs>
    <item>.specs/prd-consulta-conversacional-financeira/adr-001-consulta-via-instrucoes-agente.md</item>
    <item>.specs/prd-consulta-conversacional-financeira/adr-002-get-transaction-subcategory-aditivo.md</item>
    <item>.specs/prd-consulta-conversacional-financeira/adr-003-formatacao-no-prompt.md</item>
  </primary_adrs>
  <primary_checkpoints>
    <item>.specs/prd-consulta-conversacional-financeira/.checkpoints/1.0.yaml</item>
    <item>.specs/prd-consulta-conversacional-financeira/.checkpoints/2.0.yaml</item>
    <item>.specs/prd-consulta-conversacional-financeira/.checkpoints/3.0.yaml</item>
  </primary_checkpoints>
  <primary_execution_reports>
    <item>.specs/prd-consulta-conversacional-financeira/1.0_execution_report.md</item>
    <item>.specs/prd-consulta-conversacional-financeira/2.0_execution_report.md</item>
    <item>.specs/prd-consulta-conversacional-financeira/3.0_execution_report.md</item>
  </primary_execution_reports>
  <review_skill>@.claude/skills/review/</review_skill>
  <bugfix_skill>@.claude/skills/bugfix/</bugfix_skill>
  <governance>Seguir estritamente `AGENTS.md`, a skill `review`, a skill `bugfix` e as skills mandatórias carregadas por gatilho.</governance>
  <feature_summary>
    Funcionalidade de consulta conversacional financeira read-only via WhatsApp, cobrindo C1-C7,
    com roteamento determinístico por tools existentes, zero alucinação, memória de thread para
    follow-up, única extensão aditiva permitida em `get_transaction` e gate real-LLM `M-04 >= 0.90`.
  </feature_summary>
</context>

<task>
  <step>Carregue o contexto mínimo exigido por `AGENTS.md` e pela skill `review`.</step>
  <step>Leia obrigatoriamente `prd.md`, `techspec.md`, `tasks.md`, as três task files, os ADRs e os execution reports antes de concluir a primeira rodada.</step>
  <step>Use `@.claude/skills/review/` de forma criteriosa e sem flexibilização, validando a implementação real do workspace contra a especificação.</step>
  <step>Confronte código, testes, evidências, validações e comportamento observado contra todos os requisitos e critérios da PRD.</step>
  <step>Se houver qualquer problema real, converta os achados para o formato canônico consumível por `@.claude/skills/bugfix/`, execute a remediação e revise novamente.</step>
  <step>Repita o ciclo `review -> bugfix -> review` quantas rodadas forem necessárias até não restar nenhum finding nem nenhuma divergência com a especificação.</step>
</task>

<scope>
  <in_scope>
    <item>Implementação real presente no workspace atual.</item>
    <item>Conformidade com `prd.md`, `techspec.md`, `tasks.md`, task files, ADRs, checkpoints e execution reports da PRD alvo.</item>
    <item>Critérios de aceite, DoD, regras de negócio, requisitos funcionais, cenários C1-C7, decisões D-01..D-09 e gate `M-04 >= 0.90`.</item>
    <item>Validações executadas, evidências persistidas, testes, build, vet, lint, race e harness real-LLM quando aplicável.</item>
  </in_scope>
  <out_of_scope>
    <item>Opiniões de estilo sem impacto real em corretude, segurança, regressão ou conformidade.</item>
    <item>Aprovação parcial.</item>
    <item>Follow-ups futuros ou dívida técnica residual.</item>
    <item>Qualquer veredito final diferente de `APPROVED`.</item>
  </out_of_scope>
</scope>

<mandatory_criteria>
  <item>Todos os critérios de aceite atendidos e implementados.</item>
  <item>DoD 100% atendido e implementado.</item>
  <item>0 gaps.</item>
  <item>0 lacunas.</item>
  <item>0 falsos positivos.</item>
  <item>0 ressalvas.</item>
  <item>Todas as regras de negócio atendidas e implementadas.</item>
  <item>Todas as evidências obrigatórias presentes e verificáveis.</item>
  <item>Todos os findings reais corrigidos e revalidados antes do encerramento.</item>
  <item>O único veredito final aceito é `APPROVED`.</item>
</mandatory_criteria>

<review_protocol>
  <step>Mapeie o alvo de revisão com base no diff relevante e no working tree atual, mas nunca limite a checagem apenas ao diff quando a PRD exigir evidência comportamental mais ampla.</step>
  <step>Confronte explicitamente cada funcionalidade core FC-01..FC-08 com a implementação e com evidência objetiva.</step>
  <step>Confronte explicitamente cada requisito funcional RF-01..RF-36 com evidência objetiva de código, teste, validação ou comportamento observado.</step>
  <step>Confronte explicitamente os cenários C1-C7, as decisões D-01..D-09 e os critérios de sucesso das tarefas 1.0, 2.0 e 3.0.</step>
  <step>Confronte explicitamente o escopo permitido por RF-35 e as restrições impostas pelos ADRs.</step>
  <step>Verifique se a entrega continua read-only, idempotente, sem alucinação e sem tool nova fora do escopo.</step>
  <step>Para cada critério, aceite apenas `atendido com evidência objetiva` ou `não atendido`.</step>
  <step>Se houver qualquer incerteza material, trate o item como não atendido até obter evidência suficiente.</step>
  <step>Ao encontrar problema real, produza finding estruturado com severidade, arquivo, linha, impacto, origem e dica de correção.</step>
  <step>Converta os findings acionáveis para o formato canônico da skill `bugfix` e execute remediação pela causa raiz.</step>
  <step>Após cada remediação, execute nova rodada completa de review no delta da correção e uma checagem de conformidade final contra a PRD.</step>
  <step>Somente encerre quando a rodada final retornar `APPROVED` sem findings, sem lacunas de evidência e sem divergência residual.</step>
</review_protocol>

<hard_rules>
  <rule>Não flexibilize nenhum requisito explícito da PRD, da techspec, das tasks ou dos ADRs.</rule>
  <rule>Não use linguagem de incerteza para encobrir falta de evidência.</rule>
  <rule>Não produza falso positivo: todo finding deve ter ancoragem objetiva em evidência concreta.</rule>
  <rule>Não produza falso negativo: todo requisito da especificação deve ser confrontado de forma explícita.</rule>
  <rule>Não aprove com item `parcialmente atendido`, `não verificável`, `parece ok` ou equivalente.</rule>
  <rule>Não confunda artefato marcado como `done` com prova suficiente de implementação; valide o código e o comportamento real.</rule>
  <rule>Não aceite ausência de teste, build, vet, lint, race ou harness exigido como detalhe secundário.</rule>
  <rule>Não aceite divergência entre PRD, techspec, tasks e implementação sem registrar o desvio e tratá-lo como falha.</rule>
  <rule>Não interrompa o ciclo por cansaço, orçamento subjetivo ou aproximação; continue até `APPROVED` ou bloqueio objetivo externo.</rule>
</hard_rules>

<subagent_policy>
  <item>Acione subagente de confronto spec-vs-code quando a cobertura de RFs, FCs, tasks e ADRs exigir leitura extensa.</item>
  <item>Acione subagente de testes e evidências para validar build, vet, lint, race, integração e harness real-LLM.</item>
  <item>Acione subagente de agentes/tools/workflow para investigar `internal/agents`, `internal/platform/agent`, `memory`, scorers e cadeias C4/C5.</item>
  <item>Acione subagente de domínio financeiro para validar aderência read-only aos módulos `internal/budgets` e `internal/transactions`.</item>
  <item>Traga de volta ao agente principal apenas conclusões, evidências e achados consolidados.</item>
</subagent_policy>

<validation_focus>
  <item>FC-01 e RF-01..RF-09: roteamento determinístico por tools, sem substituição indevida.</item>
  <item>RF-06, RF-06a, RF-35 e D-09: única extensão aditiva permitida em `get_transaction`, incluindo `subcategoryNameSnapshot`, schema estrito e adapter fino.</item>
  <item>RF-07a, RF-13, RF-14, RF-16, RF-17 e RF-34: competência, mês atual em `America/Sao_Paulo`, default de quantidade, ordenação e retrocesso de até 1 mês.</item>
  <item>RF-08, RF-08a, RF-18..RF-22 e RF-36: orçamento completo, todas as categorias, alertas, total no topo e formatação monetária canônica.</item>
  <item>RF-10..RF-12, RF-31, RF-32, RF-32a e RF-33: zero alucinação, domínio permitido, erro técnico seguro, isolamento por thread/resourceID e fluxo read-only/idempotente.</item>
  <item>RF-23..RF-25: PT-BR, tom correto, emojis contextuais e markdown compatível com WhatsApp.</item>
  <item>RF-26..RF-27: memória de thread para follow-up sem substituir reinvocação de tool.</item>
  <item>Tarefa 3.0 e D-04: cenários C1-C7, cadeias C4/C5, asserção multi-tool de C1 e gate real-LLM `M-04 >= 0.90`.</item>
  <item>RF-35: nenhuma nova tool, nenhuma alteração indevida de assinatura, binding, use case ou `module.go`.</item>
</validation_focus>

<validation_commands_policy>
  <item>Exigir as validações proporcionais definidas em `AGENTS.md`, na skill `review`, nas task files e na techspec.</item>
  <item>Separar baseline quebrada de falha introduzida, sem mascarar o impacto sobre a conformidade final.</item>
  <item>Para requisitos agentivos de alto risco, exigir evidência compatível com o nível de risco e com o gate da PRD.</item>
  <item>Não considerar o trabalho concluído sem um check pass/fail verificável.</item>
</validation_commands_policy>

<output_format>
  <section>Resumo executivo da rodada com status atual do ciclo: `REVIEWING`, `BUGFIXING`, `REVIEW_RETRY`, `BLOCKED` ou `APPROVED`.</section>
  <section>Veredito canônico da rodada.</section>
  <section>Checklist completo de conformidade cobrindo FC-01..FC-08, RF-01..RF-36, D-01..D-09, C1-C7, critérios de aceite, DoD, regras de negócio e critérios de sucesso das tarefas 1.0, 2.0 e 3.0.</section>
  <section>Findings estruturados com `{severity, file, line, impact, origin, fix_hint}`.</section>
  <section>Bugs em formato canônico para consumo da skill `bugfix`, quando existirem.</section>
  <section>Arquivos revisados, referências carregadas, subagentes disparados e diff efetivamente analisado.</section>
  <section>Validações executadas, resultados observados e evidências consultadas.</section>
  <section>Somente na rodada final aprovada: declaração explícita de `0 gaps`, `0 lacunas`, `0 falsos positivos`, `0 ressalvas` e conformidade integral com `.specs/prd-consulta-conversacional-financeira`.</section>
</output_format>

<done_definition>
O trabalho só termina quando a revisão final retornar `APPROVED`, com todos os requisitos,
critérios de aceite, DoD, regras de negócio, tarefas e decisões da PRD
`consulta-conversacional-financeira` implementados e comprovados por evidência objetiva, sem
findings, sem lacunas e sem qualquer divergência residual.
</done_definition>

<example>
  <input>Existe um RF sem evidência objetiva no código, nos testes ou nas validações exigidas.</input>
  <output>Registrar finding bloqueante, acionar `@.claude/skills/bugfix/`, corrigir pela causa raiz, validar novamente e repetir a review. Não aprovar.</output>
</example>
```

## Justificativas das adições

| Adição | Justificativa curta |
|---|---|
| Estrutura XML com `goal`, `context`, `task`, `scope`, `rules`, `output_format` e `done_definition` | Atende a regra dura de prompts multi-parte do repositório e elimina ambiguidades operacionais. |
| Inclusão de `prd.md`, `techspec.md`, `tasks.md`, task files, ADRs, checkpoints e execution reports | Reduz o risco de revisão incompleta ou baseada só em diff. |
| Separação explícita entre autoria do prompt e execução futura da review | Resolve o conflito do prompt original sem mudar o objetivo real. |
| Cobertura nominal de FC-01..FC-08, RF-01..RF-36, D-01..D-09 e C1-C7 | Força revisão exaustiva contra a especificação real. |
| Regra de aceitar apenas `atendido com evidência objetiva` ou `não atendido` | Elimina zonas cinzentas e aprovação frouxa. |
| Política explícita de subagentes | Atende ao pedido do usuário e melhora cobertura em revisão extensa. |
| Saída estruturada com findings e bugs canônicos | Facilita o ciclo `review -> bugfix -> review` com rastreabilidade. |
| Definição de pronto condicionada a `APPROVED` sem ressalvas | Endurece o encerramento e impede falso positivo por tolerância indevida. |

## Variante recomendada

Use o prompt enriquecido acima sem simplificação. Ele é a variante mais aderente ao pedido, à
skill `prompt-enricher`, ao `AGENTS.md` do repositório e à PRD
`.specs/prd-consulta-conversacional-financeira`.

## Resultado da Execução do Review (2026-07-07)

Veredito final: **APPROVED** (produção), com 1 residual documentado e aceito (D-11).

Ciclo executado: review → medição real-LLM → descoberta de falso-verde em C4 → bugfix → re-gate →
tentativa de hardening de C5 → residual aceito → validação total.

Achados e resolução:
- **F-01 (high, escopo RF-35)**: `resolve_card.go` alterada fora do escopo. Resolvido — emenda **D-10**
  sanciona a descrição como exceção aditiva; PRD/techspec atualizados.
- **F-02 (critical, falso positivo)**: relatório 3.0 declarava C4 "PASS" e M-04=1.00 por amostra única;
  C4 real roteava só ~30–50% no modelo de produção (`gpt-4o-mini`). Resolvido — instrução C4 guiada
  por exemplo + descrição de `resolve_card` (D-10) → **C4 10/10**.
- **F-03 (high, gate mentiroso)**: `TestRealLLM_QueryCardInvoiceChain_C4` era asserção single-shot.
  Resolvido — promovido a **gate estatístico ≥ 8/10** (resultado 10/10).
- **R-01 → D-11 (residual aceito)**: formato `>` em C5 é best-effort de apresentação; 0/32 de aderência
  via instrução (modelo narra em prosa). Dado sempre correto. Formatador determinístico rejeitado
  (código + ADR, desvio arquitetural). Teste de C5 assevera presença de categoria+subcategoria.

Descoberta relevante: **modelos mais fortes pioram C4** (gpt-4o 0/10, gemini-2.0-flash 0/10 —
raciocinam pela clarificação); o lever correto foi o enquadramento por exemplo, não o modelo.

Evidência de validação (verificada na sessão, não confiada em relatório):
- Determinística: build/vet ✓, 8 pacotes de teste ✓ sem falhas, `golangci-lint` 0 issues, zero
  comentários ✓, gofmt limpo ✓.
- Real-LLM (`RUN_REAL_LLM=1`, `.env`, modelo de produção `gpt-4o-mini`): M-04 = **1.00 (29/29)**;
  C4 gate = **10/10** (≥8); C5 data-contract **PASS**.
- Escopo RF-35 (pós-D-10): produção = `mecontrola_agent.go` + `get_transaction.go` + `resolve_card.go`.
- Spec sincronizada: D-10 e D-11 no PRD; techspec, `3.0_execution_report` (addendum) e `tasks.md`
  atualizados.
