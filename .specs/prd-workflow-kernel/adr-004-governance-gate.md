# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Gate de governança — `R-WF-KERNEL-001` + addendum `R-AGENT-WF-001.6/.8`
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Plataforma, governança de regras (R-GOV-001)
- **Relacionados:** PRD (RF-22 no PRD = RF-27, RF-28, RF-29 desta techspec); `.claude/rules/agent-workflows-tools.md`; `.claude/rules/go-adapters.md`; `.claude/rules/governance.md`

## Contexto

- `R-AGENT-WF-001.6/.8` declaram, hoje, Thread/Run/WorkingMemory/PendingStep **exclusivos** de
  `internal/agent` e proíbem esses padrões fora do módulo.
- O kernel genérico vive em `internal/platform/workflow` e oferece mecanismo de Run/estado durável/
  suspend-resume — o que, sem distinção textual, colide com a regra hard vigente.
- A precedência DMMF (`governance.md`) e a política de evidência exigem resolver o conflito
  explicitamente, não normalizá-lo em silêncio.

## Decisão

- **Gate bloqueante (RF-29)**: redigir a governança **antes** de qualquer código do kernel.
- Criar `.claude/rules/workflow-kernel.md` (`R-WF-KERNEL-001`, hard) definindo o kernel genérico em
  `internal/platform/workflow`:
  - proibido import de pacote de domínio (`intent`, `agent`, `transactions`, ...);
  - proibido regra de negócio, SQL de domínio e branching de domínio dentro do kernel;
  - estados (`RunStatus`/`StepStatus`/`SuspendReason`) são tipos fechados (state-as-type);
  - métricas com cardinalidade controlada (sem `user_id`/`correlation_key`/`category_id`);
  - LLM proibido no kernel.
- Aditar `R-AGENT-WF-001.6/.8` com addendum que **distingue**:
  - "kernel genérico de workflow" (mecanismo, permitido em `internal/platform`) de
  - "workflow de intent + Thread/Run/WorkingMemory/PendingStep **semânticos**" (exclusivos de
    `internal/agent`).
  Mantém a proibição original quanto à **semântica**; permite o agent **consumir** o kernel.

## Alternativas Consideradas

- **Redigir as regras só depois (na implementação)**: risco de drift entre código e regra hard;
  rejeitada — o PRD definiu gate antes do código.
- **Só anotar como pendência**: sem força de gate; rejeitada por risco de violação silenciosa.
- **Não tocar governança**: o kernel violaria `R-AGENT-WF-001.6` ao existir fora do agent; rejeitada.

## Consequências

### Benefícios Esperados

- Fronteira inequívoca entre mecanismo genérico e semântica de domínio.
- Gates de revisão verificáveis para o kernel (grep de imports de domínio, comentários, SQL, labels).
- Sem drift entre regra hard e implementação.

### Trade-offs e Custos

- Trabalho de governança antecede o código (sequência rígida).
- Mais uma regra hard a manter.

### Riscos e Mitigações

- **Risco:** addendum ambíguo reabrir brecha para vazar semântica ao kernel. **Mitigação:** texto do
  addendum referencia explicitamente os 4 conceitos semânticos e os mantém exclusivos do agent.
- **Rollback:** governança é documental; reverter = remover a regra nova e o addendum (sem efeito em runtime).

## Plano de Implementação

1. Escrever `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001) com gates de verificação (grep).
2. Aditar `.claude/rules/agent-workflows-tools.md` (.6/.8) com a distinção mecanismo vs semântica.
3. Atualizar `.claude/rules/governance.md` (Regras de Módulo) referenciando a nova regra.
4. Conclusão: gates do kernel definidos e executáveis antes do item 2 da Ordem de Build.

## Monitoramento e Validação

- Gates (grep) no CI/pre-merge: imports de domínio no kernel, comentários proibidos, SQL direto,
  labels de cardinalidade.
- Sucesso: kernel passa nos gates de `R-WF-KERNEL-001` e o agent nos de `R-AGENT-WF-001` (incl. addendum).

## Impacto em Documentação e Operação

- `.claude/rules/` (nova regra + addendum + índice em governance.md).
- `CLAUDE.md`/`AGENTS.md`: referência à nova regra de módulo.
- Skill `mastra`: nota de precedência kernel genérico vs semântica do agent.

## Revisão Futura

- Reavaliar o addendum quando um segundo módulo consumir o kernel (validar que a distinção continua
  suficiente).
