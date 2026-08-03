# Registro de Decisão Arquitetural (ADR-002)

## Metadados

- **Título:** Alertas de burn-rate multi-janela para SLO, coexistindo com thresholds estáticos
- **Data:** 2026-08-03
- **Status:** Aceita
- **Decisores:** Engenharia de plataforma (on-call)
- **Relacionados:** PRD (RF-04, RF-05, RF-06), techspec.md, `alerting-slo.md` da skill golden-signals

## Contexto

O Grafana provisionado já tem alertas de RED por threshold estático: `mc-api-5xx` (5xx > 5% por 5m) e `mc-api-latency-p99` (p99 > 1s por 10m) em `provisioning/alerting/rules.yaml`. Não há SLO formal nem alerta ligado ao error budget. O padrão golden-signals recomenda, havendo SLO, alertas de burn-rate multi-janela para equilibrar rapidez de detecção e ruído. O PRD fixa SLO de disponibilidade 99,9% (budget 0,1%) e latência P95 < 500ms em 30 dias.

## Decisão

Adicionar um grupo `slo` em `rules.yaml` com quatro alertas de burn-rate de disponibilidade derivados do budget de 0,1% (esquema do Google SRE Workbook): 14,4× (1h+5m, página), 6× (6h+30m, página), 3× (1d+2h, ticket), 1× (3d+6h, ticket). Adicionar um alerta de latência ligado ao SLO P95 < 500ms. Os thresholds estáticos existentes são PRESERVADOS (defesa em profundidade), não substituídos. O SLI de disponibilidade usa a série canônica do histograma HTTP (ver ADR-004).

## Alternativas Consideradas

- **Substituir os thresholds pelos burn-rate**: perde a rede de segurança simples e legível. Rejeitada; manter ambos.
- **Somente 2 alertas (14,4× + 3×)**: menos cobertura de cenários de burn. Rejeitada em favor do esquema completo de 4 (robustez).
- **Manter só thresholds estáticos**: não liga o alarme ao orçamento de erro (contraria o padrão). Rejeitada.

## Consequências

### Benefícios Esperados

- Alarme ligado ao error budget; detecção precoce de incidentes agudos e de degradação lenta.
- Coexistência com thresholds preserva legibilidade e defesa em profundidade.

### Trade-offs e Custos

- Mais regras para manter; queries de burn-rate multi-janela mais complexas.

### Riscos e Mitigações

- **Risco:** ruído por janelas mal calibradas. **Mitigação:** usar os multiplicadores/janelas canônicos do Workbook; janela curta de confirmação evita alarme por pico.
- **Risco:** divergência de rótulo `job`. **Mitigação:** reusar `job="mecontrola-api"` como nos alertas existentes.

## Plano de Implementação

1. Confirmar a série e os rótulos de erro/total (ADR-004).
2. Escrever o grupo `slo` com as 4 regras de burn-rate (schema Grafana unified alerting já usado em `rules.yaml`).
3. Adicionar alerta de latência SLO P95 < 500ms.
4. Referenciar runbook em cada alerta de página (RF-10).

## Monitoramento e Validação

- Sucesso: alertas avaliam sem `execErrState`; disparo coerente em teste de injeção de erro.
- Revisar multiplicadores se o volume real gerar ruído ou detecção tardia.

## Impacto em Documentação e Operação

- `observability/STANDARD.md`: SLO, error budget e tabela de burn-rate.
- Runbooks de disponibilidade e latência.

## Revisão Futura

- Recalibrar quando o tráfego crescer significativamente ou o SLO mudar.
