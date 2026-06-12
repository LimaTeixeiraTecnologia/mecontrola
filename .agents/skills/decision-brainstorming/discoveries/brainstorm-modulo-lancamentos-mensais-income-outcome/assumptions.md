# Hipóteses e Premissas

## Hipóteses Confirmadas
| ID | Hipótese | Evidência | Impacto | Status |
| --- | --- | --- | --- | --- |
| H1 | Modulos existentes (`internal/budgets`, `internal/card`, `internal/categories`, `internal/identity`) seguem padrao de agregados ricos + outbox + idempotency-key + version. | AGENTS.md, CLAUDE.md, commits b20d3c8/099671e e arquivos sob `internal/*/domain` e `internal/*/infrastructure`. | O novo modulo deve replicar esse padrao para nao quebrar coerencia arquitetural. | confirmada |
| H2 | Internal/card expõe datas `closing_day` e `due_day` por cartao do usuario, suficientes para calcular competencia de fatura via reader. | Modulo `internal/card` ja entregue no commit 099671e e b710c22. | Snapshot na criacao do lancamento de credito e tecnicamente viavel sem acoplar adapter ao card. | confirmada |
| H3 | `outbox.Publisher` com idempotencia por `event_id` esta operacional no repositorio. | Secao "Outbox" em AGENTS.md e CLAUDE.md, uso em budgets/identity/billing. | Publicar `Transaction*` e `CardPurchase*` via outbox e padrao do repo, sem invencao. | confirmada |
| H4 | Internal/budgets ja consome eventos de dominio para atualizar consumo por categoria. | Commit 099671e ("MVP production-ready de budgets-monthly") e referencias em AGENTS.md. | A integracao entre lancamentos e budget via evento e o caminho oficial; nao e preciso criar API REST sync. | confirmada |
| H5 | Usuario aceita consistencia eventual em `monthly_summary` (janela ms-seg entre commit do lancamento e atualizacao da projecao). | Resposta explicita na Rodada 2 (Q6 = Disponibilidade). | Justifica modelo CQRS leve com consumer reativo, sem trigger SQL. | confirmada |

## Hipóteses Não Validadas
| ID | Hipótese | Risco se falsa | Como validar | Dono |
| --- | --- | --- | --- | --- |
| H6 | Lista de formas de pagamento (Debito em conta, Pix, Cartao credito, Cartao debito, TED, DOC) e a nomenclatura canonica do BACEN/SPB em 2026. DOC pode estar oficialmente descontinuado (Resolucao BCB 290/2023, fim do servico anunciado para 2024). | Modelo de dados pode conter forma de pagamento legada; nomenclatura UI pode divergir do oficial. | Consultar documentacao oficial do BACEN/SPB no discovery tecnico (`technical-discovery-production`). | technical-discovery-production |
| H7 | Recorrencia automatica (salário mensal, assinatura) NAO entra no MVP. Usuario nao confirmou explicitamente. | Se for confirmada como dentro do MVP, modelo de dominio precisa suportar serie recorrente (template + materializacao por mes), aumentando o escopo. | Confirmar com usuario antes do PRD; default atual = fora do MVP. | usuario |
| H8 | Volume esperado por usuario nos primeiros 6 meses cabe em uma unica tabela `transactions` sem particionamento. Sem dados reais de carga. | Risco de degradacao em listagens longas; pode exigir particionamento por user_id ou ref_month antes do esperado. | Discovery tecnico deve estimar volumetria com base em concorrentes (Mobills, Organizze) e plano de negocio. | technical-discovery-production |
| H9 | Edicao retroativa de fatura fechada e aceitavel sob ponto de vista contabil/produto. | Pode confundir usuario que ja registrou pagamento da fatura no banco fisico; risco de bug semantico. | Validar com usuario/produto antes do PRD final. Considerar travar edicao pos-pagamento como evolucao. | usuario |
| H10 | Categoria e subcategoria sao gerenciadas exclusivamente em `internal/categories` e o lancamento guarda apenas referencia por ID + snapshot do nome para historico. | Se categoria for excluida ou renomeada, sem snapshot o lancamento fica orfao. | Definir no discovery tecnico se ha snapshot de nome ou apenas FK; alinhar com politica de exclusao de categoria. | technical-discovery-production |

## Restrições Confirmadas
- Stack Go conforme `go.mod` do repositorio; respeitar Regras Estritas R0-R7 da skill `go-implementation`.
- R-ADAPTER-001 e inegociavel: zero comentarios em `.go` de producao e adapter fino (handler/consumer/job/producer -> usecase).
- Persistencia em PostgreSQL com testes de integracao via testcontainers.
- Outbox + idempotency-key + optimistic locking sao padrao mandatorio para mutacoes.
- Mensageria de dominio passa exclusivamente pelo `outbox.Publisher`; sem broker externo no MVP.
- Multi-tenant logico: toda query filtra por `user_id` derivado do principal middleware.
- Observabilidade OTel (spans, metricas, logs estruturados) obrigatoria em handler, usecase, repo e consumer.
- MVP somente em BRL; multi-moeda fora de escopo.
- Sem upload de anexos no MVP.
- Sem transferencia entre contas no MVP.

## Preferências Não Bloqueantes
- Nome do modulo provisorio: `internal/transactions` (a confirmar em discovery tecnico, possiveis alternativas: `internal/ledger`, `internal/finance-entries`).
- Identificadores ULID/UUIDv7 conforme padrao do repositorio (a confirmar em discovery tecnico).
- Paginacao por cursor (keyset) preferida sobre offset-based para listagens.
- Endpoint REST sob `/v1/transactions`, `/v1/card-purchases`, `/v1/months/{ref_month}` (rotas a confirmar em PRD/techspec).
- Erros de dominio retornados via `problem+json` consistente com modulos existentes.
- Metricas Prometheus com nomes padronizados (`transactions_created_total`, `monthly_summary_recompute_duration_seconds`, etc.), nomenclatura final no discovery tecnico.
