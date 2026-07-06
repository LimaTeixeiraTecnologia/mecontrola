# Prompt de Análise Crítica — Módulo `internal/agents`

**Data de referência:** 2026-07-06
**Idioma de execução:** pt-BR
**Modo de resposta:** estrito, direto, sem flexibilidade
**Escopo:** análise apenas; nenhuma implementação deve ser realizada.

---

## Instrução Principal

Analise minuciosa e criteriosamente o módulo `internal/agents` deste repositório. O objetivo é identificar, com precisão absoluta, qualquer gap, ressalva, falso positivo, falta de robustez ou problema de integração que possa comprometer o funcionamento do projeto como um todo — especialmente a comunicação com os módulos `internal/transactions`, `internal/categories`, `internal/budgets` e `internal/card`.

A análise deve ser implacável. Não aceite superficialidade. Não ignore comportamentos que pareçam "funcionar" mas que possam falhar em produção com dados reais, concorrência, erros de rede, estados inválidos ou mudanças futuras.

---

## Contexto Obrigatório a Considerar

- O projeto é um monolito modular em Go.
- `internal/agents` é um consumidor da plataforma de agentes (`internal/platform/agent`, `internal/platform/workflow`, `internal/platform/memory`, `internal/platform/llm`, `internal/platform/tool`, `internal/platform/scorer`).
- A comunicação cross-module deve usar interfaces declaradas pelo consumidor, domain events/outbox ou contratos explícitos.
- `internal/agents` não pode implementar regras de negócio de outros módulos nem acessar repositórios alheios diretamente.
- Todo fluxo de agente deve ser auditável via `Run`, `Thread`, `RunStatus` e `ToolOutcome`.
- Estados de fronteira devem ser tipos fechados (state-as-type); strings livres em assinaturas públicas são proibidas.
- O kernel de workflow é genérico e não pode conhecer tipos semânticos de domínio.

---

## O que Deve ser Analisado

1. **Existência e correção de artefatos**
   - Agentes, tools, workflows, scorers, memory/recall, routers, jobs, consumers, producers.
   - Cada artefato está no diretório correto? Segue o padrão de módulo?
   - Há código morto, stubs, placeholders, comentários temporários ou TODOs não resolvidos?

2. **Fronteiras arquiteturais**
   - `internal/agents` importa apenas o que lhe é permitido?
   - Existe vazamento de `internal/transactions`, `internal/categories`, `internal/budgets` ou `internal/card` para dentro de `internal/agents/domain`?
   - Existe vazamento de `internal/agents` para a camada `domain` de outros módulos?

3. **Integração com módulos de negócio**
   - Como `internal/agents` consulta ou comanda `internal/transactions`?
   - Como interage com `internal/categories`, `internal/budgets` e `internal/card`?
   - As interfaces são declaradas pelo consumidor? Os adapters são finos?
   - Há acoplamento indevido a structs concretas de outros módulos?
   - Há chamadas diretas a repositórios, banco ou HTTP de outros módulos?

4. **Robustez e produção**
   - Tratamento de erro: erros são propagados corretamente? Há `panic`? Há erros ignorados?
   - Idempotência: execuções repetidas produzem o mesmo resultado?
   - Concorrência: execuções paralelas do mesmo `thread` são seguras?
   - Timeouts e cancellation via `context.Context` estão presentes em toda fronteira de IO?
   - Retry, circuit breaker ou políticas de resiliência estão aplicadas onde necessário?
   - Logs estruturados com `log/slog`? Métricas? Rastreamento?

5. **Workflow e agent runtime**
   - O loop tool-calling termina corretamente? Há risco de loop infinito?
   - Estados suspensos (`SuspendReason`) são recuperáveis sem perda de contexto?
   - Resume por merge-patch é seguro contra estados corrompidos?
   - `PendingStep` e `WorkingMemory` estão modelados como tipos fechados?

6. **LLM e tools**
   - O provider é OpenRouter exclusivamente? Há fallback indevido?
   - As tools são adapters finos? Delegam para use cases?
   - Os schemas de tool são válidos e versionados?
   - Há prompt injection, vazamento de dados sensíveis ou comportamento não determinístico?

7. **Testes e evidências**
   - Existem testes unitários, de integração e/ou e2e cobrindo o módulo?
   - Há mocks gerados? Os testes são determinísticos?
   - A cobertura é mínima ou há caminhos não testados?

---

## Formato de Saída Obrigatório

A resposta deve ser um relatório técnico estruturado em Markdown com as seções abaixo, na ordem exata:

### 1. Sumário Executivo
- Status geral: **PRONTO PARA MAIN**, **CONDIÇIONAL** ou **NÃO PRONTO**.
- Nota geral de 0 a 10 para robustez.
- Justificativa em até 5 linhas.

### 2. O que Está Bom
- Lista numerada de pontos fortes objetivos e verificáveis.
- Cada item deve indicar o arquivo/pacote relevante quando aplicável.

### 3. Gaps Encontrados
- Lista numerada. Para cada gap:
  - Descrição clara.
  - Severidade: **CRÍTICO**, **ALTO**, **MÉDIO** ou **BAIXO**.
  - Localização: caminho do arquivo e/ou pacote.
  - Impacto no projeto e na comunicação com outros módulos.
  - Evidência (linha de código, assinatura, padrão violado).

### 4. Ressalvas
- Lista numerada de comportamentos que funcionam mas geram risco ou dívida técnica.

### 5. Falsos Positivos
- Lista numerada de coisas que parecem corretas/proteções mas que, sob análise, não garantem robustez real.

### 6. Plano de Ação
- Lista numerada e ordenada por prioridade (mais crítico primeiro).
- Cada ação deve ter:
  - Descrição exata do que deve ser feito.
  - Responsável sugerido: **backend**, **arquiteto**, **QA**, **DevOps**, **produto**.
  - Estimativa de esforço: **XS**, **S**, **M**, **L**, **XL**.
  - Critério de aceitação mensurável.

### 7. Parecer sobre Produção
- Resposta direta: **pode ser usado por usuários reais agora** ou **não pode**.
- Se condicional, liste os pré-requisitos exatos que devem ser cumpridos antes.

### 8. Registro de Suposições
- Se alguma informação não pôde ser verificada, registre aqui o que foi assumido e por quê.

---

## Restrições Rígidas

- Não implemente nenhuma alteração de código.
- Não sugira ferramentas, bibliotecas ou serviços externos não presentes no repositório.
- Não adicione flexibilidade ao parecer: seja categórico.
- Não omita problemas por parecerem pequenos.
- Use apenas fatos observáveis no código, em testes, em configs ou em documentação do repositório.
- Se um arquivo não existir, registre isso como gap.
- A data de referência desta análise é 2026-07-06.

---

## Prompt de Execução para o Agente

> Execute a análise acima no módulo `internal/agents` e nos pontos de contato com `internal/transactions`, `internal/categories`, `internal/budgets` e `internal/card`. Produza o relatório no formato especificado. Seja implacável: 0 gaps tolerados, 0 falsos positivos aceitos, 0 lacunas sem registro.
