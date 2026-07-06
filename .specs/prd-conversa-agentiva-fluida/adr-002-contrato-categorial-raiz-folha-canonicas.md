# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Contrato categorial com raiz e subcategoria folha canônicas
- **Data:** 2026-07-06
- **Status:** Aceita
- **Decisores:** Engenharia / Produto MeControla
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O PRD exige 0 falso positivo em escrita categorizada. O usuário explicitou que uma categoria persistível deve carregar raiz canônica, por exemplo `66cb85a0-3266-5900-b8e3-13cdcd00ab62` + `custo-fixo`, e subcategoria folha, por exemplo `c2fda6a3-c329-52c8-81ea-771b6ea4f365` + `aluguel`.

## Decisão

Toda escolha categorial usada para escrita deve conter `rootCategoryId`, `rootSlug`, `subcategoryId`, `subcategorySlug`, path e versão editorial. `SearchDictionary` só produz candidatos; a escrita só pode seguir após `internal/categories` validar o par raiz + folha por `ResolveForWrite` e `internal/transactions` aceitar a evidência pelo `CategoryWriteGate`.

## Alternativas Consideradas

- Usar apenas subcategoria e inferir raiz: reduz payload, mas enfraquece auditoria e clareza do contrato.
- Mostrar apenas nome amigável sem IDs no contrato: bom para UX, insuficiente para persistência auditável.
- Aceitar categoria raiz sem folha: rejeitado por violar o PRD e o gate de transações.

## Consequências

### Benefícios Esperados

- Escrita auditável e reproduzível.
- Bloqueio objetivo para raiz sem folha e categoria ambígua.
- Melhor diagnóstico de falhas de categoria.

### Trade-offs e Custos

- Pode exigir enriquecer candidatos com slug se `SearchDictionary` não retornar slug diretamente.
- Payload de pendência fica maior.

### Riscos e Mitigações

- Risco de drift editorial entre classificação e escrita. Mitigação: `ExpectedVersion` em `ResolveForWrite`.
- Risco de usuário escolher texto livre ambíguo. Mitigação: resolver novamente e apresentar opções canônicas.
- Risco de confundir `CategoryID` do candidato com raiz. Mitigação: contrato diferencia explicitamente `RootCategoryID` e `SubcategoryID` e bloqueia raiz igual à folha.

## Plano de Implementação

1. Enriquecer candidato categorial no adapter.
2. Persistir candidatos no snapshot da pendência.
3. Validar escolha via `ResolveForWrite`.
4. Montar evidência categorial antes de chamar `TransactionsLedger`.

## Monitoramento e Validação

Validar por testes que toda escrita tem raiz, folha, slugs, versão e evidência. Monitorar bloqueios com outcome enum, sem labels de categoria.

## Impacto em Documentação e Operação

Atualizar documentação de contratos agentivos e cenários de aceite categorial.

## Revisão Futura

Revisar apenas se o domínio permitir criação/edição de taxonomia pelo usuário.
