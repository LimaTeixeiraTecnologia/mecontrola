<!-- spec-version: 1 -->

# Documento de Requisitos do Produto (PRD)

## Visão Geral

Esta iniciativa define a evolução da malha de testes do módulo `internal/agents` para que contratos críticos de retenção, idempotência, adapters financeiros, inventário de tools e invariantes agentivos tenham prova determinística no fluxo padrão de validação do módulo. O problema atual é que parte relevante da robustez do módulo depende de suites `integration` ou `realllm`, o que permite regressões silenciosas quando o baseline padrão roda sem ambiente externo, segredos ou provider real.

O público principal é o mantenedor do módulo `internal/agents` e, de forma secundária, a engenharia responsável por CI, qualidade e evolução do runtime agentivo. O valor da iniciativa é reduzir falso positivo de cobertura, aumentar confiança de refatoração e tornar falhas críticas detectáveis no caminho padrão de testes.

## Objetivos

- Garantir que contratos críticos de jobs, persistência idempotente, adapters financeiros, inventário de tools e invariantes agentivos sejam exercitados sem depender de rede, container ou credenciais externas.
- Fazer a suite padrão de `internal/agents` falhar quando houver drift entre comportamento prometido e comportamento efetivamente provado.
- Preservar suites `integration` e `realllm` como camadas complementares de aderência, e não como prova exclusiva de invariantes críticos.
- Reduzir o risco de regressão silenciosa em componentes de infraestrutura e bindings com transformação manual de payloads.
- Considerar a iniciativa concluída apenas quando o baseline padrão do módulo cobrir os contratos críticos definidos neste PRD e qualquer regressão nesses contratos falhar sem depender de `integration` ou `realllm`.

Métricas de sucesso:
- A suite padrão do módulo passa a exercitar contratos mínimos dos jobs de retenção/confirmação e do `write_ledger_repository`.
- O inventário real de tools do módulo e o harness de cobertura deixam de depender de contagem hardcoded divergente.
- Invariantes críticos hoje cobertos apenas por `integration` ou `realllm` passam a ter uma camada offline reproduzível.
- Regressões nesses contratos passam a falhar no fluxo padrão de teste do módulo.

## Histórias de Usuário

- Como mantenedor de `internal/agents`, quero testes determinísticos para jobs e para o `write_ledger_repository` para detectar regressões de agendamento, propagação de erro e idempotência sem depender apenas de `integration`.
- Como mantenedor de `internal/agents`, quero cobertura explícita para operações públicas ainda não provadas do `transactions_ledger_adapter` para evitar falso positivo em mapeamento, identidade e transformação de payload.
- Como mantenedor de `internal/agents`, quero que a prova de cobertura das tools derive do inventário real montado no módulo para que a suite falhe automaticamente quando novas tools forem adicionadas sem cenário correspondente.
- Como mantenedor de `internal/agents`, quero uma camada offline para invariantes críticos do agente para que parsing, roteamento e guardrails não fiquem invisíveis quando suites `realllm` forem puladas.

Personas:
- Primária: mantenedor do módulo `internal/agents`.
- Secundária: engenharia de plataforma/qualidade responsável por CI e confiabilidade dos testes.

## Funcionalidades Core

### 1. Prova determinística de jobs e write ledger

Define a necessidade de prova padrão para os jobs `ConfirmReaperJob` e `LedgerRetentionJob` e para os contratos críticos do `write_ledger_repository`, incluindo propagação de erro, defaults operacionais, tradução de erro e idempotência.

### 2. Cobertura comportamental dos adapters financeiros

Estabelece que operações públicas do `transactions_ledger_adapter` precisam de prova explícita de sucesso, erro e edge cases de identidade e transformação de payload, preservando o papel do adapter como camada fina.

### 3. Sincronização entre inventário real e harness de tools

Exige uma fonte única e verificável para cobertura de tools do módulo, impedindo que mensagens textuais ou contagens fixas declarem cobertura total quando o inventário real divergir.

### 4. Camada offline para invariantes críticos do agente

Determina que invariantes críticos de onboarding com extração combinada de objetivo e valor, honestidade em falha de tool e roteamento financeiro mínimo das tools principais tenham prova reproduzível sem provider real, mantendo suites `realllm` como evidência complementar.

## Requisitos Funcionais

- RF-01: O módulo deve possuir prova determinística no fluxo padrão para os contratos básicos de `ConfirmReaperJob` e `LedgerRetentionJob`, cobrindo nome, agenda padrão, timeout e propagação de erro.
- RF-02: O módulo deve possuir prova determinística no fluxo padrão para os contratos críticos do `write_ledger_repository`, incluindo tradução de `sql.ErrNoRows`, tratamento idempotente de `UniqueViolation`, erro genérico de banco com contexto e falha em `RowsAffected`.
- RF-03: A prova determinística de jobs e de `write_ledger_repository` deve cobrir os contratos críticos mínimos no baseline padrão e manter a suite `integration` como prova complementar, sem espelhar integralmente a malha integrada.
- RF-04: O `transactions_ledger_adapter` deve ter prova explícita para cada operação pública prioritária do escopo, cobrindo ao menos um cenário de sucesso, um de erro e um edge case relevante de transformação ou identidade.
- RF-05: Operações do `transactions_ledger_adapter` que exigem principal/autenticação devem falhar explicitamente quando a identidade inbound estiver ausente ou inválida, sem chamar o caso de uso downstream.
- RF-06: A cobertura de tools do módulo deve usar o inventário real de `tool.ID()` retornado por `buildFinancialTools` como fonte de verdade, derivando ou validando o harness contra esse conjunto.
- RF-07: A suite deve falhar quando uma tool existente no módulo não possuir cenário correspondente no harness de cobertura.
- RF-08: A cobertura deve distinguir inventário real de tools de cenários complementares de roteamento, impedindo que cenários extras mascarem cobertura incompleta.
- RF-09: Invariantes críticos de produto hoje presos a suites `integration` ou `realllm` devem ter camada offline e reproduzível no fluxo padrão do módulo.
- RF-10: A camada offline deve cobrir obrigatoriamente os invariantes de extração combinada de objetivo e valor no onboarding, honestidade em falha de tool e roteamento financeiro mínimo das tools principais.
- RF-11: Suites `integration` e `realllm` devem permanecer válidas como prova complementar e não devem ser removidas por esta iniciativa.
- RF-12: O resultado da iniciativa deve permitir que regressões nos contratos cobertos falhem no baseline padrão de testes do módulo, mesmo quando `RUN_REAL_LLM` e `OPENROUTER_API_KEY` estiverem ausentes.

## Restrições Técnicas de Alto Nível

- A iniciativa deve respeitar a arquitetura atual do módulo `internal/agents` e o princípio de adapters finos já adotado no repositório.
- A solução não deve introduzir dependência obrigatória de provider externo, rede, container ou banco real para a prova mínima dos contratos críticos cobertos por este PRD.
- Suites `integration` e `realllm` continuam fazendo parte da estratégia de qualidade, mas não podem permanecer como prova exclusiva dos invariantes críticos definidos neste documento.
- A cobertura deve se alinhar ao inventário real de tools e aos contratos públicos existentes no módulo, sem redefinir escopo funcional do agente.
- O escopo é interno de engenharia e não introduz novos papéis de negócio, permissões de usuário final ou mudanças de contrato externo de produto.

## Fora de Escopo

- Redesenhar o `write_ledger_repository`, alterar schema de banco ou mudar semântica dos jobs existentes.
- Alterar a lógica de negócio do módulo `transactions` ou redesenhar adapters apenas por conveniência de teste.
- Remover suites `integration` ou `realllm`, trocar provider OpenRouter ou redesenhar a arquitetura agentiva do módulo.
- Provar offline todos os comportamentos possíveis de LLM; o foco é o subconjunto crítico e estável de invariantes de produto.
- Mudar o conjunto de intenções suportadas pelo agente ou expandir o inventário funcional de tools além do escopo de sincronização de cobertura.
- Incluir neste PRD todos os blind spots parciais adicionais identificados no inventário completo fora dos quatro eixos principais; esses itens devem ser tratados em backlog separado, se priorizados depois.

## Suposições e Questões em Aberto

- Suposição: a user story de auditoria representa uma iniciativa única de qualidade de engenharia e não quatro produtos independentes, portanto o PRD consolida os quatro eixos em um único escopo.
- Suposição: não há PRD prévio nem artefatos downstream para este slug, então esta é a primeira versão formal do requisito.
