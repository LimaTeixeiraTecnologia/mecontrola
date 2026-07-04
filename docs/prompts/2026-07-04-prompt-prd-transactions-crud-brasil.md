# 2026-07-04 — Prompt enriquecido para PRD de CRUD completo de transações no Brasil

## Prompt original

> Eu quero que o módulo internal/transactions funcione da seguinte forma eu quero um CRUD completo de transações, a forma de pagamento deve ser TODAS disponíveis no Brasil hoje, pode ter uma tabela igual internal/card/domain/valueobjects/bank_code.go, SOMENTE O CARTÃO DE CRÉDITO DEVE CONSULTAR SE TEM FATURA ABERTA E PERMITIR PARCELAMENTO, outro ponto extremamente relevante são as categorias, a categoryID principal deve ser select * from categories c;, custo fixo, conhecimento, prazeres, metas, liberdade-financeira, metas, ou seja quem não tem parent_id, e a subcategoria é obrigatória também para detalhar que paguei uma conta de água atrelado ao custo fixo seja extremamente criterioso e sem achar, sem desvios me de um diagnóstico completo e efetivo e de forma efetiva, robusto, econômico, eficiente, 0 gaps, 0 falso positivo, 0 ressalvas, 0 lacunas.
>
> O output desse prompt deve ser o input do @.agents/skills/create-prd/ sempre fazendo rodadas de perguntas até zerar todas questões e suposições em aberto SEMPRE COM RECOMENDAÇÃO e confrontando com o codebase.

## Ambiguidades e conflitos que o prompt enriquecido elimina

- `CRUD completo de transações` conflita com o estado atual do módulo, que já separa superfícies de `transactions`, `card-purchases`, `recurring templates`, `monthly views` e `card invoice by month`. O PRD precisa primeiro decidir se a experiência futura continua separando compra no cartão de transação genérica ou se tudo passa a ser unificado sob um único agregado funcional.
- `todas as formas de pagamento disponíveis no Brasil hoje` é amplo, mutável e pode gerar falso positivo se não houver recorte explícito. O prompt enriquecido obriga a transformar isso em inventário fechado, auditável e adequado ao contexto de finanças pessoais, com recomendação e validação contra o estado atual do codebase.
- `somente o cartão de crédito deve consultar se tem fatura aberta e permitir parcelamento` ainda deixa aberto o comportamento quando não houver fatura aberta, quando a compra cair perto do fechamento, se parcelamento é obrigatório/opcional e se cartões múltiplos podem coexistir.
- `categoryID principal` precisava ser convertido para a hierarquia real existente no banco: categorias raiz de despesa são as linhas sem `parent_id`, e a subcategoria precisa ser tratada como regra funcional explícita, não apenas exemplo textual.
- Há duplicidade de `metas` no texto original; o prompt enriquecido normaliza a lista sem inventar novas categorias.
- `0 gaps / 0 lacunas / 0 ressalvas` não pode ser promessa prévia. O prompt enriquecido converte isso em regra de evidência: o agente deve parar para perguntar sempre que ainda houver ambiguidade material.

## Prompt enriquecido — versão pronta para uso como input do `create-prd`

```text
Atue como um redator sênior de PRD, extremamente criterioso, orientado a evidências e incapaz de assumir contexto ausente.

Sua missão é produzir o PRD de uma funcionalidade para o repositório `mecontrola`, confrontando explicitamente o pedido do usuário com o codebase atual antes de fechar qualquer requisito. O objetivo NÃO é implementar nada. O objetivo é sair com um PRD completo, rastreável e sem suposições silenciosas.

Mandatos inegociáveis:
1. Não implemente nada.
2. Não proponha código, migrations, endpoints finais ou desenho técnico detalhado como resposta principal.
3. Não invente contexto de negócio, meios de pagamento, regras de parcelamento, tabelas, contratos ou fluxos ausentes.
4. Toda conclusão precisa nascer do confronto entre:
   - pedido do usuário
   - codebase atual
   - e, quando necessário para fatos externos atuais do Brasil, fontes oficiais ou amplamente reconhecidas
5. Se uma decisão material não puder ser provada nem confirmada, você deve perguntar.
6. Você deve conduzir rodadas sucessivas de esclarecimento até zerar ambiguidades materiais. Se o seu fluxo operacional limitar a redação final antes disso, interrompa com `needs_input`, liste objetivamente o que falta e continue as rodadas na próxima interação.
7. Toda pergunta deve vir com recomendação explícita.
8. Faça uma pergunta por turno quando houver decisão material.
9. Não aceite `0 gaps`, `0 lacunas`, `0 ressalvas` como premissa. Isso só pode ser conclusão se tudo estiver fechado e provado.
10. O working tree atual é a fonte da verdade.
11. O ponto de partida obrigatório para confronto do bootstrap real é `cmd/server/server.go` e `cmd/worker/worker.go`.
12. É proibido usar `internal/platform/runtime` como ponto central de análise.

Contexto obrigatório já conhecido do codebase atual, que deve ser confirmado e usado no diagnóstico:
1. O projeto é um monólito modular em Go.
2. O módulo `internal/transactions` já existe e já é montado em `cmd/server/server.go` e `cmd/worker/worker.go`.
3. O módulo atual já expõe superfícies para:
   - `transactions`
   - `card-purchases`
   - `recurring templates`
   - `monthly views`
   - `card invoice by month`
4. O estado atual de formas de pagamento em `internal/transactions/domain/valueobjects/payment_method.go` é:
   - `pix`
   - `ted`
   - `debit_in_account`
   - `debit_card`
   - `cash`
   - `boleto`
   - `credit_card`
   - `doc` apenas como legado read-only
5. O módulo `internal/card` já possui capacidades ligadas a fatura e melhor dia de compra.
6. O módulo `internal/categories` já existe e já oferece listagem, resolução por slug e validação de subcategoria.
7. As categorias raiz de despesa atualmente sem `parent_id` na migration inicial são:
   - `custo-fixo`
   - `conhecimento`
   - `prazeres`
   - `metas`
   - `liberdade-financeira`
8. Já existem subcategorias semeadas relevantes, incluindo por exemplo:
   - `custo-fixo/agua`
   - `custo-fixo/energia`
   - `custo-fixo/internet`
   - `conhecimento/cursos-e-treinamentos`
   - `prazeres/delivery`
   - `metas/viagem-planejada`
   - `liberdade-financeira/reserva-de-emergencia`
9. O OpenAPI atual de transactions já sinaliza `subcategory_id` como obrigatório quando a direção for despesa (`outcome`), então o PRD deve esclarecer se a regra desejada:
   - permanece somente para despesa
   - ou muda de escopo

Objetivo de produto a ser refinado:
Queremos definir o PRD para um CRUD completo de transações no `internal/transactions`, com foco em:
1. cadastro, edição, consulta, listagem e remoção de transações
2. meios de pagamento aderentes ao Brasil
3. regra especial de cartão de crédito:
   - somente cartão de crédito consulta fatura aberta
   - somente cartão de crédito pode parcelar
4. uso obrigatório de categoria raiz + subcategoria para detalhamento de despesa
5. aderência ao catálogo de categorias já existente no banco e no módulo `internal/categories`

Diagnóstico obrigatório antes de redigir o PRD:
1. Compare o pedido com o estado real do codebase.
2. Explique objetivamente o que já existe, o que está parcial, o que conflita e o que ainda é decisão de produto.
3. Destaque explicitamente se o escopo pedido:
   - reaproveita a separação atual entre `transaction` e `card purchase`
   - ou exige consolidação em uma experiência única
4. Destaque explicitamente se o pedido de `todas as formas de pagamento disponíveis no Brasil hoje` extrapola o escopo saudável de finanças pessoais e precisa de recorte funcional.

Perguntas obrigatórias que você deve zerar antes do PRD final, sempre com recomendação:
1. A experiência futura deve:
   - manter `compra no cartão` separada de `transação genérica` (Recomendado, porque o codebase já separa esses fluxos)
   - ou unificar tudo sob um único CRUD de transações com `payment_method=credit_card`
2. O inventário de meios de pagamento deve cobrir:
   - apenas meios relevantes para registro de finanças pessoais do usuário final (Recomendado)
   - ou um catálogo exaustivo de instrumentos financeiros existentes no Brasil, inclusive legados, corporativos ou de baixa aderência ao app
3. O sistema deve permitir criar novas transações com `doc`:
   - não, manter `doc` apenas legado read-only (Recomendado, porque esse já é o estado atual)
   - ou sim, reabrir `doc` para criação
4. Para cartão de crédito sem fatura aberta no mês de competência:
   - bloquear lançamento parcelado e orientar o usuário (Recomendado)
   - permitir lançamento mesmo sem fatura aberta
   - criar comportamento alternativo, que deve ser explicitado
5. Parcelamento no cartão de crédito deve exigir:
   - apenas número de parcelas
   - número de parcelas + valor por parcela
   - número de parcelas + regra de distribuição temporal + tratamento de arredondamento
   Recomende a opção mais segura para PRD orientado a negócio e rastreabilidade.
6. A subcategoria obrigatória deve valer:
   - apenas para despesas/outcome (Recomendado, alinhado ao OpenAPI atual e ao exemplo do usuário)
   - para qualquer transação, inclusive receita
7. A categoria raiz deve ser:
   - sempre derivada das categorias sem `parent_id`, com subcategoria filha obrigatória quando aplicável (Recomendado)
   - ou escolhida livremente sem relação hierárquica obrigatória
8. O CRUD deve contemplar somente transações avulsas:
   - não, deve também esclarecer a relação com recorrência e visão mensal já existentes no módulo
   - ou sim, deve deixar recorrência explicitamente fora do PRD desta feature
   Recomende a alternativa mais coerente com o estado atual do módulo.
9. O usuário quer `todas as formas de pagamento no Brasil hoje`; portanto, você deve perguntar e fechar:
   - a lista exata suportada no PRD
   - a lista explicitamente fora de escopo
   - o racional de cada exclusão
   Não avance enquanto isso não estiver fechado.

Lista mínima de arquivos que você deve confrontar antes de concluir:
- `AGENTS.md`
- `go.mod`
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- `internal/transactions/module.go`
- `internal/transactions/openapi.yaml`
- `internal/transactions/domain/valueobjects/payment_method.go`
- `internal/card/module.go`
- `internal/card/domain/valueobjects/bank_code.go`
- `internal/categories/module.go`
- `internal/categories/openapi.yaml`
- `migrations/000001_initial_schema.up.sql`

Regras de saída:
1. Primeiro, apresente um diagnóstico confrontado com o codebase em linguagem objetiva:
   - o que já existe
   - o que está aderente ao pedido
   - o que conflita
   - o que ainda depende de decisão de produto
2. Depois, faça as perguntas obrigatórias, uma por turno, sempre com recomendação.
3. Só redija o PRD final quando não restarem ambiguidades materiais.
4. Se ainda houver dúvida aberta, retorne `needs_input` e não feche o PRD.
5. Quando o PRD estiver pronto:
   - foque em problema, objetivo, atores, escopo incluído, escopo excluído, restrições, critérios de sucesso e requisitos funcionais numerados
   - diferencie claramente regra de transação genérica versus regra exclusiva de cartão de crédito
   - registre explicitamente a hierarquia `categoria raiz -> subcategoria`
   - registre explicitamente a lista final de meios de pagamento suportados e excluídos
   - registre explicitamente como a regra de fatura aberta impacta parcelamento
6. Não escreva desenho técnico detalhado no lugar de requisito de produto.

Critérios de aceitação do seu trabalho:
1. Nenhuma ambiguidade material fica sem pergunta.
2. Nenhuma pergunta vem sem recomendação.
3. O diagnóstico inicial referencia o codebase atual, não um sistema imaginado.
4. O PRD final deixa explícito:
   - escopo do CRUD
   - catálogo de meios de pagamento suportados
   - comportamento exclusivo de cartão de crédito
   - regra de categoria raiz e subcategoria
   - itens fora de escopo
   - métricas de sucesso
5. Se você não conseguir provar algum ponto pelo codebase ou pela resposta do usuário, trate isso como pendência explícita e não como fato.
```
