# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Fronteiras de pacote e layering unidirecional em `internal/platform`
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-23, RF-31..RF-34), techspec, ADR-001, `R-WF-KERNEL-001`, `R-AGENT-WF-001`

## Contexto

Com `internal/agent` descontinuado, Thread/Run/WorkingMemory/PendingStep e o primitivo Agent passam a viver em `internal/platform` como mecanismos genéricos. É necessário definir fronteiras de pacote que (a) preservem a pureza do kernel, (b) impeçam vazamento de domínio para a plataforma e (c) evitem inversão de dependência (kernel importando camada superior).

## Decisão

Adotar layering unidirecional em pacotes irmãos:

- **Camada 0 (kernel):** `internal/platform/workflow` — não importa nenhuma camada superior nem domínio.
- **Camada 1 (provider):** `internal/platform/llm` — provider OpenRouter genérico (complete/stream/embed); sem domínio.
- **Camada 2 (primitivos):** `internal/platform/tool`, `internal/platform/memory`, `internal/platform/agent`, `internal/platform/scorer` — consomem camada 0/1; `agent` orquestra `tool`/`memory`/`scorer`.
- **Camada 3 (consumidor):** módulos de domínio e o consumidor de referência weather — consomem camada 2.

Regra dura: dependências apontam só para baixo. O kernel é proibido de importar `internal/platform/{agent,memory,llm,scorer,tool}` (gate reemitido). Estados na fronteira são tipos fechados (state-as-type). Nenhuma coluna/label com semântica de domínio.

## Alternativas Consideradas

- **Pacote único `internal/platform/mastra`.** Vantagem: simplicidade superficial. Desvantagem: acopla kernel e LLM, quebra a proibição de LLM no kernel, dificulta o gate. Rejeitada.
- **Manter agent fora de platform (em módulo próprio).** Desvantagem: recria o acoplamento que o PRD elimina e contraria "tudo reutilizável em internal/platform". Rejeitada.

## Consequências

### Benefícios Esperados

- Pureza do kernel garantida por construção e por gate automatizável.
- Reuso por múltiplos módulos sem acoplamento.
- Fronteira de domínio defensável em revisão.

### Trade-offs e Custos

- Mais pacotes e indireção; bindings explícitos entre camadas.
- Exige disciplina de import e gates em CI.

### Riscos e Mitigações

- **Risco:** import cíclico ou inversão acidental. **Mitigação:** gates grep + revisão; camada 0/1 sem dependência de camada 2.
- **Risco:** semântica de domínio infiltrada. **Mitigação:** gate de labels/colunas; chaves opacas obrigatórias.

## Plano de Implementação

1. Criar os pacotes por camada.
2. Definir portas (interfaces no consumidor) entre camadas.
3. Reemitir gates grep de import e cardinalidade.
4. Validar em CI.

## Monitoramento e Validação

- Gates grep vazios; ausência de ciclos de import (`go build`/`go vet`).
- Critério de sucesso: RF-31..RF-34 verificáveis por gate.

## Impacto em Documentação e Operação

- `R-WF-KERNEL-001` e `R-AGENT-WF-001` referenciam estas fronteiras; techspec lista os gates finais.

## Revisão Futura

- Revisitar se um novo primitivo de plataforma exigir nova camada; registrar ADR.
