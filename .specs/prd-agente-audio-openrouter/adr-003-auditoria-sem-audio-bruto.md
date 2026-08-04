# Registro de Decisao Arquitetural (ADR)

## Metadados

- **Titulo:** Persistencia minima de auditoria sem audio bruto
- **Data:** 2026-08-04
- **Status:** Aceita
- **Decisores:** Engenharia Me Controla
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O PRD exige descartar audio original e reter hash, metadados tecnicos e transcricao quando aplicavel.
O runtime atual persiste mensagem do usuario e assistant em `internal/platform/agent/runtime.go:213`,
mas nao possui campos para media id, hash, tamanho, modelo STT, custo, outcome ou reason tecnico.

O projeto usa PostgreSQL 16 por configuracao local em `deployment/compose/compose.yml:11`. Pelas regras
oficiais do PostgreSQL, tabela persistente deve ter chave primaria e constraints coerentes com a
integridade exigida.

## Decisao

Criar uma tabela pequena de auditoria de audio WhatsApp em `mecontrola`, com `wamid` como chave
primaria, metadados tecnicos, outcome/reason fechados via `CHECK`, transcricao opcional e nenhum campo
para audio bruto/base64/URL de download.

## Alternativas Consideradas

- Guardar apenas logs: rejeitado porque logs nao garantem retencao auditavel nem integridade por WAMID.
- Reusar `platform_messages` para todos os metadados: rejeitado porque polui o historico do agente e nao
  modela outcome tecnico.
- Persistir audio bruto temporariamente: rejeitado por privacidade e custo operacional.

## Consequencias

### Beneficios Esperados

- Auditoria suficiente para troubleshooting e compliance interno.
- Menor exposicao de dados sensiveis.
- Integridade por WAMID no banco.

### Trade-offs e Custos

- Nova migration e repository.
- Necessidade de teste de integracao up/down.

### Riscos e Mitigacoes

- Risco: transcricao conter dado financeiro sensivel.
- Mitigacao: nao logar transcricao completa; restringir acesso via roles existentes e revisar retencao na
  task de implementacao.

## Plano de Implementacao

1. Criar migration `000015_agents_whatsapp_audio_messages`.
2. Implementar repository em `internal/agents/infrastructure/persistence`.
3. Cobrir insert/update/idempotencia com integration tests.
4. Validar que nao ha audio bruto persistido.

## Monitoramento e Validacao

- Teste de integracao confirma PK, checks e up/down.
- Grep confirma ausencia de persistencia de bytes/base64 em tabela.

## Impacto em Documentacao e Operacao

Atualizar runbook de troubleshooting para consultar outcome/reason por WAMID.

## Revisao Futura

Revisar politica de retencao depois do beta, com dados reais de volume e custo.
