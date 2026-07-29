# Próximos Passos — Production-Ready/Proof (2026-07-29)

Guia de ação do usuário. Tudo aqui tem evidência registrada em
`docs/runs/resultados-e2e-agente-2026-07-28.txt` (RODADAs 14 e 15).
Nada neste guia é suposição: cada item referencia validação real em produção.

## Estado atual (provado em produção)

- Bugs B-14, B-15, B-16, B-02 corrigidos e validados em produção.
- Suíte 0-falso-positivo validada (vale-refeição, multi-item, débito).
- Tracing do agente: 100% dos traces retidos (fix de tail sampling 10%),
  instância identificável via `service.instance.id` em worker-1/2 e server-1/2.
- 3 alertas ativos no Grafana: `mc-outbox-dead-letter`,
  `mc-tracing-spans-absent`, `mc-tracing-export-failed`.
- Dead-letter histórico do outbox descartado com auditoria (dead-letters = 0).

---

## Passo 1 — Commitar a evidência (agora)

As RODADAs 14 e 15 estão no working tree, não commitadas.

```bash
git add docs/runs/resultados-e2e-agente-2026-07-28.txt
git commit -m "docs(runs): rodadas 14-15 - observabilidade agente e prova final"
git push
```

## Passo 2 — Suíte canônica de regressão (30-40 min, quando quiser a chancela final)

Enviar as mensagens abaixo no WhatsApp, **na ordem**, uma de cada vez,
aguardando a resposta do bot antes da próxima. Ao final, avisar no chat
para eu verificar DB, runs e traces de cada passo.

| # | Mensagem exata | Resultado esperado |
|---|----------------|--------------------|
| 1 | `gastei 45 na farmácia no pix` | Confirmação direta com categoria Custo Fixo > Medicamentos e Farmácia. NUNCA perguntar cartão. |
| 2 | `gastei 30 no almoço no vale` | Forma de pagamento vale-refeição, sem reprompt indevido. |
| 3 | `gastei 30 no ônibus e 15 no café` | Bloqueio multi-item ("registro um de cada vez"). NUNCA registrar os 2. |
| 4 | `no cartão eu gastei 35 e não 30` | Edit_entry com busca de candidatos. NUNCA "Me conta de novo esse lançamento". |
| 5 | `quanto gastei hoje?` | Total com casas decimais corretas e período do dia inteiro. |
| 6 | `quanto está minha fatura do cartão nubank?` | Fatura do mês vigente/próximo com vencimento correto. |
| 7 | (verificação minha) | Runs sem `failed` novo; outbox sem dead-letter novo; traces no Tempo com `service.instance.id`. |

Critério de aprovação: **7/7 sem desvio**. Qualquer desvio = bug novo,
anotar horário exato da mensagem para eu rastrear no Tempo/Loki/DB.

## Passo 3 — Backlog consciente (NÃO fazer agora)

Registrado na RODADA 14. Executar somente quando o gatilho real aparecer:

- **Fuso do "hoje"**: `occurred_at` gravado como 00:00 UTC (21:00 BRT do
  dia anterior). Consistente hoje; atacar quando multi-fuso for requisito.
- **Índice `(user_id, occurred_at)`**: criar migration somente quando
  `EXPLAIN` mostrar necessidade por volumetria.
- **Alerta de spans por instância**: hoje o alerta é global. Criar versão
  por instância somente se uma perda parcial real acontecer.

## Passo 4 — Rotina pós-deploy (toda vez que o agente/worker mudar)

1. Acompanhar o CI até `Deploy Swarm Stack` + `Healthcheck` verdes.
2. Se a mudança tocar `deployment/telemetry/grafana/**`: me pedir o
   restart do `mecontrola_otel-lgtm` (bind mount não reinicia sozinho).
3. Rodar a suíte do Passo 2 (ou ao menos os passos 1, 4 e 5).
4. Eu verifico: `platform_runs` sem failed, outbox sem status=4,
   traces com `service.instance.id` das 4 réplicas.

## Monitoramento contínuo (o que observar)

- Alertas do Grafana (Telegram): dead-letter, ausência de spans >30min,
  falha de export de spans.
- Query semanal de guard (esperado 0 em operação normal):

```sql
SELECT count(*) FROM mecontrola.platform_runs
WHERE status='failed' AND error LIKE 'guard %'
  AND started_at > now() - interval '24 hours';
```
