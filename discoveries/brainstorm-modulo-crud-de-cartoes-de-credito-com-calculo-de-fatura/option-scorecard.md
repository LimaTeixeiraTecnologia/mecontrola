# Scorecard de Alternativas

Escala: 1 = pior ou mais oneroso; 5 = melhor ou menos oneroso no contexto da decisão (MVP production-ready, `mecontrola` Go, BR 2026).

| Alternativa | Complexidade | Tempo de entrega | Custo | Escalabilidade | Segurança | Confiabilidade | Observabilidade | Manutenibilidade | Risco operacional | Total | Observação |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Alternativa 1 - Função pura InvoiceFor + clamp + tests exaustivos | 5 | 5 | 5 | 5 | 5 | 5 | 4 | 5 | 5 | 44 | Stateless, O(1), sem IO; cobertura via table-driven 50+ fixtures; observabilidade via OTel span e logs. Reaproveitável por todos os módulos consumidores. |
| Alternativa 2 - Pré-computação materializada (invoices table) | 2 | 2 | 3 | 4 | 4 | 3 | 4 | 2 | 2 | 26 | Duas fontes de verdade (regra + dados); job de rollover obrigatório; risco de drift se cron falha; complexidade extra para MVP. |
| Alternativa 3 - Híbrido função pura + materialized view Postgres | 3 | 3 | 3 | 4 | 4 | 4 | 4 | 3 | 3 | 31 | View materializada exige REFRESH a cada UPDATE em cards; ganho de leitura só compensa em escala não-MVP. |
| Alternativa 4 - Biblioteca de recorrência (RRULE/rrule-go) | 2 | 3 | 4 | 4 | 3 | 3 | 3 | 3 | 3 | 28 | Dep externa para regra trivial; RRULE não modela compra X cai em fatura Y diretamente, requer adapter custom. |

## Leitura do Resultado
- Alternativa mais equilibrada: Alternativa 1 - Função pura InvoiceFor (44 pts).
- Alternativa mais rápida: Alternativa 1 - ~6 dias de implementação ponta a ponta com robustez extrema.
- Alternativa mais segura: Alternativa 1 - função pura é determinística, auditável e não introduz superfície de ataque.
- Alternativa mais barata: Alternativa 1 - sem dependência externa, sem job adicional, sem infraestrutura.
- Alternativa com maior risco operacional: Alternativa 2 - depende de job cron de rollover; falha silenciosa equivale a compras sem fatura.

## Critérios não-puramente quantitativos

### Viabilidade técnica
- Alternativa 1: trivial em Go puro; time.Time, time.LoadLocation, lógica calendárica simples.
- Alternativas 2 e 3: exigem migrations, jobs, observabilidade dedicada.
- Alternativa 4: dep externa sem benefício correspondente.

### Capacidade da equipe
- O mecontrola já segue Padrão Obrigatório de Módulo (R0 a R7). Alternativa 1 encaixa na competência atual; as demais demandam padrões novos.

### Aderência ao AGENTS.md e regras R0 a R7
- Alternativa 1: aderente - função pura em domínio, sem init, sem panic, sem mockar tempo (R7 explícita: tempo inline no ponto de uso).
- Alternativas 2 e 3: exigem job com goroutine cancelável (R6) e wiring de cron - possível, mas escopo desnecessário no MVP.

### Tempo de implementação estimado
- Alternativa 1: 2 dias domain + tests, 1 dia adapter Postgres, 1 dia handler+wiring, 1 dia observabilidade, 1 dia robustez extrema = 6 dias.
- Alternativa 2: 9 dias.
- Alternativa 3: 8 dias.
- Alternativa 4: 6 dias com lock-in em lib externa.
