# Validacao manual da API MeControla

- Data: 2026-06-19 07:56:02
- Base URL: http://localhost:8080
- Resumo: 55 ok, 0 drift, 0 blocked

| Request | Metodo | Path | Resultado | HTTP | Observacao |
|---|---|---|---|---:|---|
| GET /health | GET | `/health` | ok | 200 |  |
| GET /readiness | GET | `/readiness` | ok | 200 |  |
| GET /healthz | GET | `/healthz` | ok | 200 |  |
| GET /livez | GET | `/livez` | ok | 200 |  |
| POST /api/v1/onboarding/checkout | POST | `/api/v1/onboarding/checkout` | ok | 201 |  |
| OPTIONS /api/v1/onboarding/checkout | OPTIONS | `/api/v1/onboarding/checkout` | ok | 204 |  |
| GET /api/v1/onboarding/tokens/{token}/state | GET | `/api/v1/onboarding/tokens/token-inexistente-para-teste/state` | ok | 200 |  |
| OPTIONS /api/v1/onboarding/tokens/{token}/state | OPTIONS | `/api/v1/onboarding/tokens/token-inexistente-para-teste/state` | ok | 204 |  |
| POST /api/v1/identity/users | POST | `/api/v1/identity/users` | ok | 200 |  |
| GET /api/v1/categories | GET | `/api/v1/categories?kind=expense&include_deprecated=false` | ok | 200 |  |
| GET /api/v1/categories/{id} | GET | `/api/v1/categories/66cb85a0-3266-5900-b8e3-13cdcd00ab62?include_deprecated=false` | ok | 200 |  |
| GET /api/v1/category-dictionary | GET | `/api/v1/category-dictionary?kind=expense&page_size=20` | ok | 200 |  |
| GET /api/v1/category-dictionary/search | GET | `/api/v1/category-dictionary/search?q=aliment&kind=expense` | ok | 200 |  |
| POST /api/v1/cards | POST | `/api/v1/cards` | ok | 201 |  |
| GET /api/v1/cards | GET | `/api/v1/cards?limit=20` | ok | 200 |  |
| GET /api/v1/cards/{id} | GET | `/api/v1/cards/b278b195-d17a-47b3-965a-46b056c1ef4c` | ok | 200 |  |
| PUT /api/v1/cards/{id} | PUT | `/api/v1/cards/b278b195-d17a-47b3-965a-46b056c1ef4c` | ok | 200 |  |
| PATCH /api/v1/cards/{id}/limit | PATCH | `/api/v1/cards/b278b195-d17a-47b3-965a-46b056c1ef4c/limit` | ok | 200 |  |
| GET /api/v1/cards/{id}/invoices?for= | GET | `/api/v1/cards/b278b195-d17a-47b3-965a-46b056c1ef4c/invoices?for=2026-06-15` | ok | 200 |  |
| POST /api/v1/cards (aux delete) | POST | `/api/v1/cards` | ok | 201 |  |
| DELETE /api/v1/cards/{id} | DELETE | `/api/v1/cards/b000ec2d-15d6-406a-ac21-eb6e12a46c2e` | ok | 204 |  |
| POST /api/v1/transactions | POST | `/api/v1/transactions` | ok | 201 |  |
| GET /api/v1/transactions | GET | `/api/v1/transactions?ref_month=2026-06&limit=50` | ok | 200 |  |
| GET /api/v1/transactions/{id} | GET | `/api/v1/transactions/99ea6f85-9a85-434c-8f6b-54c4698ac747` | ok | 200 |  |
| PATCH /api/v1/transactions/{id} | PATCH | `/api/v1/transactions/99ea6f85-9a85-434c-8f6b-54c4698ac747` | ok | 200 |  |
| DELETE /api/v1/transactions/{id} | DELETE | `/api/v1/transactions/99ea6f85-9a85-434c-8f6b-54c4698ac747` | ok | 204 |  |
| POST /api/v1/card-purchases | POST | `/api/v1/card-purchases` | ok | 201 |  |
| GET /api/v1/card-purchases | GET | `/api/v1/card-purchases?ref_month=2026-06&limit=50` | ok | 200 |  |
| GET /api/v1/card-purchases/{id} | GET | `/api/v1/card-purchases/4a6e077f-98ab-4360-9146-2df24f023a40` | ok | 200 |  |
| PATCH /api/v1/card-purchases/{id} | PATCH | `/api/v1/card-purchases/4a6e077f-98ab-4360-9146-2df24f023a40` | ok | 200 |  |
| GET /api/v1/cards/{card_id}/invoices/{ref_month} | GET | `/api/v1/cards/b278b195-d17a-47b3-965a-46b056c1ef4c/invoices/2026-07` | ok | 200 |  |
| DELETE /api/v1/card-purchases/{id} | DELETE | `/api/v1/card-purchases/4a6e077f-98ab-4360-9146-2df24f023a40` | ok | 204 |  |
| POST /api/v1/recurring-templates | POST | `/api/v1/recurring-templates` | ok | 201 |  |
| GET /api/v1/recurring-templates | GET | `/api/v1/recurring-templates?limit=50` | ok | 200 |  |
| GET /api/v1/recurring-templates/{id} | GET | `/api/v1/recurring-templates/b3279840-18d4-4191-83cd-1a7905d9e561` | ok | 200 |  |
| PATCH /api/v1/recurring-templates/{id} | PATCH | `/api/v1/recurring-templates/b3279840-18d4-4191-83cd-1a7905d9e561` | ok | 200 |  |
| DELETE /api/v1/recurring-templates/{id} | DELETE | `/api/v1/recurring-templates/b3279840-18d4-4191-83cd-1a7905d9e561` | ok | 204 |  |
| GET /api/v1/months/{ref_month} | GET | `/api/v1/months/2026-06` | ok | 200 |  |
| GET /api/v1/months/{ref_month}/entries | GET | `/api/v1/months/2026-06/entries?limit=50` | ok | 200 |  |
| POST /api/v1/budgets | POST | `/api/v1/budgets` | ok | 201 |  |
| POST /api/v1/budgets/recurrence | POST | `/api/v1/budgets/recurrence` | ok | 207 |  |
| GET /api/v1/budgets/alerts | GET | `/api/v1/budgets/alerts?competence=2026-12` | ok | 200 |  |
| POST /api/v1/budgets/expenses | POST | `/api/v1/budgets/expenses` | ok | 201 |  |
| PATCH /api/v1/budgets/expenses/{id} | PATCH | `/api/v1/budgets/expenses/c85d8c3d-2393-4765-a901-f4ffda255a18` | ok | 200 |  |
| DELETE /api/v1/budgets/expenses/{id} | DELETE | `/api/v1/budgets/expenses/c85d8c3d-2393-4765-a901-f4ffda255a18` | ok | 204 |  |
| POST /api/v1/budgets/{competence}/activate | POST | `/api/v1/budgets/2026-12/activate` | ok | 200 |  |
| GET /api/v1/budgets/{competence}/summary | GET | `/api/v1/budgets/2026-12/summary` | ok | 200 |  |
| DELETE /api/v1/budgets/{competence} | DELETE | `/api/v1/budgets/2027-01` | ok | 204 |  |
| POST /api/v1/billing/webhooks/kiwify | POST | `/api/v1/billing/webhooks/kiwify` | ok | 202 |  |
| GET /api/v1/whatsapp/verify | GET | `/api/v1/whatsapp/verify?hub.mode=subscribe&hub.verify_token=local-verify-token&hub.challenge=meu-challenge-12345` | ok | 200 |  |
| GET /api/v1/whatsapp/inbound | GET | `/api/v1/whatsapp/inbound?hub.mode=subscribe&hub.verify_token=local-verify-token&hub.challenge=meu-challenge-12345` | ok | 200 |  |
| POST /api/v1/whatsapp/inbound | POST | `/api/v1/whatsapp/inbound` | ok | 200 |  |
| POST /api/v1/whatsapp/inbound invalid signature | POST | `/api/v1/whatsapp/inbound` | ok | 401 |  |
| POST /api/v1/channels/telegram/webhook | POST | `/api/v1/channels/telegram/webhook` | ok | 200 |  |
| POST /api/v1/channels/telegram/webhook invalid token | POST | `/api/v1/channels/telegram/webhook` | ok | 401 |  |
