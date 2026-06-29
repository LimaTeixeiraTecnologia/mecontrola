# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Aproveitar e evoluir o kernel `internal/platform/workflow` como base da plataforma
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma, autor do PRD
- **Relacionados:** PRD `.specs/prd-platform-mastra/prd.md` (RF-11..RF-14, RF-34), techspec, ADR-002, ADR-007, `R-WF-KERNEL-001`

## Contexto

O PRD exige paridade comportamental com o Mastra para workflows por steps, com persistência durável e suspend/resume. Já existe em `internal/platform/workflow` um kernel genérico maduro: `Engine[S any]` (`engine.go`), `Step[S]`/`StepOutput[S]` e tipos fechados `RunStatus`/`StepStatus`/`SuspendReason` (`step.go`), `Store`/`Snapshot`/`StepRecord` com CAS por `version` (`store.go`), `Codec.MergePatch` (RFC 7386, `codec.go`), combinadores `Sequence/Branch/Parallel/Retry` (`combinators.go`), housekeeping e adapter Postgres. O usuário determinou explicitamente que esse kernel "deve ser aproveitado" como parte da inspiração Mastra. O módulo `internal/agent` será apagado, mas o kernel não pertence a ele.

## Decisão

Reaproveitar o kernel existente como base evolutiva, sem reescrita. As capacidades de workflow da plataforma (RF-11..RF-14) são entregues consumindo `Engine[S]`, `Step[S]` e combinadores atuais. A única evolução permitida no kernel é tornar explícito o repasse do `RuntimeContext` por `context.Context` aos steps (ADR-007), preservando todas as invariantes hard de `R-WF-KERNEL-001`: sem import de domínio nem de camada superior, sem LLM, sem SQL fora do adapter, estados como tipos fechados, cardinalidade controlada, resume por merge-patch. O agent-como-step (RF-13) é implementado como `Step[S]` que invoca o primitivo `agent` da camada superior — o kernel continua sem conhecer agent.

## Alternativas Consideradas

- **Reescrever um novo motor de workflow Mastra-like do zero.** Vantagem: liberdade de design. Desvantagem: descarta código testado e maduro, reintroduz risco em suspend/resume/CAS, contraria diretiva explícita do usuário. Rejeitada.
- **Fork do kernel dentro de `internal/platform/agent`.** Vantagem: isolamento. Desvantagem: duplicação, divergência de manutenção, quebra do reuso por outros módulos. Rejeitada.

## Consequências

### Benefícios Esperados

- Reuso imediato de durabilidade, CAS, merge-patch e housekeeping já validados.
- Menor superfície de risco; foco do esforço nos primitivos novos (agent/memory/scorer).
- Preserva a pureza do kernel e o layering.

### Trade-offs e Custos

- O design dos primitivos superiores fica restrito ao contrato genérico do kernel (`S any`, `correlationKey` opaca) — semântica de agent precisa morar na camada superior.
- Evoluções do kernel exigem rigor para não violar `R-WF-KERNEL-001`.

### Riscos e Mitigações

- **Risco:** vazar semântica de agent para o kernel ao adicionar agent-como-step. **Mitigação:** agent-como-step é um `Step[S]` no consumidor/primitivo, não no kernel; gate grep de import em CI.
- **Rollback:** evoluções do kernel são aditivas e cobertas por `engine_test.go`/`codec_test.go`; reverter o repasse de RuntimeContext não afeta o estado durável.

## Plano de Implementação

1. Congelar o contrato público atual do kernel como base.
2. Adicionar repasse de RuntimeContext via context (ADR-007) com teste de não-persistência.
3. Implementar primitivos superiores consumindo o kernel.
4. Validar invariantes via gates grep e suíte do kernel.

## Monitoramento e Validação

- Métricas do kernel preservadas (`workflow_*`).
- Gate de import vazio em CI; suíte do kernel verde.
- Critério de sucesso: RF-11..RF-14 atendidos sem alteração de assinatura pública do kernel além do RuntimeContext.

## Impacto em Documentação e Operação

- `R-WF-KERNEL-001` (gate de import estendido) e techspec referenciam esta decisão.
- Runbooks de workflow permanecem válidos.

## Revisão Futura

- Revisitar se um requisito exigir mudança incompatível no contrato do kernel (ex.: steps assíncronos persistentes), registrando novo ADR.
