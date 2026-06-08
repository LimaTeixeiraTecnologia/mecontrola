# Scorecard de Alternativas

Escala: 1 = pior ou mais oneroso; 5 = melhor ou menos oneroso no contexto da decisão (MVP production-ready, WhatsApp+LLM in-process hoje, VPS Hostinger, Postgres único, zero dependência externa nova).

| Alternativa | Complexidade | Tempo de entrega | Custo | Escalabilidade | Segurança | Confiabilidade | Observabilidade | Manutenibilidade | Risco operacional | Total | Observação |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Alternativa 1 - Principal in-process via ctx | 5 | 5 | 5 | 4 | 4 | 5 | 4 | 4 | 4 | 40 | Mais simples e rápido; nenhuma criptografia no MVP. Risco: contrato de auth ad-hoc, refactor pesado quando app/web exigirem token externo. |
| Alternativa 2 - JWT Ed25519 curto + JWKs | 2 | 2 | 3 | 5 | 4 | 4 | 4 | 4 | 3 | 31 | Fundação completa, mas adiciona gestão de chaves+kid+rotação no MVP cirúrgico. Excesso de escopo agora. |
| Alternativa 3 - Sessão opaca persistida em Postgres | 3 | 3 | 4 | 3 | 5 | 3 | 5 | 4 | 3 | 33 | Revogação ativa nativa, audit trivial. Carga PG por requisição; se PG cai, auth cai. Não resolve LLM in-process (over-engineered). |
| Alternativa 4 - Boundary-explicit (ctx + interface JWT documentada) | 4 | 5 | 5 | 5 | 5 | 5 | 4 | 5 | 4 | 42 | Mesmo custo MVP que A, mas contrato `auth.Principal` + ADR de boundary HTTP futura tornam evolução para app/web sem refactor de domínio. Exige disciplina arquitetural. |

## Leitura do Resultado
- Alternativa mais equilibrada: **D — Boundary-explicit (ctx + interface JWT documentada)**.
- Alternativa mais rápida: **A** (empata com D em tempo; difere por não declarar a interface JWT).
- Alternativa mais segura: **D** (igual a C em segurança no MVP, supera C em flexibilidade futura).
- Alternativa mais barata: **A** e **D** empatam em custo MVP (zero infra nova, zero dep nova).
- Alternativa com maior risco operacional: **B** (gestão de chaves Ed25519 + JWKs em VPS sem KMS é vetor de erro humano).

## Comparativo Específico A vs D (decisão tensa)

| Dimensão | A | D |
| --- | --- | --- |
| Código MVP a escrever | `auth.Principal` + `ctx` helpers + WhatsApp boundary | igual a A + 1 interface Go vazia (`JWTBoundary`) + 1 ADR documentando o contrato |
| Refactor futuro para app/web | Reabrir domínio para garantir Principal vem da nova boundary | Implementar `JWTBoundary` — domínio inalterado |
| Risco de drift arquitetural | Alto — sem contrato explícito, devs podem ler header direto no handler | Baixo — interface declarada força handlers a passarem por boundary |
| Disciplina exigida | Baixa hoje, alta no refactor futuro | Alta hoje, baixa depois |

D = A + 1 interface vazia + 1 ADR. Custo marginal: ~30 linhas de código + 1 documento.
