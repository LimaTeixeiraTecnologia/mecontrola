# Runbook: Outbox aggregate_user_id

**Servico:** `internal/platform/outbox`
**Owner:** Plataforma
**Dashboard:** [Outbox Adoption](../../docs/dashboards/outbox.json)
**Alertas:** [docs/alerts/outbox.yaml](../../docs/alerts/outbox.yaml)

---

## Significado da Metrica

`outbox_events_inserted_total{has_user_id}` conta eventos inseridos na tabela
`outbox_events` segregados pelo label `has_user_id`:

- `has_user_id="true"`: evento inserido com `aggregate_user_id` preenchido (UUID valido).
- `has_user_id="false"`: evento inserido sem `aggregate_user_id` (campo ausente ou vazio).

A taxa de adocao e calculada como:

```promql
sum(rate(outbox_events_inserted_total{has_user_id="true"}[5m]))
/
sum(rate(outbox_events_inserted_total[5m]))
```

**Meta (ADR-001 v1):** adocao >= 99% em estado estacionario. Tolerancia de 1% para eventos
de sistema legitimamente sem dono (ver allowlist em
`internal/platform/outbox/system_event_allowlist.go`).

**Criterio v2 (futuro):** quando adocao atingir 30 dias consecutivos >= 99.99%, promover
validacao obrigatoria (`NOT NULL` + erro em `NewEvent`). Decisao registrada em ADR-001.

---

## Alerta: OutboxMissingUserID

**Condicao:** `rate(outbox_events_inserted_total{has_user_id="false"}[5m]) / rate(outbox_events_inserted_total[5m]) > 0.01` por 10 min.

**Severidade:** warning

### Sintomas

- Painel "Outbox Adoption %" no dashboard cai abaixo de 99%.
- Painel "Outbox Missing User ID Ratio" ultrapassa a linha vermelha (0.01).
- Logs warn estruturados com `msg="outbox.event.missing_aggregate_user_id"` aumentam.

### Diagnostico

1. Verificar painel "Outbox Missing User ID Rate" para identificar quando iniciou.

2. Identificar o `event_type` responsavel via logs:

   ```
   msg="outbox.event.missing_aggregate_user_id" event_type=<tipo>
   ```

   Filtrar no Loki:

   ```logql
   {app="mecontrola"} |= "outbox.event.missing_aggregate_user_id"
   ```

3. Verificar se o `event_type` esta na allowlist:

   ```bash
   grep -n "event_type" internal/platform/outbox/system_event_allowlist.go
   ```

   Se estiver na allowlist e o alerta disparou, o volume de eventos de sistema aumentou
   acima do esperado — avaliar se e comportamento normal ou spike.

4. Se NAO estiver na allowlist, identificar o caller que constroi `outbox.EventInput`
   para esse `event_type` sem popular `AggregateUserID`:

   ```bash
   grep -rn "Type:.*<event_type>" internal/ --include="*.go"
   ```

5. Verificar se houve deploy recente de um novo producer ou alteracao em producer existente:

   ```bash
   git log --oneline -20 -- internal/*/infrastructure/messaging/database/producers/
   ```

### Remediacao

1. Identificar o arquivo do producer/use case que constroi `EventInput` sem `AggregateUserID`.

2. Popular o campo com o UUID do usuario dono do agregado:

   ```go
   outbox.EventInput{
       AggregateUserID: evt.UserID.String(),
   }
   ```

3. Abrir PR com a correcao. Gate `task lint:outbox-user-id` deve passar apos a correcao.

4. Se o `event_type` for legitimamente de sistema (sem dono de usuario), adicionar a
   allowlist seguindo a politica de adicao descrita abaixo.

5. Fazer deploy e monitorar o painel "Outbox Adoption %" por pelo menos 30 minutos.

### Validacao pos-remediacao

```bash
task lint:outbox-user-id
task lint && task test && task vulncheck
```

---

## Politica de Adicao a Allowlist (ADR-004)

A allowlist `systemEventTypes` em `internal/platform/outbox/system_event_allowlist.go`
declara os `EventType` legitimamente sem `AggregateUserID` (eventos de sistema sem dono).

**Criterios para adicao:**

1. O evento nao possui um usuario dono identificavel (ex: evento de manutencao de plataforma,
   tarefas de limpeza de sistema, housekeeping interno).
2. A ausencia de `aggregate_user_id` e intencional e documentada.
3. O `event_type` deve ser uma constante tipada em `internal/platform/outbox/`.

**Processo:**

1. Abrir PR adicionando a constante `EventType` em `system_event_allowlist.go`.
2. Referenciar o ADR-004 na descricao do PR explicando por que o evento nao tem dono.
3. Atualizar este runbook na secao de eventos conhecidos abaixo.
4. Obter aprovacao do time de plataforma.

### Eventos de sistema conhecidos na allowlist

Consultar `internal/platform/outbox/system_event_allowlist.go` para lista atual.
No momento do PRD outbox-aggregate-user-id v1: apenas `auth.failed` (evento de sistema
sem agregado de usuario — falha de autenticacao no gateway antes de estabelecer principal).

---

## Smoke Staging

Para verificar que eventos reais populam `aggregate_user_id` em staging:

```bash
task smoke:outbox-user-id
```

A receita dispara eventos de cada modulo e valida via SQL que `aggregate_user_id IS NOT NULL`
para cada evento inserido. Ver `taskfiles/smoke.yml` para detalhes.

Para validacao adversarial (forcar evento sem user_id e confirmar alerta):

```bash
task smoke:outbox-user-id-adversarial
```

---

## Referencias

- [PRD outbox-aggregate-user-id](.specs/prd-outbox-aggregate-user-id/prd.md)
- [ADR-001 — Coluna NULL na v1](.specs/prd-outbox-aggregate-user-id/adr-001-nullable-v1-strict-v2.md)
- [ADR-004 — Allowlist eventos sistema](.specs/prd-outbox-aggregate-user-id/adr-004-allowlist-eventos-sistema.md)
- [Dashboard Outbox Adoption](../../docs/dashboards/outbox.json)
- [Alertas outbox](../../docs/alerts/outbox.yaml)
