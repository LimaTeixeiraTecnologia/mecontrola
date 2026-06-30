# Tarefa 2.0: Migration aditiva (índice telefone, timestamps, throttle)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar, de forma puramente aditiva, o índice de correlação por telefone, as colunas de timestamps da jornada e a tabela durável de throttle de no-match.

<requirements>
- RF-23: índice que suporte `WHERE status='PAID' AND customer_mobile_e164=$1 ORDER BY paid_at DESC`.
- RF-35: colunas `email_sent_at`, `page_opened_at`, `activation_started_at`, `whatsapp_opened_at`.
- RF-24: tabela durável `onboarding_activation_nomatch_throttle` (PK `mobile_e164, window_start`).
- Migration up + down; aditiva, sem alterar dados existentes; `down` faz drop dos novos objetos.
</requirements>

## Subtarefas

- [ ] 2.1 Criar `migrations/0000NN_activation_journey.up.sql` com o índice parcial `onboarding_tokens_mobile_activable_idx`, o `ALTER TABLE ADD COLUMN IF NOT EXISTS` das 4 colunas e o `CREATE TABLE` do throttle (ver DDL na techspec, seção "Modelos de Dados").
- [ ] 2.2 Criar o `.down.sql` correspondente (drop da tabela, colunas e índice).
- [ ] 2.3 Garantir numeração sequencial correta na pasta `migrations/`.

## Detalhes de Implementação

Ver techspec.md, seção "Modelos de Dados" (bloco `migrations/0000NN_activation_journey.up.sql`). Reusar o schema `mecontrola`. Sem lógica de domínio na migration.

## Critérios de Sucesso

- Migration aplica e reverte sem erro em banco limpo e em banco com dados.
- Índice parcial e tabela de throttle criados conforme DDL da techspec.
- Sem alteração destrutiva em `onboarding_tokens`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste de integração de migração up/down (testcontainers) verificando objetos criados/removidos.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/0000NN_activation_journey.up.sql` (novo)
- `migrations/0000NN_activation_journey.down.sql` (novo)
- `migrations/000001_initial_schema.up.sql` (referência de schema)
