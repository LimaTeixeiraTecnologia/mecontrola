# Review PRD - Registro Conversacional Robustez

## Metadados

- Data: `2026-07-08`
- Skill aplicada: `prompt-enricher`
- Alvo da revisão futura: `.specs/prd-registro-conversacional-robustez`
- Skills a invocar pelo executor do prompt:
  - `.claude/skills/review/SKILL.md`
  - `.claude/skills/bugfix/SKILL.md`

## Prompt Original

```text
Objetivo:
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-registro-conversacional-robustez
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
Não implemente nada. Apenas crie/enriqueça o prompt e salve o arquivo em docs/reviews/.
```

## Ambiguidades e Resolucoes Aplicadas

- `Validando estritamente contra .specs/prd-registro-conversacional-robustez` foi expandido para confronto obrigatório com `prd.md`, `techspec.md`, `tasks.md`, `task-*.md`, ADRs e código real do workspace.
- `DoD 100% atendido` foi tratado como cumprimento integral dos critérios de aceite, regras de negócio, validações proporcionais e evidência real de implementação, não apenas relato declarativo.
- O ciclo `review -> bugfix -> review` foi mantido como instrução para o executor futuro do prompt; este arquivo não executa revisão nem remediação.
- `0 gaps`, `0 lacunas`, `0 falsos positivos` e `0 ressalvas` foram convertidos em matriz de rastreabilidade completa e critério estrito para `APPROVED`.

## Prompt Enriquecido

```xml
<goal>
Obter um veredito final `APPROVED` para a implementação da spec `.specs/prd-registro-conversacional-robustez`, com evidência objetiva de que 100% dos requisitos, critérios de aceite, DoD e regras de negócio foram implementados, sem gaps, sem lacunas, sem falsos positivos, sem ressalvas e sem divergência entre especificação, código, testes e validações reais.
</goal>

<context>
- Repositório: `/Users/jailtonjunior/Git/mecontrola`
- Fonte canônica de governança: `AGENTS.md`
- Skill de revisão obrigatória: `.claude/skills/review/SKILL.md`
- Skill de remediação obrigatória quando houver achados: `.claude/skills/bugfix/SKILL.md`
- Spec raiz a confrontar de forma estrita: `.specs/prd-registro-conversacional-robustez`
- Documentos mínimos obrigatórios da spec:
  - `.specs/prd-registro-conversacional-robustez/prd.md`
  - `.specs/prd-registro-conversacional-robustez/techspec.md`
  - `.specs/prd-registro-conversacional-robustez/tasks.md`
  - `.specs/prd-registro-conversacional-robustez/task-1.0-seed-salario-leaf.md`
  - `.specs/prd-registro-conversacional-robustez/task-2.0-consolidar-brl.md`
  - `.specs/prd-registro-conversacional-robustez/task-3.0-propagacao-erro.md`
  - `.specs/prd-registro-conversacional-robustez/task-4.0-retry-transitorio.md`
  - `.specs/prd-registro-conversacional-robustez/task-5.0-guarda-kind.md`
  - `.specs/prd-registro-conversacional-robustez/task-6.0-confirmacao-unica.md`
  - `.specs/prd-registro-conversacional-robustez/task-7.0-heuristica-e-pagamento.md`
  - `.specs/prd-registro-conversacional-robustez/task-8.0-suite-e2e-real-llm.md`
  - `.specs/prd-registro-conversacional-robustez/adr-001-salario-leaf-seed.md`
  - `.specs/prd-registro-conversacional-robustez/adr-002-error-propagation-observability.md`
  - `.specs/prd-registro-conversacional-robustez/adr-003-bounded-transient-retry.md`
  - `.specs/prd-registro-conversacional-robustez/adr-004-single-confirmation-ownership.md`
  - `.specs/prd-registro-conversacional-robustez/adr-005-kind-guard-reclassification.md`
- Execution reports podem ser usados apenas como apoio de rastreabilidade e histórico de execução, nunca como prova suficiente de implementação:
  - `.specs/prd-registro-conversacional-robustez/1.0_execution_report.md`
  - `.specs/prd-registro-conversacional-robustez/2.0_execution_report.md`
  - `.specs/prd-registro-conversacional-robustez/3.0_execution_report.md`
  - `.specs/prd-registro-conversacional-robustez/4.0_execution_report.md`
  - `.specs/prd-registro-conversacional-robustez/6.0_execution_report.md`
- O alvo da revisão é o working tree atual e, quando aplicável, o diff relevante da implementação desta spec.
</context>

<task>
1. Carregue e siga `AGENTS.md` sem flexibilização.
2. Execute a skill `.claude/skills/review/SKILL.md` com foco em corretude, regressões, cobertura integral de requisitos, evidência real de implementação e ausência de falso positivo.
3. Valide estritamente a implementação contra toda a spec alvo, confrontando código, testes, prompts, workflows, persistência, observabilidade, heurísticas, confirmação, idempotência e validações executadas localmente.
4. Se surgir qualquer achado, converta os bugs para o formato canônico esperado pela skill de bugfix e acione `.claude/skills/bugfix/SKILL.md`.
5. Repita obrigatoriamente o ciclo `review -> bugfix -> review` até alcançar `APPROVED`.
6. Dispare subagentes especializados quando isso elevar a qualidade da revisão, especialmente para:
   - confronto `prd.md` x código real;
   - confronto `techspec.md` x código real;
   - validação das task files e seus critérios de aceite;
   - checagem do fluxo agent/workflow/HITL e confirmação única;
   - checagem de persistência, idempotência, observabilidade e retry;
   - checagem da suite E2E real-LLM, heurísticas BRL e pagamento.
7. Não conclua o trabalho com `APPROVED_WITH_REMARKS`, `REJECTED` ou `BLOCKED` como resultado final, exceto se existir bloqueio externo incontornável explicitado com evidência concreta.
</task>

<rules>
- Trate `prd.md`, `techspec.md`, `tasks.md`, `task-*.md` e ADRs da spec como contrato de implementação.
- Confronte cada requisito funcional RF-01 a RF-32 com evidência direta em código e/ou testes.
- Confronte todos os critérios de sucesso/aceite de cada task file com evidência objetiva.
- Confronte todas as regras de negócio materiais descritas no PRD, techspec e ADRs.
- Considere DoD atendido apenas quando todos os itens exigidos pela spec e pela skill de review estiverem implementados e validados.
- Exija evidência direta no código real, nos testes reais e nas validações reais; não aceite intenção, comentário, TODO, relatório ou narrativa como prova suficiente.
- Não gere falso positivo: se um achado não puder ser sustentado por evidência concreta, descarte-o.
- Não gere falso negativo: se houver requisito, regra, critério de aceite ou validação sem prova suficiente, trate como achado.
- Não aceite conformidade parcial.
- Não trate risco residual, lacuna de teste, cobertura incompleta, drift documental, dúvida de comportamento ou validação ausente como aceitável para `APPROVED`.
- Não implemente correções diretamente fora da skill de bugfix.
- Se houver incerteza, declare explicitamente a incerteza, colete mais evidência e só então conclua.
- Sempre preferir fontes primárias locais: código do workspace, testes, specs, ADRs, diffs e comandos de validação executados localmente.
- Sempre verificar se a implementação preserva:
  - categoria correta de salário base e separação de Décimo Terceiro;
  - guarda `income` versus `expense` com reclassificação defensiva e `ErrKindMismatch` como defesa final;
  - propagação integral de erro para `platform_runs.error`, span pesquisável e log ERROR;
  - confirmação única por lançamento, com ownership exclusivo do gate HITL;
  - heurística que não confunde BRL com múltiplos lançamentos;
  - retry transitório limitado, idempotente e retomável;
  - `money.BRL()` como fonte única de formatação;
  - pergunta explícita de forma de pagamento em despesa sem forma declarada, sem inferência indevida.
</rules>

<acceptance_criteria>
- Todos os requisitos funcionais da spec estão implementados e verificáveis.
- Todos os critérios de aceite das tasks estão implementados e verificáveis.
- Todas as regras de negócio estão implementadas e verificáveis.
- DoD 100% atendido com evidência real.
- Nenhum gap.
- Nenhuma lacuna.
- Nenhum falso positivo.
- Nenhuma ressalva.
- Nenhum risco residual.
- Nenhuma validação crítica faltando.
- Veredito final obrigatório: `APPROVED`.
</acceptance_criteria>

<method>
- Monte primeiro uma matriz de rastreabilidade:
  - uma linha por RF;
  - uma linha por critério de aceite/sucesso das task files;
  - uma linha por regra de negócio material do PRD, techspec e ADRs.
- Para cada linha da matriz, preencha:
  - identificador;
  - descrição curta do requisito;
  - arquivos/evidências lidas;
  - status: `atendido`, `não atendido` ou `não verificável`;
  - justificativa objetiva.
- Se qualquer item ficar como `não atendido` ou `não verificável`, gere achado bloqueante e acione bugfix.
- Após cada bugfix, revise novamente o delta de remediação e revalide a matriz completa dos itens afetados e dos contratos relacionados.
- Antes de declarar `APPROVED`, reexecute a checagem de inconsistências para garantir `0 falsos positivos`, `0 lacunas` e `0 gaps`.
</method>

<output>
Responda em markdown com as seções abaixo, nesta ordem:

1. `Status`
   - valor único final: `APPROVED`, `REJECTED`, `APPROVED_WITH_REMARKS` ou `BLOCKED`
2. `Escopo Revisado`
   - arquivos, specs e evidências efetivamente lidos
3. `Matriz de Rastreabilidade`
   - tabela com todos os RFs, critérios de aceite das tasks e regras de negócio relevantes
4. `Achados`
   - lista estruturada com severidade, arquivo, linha, impacto, evidência e correção sugerida
   - se não houver achados, escreva explicitamente `Sem achados`
5. `Validações Executadas`
   - comandos executados/consultados e resultado
6. `Decisão`
   - justificativa objetiva do veredito
7. `Próxima Ação`
   - se houver achados: instrução explícita para executar bugfix
   - se não houver achados: `Nenhuma; implementação aprovada`

Regras do output:
- Não omita itens da matriz.
- Não resuma requisitos em blocos genéricos.
- Não marque `APPROVED` se existir qualquer item fora de `atendido`.
- Não produza ressalvas no caso de `APPROVED`.
</output>

<example>
Entrada esperada:
- Spec `.specs/prd-registro-conversacional-robustez`
- Código atual do workspace
- Diff ou arquivos relevantes da implementação

Saída esperada em caso de problema:
- `Status: REJECTED`
- matriz com itens `não atendido` ou `não verificável`
- achados convertíveis para bugfix
- instrução explícita para rodar `.claude/skills/bugfix/SKILL.md`

Saída esperada em caso ideal:
- `Status: APPROVED`
- todos os itens da matriz como `atendido`
- `Achados: Sem achados`
- `Próxima Ação: Nenhuma; implementação aprovada`
</example>
```

## Justificativas das Adicoes

- Estrutura em XML: atende a regra dura de prompts complexos definida no `AGENTS.md`.
- Contexto com caminhos reais: evita alucinação e força confronto com os artefatos corretos da spec.
- Matriz de rastreabilidade: transforma `0 gaps`, `0 lacunas` e `DoD 100%` em verificação auditável.
- Regras de evidência: bloqueiam aprovação baseada em intenção, relatório ou narrativa sem prova no código.
- Escopo explícito de subagentes: melhora a qualidade da revisão sem misturar review com implementação.
- Output determinístico: facilita auditoria, repetição do ciclo e handoff estruturado para `bugfix`.

## Variante Curta

Se quiser uma versão mais enxuta para colar diretamente em outro agente, use apenas a seção `Prompt Enriquecido`.
