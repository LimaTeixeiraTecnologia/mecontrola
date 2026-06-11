# Card Flows

![Card container context](../system/mecontrola-container.svg)

## Objetivo do modulo

`internal/card` oferece CRUD autenticado de cartoes de credito e uma consulta derivada de fatura por competencia.

## Arquivos .puml por fluxo

- [CARD-01-create-card.puml](./CARD-01-create-card.puml)
- [CARD-02-list-cards.puml](./CARD-02-list-cards.puml)
- [CARD-03-get-card.puml](./CARD-03-get-card.puml)
- [CARD-04-update-card.puml](./CARD-04-update-card.puml)
- [CARD-05-delete-card.puml](./CARD-05-delete-card.puml)
- [CARD-06-invoice-for.puml](./CARD-06-invoice-for.puml)

## Entradas, saidas e artefatos

### Endpoints

- `POST /api/v1/cards/`
- `GET /api/v1/cards/`
- `GET /api/v1/cards/{id}/`
- `PUT /api/v1/cards/{id}/`
- `DELETE /api/v1/cards/{id}/`
- `GET /api/v1/cards/{id}/invoices`

### Middleware relevante

- `InjectPrincipalFromHeaderWithO11y`
- `RequireUserWithO11y`
- `idempotency.Middleware("card", storage, 24h, o11y)` em create/update/delete

### Saidas

- Escrita em repositorio de cartoes
- Escrita em armazenamento de idempotencia

## Matriz de fluxos

| ID | Origem | Tipo | Saida principal |
| --- | --- | --- | --- |
| CARD-01 | `POST /api/v1/cards/` | sync | Cria cartao e grava chave de idempotencia |
| CARD-02 | `GET /api/v1/cards/` | sync | Lista cartoes do usuario |
| CARD-03 | `GET /api/v1/cards/{id}/` | sync | Busca detalhe do cartao |
| CARD-04 | `PUT /api/v1/cards/{id}/` | sync | Atualiza cartao com idempotencia |
| CARD-05 | `DELETE /api/v1/cards/{id}/` | sync | Soft delete com idempotencia |
| CARD-06 | `GET /api/v1/cards/{id}/invoices` | sync | Calcula janela de fatura |

## Percurso detalhado

### CARD-01 - Criar cartao

Origem:
- `CreateCardHandler.Handle`

Percurso:
1. O router aplica middlewares de principal e autenticacao.
2. O router aplica `idempotency.Middleware` no POST.
3. O handler decodifica `name`, `nickname`, `closing_day`, `due_day`.
4. Chama `CreateCard.Execute`.
5. O use case usa UoW de escrita e `idempotency.Storage`.
6. O repositorio persiste o cartao.
7. O middleware retorna a resposta original em retries equivalentes dentro da janela.

Banco:
- Escrita em tabela de cartoes
- Escrita em storage de idempotencia

### CARD-02 - Listar cartoes

Origem:
- `ListCardsHandler.Handle`

Percurso:
1. O principal autenticado e lido do contexto.
2. O handler chama `ListCards.Execute`.
3. O use case consulta repositorio por `user_id`.
4. Retorna lista redigida via DTO de saida.

### CARD-03 - Buscar cartao

Origem:
- `GetCardHandler.Handle`

Percurso:
1. O handler extrai `{id}`.
2. Chama `GetCard.Execute`.
3. O use case consulta repositrio por `card_id` e `user_id`.

### CARD-04 - Atualizar cartao

Origem:
- `UpdateCardHandler.Handle`

Percurso:
1. O router aplica middleware de idempotencia no PUT.
2. O handler decodifica payload e `id`.
3. Chama `UpdateCard.Execute`.
4. O use case valida VO de nome/apelido/dias de ciclo.
5. Persiste a atualizacao.

Banco:
- Leitura/escrita em cartoes
- Escrita em storage de idempotencia

### CARD-05 - Excluir cartao

Origem:
- `DeleteCardHandler.Handle`

Percurso:
1. O router aplica middleware de idempotencia no DELETE.
2. O handler chama `SoftDeleteCard.Execute`.
3. O use case marca o cartao como removido logicamente.

Banco:
- Escrita em cartoes
- Escrita em storage de idempotencia

### CARD-06 - Consultar janela de fatura

Origem:
- `InvoiceForHandler.Handle`

Percurso:
1. O handler extrai `card_id` e parametros de consulta.
2. Chama `InvoiceFor.Execute`.
3. O use case usa `services.NewSaoPauloLocation()` para calcular o ciclo de fatura.
4. Le o cartao, calcula datas de abertura/fechamento/vencimento e responde DTO.

## Rotas internas e dependencias cruzadas

- O modulo `card` nao publica eventos e nao registra consumers ou jobs.
- O acoplamento com `identity` acontece apenas por middleware de autenticacao no processo HTTP.

## Observacoes arquiteturais

- A idempotencia e parte do contrato HTTP de mutacoes do modulo.
- O modulo e estritamente sincrono no estado atual.

## Eficiencia, robustez e operacao

- `Caminho critico`
  - o custo dominante e IO de banco para cartoes e storage de idempotencia.
- `Controles de robustez`
  - middlewares de principal e autenticacao em todas as rotas;
  - idempotencia em create, update e delete com TTL de 24h;
  - soft delete reduz risco de perda irreversivel em mutacao incorreta.
- `Falhas esperadas`
  - payload invalido ou violacao de VO: falha definitiva de request;
  - indisponibilidade do storage de idempotencia: falha transiente que afeta mutacoes;
  - retries do cliente com mesma key devem reproduzir resposta anterior.
- `Observabilidade`
  - handlers registram spans e logs redigidos;
  - monitorar taxa de reuse de idempotency key e conflitos de mutacao.
- `Capacidade`
  - modulo simples e sincrono; gargalos tendem a ser conexao SQL e volume de requests HTTP.

## Guardrails operacionais

### Precondicoes e pos-condicoes

- mutacoes:
  - pre: principal autenticado e chave de idempotencia presente quando exigida pelo cliente;
  - pos: cartao criado/atualizado/removido logicamente sem duplicacao de efeito em retries.
- leituras:
  - pre: principal autenticado;
  - pos: somente cartoes do usuario corrente sao retornados.

### Invariantes

- `user_id` do contexto delimita totalmente o escopo de acesso;
- retries com a mesma chave nao podem criar dois cartoes;
- soft delete nao deve reaparecer em listagens normais.

### Runbook resumido

- mutacoes duplicadas:
  - verificar propagacao da chave de idempotencia pelo cliente;
  - checar storage de idempotencia e TTL.
- aumento de 5xx:
  - inspecionar conexoes SQL e latencia do storage de idempotencia;
  - validar erros de serializacao de resposta cacheada.

### Sinais e thresholds recomendados

- alerta em falhas repetidas do storage de idempotencia;
- alerta se latencia de create/update/delete divergir materialmente das leituras;
- alerta em crescimento anomalo de retries idempotentes.
