# Prompt canônico — `create-prd`

Use a skill `create-prd` para produzir ou evoluir um PRD de forma mandatória, robusta, production-ready, eficiente e econômica, sem desvio de escopo e sem antecipar implementação.

## Gate obrigatório de entrada

Antes de qualquer redação, valide se eu informei claramente o diretório alvo da spec.

- Se eu **não** informar o path da spec, responda obrigatoriamente com `needs_input` pedindo exatamente:
  1. o diretório no formato `.specs/prd-<slug>/`
  2. a solicitação de produto/funcionalidade que deve virar o PRD
- Não derive slug nem escolha diretório final no escuro quando a intenção ainda estiver ambígua.

## Objetivo

Criar ou evoluir o PRD no path informado, com foco exclusivo em produto:

- problema e objetivo
- usuário/ator principal
- escopo incluído
- escopo excluído
- restrições e conformidade
- critérios de sucesso mensuráveis

## Fonte de verdade e anti-alucinação

1. Use `AGENTS.md` como fonte canônica do repositório quando restrições do projeto forem relevantes para o PRD.
2. Use sempre o working tree atual como fonte da verdade quando houver divergência entre docs antigas, prompts históricos e o estado atual do repositório.
3. Não invente contexto de negócio, personas, integrações, métricas, regulações, constraints ou requisitos que não estejam sustentados pela solicitação ou por contexto verificável.
4. Se a solicitação misturar produto com implementação, preserve a intenção de produto e mova detalhes técnicos apenas para a seção de restrições de alto nível, sem transformar o PRD em techspec.

## Regras mandatórias

1. Trabalhe somente no path `.specs/prd-<slug>/` informado ou confirmado via `needs_input`.
2. Se `.specs/prd-<slug>/prd.md` já existir, leia primeiro e evolua o artefato existente.
3. Se existirem artefatos derivados no mesmo diretório (`techspec.md`, `tasks.md`, `task-*.md`, `adr-*.md`, reports), pare com `needs_input` e peça confirmação explícita antes de editar o PRD.
4. Faça no máximo duas rodadas de esclarecimento.
5. Se ainda faltarem dados objetivos após duas rodadas, retorne `needs_input` e não redija um PRD especulativo.
6. Não implemente código, não proponha plano de execução, não escreva tasks e não tome decisão arquitetural detalhada.
7. Não crie cópias ad hoc fora de `.specs/prd-<slug>/prd.md`.

## Formato obrigatório do artefato

O PRD final deve:

1. seguir a intenção do template oficial da skill `create-prd`
2. numerar requisitos funcionais para rastreabilidade
3. manter foco no `o que` e `por que`, não no `como`
4. incluir `Suposições e Questões em Aberto` sempre que restar alguma hipótese
5. incluir `<!-- spec-version: 1 -->` no topo na primeira versão, ou incrementar ao evoluir um PRD existente

## Critérios de aceite inegociáveis

Considere o trabalho concluído apenas se:

1. o PRD estiver salvo exatamente em `.specs/prd-<slug>/prd.md`
2. as seis categorias obrigatórias estiverem cobertas de forma concreta e auditável
3. os requisitos funcionais estiverem numerados
4. qualquer drift downstream tiver sido tratado com `needs_input` antes da edição
5. o documento não contiver implementação, código, pseudo-código, plano de execução nem decisões técnicas detalhadas

## Formato de saída esperado

Responda em PT-BR e retorne apenas:

1. `status_final`: `done` ou `needs_input`
2. `spec_alvo`
3. `path_final`
4. `resumo_funcional`
5. `suposicoes_abertas`
6. `artefatos_downstream_detectados`
7. `proximos_passos` apenas se houver dependência real de esclarecimento
