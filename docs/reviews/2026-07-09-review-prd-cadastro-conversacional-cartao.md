# Review PRD - Cadastro Conversacional Cartao

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-cadastro-conversacional-cartao
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

## Ambiguidades resolvidas no enriquecimento

1. O alvo de validação foi explicitado como a pasta `.specs/prd-cadastro-conversacional-cartao`, com leitura obrigatória de `prd.md`, `techspec.md`, `tasks.md` e task files relacionadas.
2. O escopo de revisão foi definido contra o working tree/diff atual, sem executar correções.
3. O ciclo `review -> bugfix -> review` foi mantido apenas como instrução operacional futura dentro do prompt, mas com bloqueio explícito de implementação nesta execução.
4. O formato de saída foi fechado para evitar respostas vagas, falsos positivos e pareceres sem evidência.

## Prompt enriquecido

```xml
<goal>
Obter um veredito final APPROVED, por evidência estrita e sem flexibilização, para a implementação relacionada a `.specs/prd-cadastro-conversacional-cartao`, validando integralmente PRD, techspec, tasks, critérios de aceite, DoD e regras de negócio.
</goal>

<context>
- Repositório: `/Users/jailtonjunior/Git/mecontrola`
- Especificação alvo: `.specs/prd-cadastro-conversacional-cartao`
- Documentos mínimos obrigatórios: `prd.md`, `techspec.md`, `tasks.md` e todos os `task-*.md` pertinentes ao diff revisado
- Skill principal: `@.claude/skills/review/`
- Skill de remediação, se houver achados reais: `@.claude/skills/bugfix/`
- Regras canônicas do repositório: `AGENTS.md`
- Restrições do pedido atual: nesta execução, não implementar nada; apenas revisar ou, se este prompt estiver sendo reutilizado futuramente por outro agente, instruir o ciclo operacional de remediação sem flexibilização
</context>

<task>
1. Execute a skill `@.claude/skills/review/` com postura de dono do código e validação estrita contra `.specs/prd-cadastro-conversacional-cartao`.
2. Confronte, um a um, todos os requisitos funcionais, fluxos, critérios de sucesso, critérios de aceite, regras de negócio, restrições técnicas, fora de escopo relevantes para evitar falso positivo e Definition of Done.
3. Revise apenas com base em evidência observável no código, testes, wiring, diffs, artefatos e validações reais do workspace.
4. Se houver qualquer problema real, inconsisência, ausência de implementação, cobertura insuficiente, violação arquitetural, lacuna de evidência ou requisito não atendido, registre como finding e marque o veredito apropriado; não force APPROVED.
5. Se este prompt for executado em um fluxo operacional com permissão para correção, acione `@.claude/skills/bugfix/` somente para findings reais e depois repita o ciclo `review -> bugfix -> review` até obter `APPROVED`.
6. Dispare subagentes especializados quando isso aumentar a qualidade da revisão, especialmente para:
   - cobertura de requisitos PRD vs código
   - validação de fluxos conversacionais/agentes
   - validação de domínio Go
   - validação de testes e harness
   - validação de wiring, observabilidade, idempotência e guardrails
7. Nesta execução específica solicitada pelo usuário, não implemente nenhuma correção; produza apenas a revisão ou o prompt operacional de revisão.
</task>

<rules>
- Não flexibilize critérios.
- Não invente evidência ausente.
- Não marque item como atendido sem apontar prova concreta.
- Não gere falsos positivos.
- Não gere ressalvas cosméticas se não houver impacto real.
- Não trate ausência de evidência como conformidade.
- Não ignore critérios de aceite mesmo que o diff não toque diretamente os arquivos esperados.
- Não implemente nada nesta execução.
- Se estiver incerto, declare explicitamente a incerteza e classifique como não verificável ou risco residual, conforme a skill de review.
- Toda conclusão deve citar arquivo(s), evidência técnica e vínculo com requisito(s) da especificação.
</rules>

<mandatory_criteria>
- Todos os critérios de aceite devem estar atendidos e implementados.
- DoD deve estar 100% atendido e implementado.
- Deve haver 0 gaps.
- Deve haver 0 lacunas.
- Deve haver 0 falsos positivos.
- Deve haver 0 ressalvas no resultado final APPROVED.
- Todas as regras de negócio da especificação devem estar atendidas e implementadas.
- O resultado final só pode ser `APPROVED` quando houver conformidade total com a especificação.
</mandatory_criteria>

<review_protocol>
- Ler `AGENTS.md` e aplicar as regras duras relevantes.
- Seguir estritamente o procedimento de `@.claude/skills/review/`.
- Ler obrigatoriamente:
  - `.specs/prd-cadastro-conversacional-cartao/prd.md`
  - `.specs/prd-cadastro-conversacional-cartao/techspec.md`
  - `.specs/prd-cadastro-conversacional-cartao/tasks.md`
  - task files `task-*.md` relacionadas ao escopo revisado
- Mapear cada RF do PRD para evidência concreta de implementação ou finding.
- Mapear critérios de sucesso e cenários de experiência do usuário para evidência concreta de implementação/teste.
- Verificar guardrails arquiteturais, idempotência, persistência de erro real, métricas, estado de espera, TTL, exclusão mútua, slot-filling, confirmação explícita, banco reconhecido vs não reconhecido, e proibição de respostas sem tool call.
- Validar se os testes existentes realmente cobrem a regressão do incidente e o gate estatístico/harness exigido.
- Verificar se o comportamento implementado respeita o fora de escopo do PRD.
</review_protocol>

<output_format>
Responder em Markdown com as seções exatas abaixo:

1. `Veredito`
   - Um de: `BLOCKED`, `REJECTED`, `APPROVED_WITH_REMARKS`, `APPROVED`

2. `Escopo Revisado`
   - Diff/base revisada
   - Arquivos revisados
   - Documentos da especificação carregados
   - Subagentes acionados e objetivo de cada um

3. `Matriz de Conformidade`
   - Tabela com colunas:
     - `Item`
     - `Tipo` (`RF`, `Critério de aceite`, `DoD`, `Regra de negócio`, `Restrição técnica`)
     - `Status` (`atendido`, `não atendido`, `não verificável`)
     - `Evidência`
     - `Arquivos`

4. `Findings`
   - Lista de achados reais no formato canônico da skill review
   - Incluir severidade, arquivo, linha quando aplicável, impacto e correção sugerida
   - Se não houver achados, escrever exatamente `Sem achados.`

5. `Riscos Residuais`
   - Listar somente riscos reais restantes
   - Se não houver, escrever exatamente `Sem riscos residuais.`

6. `Validações`
   - Comandos executados ou evidências consultadas

7. `Decisão Final`
   - Explicar por que o veredito é o único compatível com a evidência
   - Só emitir `APPROVED` se todos os itens estiverem atendidos, implementados e sem ressalvas
</output_format>

<bugfix_loop>
Se o veredito não for `APPROVED` e este prompt estiver sendo usado num fluxo com permissão de correção:
1. Converter cada finding acionável em entrada para `@.claude/skills/bugfix/`
2. Corrigir apenas a causa raiz de cada finding real
3. Reexecutar a review sobre o delta da remediação
4. Repetir até `APPROVED`
5. Encerrar somente quando houver:
   - 0 gaps
   - 0 lacunas
   - 0 falsos positivos
   - 0 ressalvas
   - 100% de conformidade com `.specs/prd-cadastro-conversacional-cartao`
</bugfix_loop>

<done_definition>
Considere concluído somente quando a revisão demonstrar, por evidência concreta, que a implementação está 100% aderente à especificação `.specs/prd-cadastro-conversacional-cartao` e o resultado final for `APPROVED` sem falsos positivos, sem lacunas e sem ressalvas.
</done_definition>
```

## Justificativas das adições

- Estrutura em XML: atende a regra dura do repositório para prompts multi-parte.
- Escopo documental explícito: evita revisão parcial da especificação.
- Matriz de conformidade: força rastreabilidade requisito -> evidência.
- Regras negativas e positivas: reduzem falso positivo e aprovação sem prova.
- Protocolo de bugfix isolado: preserva a intenção do usuário sem executar implementação nesta entrega.
- Formato de saída fechado: evita respostas vagas e facilita auditoria.
