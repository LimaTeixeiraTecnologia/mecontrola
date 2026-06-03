# Scorecard de Alternativas

Escala: 1 = pior ou mais oneroso; 5 = melhor ou menos oneroso no contexto da decisão.

| Alternativa | Complexidade | Tempo de entrega | Custo | Escalabilidade | Segurança | Confiabilidade | Observabilidade | Manutenibilidade | Risco operacional | Total | Observação |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Alternativa 1 - Monolito modular com identidade/RBAC próprios e Mercado Pago Pix por adapter | 4 | 4 | 5 | 4 | 4 | 4 | 4 | 4 | 4 | 37 | Boa aderência ao codebase; exige implementar auth/RBAC com disciplina, mas mantém domínio e ownership sob controle do MeControla. |
| Alternativa 2 - Auth gerenciado + Mercado Pago Pix | 3 | 5 | 4 | 4 | 4 | 4 | 3 | 3 | 3 | 33 | Acelera autenticação, mas autorização, plano e ownership ainda precisam morar no MeControla para evitar lock-in. |
| Alternativa 3 - Billing-first incremental com Mercado Pago Pix e RBAC mínimo | 5 | 5 | 5 | 3 | 3 | 3 | 3 | 3 | 2 | 32 | Mais rápida para validar receita, porém arriscada para o requisito inaceitável de vazamento cross-user e auditoria robusta. |
| Alternativa 4 - Plataforma multi-tenant robusta desde o início | 1 | 1 | 2 | 5 | 5 | 4 | 4 | 3 | 2 | 27 | Cobre futuro amplo, mas pesa demais para MVP e aumenta chance de atraso antes de validar pagamento. |
| Alternativa 5 - Híbrida em fases com contratos fortes e Mercado Pago Pix no MVP | 4 | 4 | 5 | 4 | 5 | 4 | 4 | 5 | 4 | 39 | Melhor equilíbrio: valida Pix recorrente agora, mantém identidade/ownership/RBAC no domínio e deixa portas para SSO, organizações e múltiplos PSPs. |

## Leitura do Resultado
- Alternativa mais equilibrada: Alternativa 5 - Híbrida em fases com contratos fortes e Mercado Pago Pix no MVP.
- Alternativa mais rápida: Alternativa 3 - Billing-first incremental com Mercado Pago Pix e RBAC mínimo.
- Alternativa mais segura: Alternativa 5 e Alternativa 4, com vantagem prática para 5 por reduzir escopo sem abandonar ownership/RBAC.
- Alternativa mais barata: Alternativas 1, 3 e 5 empatam em custo externo baixo ao concentrar custo em Mercado Pago Pix e evitar identidade gerenciada paga.
- Alternativa com maior risco operacional: Alternativa 3, porque acelera billing antes de consolidar auditoria e autorização robusta.
