# Tarefa 3.0: Integração CategoriesReader — candidatos raiz+folha e ResolveForWrite

<critical>Ler prd.md, techspec.md e scenarios.md desta pasta — tarefa invalidada se pulado</critical>

## Visão Geral

Implementar a integração do workflow de pendência com `internal/categories` via `CategoriesReader`: busca de candidatos com `SearchDictionary`, enriquecimento de `rootSlug`/`subcategorySlug`, apresentação de múltiplos candidatos, bloqueio de raiz sem folha e validação final via `ResolveForWrite`. `internal/categories` é a única autoridade canônica de classificação — nenhum ID de categoria pode vir do LLM ou de texto livre sem revalidação.

<requirements>
- SearchDictionary retorna candidatos; enriquecer rootSlug e subcategorySlug via ResolveForWrite ou ListCategories antes de apresentar ao usuário
- Toda opção persistível deve carregar rootCategoryId, rootSlug, subcategoryId, subcategorySlug (RF-28, RF-29, D-04)
- Raiz sem subcategoria folha bloqueia escrita (RF-30) — agente apresenta subcategorias disponíveis em vez de registrar
- Quando SearchDictionary retorna múltiplos candidatos plausíveis, apresentar lista curta numerada com raiz+folha; não escolher o primeiro automaticamente (RF-27)
- Seleção de candidato aceita índice numérico OU nome da categoria (RF-42); ambos resolvem o mesmo par raiz+folha canônico antes da revalidação por ResolveForWrite
- Quando usuário informa categoria por nome livre, resolver novamente via contrato canônico antes de persistir (RF-14)
- ResolveForWrite(rootID, subcategoryID, kind, expectedVersion) deve ser chamado antes de montar RawTransaction (RF-11, RF-13)
- CategorySource = "user_selected_candidate" quando usuário escolheu após clarificação (RF-13)
- Zero fallback para categoria genérica, raiz sem folha, primeira da lista ou LLM-estimada (RF-35)
- Zero comentários Go de produção
</requirements>

## Subtarefas

- [ ] 3.1 Verificar/completar a interface `CategoriesReader` em `internal/agents/application/interfaces/` para expor `SearchDictionary(ctx, text, kind)` e `ResolveForWrite(ctx, rootID, subcategoryID, kind, expectedVersion)`
- [ ] 3.2 Implementar `DecideCategoryChoice` (já criada em 1.0) para usar os candidatos enriquecidos; validar que cada candidato tem `SubcategoryID != uuid.Nil` antes de aceitar como escolha válida
- [ ] 3.3 Implementar step do workflow responsável por chamar `SearchDictionary` quando `AwaitingSlot=Category` e `ResumeText` é fornecido — enriquecer candidatos com slugs antes de retornar para decisão
- [ ] 3.4 Implementar bloqueio de raiz sem folha: se candidato escolhido tem `SubcategoryID == uuid.Nil` ou `ResolveForWrite` falha, manter pendência e apresentar subcategorias via `ListCategories`
- [ ] 3.5 Implementar caminho de múltiplos candidatos: quando `SearchDictionary` retorna ≥2 candidatos plausíveis, construir lista curta numerada com raiz+folha canônicos para apresentação; `DecideCategoryChoice` aceita índice numérico OU nome (RF-42); não escolher automaticamente
- [ ] 3.6 Implementar revalidação de nome livre: quando usuário responde com texto (ex.: "farmácia"), chamar `SearchDictionary` → enriquecer → `ResolveForWrite` antes de aceitar candidato
- [ ] 3.7 Testes unitários com double de `CategoriesReader` cobrindo: candidato único aceito, múltiplos candidatos listados, raiz sem folha bloqueada, `ResolveForWrite` falhando rejeita candidato

## Detalhes de Implementação

Ver `techspec.md` seções **"Candidato Categorial Canônico"**, **"Retomada de Pendência"** (passos 3–5) e **"Interfaces Chave"**.

Contrato de candidato canônico (RF-28):

```
RootCategoryID  uuid.UUID  (ex: 66cb85a0-3266-5900-b8e3-13cdcd00ab62)
RootSlug        string     (ex: "custo-fixo")
SubcategoryID   uuid.UUID  (ex: 97fa4b86-d43c-5ad5-a99b-c88c8427fb30)
SubcategorySlug string     (ex: "supermercado")
```

Fluxo de resolução (RF-11, RF-13, RF-35):

1. `SearchDictionary(ctx, resumeText, kind)` → lista de candidatos brutos
2. Para cada candidato com par root+sub, chamar `ResolveForWrite(rootID, subID, kind, version)` para enriquecer slugs e validar
3. Se 1 candidato válido → `DecideCategoryChoice` retorna aceitar
4. Se ≥2 candidatos válidos → montar lista curta numerada para apresentação; aguardar escolha por índice numérico OU nome (RF-42), ambos resolvendo o mesmo par raiz+folha (`AwaitingSlot=Category` continua)
5. Se 0 candidatos válidos → reprompt com sugestões de categorias raiz disponíveis

Raiz sem folha (RF-30): se `SubcategoryID == uuid.Nil` ou `ResolveForWrite` retorna erro para o par, bloquear e listar subcategorias da raiz via `ListCategories(rootID)`.

Cenário G7-03 do scenarios.md: usuário responde "custo fixo" (raiz) → listar subcategorias disponíveis de Custo Fixo → `AwaitingSlot=Category` permanece; zero escrita.

Cenário G7-11: candidato escolhido cujo `ResolveForWrite` falha → manter pendência, pedir nova escolha.

`CategoryVersion` no `PendingEntryState` deve ser preenchido com a versão editorial retornada por `ResolveForWrite` para garantir que a escrita em 4.0 use a versão correta.

## Critérios de Sucesso

- `go build ./internal/agents/...` passa após integração
- `go test -race -count=1 ./internal/agents/application/...` verde
- SearchDictionary → candidato único → aceite direto sem apresentar lista (G1-01..G6-15 quando categoria é clara)
- SearchDictionary → múltiplos candidatos → lista apresentada com rootSlug+subcategorySlug (G7-10)
- Raiz sem folha → bloqueio → subcategorias listadas (G7-03, CA-09)
- ResolveForWrite falhando para candidato → zero escrita → nova escolha solicitada (G7-11)
- Gate RF-13: grep para qualquer uso de ID de categoria vindo de string livre sem passar por ResolveForWrite deve retornar vazio

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — SearchDictionary e ResolveForWrite são bindings de CategoriesReader consumidos pelo workflow pending-entry do consumidor internal/agents; contrato categorial raiz+folha é primitivo do substrato agent da plataforma

## Testes da Tarefa

- [ ] `categories_reader_pending_test.go` (ou similar): double de CategoriesReader — candidato único aceito, múltiplos listados, raiz sem folha bloqueada, ResolveForWrite falha → rejeita
- [ ] Cenários G7-03 (raiz sem folha), G7-10 (múltiplos candidatos), G7-11 (ResolveForWrite falha), G7-12 (correção de descrição → re-resolve), CA-15 (escolha por índice numérico e por nome resolvem o mesmo par raiz+folha)
- [ ] Cenários representativos G1-G6: G1-02 (supermercado), G1-25 (farmácia), G3-01 (delivery), G6-03 (décimo terceiro) como unit tests de decisão com double

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/interfaces/categories_reader.go` (verificar/completar)
- `internal/agents/application/interfaces/types.go` (adicionar campos de candidato categorial se ausentes)
- `internal/agents/infrastructure/binding/categories_reader_adapter.go` (adapter existente — verificar se expõe SearchDictionary e ResolveForWrite)
- `internal/agents/application/workflows/pending_entry_workflow.go` (de 2.0 — step de resolução categorial)
- `internal/agents/application/workflows/pending_entry_state.go` (de 1.0 — PendingCategoryCandidate)
- `internal/categories/application/usecases/search_dictionary.go`
- `internal/categories/application/usecases/resolve_category_for_write.go`
- `internal/transactions/application/interfaces/category_write_gate.go`
- `.specs/prd-conversa-agentiva-fluida/techspec.md` (seção "Candidato Categorial Canônico")
- `.specs/prd-conversa-agentiva-fluida/scenarios.md` (G7-03, G7-10, G7-11, G7-12, CA-04, CA-09)
