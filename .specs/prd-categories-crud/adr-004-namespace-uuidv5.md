# ADR-004: Namespace UUIDv5 derivado do domínio

## Metadados

- **Título:** Namespace UUIDv5 derivado do domínio
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Time de engenharia MeControla
- **Relacionados:** PRD RF-01

## Contexto

O PRD exige IDs determinísticos UUIDv5 para categorias e entradas de dicionário, calculados sobre "namespace fixo do módulo categories e o par `(kind, slug)`". Não havia padrão prévio no codebase.

## Decisão

O namespace fixo é gerado por UUIDv5 a partir do nome DNS do domínio do produto:

```go
var categoryNamespace = uuid.NewSHA1(uuid.Nil, []byte("mecontrola.io/categories"))
```

Uso:
```go
id := uuid.NewSHA1(categoryNamespace, []byte(kind+"+"+slug))
```

## Alternativas Consideradas

| Alternativa | Vantagens | Desvantagens | Motivo de rejeição |
|---|---|---|---|
| UUID Nil direto como namespace | Mínimo, nenhuma computação | Se outro módulo usar UUIDv5 com nil e nomes coincidirem, colisão | Sem isolamento de módulo |
| UUID v4 fixo arbitrário | Único e isolado | Valor mágico sem significado; precisa documentar em múltiplos lugares | Menos previsível e auditável |
| Namespace por tabela (categories vs dictionary) | Isolamento ainda maior | Overhead desnecessário; PRD fala em "namespace fixo do módulo" (singular) | Excesso de granularidade |

## Consequências

### Benefícios Esperados

- Determinístico e reproduzível em qualquer ambiente.
- Isolado de outros módulos que possam adotar UUIDv5 no futuro.
- Documentado semanticamente (deriva do domínio).

### Trade-offs e Custos

- Se o domínio do produto mudar, o namespace permanece o mesmo (imutável por contrato). Isso é desejável.

### Riscos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Colisão interna com mesmo kind+slug | Dois itens com mesmo ID | Índice único `(kind, slug)` impede inserção duplicada; teste de integração valida |

## Plano de Implementação

1. Declarar `categoryNamespace` como `var` no pacote `domain/entities` ou `domain/valueobjects`.
2. Adicionar factory `NewCategoryID(kind, slug string) uuid.UUID`.
3. Usar factory em migrations e em validações de teste.

## Monitoramento e Validação

- Teste de integração que recalcula UUIDv5 a partir de kind+slug e confere com valor persistido.
