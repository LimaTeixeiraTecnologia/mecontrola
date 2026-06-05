# Prompt enriquecido — `create-technical-specification` para `identity-foundation`

## Prompt original

```text
Eu quero usar a skill .agents/skills/create-technical-specification de forma eficiente, robusta e mandatória, sem desvios para implementação com base no .specs/prd-identity-foundation/prd.md.

É inegociável que seja robusto, eficiente e production-ready/proof inegociável.
É obrigatório que carregue em toda implementação golang a skill go-implementation e seus exemplos sobdemanda.

o handler http deve ser escrito em: internal/identity/infrastructure/http/server como ponto de entrada e registrado a rota em: internal/identity/infrastructure/http/server e depois usar o srv.RegisterRouters(userModule.UserRouter)
o handler job deve ser escrito em: internal/identity/infrastructure/jobs/handlers para ser utilizado depois em: cmd/worker/worker.go
o handler consumer deve ser escrito em: internal/identity/infrastructure/messaging/database/consumers e também com registro em: cmd/worker/worker.go
o handler producer deve ser escrito em: internal/identity/infrastructure/messaging/database/producers e também com registro em: cmd/worker/worker.go

Mandatório e inegociável seguir esse modelo em TODOS os módulos. utilizar em todas camadas handler -> usecase e ou service -> repository e ou client
via DI o11y observability.Observability e o seguinte
    ctx, span := r.o11y.Tracer().Start(ctx, "user_repository.insert")
    defer span.End()

logger e tracing no que realmente é importante e propagar para as outras camadas para ter o caminho de pão corretamente. (mandatório e inegociável, sera invalido se não seguir)
no repositório deve receber via DI db database.DBTX e usar: query := `insert into ...`; stmt, err := r.db.PrepareContext(ctx, query); ... span.RecordError(err) ... r.o11y.Logger().Error(...)

E toda DI deve ser registrado em: module.go ... toda rota http, job, e consumer, deve permitir registro fácil no startup do projeto.

Não implemente nada, somente crie/enriqueça o prompt.
```

## Diagnóstico do prompt original

### Intenção principal

Produzir um prompt de alto rigor para a skill `create-technical-specification`, em PT-BR, usando `.specs/prd-identity-foundation/prd.md` como fonte principal, sem permitir desvio para implementação e já carregando no documento técnico todas as restrições mandatórias de arquitetura, DI, observabilidade, wiring e pontos de entrada do módulo `internal/identity`.

### Contexto confirmado no working tree

- O repositório segue `AGENTS.md` como fonte canônica de governança.
- O projeto é um monólito modular em Go com bounded contexts em `internal/`.
- `go.mod` declara `go 1.26.2` com `toolchain go1.26.4`.
- Os entrypoints reais do runtime hoje são `cmd/server/server.go` e `cmd/worker/worker.go`.
- `cmd/server/server.go` ainda não registra módulos de negócio; portanto qualquer proposta deve partir desse entrypoint real e registrar drift quando necessário.
- `cmd/worker/worker.go` já monta `jobs []worker.Job` e `worker.NewManager(...)`; portanto jobs, consumers e producers devem ser especificados de forma compatível com esse bootstrap real.
- `internal/identity/module.go` existe, mas está vazio além do `package identity`; logo o prompt precisa exigir tech spec ancorada no working tree atual e registrar explicitamente lacunas em vez de inventar implementação pronta.

### Ambiguidades e riscos que precisavam ser fechados

1. O pedido mistura **escopo de especificação técnica** com **restrições obrigatórias para implementação futura**.
2. O pedido define caminhos e padrões mandatórios mesmo quando parte da estrutura ainda não está materializada no código atual.
3. O trecho de repositório fornecido é prescritivo, mas precisa ser tratado como **shape obrigatório de design/handoff**, não como instrução para codar agora.
4. Faltava um formato de saída objetivo para impedir respostas vagas, genéricas ou que saiam da rota da skill `create-technical-specification`.

## Prompt enriquecido

```text
Você deve executar a skill `.agents/skills/create-technical-specification` de forma mandatória, rigorosa, eficiente, robusta e production-ready/proof, usando como fonte principal o arquivo `.specs/prd-identity-foundation/prd.md`.

Sua missão NÃO é implementar nada.
Sua missão é produzir exclusivamente a especificação técnica downstream desse PRD, pronta para handoff de execução, sem gerar código, sem propor scaffolding executado, sem migrations implementadas, sem handlers implementados, sem editar runtime, e sem desviar para codificação.

## Objetivo

Gerar uma especificação técnica completa, executável por outro agente/time, que:

1. respeite integralmente `AGENTS.md` como fonte canônica;
2. use o working tree atual como fonte da verdade quando houver conflito documental;
3. parta obrigatoriamente dos entrypoints reais `cmd/server/server.go` e `cmd/worker/worker.go`;
4. detalhe como o módulo `internal/identity` deve ser estruturado e conectado sem implementar;
5. deixe explícitas as restrições mandatórias de arquitetura, DI, observabilidade, tracing, logging, handlers, repositories, clients, jobs, consumers, producers e registro em startup;
6. produza um artefato técnico robusto o suficiente para implementação posterior sem ambiguidades e sem falso positivo de pronto.

## Fontes obrigatórias

Leia e use como base mínima:

- `AGENTS.md`
- `.agents/skills/agent-governance/SKILL.md`
- `.specs/prd-identity-foundation/prd.md`
- `go.mod`
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- `internal/identity/module.go`

Se alguma documentação histórica divergir do working tree atual, o working tree prevalece. Registre o drift explicitamente em vez de mascará-lo.

## Restrições mandatórias e inegociáveis

### 1. Escopo da entrega

- Entregar somente a **technical specification**.
- Não implementar código.
- Não gerar diff.
- Não criar mocks de implementação.
- Não propor “já deixei pronto” ou qualquer desvio para execução.
- Não inventar arquivos, handlers, routers, consumers, producers, adapters, repositories ou wiring como se já existissem no workspace.
- Quando algo ainda não existir no código atual, descreva como **alvo da especificação** e marque como lacuna/drift do estado atual.

### 2. Governança obrigatória

- Respeitar `AGENTS.md` integralmente.
- Assumir monólito modular em Go com bounded contexts em `internal/`.
- Respeitar o fluxo `infrastructure -> application -> domain`.
- `domain` não pode depender de `application`, `infrastructure`, `platform`, banco, HTTP, filas, serialização, configuração ou drivers.
- `application` pode depender de `domain`, mas não de `infrastructure`.
- Comunicação cross-module só pode acontecer via interface declarada pelo consumidor, domain event/outbox ou contrato explícito.
- Proibir abstrações prematuras e composição genérica fora do padrão do repositório.

### 3. Regra obrigatória para futuras implementações Go

A tech spec deve declarar explicitamente uma seção de **handoff obrigatório para implementação** dizendo que:

1. toda implementação Go decorrente desta spec DEVE carregar `.agents/skills/go-implementation/SKILL.md`;
2. a implementação DEVE carregar os exemplos e referências da skill `go-implementation` apenas sob demanda, conforme a superfície alterada;
3. a implementação DEVE verificar a versão de Go em `go.mod` antes de usar APIs, padrões ou dependências;
4. a implementação DEVE seguir as etapas 1 a 5 da skill `go-implementation`;
5. qualquer execução futura sem esse carregamento deve ser considerada inválida.

### 4. Ponto de partida obrigatório do runtime

É proibido usar `internal/platform/runtime` como ponto de partida.

A especificação deve partir obrigatoriamente de:

- `cmd/server/server.go` para HTTP
- `cmd/worker/worker.go` para jobs, consumers e producers

Explique como o wiring deve se conectar a esses entrypoints reais, respeitando o bootstrap atual e registrando quaisquer lacunas do working tree.

### 5. Layout mandatório por superfície

A technical specification deve afirmar como regra obrigatória e inegociável para `internal/identity` — e como modelo a ser repetido em todos os módulos — o seguinte layout:

- HTTP handler e router inbound em `internal/identity/infrastructure/http/server`
- Job handler em `internal/identity/infrastructure/jobs/handlers`
- Consumer handler em `internal/identity/infrastructure/messaging/database/consumers`
- Producer handler em `internal/identity/infrastructure/messaging/database/producers`
- DI centralizada em `internal/identity/module.go`

### 6. Registro em startup obrigatório

A spec deve deixar explícito que:

- o registro HTTP deve permitir encaixe simples no bootstrap do servidor via `srv.RegisterRouters(userModule.UserRouter)` ou forma equivalente aderente ao artefato real do módulo;
- jobs, consumers e producers devem ser desenhados para registro simples em `cmd/worker/worker.go`;
- toda rota HTTP, job, consumer e producer deve sair da composição do módulo pronta para bootstrap simples no startup do projeto;
- a spec deve mostrar o caminho de composição do bootstrap, mas sem implementar.

### 7. Cadeia obrigatória de dependências

A tech spec deve impor que o desenho siga sempre a cadeia:

- `handler -> usecase`
- e/ou `handler -> service`
- e depois `usecase/service -> repository`
- e/ou `usecase/service -> client`

Não permita acoplamento indevido pulando camadas.
Handlers não acessam repository direto.
Jobs e consumers delegam para a camada `application`.

### 8. DI obrigatória em `module.go`

A technical specification deve exigir DI manual explícita em `module.go`, no padrão do repositório, com construtor do módulo, struct concreta e campos nomeados apenas para artefatos reais do bounded context.

O documento deve orientar que a composição siga a ordem:

- repository/client
- usecase
- handler
- router/job/consumer/producer

O documento deve usar como referência estrutural o shape abaixo, adaptado ao contexto real do repositório e SEM copiar cegamente imports, nomes externos ou dependências de outro projeto:

```go
type UserModule struct {
    UserRouter *userHttp.UserRouter
}

func NewUserModule(
    db database.DBTX,
    cfg *configs.Config,
    o11y observability.Observability,
    tokenGenerator auth.TokenGenerator,
    tokenValidator auth.TokenValidator,
) UserModule {
    // wiring manual explícito:
    // repository -> usecase -> handler -> router
}
```

A spec deve deixar claro que esse snippet é apenas **shape de composição mandatória**, não código a ser copiado literalmente.

### 9. Observabilidade obrigatória via `o11y`

A technical specification deve declarar como regra mandatória:

- usar `o11y observability.Observability` via DI;
- preferir o identificador `o11y`, não `provider`;
- propagar contexto entre camadas;
- criar tracing e logging apenas nos pontos relevantes;
- preservar o “caminho de pão” de observabilidade entre handler, use case/service, repository e client quando houver I/O relevante.

Quando exemplificar o padrão, use este shape:

```go
ctx, span := r.o11y.Tracer().Start(ctx, "user_repository.insert")
defer span.End()
```

Explique que a spec deve indicar onde abrir spans, quais operações merecem log estruturado e como os erros devem ser anotados sem poluir camadas desnecessariamente.

### 10. Regra mandatória para repository com `database.DBTX`

A tech spec deve exigir que repositories recebam via DI:

- `db database.DBTX`
- `o11y observability.Observability`

E deve explicitar o shape obrigatório de acesso SQL com `PrepareContext`, tratamento de erro observável e fechamento de statement, por exemplo:

```go
query := `insert into
            users (
                id,
                name,
                email,
                password,
                created_at,
                updated_at,
                deleted_at
            )
            values
            ($1, $2, $3, $4, $5, $6, $7)`

stmt, err := r.db.PrepareContext(ctx, query)
if err != nil {
    span.RecordError(err)
    r.o11y.Logger().Error(ctx, "query_failed",
        observability.String("operation", "insert"),
        observability.String("layer", "repository"),
        observability.String("entity", "user"),
        observability.String("user_id", user.ID.String()),
        observability.Error(err),
    )
    r.fm.RecordRepositoryFailure(ctx, "insert", "user", "infra", time.Since(start))
    return nil, fmt.Errorf("preparing insert user statement: %w", err)
}

defer func() {
    if closeErr := stmt.Close(); closeErr != nil {
        span.RecordError(closeErr)
        r.o11y.Logger().Error(ctx, "Insert: failed to close stmt",
            observability.Error(closeErr),
        )
    }
}()
```

Deixe explícito que esse trecho representa um **shape mandatório de design para o repositório**, a ser adaptado ao contexto real do módulo, e que a spec deve derivar regras de implementação a partir dele sem implementar agora.

### 11. Proibição de desvio para implementação

É proibido:

- transformar a tech spec em diff;
- gerar código de exemplo executável além de pequenos snippets ilustrativos de shape;
- sugerir “implementar já”;
- pular diretamente para handlers, repositories, module wiring ou migrations prontos;
- responder com generalidades que não se conectem ao working tree real.

## O que a saída deve conter obrigatoriamente

Entregue a technical specification em Markdown, PT-BR, cobrindo no mínimo:

1. **Resumo executivo**
   - objetivo da spec;
   - vínculo direto com `.specs/prd-identity-foundation/prd.md`;
   - confirmação explícita de que não haverá implementação neste passo.

2. **Leitura do estado atual**
   - o que existe hoje em `cmd/server/server.go`;
   - o que existe hoje em `cmd/worker/worker.go`;
   - o que existe hoje em `internal/identity/module.go`;
   - lacunas, placeholders e drift relevantes.

3. **Escopo incluído / fora de escopo**
   - o que entra nesta tech spec;
   - o que fica para execução posterior;
   - o que não deve ser antecipado.

4. **Arquitetura e fronteiras**
   - bounded context;
   - camadas;
   - contratos entre camadas;
   - limites entre `domain`, `application` e `infrastructure`;
   - contratos cross-module, se houver.

5. **Design por superfície**
   - HTTP inbound em `internal/identity/infrastructure/http/server`;
   - jobs em `internal/identity/infrastructure/jobs/handlers`;
   - consumers em `internal/identity/infrastructure/messaging/database/consumers`;
   - producers em `internal/identity/infrastructure/messaging/database/producers`;
   - repositories/clients necessários;
   - como cada superfície se conecta ao módulo e ao bootstrap.

6. **Wiring e DI**
   - desenho detalhado do `module.go`;
   - dependências de entrada;
   - artefatos expostos pelo módulo;
   - sequência de composição `repository/client -> usecase -> handler -> router/job/consumer/producer`;
   - como permitir registro fácil em `cmd/server/server.go` e `cmd/worker/worker.go`.

7. **Observabilidade e logging**
   - uso mandatória de `o11y observability.Observability`;
   - propagação de contexto;
   - abertura de spans;
   - logs relevantes e não excessivos;
   - trilha correta entre camadas.

8. **Persistência**
   - uso de `database.DBTX` por DI;
   - shape obrigatório de repository com `PrepareContext`;
   - tratamento de erro, `span.RecordError`, logging estruturado e fechamento de resources;
   - decisões de transação, idempotência e consistência, se aplicável ao PRD.

9. **Bootstrap e runtime**
   - como o HTTP será registrado a partir de `cmd/server/server.go`;
   - como jobs/consumers/producers serão registrados a partir de `cmd/worker/worker.go`;
   - quais adaptações de startup serão necessárias;
   - o que é drift atual versus alvo da spec.

10. **Critérios de aceite da própria tech spec**
   - a spec será considerada inválida se não ancorar as decisões no working tree;
   - a spec será considerada inválida se desviar para implementação;
   - a spec será considerada inválida se não explicitar `module.go`, `o11y`, `database.DBTX`, handlers, bootstrap HTTP/worker e a cadeia entre camadas;
   - a spec será considerada inválida se não determinar o uso obrigatório de `go-implementation` nas futuras implementações Go.

11. **Plano de handoff para execução**
   - checklist do que um executor deverá carregar antes de codar;
   - dependências e referências a consultar sob demanda;
   - riscos e pontos que exigem atenção na implementação;
   - como validar aderência futura ao desenho sem sair do escopo desta etapa.

## Critérios de qualidade obrigatórios

Sua saída deve ser:

- específica ao módulo `identity`;
- ancorada no PRD e no working tree atual;
- explícita sobre lacunas e drift;
- compatível com `AGENTS.md`;
- robusta o suficiente para execução posterior sem interpretação livre;
- enxuta no que for acessório e detalhada no que for mandatória;
- production-ready/proof como especificação, não como implementação.

## Tratamento de ambiguidades

Se encontrar divergência entre:

- PRD
- working tree
- exemplos históricos
- instruções desta solicitação

faça o seguinte:

1. priorize o working tree atual;
2. preserve as restrições inegociáveis explicitadas aqui;
3. registre a divergência em uma seção de drift/decisões;
4. não invente código para “resolver” a divergência.
```

## Justificativa das adições

| Adição | Motivo |
| --- | --- |
| Fontes obrigatórias (`AGENTS.md`, PRD, `go.mod`, `cmd/server/server.go`, `cmd/worker/worker.go`, `internal/identity/module.go`) | Força a skill a partir do contexto real do repositório e reduz alucinação. |
| Regra explícita de “somente technical specification” | Bloqueia desvio para implementação, que era o principal risco do pedido original. |
| Seções mandatórias de layout, bootstrap e DI | Transforma preferências prescritivas em contrato técnico verificável no documento. |
| Handoff obrigatório para `go-implementation` | Garante que a future implementation já saia condicionada à skill correta de Go. |
| Observabilidade via `o11y` e tracing/logging propagado | Preserva a convenção exigida e evita uma spec “cega” para operação real. |
| Shape obrigatório de repository com `database.DBTX` e `PrepareContext` | Converte o padrão desejado em requisito de desenho sem antecipar código. |
| Critérios de invalidação da própria tech spec | Torna a qualidade mensurável e diminui margem para respostas genéricas. |

## Variante curta de execução

Se quiser uma versão mais curta para colar diretamente na skill, use:

```text
Execute `.agents/skills/create-technical-specification` em PT-BR usando `.specs/prd-identity-foundation/prd.md` como fonte principal, sem implementar nada. A saída deve ser exclusivamente uma technical specification production-ready/proof, ancorada em `AGENTS.md`, `go.mod`, `cmd/server/server.go`, `cmd/worker/worker.go` e `internal/identity/module.go`, registrando drift quando o working tree divergir dos docs.

A spec deve impor como regra mandatória para `internal/identity` — e como modelo para todos os módulos — o layout `internal/identity/infrastructure/http/server`, `internal/identity/infrastructure/jobs/handlers`, `internal/identity/infrastructure/messaging/database/consumers`, `internal/identity/infrastructure/messaging/database/producers`, com DI centralizada em `module.go`, cadeia `repository/client -> usecase -> handler -> router/job/consumer/producer`, observabilidade obrigatória via `o11y observability.Observability`, propagação de contexto, tracing/logging relevante e repository recebendo `database.DBTX` por DI com shape de `PrepareContext`, `span.RecordError`, log estruturado e fechamento explícito de statement.

É obrigatório declarar no handoff que toda implementação Go posterior deve carregar `.agents/skills/go-implementation/SKILL.md` e seus exemplos sob demanda, verificando `go.mod` antes de usar APIs ou dependências. É proibido partir de `internal/platform/runtime`; use obrigatoriamente `cmd/server/server.go` e `cmd/worker/worker.go` como entrypoints reais. A spec será inválida se desviar para implementação ou se não explicitar bootstrap HTTP/worker, module wiring, handlers, `o11y`, `database.DBTX` e critérios de aceite da própria spec.
```
