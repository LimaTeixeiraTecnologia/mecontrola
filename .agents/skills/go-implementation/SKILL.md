---
name: go-implementation
version: 2.0.0
category: language
prerequisites: [agent-governance]
description: Implementa alteracoes em codigo Go usando governanca base, selecao deterministica de contexto, regras estritas por risco e validacao proporcional ao escopo alterado. Use quando a tarefa exigir adicionar, corrigir, refatorar ou validar codigo Go. Nao use para tarefas sem codigo Go, documentacao geral ou triagem sem alteracao.
---

# Implementacao Go

## Objetivo

Ser o entrypoint canonico para tarefas Go sem carregar contexto em excesso nem impor gates globais por padrao. Esta skill:

1. valida o baseline (`AGENTS.md`, `go.mod`, arquitetura);
2. classifica a mudanca por superficie e risco;
3. carrega apenas as referencias exigidas pela matriz;
4. escolhe validacoes proporcionais ao escopo;
5. separa claramente o que e `authoring-ready`, `merge-ready` e `production-ready`.

## Procedimentos

### Etapa 1: Carregar base obrigatoria
1. Confirmar que o contrato de carga base definido em `AGENTS.md` foi cumprido.
2. Ler `references/architecture.md`.
3. Executar `bash scripts/verify-go-mod.sh`.
4. Ler `go.mod` do modulo resolvido pelo script.
5. Carregar as **Regras Estritas Obrigatorias (R0-R7)** desta skill.
6. Ler `references/INDEX.yaml` para resolver `task_type`, `required_refs`, `optional_refs`, `forbidden_refs` e `validation_profile`.

### Etapa 2: Classificar a mudanca antes de carregar contexto
Classificar a tarefa em um `task_type` da matriz. Escolher o mais especifico possivel:

- `usecase-read`
- `usecase-write`
- `repository`
- `module-wiring`
- `http-handler`
- `consumer`
- `job-handler`
- `producer`
- `testing-unit`
- `testing-integration`
- `cross-cutting`

Regras de classificacao:
- usar primeiro `file_patterns`;
- usar `diff_signals` apenas para desempate;
- promover para `cross-cutting` quando a mudanca tocar multiplas superficies com contratos acoplados;
- quando houver ambiguidade, preferir a opcao mais segura dentro do escopo alterado, nao o gate global.

### Etapa 3: Carregar somente o contexto exigido
1. Carregar todas as referencias em `required_refs` do `task_type`.
2. Carregar `optional_refs` apenas se a superficie alterada realmente exigir.
3. Respeitar `forbidden_refs`.
4. Nunca carregar mais de 4 referencias simultaneas alem do entrypoint e `architecture.md`; se houver mais candidatas, priorizar as 3 criticas e registrar as nao carregadas.

### Etapa 4: Modelar antes de editar
1. Identificar o menor conjunto seguro de mudancas.
2. Mapear dependencias, fronteiras de IO e risco de regressao.
3. Preferir tipos concretos por padrao.
4. Introduzir interface apenas quando houver fronteira consumidora real.
5. Escolher profile de validacao pela matriz:
   - `local-minimal`
   - `boundary`
   - `global`

### Etapa 5: Implementar
1. Editar seguindo a versao Go declarada no `go.mod` resolvido.
2. Adaptar exemplos ao contexto real em vez de copiar cegamente.
3. Atualizar testes apenas quando a mudanca afetar comportamento observavel ou contrato.
4. Tratar references granulares como contrato da superficie alterada.

### Etapa 6: Validar proporcionalmente
1. Seguir Etapa 4 de `.agents/skills/agent-governance/SKILL.md`.
2. Aplicar o `validation_profile` da matriz:
   - `local-minimal`: formatter + build/test/vet no pacote ou arquivos afetados; lint quando disponivel e proporcional.
   - `boundary`: validacoes do pacote alterado + bounded context ou entrypoint afetado.
   - `global`: usar somente para mudanca transversal, wiring amplo, configuracao global, contratos compartilhados ou multiplos modulos.
3. Usar checks heuristicos (`grep`, `find`, `rg`) como triagem, nunca como verdade unica sem leitura do diff.
4. Reportar quais gates rodaram, em qual escopo, e por que escopos mais amplos nao foram necessarios.

## Estados de prontidao

- `authoring-ready`: contexto suficiente foi carregado pela matriz e a implementacao pode comecar.
- `merge-ready`: os gates proporcionais ao `validation_profile` passaram ou tiveram ausencia registrada explicitamente.
- `production-ready`: alem de `merge-ready`, a superficie alterada tem evidencias minimas de seguranca operacional adequadas ao risco:
  - handler/consumer/job/producer: adapter fino, sem regra de negocio fora do use case;
  - repository: contratos e testes da fronteira de persistencia coerentes;
  - module-wiring: bootstrap, lifecycle e dependencias consistentes;
  - configuracao/runtime: defaults, timeout, cancelamento e shutdown verificados quando aplicavel.

## Regime de severidade

- `[HARD]`: regra objetiva, verificavel e bloqueante independentemente da superficie.
- `[HARD contextual]`: bloqueante apenas quando o `task_type` ou o diff ativar a regra.
- `[ADVISORY]`: nao bloqueante; exige registro da decisao quando ignorada em mudanca relevante.

Nao promover para `[HARD]` uma regra que dependa de heuristica fraca ou de interpretacao ampla do diff.

## Regras Estritas Obrigatorias (R0-R7)

### Indice das regras

| Regra | Tema | Severidade padrao | Detalhamento |
|-------|------|-------------------|--------------|
| R0 | `init()` PROIBIDA | `[HARD]` | `references/architecture.md` |
| R1 | Funcoes de dominio/aplicacao/infra como metodos de struct | `[HARD contextual]` | `references/architecture.md` |
| R2 | Proibir alias local de campo sem transformacao | `[HARD contextual]` | inline abaixo |
| R3 | Mocks via `mockery.yml` quando a estrategia de teste depender de mocks gerados | `[HARD contextual]` | `references/testing.md`, `references/testing-unit.md` |
| R4 | `testify/suite` quando a superficie e o contexto de teste exigirem suite stateful | `[HARD contextual]` | `references/testing.md`, `references/testing-unit.md` |
| R5 | Estilo e erros idiomaticos | `[HARD]` salvo marcacao | referencias tematicas |
| R6 | Contratos Go (`context`, DI, interface no consumidor) | `[HARD]` | `references/interfaces.md` e referencias granulares |
| R7 | Recursos modernos do Go conforme `go.mod` | `[HARD]` | `references/generics.md`, `references/observability.md` |

### R2 — Proibir atribuicao direta de campo sem transformacao `[HARD contextual]`

E proibido extrair um campo de struct para variavel local que apenas o renomeia, sem transformacao real.

```go
nome := user.Name
email := user.Email
return &dtos.UserOutput{Name: nome, Email: email}, nil
```

Correto:

```go
return &dtos.UserOutput{Name: user.Name, Email: user.Email}, nil
```

Ativar a regra quando o diff tocar mapeamento de DTO, projection, presenter, serializer ou transformacao de output.

### R5 — Regras transversais sempre ativas

- **5.8** Enums com `iota + 1` quando zero value significar nao inicializado.
- **5.10** Erros com `errors.New`, `%w`, sentinels e tipos customizados conforme necessidade do caller.
- **5.11** Type assertion sempre com comma-ok.
- **5.12** Sem `panic` em producao; excecoes apenas as previstas em `main`.
- **5.15** Nao usar nomes built-in como identificadores.
- **5.19** Preferir `strconv` a `fmt` para conversoes primitivas.
- **5.20** Especificar capacidade de slice/map quando conhecida.
- **5.21/5.22** Reduzir aninhamento com early return.
- **5.23** Imports em 3 grupos.
- **5.24** Nomes de pacote minusculos, especificos e sem underscore.
- **5.25** Ordem de arquivo coerente.
- **5.26** Globais nao exportados em camelCase.
- **5.27-5.31** Inicializacao idiomatica de structs, maps e zero values.
- **5.37** Limite suave de 99 caracteres por linha `[ADVISORY]`.
- **5.47-5.49** Format strings fora de Printf como `const`; funcoes Printf-style terminam com `f`.
- **5.50** Functional Options quando houver mais de 3 campos opcionais relevantes.

### R6 — Contratos e design

- **6.1** `context.Context` obrigatorio em fronteiras de I/O e como primeiro parametro.
- **6.2** Tipos concretos por padrao; interface sob demanda real.
- **6.3** Interface definida no consumidor.
- **6.4** `var _ Interface = (*Type)(nil)` PROIBIDO `[HARD]`.
- **6.6** Use case de escrita com Command Object em linguagem ubiqua `[HARD contextual]`.
- **6.7** `clock.Clock` proibido em use case e repositorio `[HARD contextual]`.

### R7 — Recursos modernos do Go

Antes de aplicar qualquer recurso, verificar `go.mod`. Se a versao declarada for inferior, nao usar e registrar a restricao.

- `any` em vez de `interface{}`
- `log/slog` para logging estruturado
- `slices`, `maps`, `cmp`, `min`, `max`, `clear` quando apropriado
- `errors.Join`
- range sobre inteiro quando a versao permitir
- generics quando remover duplicacao real
- `sync.OnceValue`/`sync.OnceValues` apenas em bootstrap
- `iter.Seq`/`iter.Seq2` apenas quando houver ganho real em iteracao lazy

## Patterns frequentes

- **Factory Function:** `New*(deps...) (*T, error)` quando houver invariantes ou dependencias obrigatorias.
- **Functional Options:** quando existir combinacao relevante de campos opcionais.
- **Adapter:** para implementar interface do consumidor delegando a dependencia externa.
- **Decorator:** para comportamento transversal como log, metrica ou retry.
- **Facade:** para orchestrar dependencias em operacao de alto nivel.

## Tratamento de Erros

- Se `go.mod` estiver ausente ou ambiguo, parar antes de assumir versao ou dependencias.
- Se a matriz nao cobrir a superficie alterada, usar `cross-cutting`, registrar a lacuna e carregar apenas referencias diretamente ligadas ao diff.
- Se um comando de validacao nao existir, registrar a ausencia; nao inventar substitutos.
- Se mais de uma abordagem parecer plausivel, preferir a que tiver menor carga de contexto, menor indirecao e menor custo de teste dentro da seguranca exigida.
