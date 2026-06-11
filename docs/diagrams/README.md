# MeControla C4 Container Diagrams

Este diretorio concentra a documentacao arquitetural no nivel de container para o sistema MeControla, agora separada por modulo e por visao sistemica.

## Estrutura

- [system](./system/)
  Diagramas globais, convencoes e legenda.
- [billing](./billing/flows.md)
- [budgets](./budgets/flows.md)
- [card](./card/flows.md)
- [categories](./categories/flows.md)
- [identity](./identity/flows.md)
- [onboarding](./onboarding/flows.md)

## Escopo

- Processos em escopo:
  - `cmd/server`
  - `cmd/worker`
- Modulos documentados:
  - `internal/billing`
  - `internal/budgets`
  - `internal/card`
  - `internal/categories`
  - `internal/identity`
  - `internal/onboarding`

## Renderizacao

Gerar ou atualizar os SVGs com:

```bash
plantuml -tsvg docs/diagrams/system/mecontrola-container.puml docs/diagrams/system/mecontrola-async-relations.puml
```

## Observacoes

- Os `.puml` em `system/` sao a fonte canonica dos diagramas globais.
- Os `.svg` em `system/` sao artefatos renderizados para leitura direta.
- O detalhamento de percurso runtime fica em `flows.md` dentro de cada modulo.
