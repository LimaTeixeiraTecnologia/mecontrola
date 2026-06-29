# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** `internal/agents` como consumidor de domínio sobre `internal/platform`
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma; solicitante do produto
- **Relacionados:** PRD `.specs/prd-agents-weather-mastra/prd.md` (RF-01..RF-08), techspec.md, ADR-004

## Contexto

O substrato genérico vive em `internal/platform` (agent, memory, scorer, tool, workflow, llm). O exemplo weather do Mastra precisa ser portado como módulo de domínio vivo, sem reintroduzir mecanismo na camada de domínio nem reviver o módulo financeiro descontinuado (`internal/agent`). A regra de layering (R-WF-KERNEL-001, R-AGENT-WF-001) exige que módulos de domínio consumam a plataforma, nunca o contrário, e que o kernel permaneça puro.

## Decisão

`internal/agents` é um **consumidor de domínio** que importa apenas `internal/platform/*` (e libs externas como `httpclient`), montando agent+tool+workflow+scorers+memória+runtime por DI manual em `module.go`. É **proibido** importar `internal/agent` (que será apagado) e **proibido** reimplementar primitivos da plataforma. O weather é o consumidor de referência vivo e o padrão canônico para futuros módulos de domínio.

## Alternativas Consideradas

- **Colocar o weather dentro de `internal/platform`**: rejeitada — viola a fronteira (domínio na plataforma); o open-meteo é IO de domínio do consumidor.
- **Manter como `test/conformance` apenas**: rejeitada — não valida no WhatsApp real nem estabelece o padrão de consumo em produção.
- **Reaproveitar a estrutura de `internal/agent`**: rejeitada — o módulo será eliminado (ADR-004) e carrega semântica financeira.

## Consequências

### Benefícios Esperados
- Fronteira arquitetural íntegra e verificável por gate de import.
- Padrão reutilizável de consumo da plataforma.
- Prova viva da plataforma no canal oficial.

### Trade-offs e Custos
- Alguma duplicação de wiring por módulo consumidor (aceitável; DI explícito).

### Riscos e Mitigações
- Risco: vazamento de domínio para a plataforma sob pressão. Mitigação: gates grep de import e revisão.

## Plano de Implementação

1. Criar `internal/agents/{domain,application,infrastructure}` + `module.go`.
2. Consumir construtores da plataforma (provider, tool, agent, runtime, memory, scorer, engine).
3. Gate: `grep internal/agent` (≠ platform) vazio.

## Monitoramento e Validação

- Build verde; gates de import/governança verdes; conformidade weather verde.

## Impacto em Documentação e Operação

- README/runbook do módulo `internal/agents`; atualizar índice de specs.

## Revisão Futura

- Revisar quando um segundo módulo de domínio consumir a plataforma (validar reuso do padrão).
