# Refactor Prompt Enriquecido — `internal/card`

## Prompt original

> Utilize factories ou algum padrão de projeto que faça sentido. Estou vendo muita regra, muito parser. Utilize DDD tático, procure referência de programação funcional para ajudar a melhorar. Uso obrigatório de `@.claude/skills/go-implementation/`, carregando sob demanda apenas `architecture.md` + `interfaces.md` + `examples-domain-flow.md` e `testing.md` quando reescrever suites, com máximo de 4 referências simultâneas. Foco em eficiência, robustez, production-ready e sem falso positivo.

## Prompt enriquecido

Você vai refatorar exclusivamente o módulo `internal/card`.

Antes de editar:

1. Carregue `AGENTS.md`.
2. Carregue `.claude/skills/go-implementation/SKILL.md`.
3. Carregue apenas:
   - `architecture.md`
   - `interfaces.md`
   - `examples-domain-flow.md`
   - `testing.md` somente se reescrever suites
4. Verifique `go.mod` e registre drift de scripts ausentes.

Arquivos prioritários:

- `internal/card/module.go`
- `internal/card/domain/services/timezone.go`
- `internal/card/domain/services/billing_cycle.go`
- `internal/card/application/usecases/`
- `internal/card/infrastructure/http/server/handlers/`

Problema a atacar:

- O bootstrap do módulo chama `services.MustLoadSaoPauloOrExit()`, o que merece revisão frente às regras do repositório contra `os.Exit` fora de `main`.
- Há oportunidade de separar melhor inicialização, política temporal e regras de billing cycle.
- O módulo tem fluxo CRUD claro; qualquer refatoração precisa manter essa simplicidade.

Objetivo da refatoração:

- Tornar o bootstrap mais seguro e previsível.
- Remover inicialização rígida ou implícita quando ela puder virar dependência explícita.
- Consolidar regras de data/ciclo em componentes pequenos e testáveis.
- Melhorar robustez sem multiplicar tipos e sem criar falso positivo arquitetural.

Direção sugerida:

- Avalie substituir a inicialização “must exit” por factory function explícita que valide dependências e retorne erro ao bootstrap do módulo.
- Avalie funções puras para cálculo de billing cycle, timezone normalization e projeções de datas, mantendo I/O fora desses pontos.
- Considere encapsular política temporal em serviço concreto pequeno, sem introduzir `clock.Clock` em use cases.
- Se houver configuração opcional demais, functional options só fazem sentido se reduzirem acoplamento real; não use por moda.

Restrições:

- Não criar abstrações adicionais no CRUD se o fluxo já estiver claro.
- Não mover regra de negócio para handlers.
- Não manter `os.Exit` fora de `main` se a análise confirmar que ele é evitável.
- Não introduzir interface nova sem consumidor real.

Critérios de aceitação:

- O módulo continua simples de entender.
- Inicialização problemática é eliminada ou isolada com erro explícito.
- Regras temporais ficam mais determinísticas e testáveis.
- Validação proporcional executada; `testing.md` apenas se houver reescrita real de suites.

Formato esperado da resposta final do executor:

1. Qual problema estrutural no bootstrap foi corrigido.
2. Quais regras ficaram puras e mais testáveis.
3. Qual padrão foi adotado e por que ele é o menor possível.
4. Evidências de validação.

## Justificativas do enriquecimento

- O prompt foi amarrado ao ponto mais sensível do módulo hoje: inicialização temporal com `os.Exit`.
- A orientação funcional aqui faz mais sentido em cálculos puros de ciclo e datas do que em patterns amplos.
- O foco é remover risco operacional mantendo o módulo enxuto.

## Variante curta

> Refatore `internal/card` focando em `module.go` e `domain/services/timezone.go`, eliminando inicialização rígida com `os.Exit` fora de `main`, tornando dependências temporais explícitas e extraindo regras de billing cycle para funções puras ou serviços concretos pequenos. Carregue `AGENTS.md`, `.claude/skills/go-implementation/SKILL.md`, `architecture.md`, `interfaces.md`, `examples-domain-flow.md` e `testing.md` apenas se reescrever suites. Preserve a simplicidade do CRUD e evite interfaces artificiais.

## Referências

- `architecture.md`, `interfaces.md`, `examples-domain-flow.md` da skill `go-implementation`
- Go Blog: Go Concurrency Patterns: Context — https://go.dev/blog/context
- Go Blog: Errors are values — https://go.dev/blog/errors-are-values
- Rob Pike: Self-referential functions and the design of options — https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html
