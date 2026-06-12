# Scorecard de Alternativas

Escala: 1 = pior ou mais oneroso; 5 = melhor ou menos oneroso no contexto da decisão.

| Alternativa | Complexidade | Tempo de entrega | Custo | Escalabilidade | Segurança | Confiabilidade | Observabilidade | Manutenibilidade | Risco operacional | Total | Observação |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Alternativa 1 - Tabela unica transactions | 5 | 5 | 5 | 3 | 3 | 2 | 3 | 2 | 2 | 30 | Simples e rapida, mas sem ancora de compra-pai a integridade do parcelamento e da fatura sofre; baixa expressividade de dominio. |
| Alternativa 2 - Transactions + CardPurchase + CardInvoice/Items | 3 | 3 | 3 | 4 | 4 | 5 | 4 | 5 | 4 | 35 | Dominio rico e alinhado ao padrao do repositorio (budgets, card); melhor base para evolucao (relatorios, recorrencia, imutabilidade de fatura). Recomendada. |
| Alternativa 3 - LedgerEntry append-only com projetores | 1 | 1 | 2 | 5 | 5 | 5 | 5 | 3 | 2 | 29 | Maxima auditabilidade e escalabilidade, mas diverge do padrao do repo (update-in-place + version) e custa muito para MVP. |
| Alternativa 4 - Transactions + Summary materializado | 4 | 4 | 4 | 4 | 3 | 3 | 4 | 3 | 3 | 32 | Pragmatica e rapida, mas esconde a entidade Fatura, dificultando UX, relatorios de cartao e evolucao do MVP. |

## Leitura do Resultado
- Alternativa mais equilibrada: Alternativa 2 - Transactions + CardPurchase + CardInvoice/Items (total 35; melhor confiabilidade, manutenibilidade e aderencia ao padrao do repositorio).
- Alternativa mais rápida: Alternativa 1 - Tabela unica transactions (menor superficie de codigo no curto prazo).
- Alternativa mais segura: Alternativa 3 - LedgerEntry append-only com projetores (maxima auditabilidade e imutabilidade).
- Alternativa mais barata: Alternativa 1 - Tabela unica transactions (menos esquema, menos consumers, menor custo inicial).
- Alternativa com maior risco operacional: Alternativa 3 - LedgerEntry append-only com projetores (operacao de projetores e replays exige maturidade que o time ainda nao possui no MVP).
