# Prompt original x prompt enriquecido — 2026-07-02

| Prompt original | Prompt enriquecido |
|---|---|
| Analisar minuciosamente `.specs/prd-mecontrola-agent-tools`, confrontar com uma conversa real acessada via `ssh root@187.77.45.48`, identificar o usuário com `select * from users u where u.id = '06edc407-4f63-42e8-b07c-946b9ef0a19c';`, confrontar com o codebase e concluir se cobre todos os cenários com robustez, economia, eficiência e zero gaps/falso positivo. | Prompt estruturado com escopo fechado, fontes obrigatórias, regras anti-alucinação, passos objetivos de execução, critérios de aceite mensuráveis, formato de saída determinístico e regra explícita de que só pode declarar **PRONTO PARA USO** se não existir nenhum gap, ressalva, lacuna, bloqueio ou divergência evidenciada. |

## Prompt enriquecido — pronto para uso

```md
Você deve executar uma análise exclusivamente investigativa e crítica. Não implemente, não edite, não gere código, não abra PR, não proponha “próximos passos” flexíveis e não suavize conclusão.

## Objetivo

Avaliar de forma minuciosa, criteriosa e determinística se `.specs/prd-mecontrola-agent-tools` está realmente completo e pronto para uso quando confrontado simultaneamente com:

1. o codebase real atual;
2. a conversa real do usuário no ambiente remoto;
3. as regras canônicas do repositório.

O resultado precisa responder, com evidência concreta, se a especificação cobre **TODOS** os cenários relevantes sem gaps, sem falso positivo, sem lacunas, sem ressalvas, sem desvios e sem flexibilidade interpretativa.

## Fontes obrigatórias

Use obrigatoriamente estas fontes, nesta ordem de precedência:

1. `AGENTS.md` como fonte canônica de regras do repositório.
2. `cmd/server/server.go` e `cmd/worker/worker.go` como ponto de partida obrigatório para entender o wiring real; não use `internal/platform/runtime` como ponto de partida.
3. Todo o conteúdo de `.specs/prd-mecontrola-agent-tools/`, incluindo no mínimo:
   - `prd.md`
   - `techspec.md`
   - `tasks.md`
   - `task-*.md`
   - `adr-*.md`
4. Working tree atual do codebase como fonte da verdade.
5. Conversa real no ambiente remoto acessível por:
   - `ssh root@187.77.45.48`
   - usuário-alvo identificado por:
     `select * from users u where u.id = '06edc407-4f63-42e8-b07c-946b9ef0a19c';`

## Restrições mandatórias

1. Não implemente nada. Não altere arquivos. Não proponha solução de código.
2. Não invente tabelas, logs, tools, workflows, handlers, bindings, use cases, schemas, eventos, rotas, contratos, cenários, dados ou comportamentos.
3. Se houver divergência entre PRD/specs e o codebase atual, o working tree atual prevalece; registre o drift explicitamente.
4. Se o acesso SSH, banco, logs, histórico de conversa ou qualquer evidência necessária não estiver disponível, marque como `BLOCKED` e explique o bloqueio com precisão. Não preencha lacunas por suposição.
5. Não conclua “pronto”, “coberto”, “robusto” ou “sem gaps” sem evidência rastreável.
6. Não use linguagem diplomática para esconder falhas. Se existir 1 gap, 1 ressalva, 1 dependência não comprovada ou 1 divergência material, a conclusão final NÃO pode ser “PRONTO PARA USO”.

## O que precisa ser confrontado

Você deve confrontar, de forma cruzada e evidenciada:

1. **PRD/specs vs codebase**
   - requisitos funcionais;
   - tasks e dependências;
   - ADRs e decisões técnicas;
   - cobertura real de tools;
   - bindings, workflows, scorers, gates destrutivos, idempotência, observabilidade e anti-simulação.

2. **PRD/specs vs conversa real**
   - intenções reais do usuário;
   - variações de linguagem natural;
   - pedidos ambíguos;
   - casos de leitura, escrita, confirmação e erro;
   - cenários destrutivos/sensíveis;
   - cenários em que o agente poderia responder sem tool, escolher tool errada, simular sucesso ou deixar gap operacional.

3. **Conversa real vs codebase**
   - se o que o usuário realmente pede é atendido pelas tools e fluxos previstos;
   - se existe cenário real não coberto no PRD;
   - se o PRD cobre algo que o codebase não sustenta;
   - se há risco de falso positivo de cobertura;
   - se há sobrecobertura desnecessária, desperdício de superfície, redundância ou desenho não econômico.

## Critérios obrigatórios de avaliação

Avalie explicitamente, com evidência, cada eixo abaixo:

1. **Cobertura funcional total**
   - todos os cenários relevantes estão cobertos?
   - existe algum cenário real da conversa que ficou sem tool/fluxo compatível?
   - existe qualquer requisito do PRD sem sustentação real no código?

2. **Robustez**
   - o desenho aguenta happy path, input incompleto, ambiguidade, confirmação destrutiva, idempotência, erro de integração e retomada?
   - existe risco de fluxo quebrado, falso sucesso, tool errada, confirmação insuficiente ou estado órfão?

3. **Economia**
   - a superfície de tools está enxuta e sem duplicação?
   - existe tool redundante, overlap desnecessário ou complexidade sem ganho?

4. **Eficiência**
   - o desenho evita passos desnecessários?
   - coleta apenas o dado faltante?
   - reaproveita corretamente bindings/use cases/workflows reais?

5. **Anti-falso-positivo**
   - há algum ponto em que seria fácil declarar cobertura sem execução real?
   - há algum requisito “nominalmente coberto”, mas não operacionalmente provado?

6. **Pronto para uso**
   - o pacote completo (`prd.md`, `techspec.md`, `tasks.md`, ADRs e task files) está realmente fechando o problema ponta a ponta?
   - existe qualquer lacuna que impeça afirmar “pronto para uso” sem ressalvas?

## Procedimento obrigatório

### Etapa 1 — Inventário canônico

1. Leia `AGENTS.md`.
2. Parta de `cmd/server/server.go` e `cmd/worker/worker.go`.
3. Identifique no codebase real:
   - agente(s) relevantes;
   - registro real de tools;
   - bindings disponíveis;
   - use cases reais;
   - workflows de confirmação;
   - scorers;
   - mecanismos de observabilidade/auditabilidade;
   - qualquer evidência já existente de anti-simulação e idempotência.

### Etapa 2 — Leitura exaustiva da spec

Leia e consolide todo o conteúdo de `.specs/prd-mecontrola-agent-tools/` em uma matriz rastreável:

- requisito/decisão/tarefa;
- evidência na spec;
- artefato de código correspondente;
- status: `coberto`, `parcial`, `ausente`, `contradito`.

### Etapa 3 — Confronto com a conversa real

No host remoto:

1. Acesse via `ssh root@187.77.45.48`.
2. Use a query fornecida para localizar o usuário-alvo.
3. Descubra, sem assumir nomes, onde a conversa real, histórico de mensagens, runs, tool calls, logs ou artefatos equivalentes ficam persistidos.
4. Extraia apenas o necessário para reconstruir os cenários reais relevantes desse usuário.
5. Classifique cada interação real em:
   - intenção primária;
   - entidade alvo;
   - necessidade de leitura/escrita;
   - necessidade de confirmação;
   - tool esperada;
   - risco de erro de seleção;
   - cobertura pelo PRD;
   - cobertura pelo codebase.

Se a conversa real não puder ser obtida com evidência suficiente, pare e marque `BLOCKED`.

### Etapa 4 — Matriz de cobertura ponta a ponta

Monte uma matriz única cruzando:

- cenário real observado;
- requisito(s) do PRD/techspec/tasks/ADRs relacionados;
- tool/fluxo/workflow/binding/use case real correspondente no codebase;
- status final: `ok`, `gap`, `parcial`, `contradito`, `blocked`.

Nenhum cenário relevante pode ficar fora da matriz.

### Etapa 5 — Teste lógico de prontidão

Aplique a seguinte regra binária:

- **PRONTO PARA USO**: somente se todos os cenários relevantes estiverem cobertos de forma real, consistente e rastreável, sem gaps, sem contradições, sem dependência implícita, sem falso positivo, sem redundância material e sem qualquer ressalva.
- **NÃO PRONTO PARA USO**: se existir qualquer gap, cobertura parcial, divergência, dependência não comprovada, fragilidade, sobreposição ruim, risco operacional, blocker ou ausência de evidência.
- **BLOCKED**: se faltar acesso ou evidência indispensável para concluir com rigor.

## Formato de saída obrigatório

Responda em **pt-BR** e em **Markdown**, com esta estrutura exata:

1. `## Veredito Executivo`
   - valor único: `PRONTO PARA USO`, `NÃO PRONTO PARA USO` ou `BLOCKED`
   - justificativa curta e objetiva, sem diplomacia

2. `## Matriz de Cobertura`
   - tabela com colunas:
     - `Cenário real`
     - `Fonte da conversa`
     - `Requisito/spec relacionado`
     - `Evidência no codebase`
     - `Tool/fluxo esperado`
     - `Status`
     - `Gap ou blocker`

3. `## Achados Críticos`
   - apenas achados materiais
   - separar em:
     - `Gaps`
     - `Contradições`
     - `Fragilidades`
     - `Excessos/Redundâncias`
     - `Riscos de falso positivo`

4. `## Avaliação por Eixo`
   - cobertura funcional total
   - robustez
   - economia
   - eficiência
   - anti-falso-positivo
   - pronto para uso
   - para cada eixo: `PASSA` ou `FALHA`, com evidência objetiva

5. `## Drift entre spec e codebase`
   - listar somente divergências comprovadas

6. `## Conclusão Final`
   - frase final única e categórica
   - proibido concluir como “pronto” com ressalva

## Regras finais inegociáveis

1. Zero flexibilidade interpretativa.
2. Zero conclusão por aproximação.
3. Zero “parece cobrir”.
4. Zero “provavelmente”.
5. Zero “pronto com pequenos ajustes”.
6. Ou está provado como pronto para uso, ou não está.
```

## Justificativa curta das adições

| Adição | Motivo |
|---|---|
| Fontes obrigatórias e ordem de precedência | Evita análise solta e força confronto entre spec, codebase real e conversa real. |
| Regras anti-alucinação e bloqueio por falta de evidência | Impede falso positivo e conclusão baseada em inferência. |
| Procedimento em etapas | Torna a execução determinística e auditável. |
| Matriz única de cobertura | Garante rastreabilidade cenário → spec → código → veredito. |
| Regra binária de prontidão | Impede “pronto com ressalvas”, alinhando com zero gaps e zero lacunas. |
| Formato de saída fechado | Força resposta objetiva, comparável e pronta para uso. |
