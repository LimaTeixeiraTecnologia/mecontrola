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

## Adendo 2026-08-04 — Ampliacao do CHECK de reason (migration 000016)

A migration `000015_agents_whatsapp_audio_messages` (task 5.0, ver "Plano de Implementacao" acima)
cravou o `CHECK` de `reason` apenas com as 7 razoes pos-STT conhecidas naquele momento da decisao
(`approved`, `stt_error`, `truncated`, `empty_text`, `incoherent`, `language_unsupported`,
`low_confidence`).

Ao integrar o consumer real na task 6.0, o decisor de dominio (`AudioReason` em
`internal/agents/application/usecases/decide_audio_transcription.go`) precisou de 6 razoes
adicionais para rejeicoes que ocorrem **antes** do STT ser chamado: `invalid_payload`,
`media_unavailable`, `size_exceeded`, `duration_unavailable`, `duration_exceeded`,
`cost_exceeded`. Essas razoes nao existiam na 000015 porque a decisao original desta ADR cobria
apenas o fluxo pos-transcricao; reusar `stt_error` para rejeicoes pre-STT seria semanticamente
incorreto.

A migration `000016_agents_whatsapp_audio_messages_widen_reason` amplia o `CHECK` da coluna `reason`
para as 13 razoes finais, alinhadas 1:1 com `AudioReason.IsValid()`. A cobertura foi confirmada por
teste de integracao real contra Postgres 16 (`TestInsertTerminalAcceptsPreSTTRejectionReasons` em
`internal/agents/infrastructure/persistence/audio_audit_repository_integration_test.go`). Detalhe
completo da origem em `.specs/prd-agente-audio-openrouter/6.0_execution_report.md` (secao
"Arquivos Alterados" e "Suposicoes").

A tabela e os campos definidos por esta ADR permanecem inalterados; apenas o conjunto de valores
validos de `reason` foi estendido de forma aditiva (sem remocao de valores existentes).

## Revisao Futura

Revisar politica de retencao depois do beta, com dados reais de volume e custo.
