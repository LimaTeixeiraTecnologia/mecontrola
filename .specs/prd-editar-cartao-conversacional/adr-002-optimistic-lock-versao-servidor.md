# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Optimistic lock com versão gerenciada no servidor e `ExpectedVersion` opcional
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Solicitante do produto, time de plataforma/agentes
- **Relacionados:** PRD `.specs/prd-editar-cartao-conversacional/prd.md` (RF-06, RF-20), techspec `.specs/prd-editar-cartao-conversacional/techspec.md`, ADR-001, ADR-003

## Contexto

A tool `update_card` exige hoje um campo `version` do LLM (`update_card.go:22,49`), mas nenhuma ferramenta de identificação (`resolve_card`/`list_cards`/`get_card`) expõe `version`: o `Version` existe na entidade (`internal/card/domain/entities/card.go:19`) mas **não é propagado** em `cardoutput.Card`, no `card_mapper`, em `interfaces.Card` nem em `interfaces.CardUpdate`. Consequência: o agente não tem como obter a versão sem inventá-la — o que é proibido — e a edição conversacional fica inviável.

Além disso, o repositório **não faz lock otimista real**: `UpdateByIDForUser` executa `version = version + 1` sem `WHERE version = $x` (`card_repository.go:280-333`). Duas edições concorrentes (por exemplo, uma via REST e outra via conversa dentro da janela de confirmação de 15 min) sobrescrevem-se silenciosamente.

O PRD decidiu (RF-06/RF-20) que a versão é gerenciada pelo servidor: o workflow captura a versão no início da confirmação e a revalida no commit, abortando se o cartão mudou; o LLM nunca lida com versão. A decisão técnica confirmada é implementar o lock **no repositório, de forma atômica**, com `ExpectedVersion` **opcional** para manter o endpoint REST compatível.

## Decisão

1. **Expor `Version` de ponta a ponta:**
   - `internal/card/application/dtos/output/card.go`: adicionar `Version int64`.
   - `internal/card/application/mappers/card_mapper.go`: mapear `c.Version`.
   - `internal/agents/application/interfaces/types.go`: `Card` ganha `Version int64`.
   - `internal/agents/infrastructure/binding/card_manager_adapter.go`: `mapCardOutput` propaga `Version`.
2. **`ExpectedVersion` opcional no contrato de update:**
   - `internal/card/application/dtos/input/update_card.go`: `UpdateCard` ganha `ExpectedVersion *int64`.
   - `internal/agents/application/interfaces/types.go`: `CardUpdate` ganha `ExpectedVersion *int64`; binding propaga.
3. **Lock otimista atômico no repositório:**
   - `UpdateByIDForUser(ctx, c entities.Card, expectedVersion *int64) (entities.Card, error)`.
   - Quando `expectedVersion != nil`, a query inclui `AND version = $expected`; em 0 linhas afetadas, uma verificação de existência decide entre `ErrCardVersionConflict` (existe, versão divergente) e `ErrCardNotFound` (não existe / soft-deleted).
   - Quando `expectedVersion == nil` (caminho REST atual), a query e o comportamento são idênticos aos de hoje.
   - Novo sentinela `ErrCardVersionConflict` em `internal/card/domain/errors.go`.
4. **Fluxo no agente:** a tool `update_card` remove `version` do schema; lê o cartão atual via `GetCard` para capturar `Version` (e os valores atuais para o de-para, ADR-003); o `CardUpdateState` guarda `ExpectedVersion`; no commit, `executeUpdateCard` chama `UpdateCard` com `ExpectedVersion = capturada`; o banco revalida atomicamente. Conflito é classificado como erro de domínio (terminal, sem retry) com mensagem determinística.
5. **Use case `update_card` do módulo card:** após carregar o cartão corrente, se `ExpectedVersion != nil && corrente.Version != *ExpectedVersion` retorna `ErrCardVersionConflict`; propaga `expectedVersion` ao repositório para o guard atômico.

## Alternativas Consideradas

- **Lock somente na camada do agente (relê e compara).** O workflow releria o cartão no commit e compararia a `Version` capturada, sem tocar `internal/card`.
  - Vantagens: menor blast radius; não altera o módulo card nem o caminho REST.
  - Desvantagens: janela TOCTOU entre o relê e o `UpdateCard` — uma escrita concorrente pode passar na fresta; não é lock real.
  - Motivo da rejeição: não garante atomicidade; robustez inferior para uma operação de escrita.
- **Expor `version` ao LLM (opção descartada no PRD).**
  - Desvantagens: a versão fica velha na janela de 15 min, gerando conflitos falsos ou desatualização; coloca responsabilidade de concorrência no LLM.
  - Motivo da rejeição: contraria RF-06.
- **Híbrido (agente compara + repo protege).**
  - Vantagens: defesa em profundidade.
  - Desvantagens: dupla checagem redundante, mais código.
  - Motivo da rejeição: o guard atômico do repositório já é suficiente e mais simples.

## Consequências

### Benefícios Esperados

- Fecha o gap que hoje inviabiliza a edição conversacional (versão obtível pelo servidor).
- Lock otimista real e atômico, detectando edição concorrente sem sobrescrita silenciosa.
- Endpoint REST 100% compatível (comportamento inalterado quando `ExpectedVersion` é nil).
- `Version` exposta também beneficia observabilidade e futuros consumidores.

### Trade-offs e Custos

- Evolui o módulo `internal/card` (DTO de saída, mapper, DTO de entrada, use case, interface e adapter do repositório) e regenera mocks.
- A assinatura de `UpdateByIDForUser` muda (novo parâmetro), exigindo atualizar o único chamador.

### Riscos e Mitigações

- Risco: desambiguar 0 linhas (conflito vs not-found) exigir consulta extra. Mitigação: uma verificação de existência barata por `id`+`user_id`; ocorre apenas no caminho de erro.
- Risco: regressão no REST. Mitigação: `ExpectedVersion` opcional (nil = comportamento atual); teste de integração cobre ambos os caminhos.
- Risco: conflito de versão tratado como retriável causaria sobrescrita. Mitigação: classificar `ErrCardVersionConflict` como erro de domínio terminal (sem retry), com mensagem orientando nova tentativa (RF-20).

## Plano de Implementação

1. `ErrCardVersionConflict`; `Version` em `cardoutput.Card` + mapper.
2. `ExpectedVersion` em `cardinput.UpdateCard`; revalidação no use case.
3. `UpdateByIDForUser(expectedVersion)` com guard atômico + desambiguação; testes de repositório.
4. `interfaces.Card.Version` / `interfaces.CardUpdate.ExpectedVersion` + binding + mocks.
5. Consumo no workflow de edição (ADR-001/003).
Concluído quando: build/vet/lint/race verdes; testes unit + integration de conflito de versão passam; REST inalterado comprovado por teste.

## Monitoramento e Validação

- Métrica `agents_card_update_confirm_total{outcome}` inclui `error` para conflito; trace do Run registra o erro de domínio.
- Critério de sucesso: edições concorrentes nunca se sobrescrevem silenciosamente; usuário recebe mensagem determinística no conflito.
- Revisar se o REST passar a exigir concorrência forte (aí adotar `ExpectedVersion` também no handler REST).

## Impacto em Documentação e Operação

- Documentar `Version` no contrato de saída do cartão e o comportamento de conflito.
- Runbook: sinal de conflito de versão e orientação de reprocessamento pelo usuário.

## Revisão Futura

- Reavaliar se o endpoint REST deve adotar lock otimista explícito (retornar 409) em uma iteração futura.
