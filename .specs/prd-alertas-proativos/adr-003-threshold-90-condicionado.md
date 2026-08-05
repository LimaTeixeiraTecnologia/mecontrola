# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Threshold 90 condicionado a migration e modelagem
- **Data:** 2026-08-05
- **Status:** Aceita
- **Decisores:** Produto e engenharia MeControla
- **Relacionados:** `.specs/prd-alertas-proativos/prd.md`, `.specs/prd-alertas-proativos/techspec.md`

## Contexto

O codebase atual aceita thresholds 80 e 100 no value object e em constraints históricas. Incluir 90 como comportamento real altera domínio e persistência.

## Decisão

Threshold 90 não entra na primeira implementação sem migration, revisão do value object e validação com `postgresql-production-standards`.

## Alternativas Consideradas

- Forçar 90 apenas em memória: rejeitada porque criaria divergência entre domínio e persistência.
- Tratar 90 como texto derivado de 80/100: rejeitada porque criaria falso positivo de alerta.
- Fazer migration na primeira fatia: rejeitada por aumentar risco antes do dry-run e do contrato de template.

## Consequências

### Benefícios Esperados

- Evita estado inválido entre código e banco.
- Reduz risco no primeiro corte.
- Mantém Release 1 executável com 80 e 100.

### Trade-offs e Custos

- O template de 90 pode estar criado na Meta, mas seu uso fica bloqueado no produto até a modelagem e persistência aceitarem 90.

### Riscos e Mitigações

- Risco: produto esperar 90 no primeiro release. Mitigação: registrar bloqueio no PRD, TechSpec e tasks.

## Plano de Implementação

1. Implementar Release 1 sem 90.
2. Abrir tarefa específica para VO, migration, constraints e testes.
3. Liberar template 90 somente após essa tarefa.

## Monitoramento e Validação

- Testes de domínio e persistência aceitando 90.
- Migração validada com rollback.

## Impacto em Documentação e Operação

Runbook deve marcar `mecontrola_category_threshold_90` como bloqueado até a migration estar em produção.

## Revisão Futura

Revisar após a primeira rodada de dry-run e aprovação dos templates Meta.
