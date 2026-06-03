# Scorecard de Alternativas

Escala: 1 = pior ou mais oneroso; 5 = melhor ou menos oneroso no contexto da decisão.
Contexto: monólito Go + Postgres fixo, volumetria média (~1M eventos/dia, p95 < 1s), 1×N in-process, multi-instância sem broker, DLQ + housekeeping 90d obrigatórios.

| Alternativa | Complexidade | Tempo de entrega | Custo | Escalabilidade | Segurança | Confiabilidade | Observabilidade | Manutenibilidade | Risco operacional | Total | Observação |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Alternativa 1 - Polling Single-Table com fan-out agregado | 5 | 5 | 5 | 3 | 4 | 3 | 2 | 4 | 4 | 35 | Simples e rápido; perde visibilidade granular por handler; falha de 1 handler obriga reexecução de todos no retry. |
| Alternativa 2 - Polling Two-Table (events + deliveries) | 3 | 3 | 4 | 4 | 4 | 5 | 5 | 4 | 4 | 36 | Retries independentes por handler; observabilidade fina; custo +1 escrita por handler no caminho de publish. |
| Alternativa 3 - LISTEN/NOTIFY com Polling Fallback two-table | 2 | 2 | 3 | 4 | 4 | 5 | 5 | 3 | 3 | 31 | Reduz latência p95 a ~50–200ms; adiciona conexão dedicada por instância e caminho dual notify+poll para depurar. |
| Alternativa 4 - Polling Particionado por hash de partition_key | 2 | 1 | 3 | 5 | 4 | 5 | 5 | 2 | 2 | 29 | Garante ordem por aggregate entre réplicas via advisory lock; over-engineering dado FIFO global fora-de-escopo. |

## Leitura do Resultado
- Alternativa mais equilibrada: **Alternativa 2 - Polling Two-Table (events + deliveries)** — total 36, ganha em confiabilidade e observabilidade sem perder muito em custo/complexidade.
- Alternativa mais rápida em entrega: **Alternativa 1 - Polling Single-Table com fan-out agregado** — total 35; vence se time-to-prod for único critério, mas falha no gate de observabilidade granular.
- Alternativa com menor latência: **Alternativa 3 - LISTEN/NOTIFY com Polling Fallback two-table** — p95 ~50–200ms vs ~250–500ms das demais.
- Alternativa mais barata em infra e manutenção: **Alternativa 1 - Polling Single-Table com fan-out agregado** — uma tabela, um query, um worker.
- Alternativa com maior risco operacional: **Alternativa 4 - Polling Particionado por hash de partition_key** — advisory locks + pool por partição introduz modos de falha não justificados pelo escopo aprovado.

## Reconciliação com a Restrição Dominante
Restrição dominante = **simplicidade operacional**. Alternativa 1 vence em simplicidade pura; Alternativa 2 perde 2 pontos em complexidade/tempo mas ganha 4 pontos somados em confiabilidade+observabilidade. Como o sucesso mínimo (Rodada 1.2) exige **DLQ + housekeeping em produção** com handler dummy e o problema é fundação multi-propósito de plataforma, observabilidade granular por handler é insumo crítico de operação — não luxo. Trade-off é aceitável.

## Recomendação Preliminar (confirmada na Rodada 5)
**Alternativa 2 - Polling Two-Table (events + deliveries)** com:
- `time.Ticker` de 250ms para o dispatcher (atende SLO p95 < 1s com folga).
- `robfig/cron/v3` para housekeeping diário (`@daily`) e reaper de claims-stuck (`@every 1m`).
- `FOR UPDATE SKIP LOCKED` em `outbox_deliveries` para coordenação multi-instância.
- LISTEN/NOTIFY fica como evolução opcional V2 se métrica de latência demonstrar necessidade.
- Particionamento (Alternativa 4) descartado por over-engineering vs. escopo aprovado.
