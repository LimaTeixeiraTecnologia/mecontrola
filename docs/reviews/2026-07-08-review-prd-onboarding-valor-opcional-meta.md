# Review PRD - Onboarding Valor Opcional Meta

## Metadados

- Data: `2026-07-08`
- Skill aplicada: `prompt-enricher`
- Alvo da revisão futura: `.specs/prd-onboarding-valor-opcional-meta`
- Skills a invocar pelo executor do prompt:
  - `.claude/skills/review/SKILL.md`
  - `.claude/skills/bugfix/SKILL.md`

## Prompt Original

```text
Objetivo:
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-onboarding-valor-opcional-meta
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

- `DoD 100% atendido` foi interpretado como confronto obrigatório contra `prd.md`, `techspec.md`, `tasks.md` e `task-*.md` da spec alvo.
- `Validando estritamente contra .specs/prd-onboarding-valor-opcional-meta` foi expandido para exigir confronto contra código real, testes, validações e evidências locais, sem confiar apenas em relatórios declarativos.
- O ciclo `review -> bugfix -> review` foi mantido como instrução para o executor futuro do prompt; este arquivo não executa revisão nem correção.
- O veredito final foi restringido a `APPROVED` somente quando não houver achados, lacunas, riscos residuais ou pendências.

## Prompt Enriquecido

```xml
<goal>
Obter um veredito final `APPROVED` para a implementação da spec `.specs/prd-onboarding-valor-opcional-meta`, com evidência objetiva de que 100% dos requisitos, critérios de aceite, DoD e regras de negócio foram implementados, sem gaps, sem lacunas, sem falsos positivos, sem ressalvas e sem divergência em relação ao código real.
</goal>

<context>
- Repositório: `/Users/jailtonjunior/Git/mecontrola`
- Fonte canônica de governança: `AGENTS.md`
- Skill de revisão obrigatória: `.claude/skills/review/SKILL.md`
- Skill de remediação obrigatória quando houver achados: `.claude/skills/bugfix/SKILL.md`
- Spec raiz a confrontar de forma estrita: `.specs/prd-onboarding-valor-opcional-meta`
- Documentos mínimos da spec:
  - `.specs/prd-onboarding-valor-opcional-meta/prd.md`
  - `.specs/prd-onboarding-valor-opcional-meta/techspec.md`
  - `.specs/prd-onboarding-valor-opcional-meta/tasks.md`
  - `.specs/prd-onboarding-valor-opcional-meta/task-1.0-fundacao-dominio-constructor-estado.md`
  - `.specs/prd-onboarding-valor-opcional-meta/task-2.0-schemas-extracao-prompts.md`
  - `.specs/prd-onboarding-valor-opcional-meta/task-3.0-buildgoalstep-reestruturacao.md`
  - `.specs/prd-onboarding-valor-opcional-meta/task-4.0-conclusao-persistencia-mensagem.md`
  - `.specs/prd-onboarding-valor-opcional-meta/task-5.0-harness-real-llm-gate.md`
  - `.specs/prd-onboarding-valor-opcional-meta/adr-001-extracao-combinada-sentinela.md`
  - `.specs/prd-onboarding-valor-opcional-meta/adr-002-estado-valor-sentinela-flag.md`
  - `.specs/prd-onboarding-valor-opcional-meta/adr-003-gate-real-llm-gpt4o-mini.md`
- Execution reports e orchestration reports podem ser usados apenas como apoio de rastreabilidade, nunca como prova suficiente de implementação.
- O alvo da revisão é o working tree atual e, quando aplicável, o diff relevante da implementação desta spec.
</context>

<task>
1. Carregue e siga `AGENTS.md` sem flexibilização.
2. Execute a skill `.claude/skills/review/SKILL.md` com foco em corretude, regressão, cobertura de requisitos e evidência real de implementação.
3. Valide estritamente a implementação contra toda a spec alvo, confrontando código, testes, prompts, schemas, persistência, comportamento e validações.
4. Se surgir qualquer achado, converta os bugs para o formato canônico esperado pela skill de bugfix e acione `.claude/skills/bugfix/SKILL.md`.
5. Repita obrigatoriamente o ciclo `review -> bugfix -> review` até alcançar `APPROVED`.
6. Use subagentes especializados quando isso aumentar a qualidade da revisão, especialmente para:
   - confronto PRD x código;
   - confronto techspec x código;
   - validação do harness real-LLM e gate de merge;
   - checagem de regressões em workflow/onboarding;
   - checagem de persistência/metadata e regras de mensagem final.
7. Não encerre com `APPROVED_WITH_REMARKS`, `REJECTED` ou `BLOCKED` como resultado final do trabalho; continue o ciclo até `APPROVED`, a menos que exista bloqueio externo incontornável, que deve ser explicitado com prova concreta.
</task>

<rules>
- Trate `prd.md`, `techspec.md`, `tasks.md` e `task-*.md` como contrato de implementação.
- Confronte cada requisito funcional RF-01 a RF-16 com evidência no código e/ou testes.
- Confronte todos os critérios de sucesso/aceite de cada task file com evidência objetiva.
- Considere DoD atendido apenas quando todos os itens exigidos pela spec e pela skill de review estiverem implementados e validados.
- Exija evidência direta no código real, nos testes reais e nas validações reais; não aceite apenas intenção, comentário, TODO, relatório ou narrativa.
- Não gere falso positivo: se um achado não puder ser sustentado por evidência concreta, descarte-o.
- Não gere falso negativo: se houver requisito sem prova de implementação, trate como achado.
- Não aceite conformidade parcial.
- Não trate risco residual, lacuna de teste, cobertura incompleta, dúvida de comportamento, drift documental ou validação ausente como algo aceitável para `APPROVED`.
- Não implemente correções diretamente fora da skill de bugfix.
- Se houver incerteza, declare explicitamente a incerteza, colete mais evidência e só então conclua.
- Sempre preferir fontes primárias locais: código do workspace, testes, specs, ADRs e comandos de validação executados localmente.
- Sempre verificar se a implementação preserva:
  - opcionalidade do valor da meta;
  - zero regressão de `step-goal`, `step-income` e `DecideIncomeCents`;
  - persistência opcional via metadata;
  - menção do valor apenas na mensagem final quando presente;
  - gate real-LLM `>= 0.90` com `openai/gpt-4o-mini`.
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
  - uma linha por regra de negócio material do PRD/techspec/ADRs.
- Para cada linha da matriz, preencha:
  - identificador;
  - descrição curta do requisito;
  - arquivos/evidências lidas;
  - status: `atendido`, `não atendido` ou `não verificável`;
  - justificativa objetiva.
- Se qualquer item ficar como `não atendido` ou `não verificável`, gere achado bloqueante e acione bugfix.
- Após cada bugfix, revise novamente apenas o delta de remediação e revalide a matriz completa dos itens afetados e dos contratos relacionados.
- Antes de declarar `APPROVED`, reexecute a checagem de inconsistências para garantir `0 falsos positivos` e `0 lacunas`.
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
- Spec `.specs/prd-onboarding-valor-opcional-meta`
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

- Estrutura em XML: atende a regra dura de prompts complexos do `AGENTS.md`.
- Goal statement explícito: reduz ambiguidade sobre o que significa sucesso.
- Contexto com caminhos concretos: evita alucinação e força confronto com os documentos corretos.
- Matriz de rastreabilidade: transforma `0 gaps` e `0 lacunas` em checagem verificável.
- Regras de evidência: impedem aprovação baseada em relatórios narrativos sem prova no código.
- Output determinístico: facilita auditoria e eventual handoff para `bugfix`.
- Exemplos de saída: reduzem variabilidade no formato final.

## Variante Curta

Se quiser uma versão mais enxuta para colar diretamente em outro agente, use apenas a seção `Prompt Enriquecido`.
