# US-CARD-CONC-001: Controle de concorrência otimista na edição e exclusão de cartão

## Resumo e decisão de escopo
História única de habilitação com resultado observável para o usuário. Fecha um gap de
integridade de dados no módulo `internal/card`: a coluna `version` do cartão é mantida e
incrementada, mas **nunca é usada como guarda de concorrência**, o que permite *lost update*
silencioso quando duas edições concorrem (dois dispositivos, retry fora da janela de
idempotência, HTTP + agente conversacional simultâneos). O restante do sistema
(`transactions`, `recurring_templates`, `budgets/expenses`) já aplica o padrão; o cartão é o
único agregado mutável que não o faz. Esta US traz o cartão à paridade com esse padrão canônico.

## Confronto com o Codebase
Investigação executada em `internal/card` e comparação com os demais agregados mutáveis.

- Coluna existe e é incrementada, mas sem guarda: `internal/card/infrastructure/repositories/postgres/card_repository.go:290` faz `version = version + 1`, porém o `WHERE` em `internal/card/infrastructure/repositories/postgres/card_repository.go:292` é `WHERE id = $6 AND user_id = $7 AND deleted_at IS NULL` — **sem `AND version = $esperado`**.
- Entidade carrega `Version` mas o decider de update o preserva sem incrementar nem comparar: `internal/card/domain/entities/card.go:19` e `internal/card/domain/services/decide_update_card.go:48`.
- DTO de update não recebe versão esperada: `internal/card/application/dtos/input/update_card.go:9-15` (campos `ID`, `UserID`, `Nickname`, `Bank`, `DueDay`; nenhum `ExpectedVersion`).
- Handler HTTP não lê versão nem `If-Match`: `internal/card/infrastructure/http/server/handlers/update.go:24-28` (corpo aceita apenas `nickname`, `bank`, `due_day`).
- Saída não expõe `version` ao cliente, impossibilitando o round-trip: `internal/card/application/dtos/output/card.go:5-16` e `internal/card/application/mappers/card_mapper.go:20-37`.
- Soft-delete também não guarda por versão: `internal/card/infrastructure/repositories/postgres/card_repository.go:335-365` (`UPDATE ... SET deleted_at ... WHERE id AND user_id AND deleted_at IS NULL`).
- Padrão canônico já aplicado em outros agregados (evidência de que cartão é o outlier): `internal/transactions/infrastructure/repositories/postgres/transaction_repository.go:141` e `:191` (`AND version = $N`); `internal/transactions/infrastructure/repositories/postgres/recurring_template_repository.go:99` e `:151` (`AND version=$N`); `internal/budgets/infrastructure/repositories/postgres/expense_repository.go:94` e `:128` (`AND version = $N`).
- Consumidores afetados pelo mesmo use case: `internal/card/infrastructure/http/server/handlers/update.go` (HTTP) e `internal/agents/module.go:157-158` (`UpdateCardUC`, `SoftDeleteCardUC` chamados pelo agente conversacional). Ambos os caminhos passam por `internal/card/application/usecases/update_card.go:50`.
- Já existe erro sentinela reutilizável de conflito no módulo: `internal/card/domain/errors.go:7` (`ErrNicknameConflict`), sinalizando que o módulo modela conflitos como erro de domínio (novo `ErrVersionConflict` seguirá o mesmo padrão).

## Declaração
Como usuário do MeControla que gerencia seus cartões pelo app e pelo WhatsApp, quero que uma
alteração ou exclusão de cartão seja rejeitada quando ela se baseia em uma versão desatualizada
do cartão, para que edições concorrentes não sobrescrevam silenciosamente umas às outras e eu
não perca dados sem aviso.

## Contexto
- Problema: o cartão mantém `version` no banco, na entidade e no `UPDATE` (incrementa em `version = version + 1`), mas o guarda de concorrência (`WHERE ... AND version = $esperado`) não existe; o campo é decorativo. Duas escritas concorrentes resultam em *last-write-wins* silencioso, sem retorno de conflito. A janela de idempotência protege apenas retries idênticos com a mesma chave, não edições concorrentes distintas.
- Resultado esperado: uma escrita que informa a versão que o cliente observou por último é aplicada apenas se essa ainda é a versão vigente; caso contrário retorna conflito determinístico (HTTP 409 no adapter, `ErrVersionConflict` no domínio), sem mutar o registro. `version` passa a ser exposto na saída para permitir o round-trip. O cartão fica em paridade com `transactions`, `recurring_templates` e `budgets/expenses`.
- Fonte: análise do módulo `internal/card` solicitada pelo usuário em 2026-07-11, com evidências em `## Confronto com o Codebase`.

## Regras de Negócio
- RN-01: `version` do cartão DEVE ser exposto na representação de saída (`output.Card`) para que o cliente possa reenviá-lo como versão esperada.
- RN-02: a operação de update DEVE aceitar uma versão esperada opcional. Quando presente, o `UPDATE` só efetiva se a versão persistida for igual à esperada; o `version` é incrementado atomicamente na mesma instrução.
- RN-03: a operação de soft-delete DEVE aceitar uma versão esperada opcional com a mesma semântica de RN-02.
- RN-04: versão esperada divergente da persistida DEVE resultar em conflito de versão (`ErrVersionConflict` no domínio, HTTP 409 no adapter), sem alterar o registro; deve ser distinguível de cartão inexistente (`ErrCardNotFound`, HTTP 404).
- RN-05: compatibilidade retroativa — quando a versão esperada não é informada, o comportamento atual (update/delete sem guarda de versão) é preservado, para não quebrar clientes existentes.
- RN-06: a mesma guarda vale para os dois caminhos que consomem `UpdateCardUC`/`SoftDeleteCardUC`: HTTP (`handlers/update.go`, `handlers/delete.go`) e agente conversacional (`internal/agents`), pois ambos passam pelo mesmo use case.
- RN-07: o guarda de versão convive com o guarda de idempotência existente sem duplicá-lo; idempotência trata retry da mesma requisição, versão trata concorrência entre requisições distintas.
- RN-08: o guarda de versão é escopado por dono — o `WHERE` continua restrito a `id`, `user_id` e `deleted_at IS NULL`, e passa a incluir `version`, preservando o isolamento por usuário.

## Critérios de Aceite
```gherkin
Cenário: edição com versão esperada correta é aplicada e incrementa a versão
  Dado um cartão do usuário na versão 3
  E o cliente informa a versão esperada 3 ao atualizar o apelido
  Quando o update é processado
  Então o apelido é atualizado
  E a versão persistida passa a ser 4
  E a resposta inclui o campo version igual a 4

Cenário: edição com versão esperada desatualizada é rejeitada sem mutar o registro
  Dado um cartão do usuário na versão 4
  E o cliente informa a versão esperada 3 ao atualizar o apelido
  Quando o update é processado
  Então a operação retorna conflito de versão
  E o adapter HTTP responde 409
  E o apelido e a versão do cartão permanecem inalterados

Cenário: exclusão com versão esperada desatualizada é rejeitada
  Dado um cartão do usuário na versão 5
  E o cliente informa a versão esperada 4 ao excluir o cartão
  Quando o soft-delete é processado
  Então a operação retorna conflito de versão
  E o adapter HTTP responde 409
  E o cartão permanece ativo, sem deleted_at

Cenário: compatibilidade retroativa sem versão esperada preserva o comportamento atual
  Dado um cliente que não informa versão esperada ao atualizar um cartão existente
  Quando o update é processado
  Então a atualização é aplicada
  E a versão é incrementada normalmente

Cenário: versão esperada correta porém cartão inexistente retorna não encontrado
  Dado um identificador de cartão que não pertence ao usuário
  E o cliente informa qualquer versão esperada
  Quando o update é processado
  Então a operação retorna cartão não encontrado
  E o adapter HTTP responde 404, distinto de 409 de conflito
```

## Dados e Permissões
- Dados obrigatórios: `id` do cartão, `user_id` do principal autenticado, versão esperada (opcional; inteiro `>= 1` quando informado), campos mutáveis já existentes (`nickname`, `bank`, `due_day`). Saída passa a incluir `version` (inteiro `>= 1`).
- Perfis/permissões: usuário autenticado dono do cartão. O fluxo herda os middlewares já aplicados em `internal/card/infrastructure/http/server/router.go:70-75` (auth de gateway, injeção de principal, `RequireUser`, rate limit por usuário). Nenhum novo perfil é introduzido.

## Dependências
- Coluna `mecontrola.cards.version` já existente em `migrations/000001_initial_schema.up.sql:574` (nenhuma migração de schema é necessária para a guarda).
- Interfaces e assinaturas de repositório de cartão em `internal/card/application/interfaces/repository.go` (assinaturas de `UpdateByIDForUser` e `SoftDeleteByIDForUser` recebem a versão esperada).
- Adapter de binding do agente em `internal/agents/infrastructure/binding/card_manager_adapter.go`, que repassa a versão esperada quando disponível ao chamar `UpdateCardUC`/`SoftDeleteCardUC`.
- Regra de zero comentários em Go de produção (R-ADAPTER-001.1) e testes canônicos testify/suite (R-TESTING-001) para os use cases alterados.

## Fora de Escopo
- Adicionar novos atributos de cartão como limite de crédito, bandeira, últimos quatro dígitos ou cor.
- Criar estado de arquivamento distinto do soft-delete.
- Expor via HTTP os use cases hoje restritos ao agente (`CountCards`, `IsBankRecognized`, `ResolveCardByNickname`).
- Reconciliação ou fluxo conversacional de re-confirmação após conflito (o agente apenas propaga o conflito nesta US; a experiência de re-confirmar pertence à US de edição conversacional de cartão).
- Alterar a semântica de idempotência já existente nos use cases de escrita.

## Evidências
- Entrada: pedido do usuário em 2026-07-11 para analisar `internal/card`, identificar gap real e produzir uma única história de usuário salva em `docs/us`.
- Base de código: `card_repository.go:290` e `:292` (incremento sem guarda de versão); `card_repository.go:335-365` (soft-delete sem guarda); `decide_update_card.go:48` (versão preservada sem incremento no domínio); `update_card.go:9-15` (DTO sem versão esperada); `handlers/update.go:24-28` (handler sem versão/If-Match); `output/card.go:5-16` e `card_mapper.go:20-37` (saída sem `version`); `entities/card.go:19` (`Version` na entidade); `errors.go:7` (padrão de conflito como erro de domínio); `transaction_repository.go:141`/`:191`, `recurring_template_repository.go:99`/`:151`, `expense_repository.go:94`/`:128` (padrão canônico já aplicado nos demais agregados); `agents/module.go:157-158` (agente consome `UpdateCardUC`/`SoftDeleteCardUC`); `migrations/000001_initial_schema.up.sql:574` (coluna `version` já existe).
- Inferências: o não-uso da coluna `version` como guarda é omissão, não decisão documentada — sustentado pela existência do incremento em SQL, da coluna na entidade e do padrão idêntico aplicado em três outros repositórios do mesmo projeto.
- Não evidenciado: não há teste, ADR ou comentário no repositório que declare intenção de manter o cartão sem controle de concorrência otimista; busca executada em `internal/card` e migrações não encontrou tal registro.

## Notas de Validação
- Estrutura conforme `assets/modelo-historia-usuario.md` da skill `user-stories`; validada com `python3 .agents/skills/user-stories/scripts/validar-historias-usuario.py docs/us/2026-07-11-us-controle-concorrencia-otimista-cartao.md` (resultado SUCESSO).
- História de habilitação com resultado observável ao usuário (proteção contra perda silenciosa de edição), portanto não é história puramente técnica.
- Cinco cenários Gherkin cobrindo feliz, conflito em update, conflito em delete, compatibilidade retroativa e distinção conflito-vs-inexistente.
- Sem lacunas materiais abertas: a semântica de conflito reutiliza o padrão canônico já presente no projeto, eliminando ambiguidade de desenho.

## Notas Técnicas para Desenvolvimento
- Domínio: `UpdateCardDecider.Decide` deve incrementar `Version` (`current.Version + 1`) ao materializar o cartão decidido, alinhando entidade e persistência; adicionar `ErrVersionConflict` em `internal/card/domain/errors.go` seguindo o estilo dos sentinelas existentes.
- Repositório: acrescentar `AND version = $esperado` ao `WHERE` de `UpdateByIDForUser` e `SoftDeleteByIDForUser` quando a versão esperada for informada; mapear `sql.ErrNoRows`/`RowsAffected == 0` distinguindo conflito de versão (registro existe em outra versão) de cartão inexistente — a distinção pode ser feita por uma checagem de existência dentro da mesma transação (o use case já roda em `uow.Do`), preservando atomicidade.
- Aplicação: estender `input.UpdateCard` e `input.SoftDeleteCard` com `ExpectedVersion *int64`; validar em `Validate()` que, quando presente, é `>= 1` (R-DTO-VALIDATE-001), logo após `defer span.End()`.
- Saída: incluir `Version int64 json:"version"` em `output.Card` e preencher em `mappers.M.ToCardOutput`.
- Adapters HTTP: aceitar a versão esperada (corpo JSON `expected_version` ou header `If-Match`) em `handlers/update.go` e `handlers/delete.go`, e mapear `ErrVersionConflict` para HTTP 409 no `mapCardError`; manter 404 para `ErrCardNotFound`.
- Agente: `card_manager_adapter` propaga a versão esperada quando o fluxo conversacional a conhece; quando não a conhece, opera em modo retrocompatível (RN-05).
- Validação proporcional ao risco (toca `domain/` e API/contrato): build, vet, test com `-race`, lint e gates de governança do projeto; testes de use case no padrão testify/suite (R-TESTING-001) cobrindo os cinco cenários.
