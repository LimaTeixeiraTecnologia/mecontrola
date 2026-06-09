# Tarefa 3.0: Seed editorial do dicionário mínimo

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar migration de seed do dicionário cobrindo: (a) nomes canônicos de todas as subcategorias, (b) aliases e frases inequívocos do RF-33, (c) termos ambíguos do RF-34. RF-35 (merchants expandidos) fica fora do MVP por decisão consciente.

<requirements>
- RF-19: estrutura do dicionário (term, signal_type, confidence, is_ambiguous)
- RF-21: confidence=high apenas para inequívocos
- RF-22: merchants/segments sempre `is_ambiguous=true`
- RF-23: mesmo termo em múltiplas subcategorias → todas ambíguas
- RF-28: proibir stopwords, meios de pagamento, valores, datas
- RF-29: canonical_name obrigatório por subcategoria
- RF-32: cobrir nomes canônicos de todas as subcategorias
- RF-33: aliases e frases mínimas
- RF-34: termos que devem ser ambíguos ou não existir como alias
- RF-35: ignorado para o MVP (arquivo inexistente)
- RF-36: seed append-only
- RF-36a: rollback por depreciação + novo ID
- RF-38: alteração por migration versionada em PR
- RF-40: validar unicidade editorial
</requirements>

## Subtarefas

- [ ] 3.1 Migration de seed de `canonical_name` para todas as subcategorias (confidence=high, is_ambiguous=false)
- [ ] 3.2 Migration de seed de aliases e frases inequívocos do RF-33 (confidence=high, is_ambiguous=false)
- [ ] 3.3 Migration de seed de termos ambíguos do RF-34 (confidence=medium/low, is_ambiguous=true)
- [ ] 3.4 Validar unicidade do índice parcial `(kind, category_id, term_normalized)`
- [ ] 3.5 Teste de integração: valida que todos os termos ambíguos estão marcados corretamente

## Detalhes de Implementação

Ver PRD seções **Conteúdo Mínimo do Dicionário** e RF-33/RF-34.

Pontos críticos:
- Cada entrada deve apontar para `category_id` correto (UUIDv5 da subcategoria).
- `term` deve ser inserido em sua forma natural (com acentos); `term_normalized` é gerada automaticamente.
- `signal_type`: `canonical_name` para nome da subcategoria; `alias` para sinônimos; `phrase` para frases; `merchant` para estabelecimentos.
- Termos de RF-34 que NÃO devem existir como alias inequívoco: `compra`, `pix`, `boleto`, `cartão`, `parcela`, `transferência`, `débito`, `mercado`, `farmácia`, `remédio`, `Uber`, `99`, `Amazon`, `celular`, `telefone`, `café`, `pão`, `posto`, `hotel`, `evento`, `ingresso`, `viagem`, `curso`, `presente`, `investimento`.
- Se algum termo de RF-34 existir no dicionário, DEVE ter `is_ambiguous=true`.

## Critérios de Sucesso

- [ ] Todos os canônicos estão presentes (uma entrada por subcategoria)
- [ ] Todos os aliases/frases do RF-33 estão presentes
- [ ] Nenhum termo de RF-34 existe como `is_ambiguous=false`
- [ ] Índice único parcial não é violado
- [ ] Teste de integração valida normalização `unaccent` (ex: "Água" e "agua" normalizam para o mesmo valor)

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste de integração: conta canônicos e valida 1 por subcategoria
- [ ] Teste de integração: valida que termos ambíguos do RF-34 têm `is_ambiguous=true`
- [ ] Teste de integração: valida unicidade de `term_normalized` por `(kind, category_id)`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/0000XX_seed_dictionary.up.sql`
- `migrations/0000XX_seed_dictionary.down.sql`
- Testes de integração em `internal/categories/infrastructure/repositories/postgres/`
