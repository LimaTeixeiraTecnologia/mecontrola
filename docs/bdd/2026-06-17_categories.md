# BDD: Módulo Categories
**Data:** 2026-06-17
**Status:** MVP Robust / Production-Ready
**Referência:** Domain Modeling Made Functional (DMMF)

## Objetivo
Prover uma taxonomia robusta para transações, permitindo a classificação automática via dicionário e busca por similaridade.

## Fluxo 1: Resolução de Categoria por Slug
**Funcionalidade:** Identificação unívoca de categorias.

**Cenário:** Busca de categoria por slug amigável
- **Dado** que o slug 'alimentacao' existe no sistema
- **Quando** o sistema solicita a resolução por esse slug
- **Então** os detalhes da categoria `Alimentação` devem ser retornados
- **E** o ID correspondente deve ser usado para futuras associações.

## Fluxo 2: Pesquisa no Dicionário de Termos
**Funcionalidade:** Classificação inteligente de despesas.

**Cenário:** Mapeamento de descrição de fatura para categoria
- **Dado** que o termo 'IFood' está mapeado para a categoria 'Alimentação' no dicionário
- **Quando** uma transação com descrição 'IFOOD *LANCHES' é processada
- **Então** o sistema deve sugerir ou aplicar automaticamente a categoria 'Alimentação'.

## Fluxo 3: Listagem e Hierarquia
**Funcionalidade:** Organização estruturada de categorias.

**Cenário:** Listagem de categorias ativas
- **Dado** que existem categorias de nível superior e subcategorias
- **Quando** o usuário solicita a listagem
- **Então** o sistema deve retornar a árvore hierárquica completa
- **E** apenas categorias não excluídas devem aparecer.

## Regras de Domínio (DMMF)
- **Value Objects:** O `Slug` deve ser normalizado (lowercase, sem espaços, sem acentos).
- **Discriminated Union:** Uma entrada de dicionário pode ser do tipo `ExactMatch` ou `PatternMatch`.
- **Pure Functions:** A lógica de validação de subcategoria em relação à categoria pai deve ser pura.

## Validação de Produção
- [ ] Verificar indexação de slugs no banco de dados para buscas rápidas.
- [ ] Validar integridade referencial entre dicionário e categorias.
- [ ] Garantir que a busca no dicionário suporte caracteres especiais e variações comuns.
