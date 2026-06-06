# Transcript do Brainstorming Decisório

## Contexto Inicial

Pedido original do usuário (2026-06-06):

> "Implementar o CRUD completo de categorias, ter no sistema todas as categorias e subcategorias de despesas pessoais, com seed das principais usadas no mercado (buscar referências na internet), com possibilidade de categorias por usuário, e um sistema robusto para subcategoria. Exemplos: categoria principal 'Custos Fixos' com subcategoria 'Aluguel/Financiamento'; categoria principal 'Conforto' com subcategoria 'Lavagem Automotiva'."

Esclarecimento adicional (mesmo turno): categorias devem cobrir tanto **despesas (saídas)** quanto **receitas (entradas)** — o domínio é "fluxo financeiro pessoal" e não apenas despesas.

Repositório de destino: `mecontrola` (Go, monolito modular, módulos `internal/identity` e `internal/billing` já consolidados em E1/E2). O novo módulo deve seguir o "Padrão Obrigatório de Módulo" descrito em `AGENTS.md`.

## Rodada 1 - Entendimento do Problema

### P1.1 - Qual o problema central que esse CRUD de categorias deve resolver primeiro?
- **Resposta: TODAS as opções acima** (escolha múltipla validada pelo usuário)
  - Padronizar taxonomia de despesas/receitas (out-of-the-box útil).
  - Permitir personalização por usuário (refletir orçamento próprio).
  - Habilitar relatórios e analytics (gasto por categoria, drill-down).
  - Pré-requisito para orçamento/budget (metas, alertas).
- Implicação: o CRUD não é "só CRUD" — é fundação multi-propósito. O design precisa servir as 4 dimensões simultaneamente; não cabe atalho que privilegie apenas uma.

### P1.2 - Qual resultado mínimo torna a entrega bem-sucedida?
- **Resposta: Tudo acima + auditoria e versionamento.**
  - CRUD por usuário, seed global (padrão de mercado), soft-delete com reatribuição, e auditoria de mudanças (quem criou/alterou/quando).
- Implicação: nível de robustez alto. Schema precisa de `created_by`, `updated_by`, `deleted_at`, e histórico de alterações (ou tabela de eventos/outbox). Reatribuição de despesas exige contrato com o módulo de lançamentos.

### P1.3 - Qual o risco mais relevante de adiar essa decisão?
- **Resposta: Bloqueio de features dependentes.**
- Implicação: categorias destravam orçamento, dashboards, metas. Prioridade alta no roadmap; deve ser entregue antes do módulo de lançamentos amadurecer.

### P1.4 - Qual a profundidade de hierarquia que o domínio exige?
- **Resposta: Exatamente 2 níveis (categoria > subcategoria).**
- Implicação: modelo simples, sem árvore arbitrária. Validação no domínio rejeita subcategoria de subcategoria. Schema permanece compreensível e relatórios têm drill-down previsível.

### Esclarecimento Adicional do Usuário
- Categorias cobrem **receitas (entradas) e despesas (saídas)** — domínio é "fluxo financeiro pessoal".
- Implicação: necessidade de um discriminador `kind` (ou `direction`) em categoria: `income` | `expense`. Subcategorias herdam o tipo da categoria pai. Seed precisa contemplar ambos lados.

## Rodada 2 - Escopo e Restrições

### P2.1 - Modelo de seed
- **Resposta: Seed global compartilhado (read-only).**
- Implicação: existe um conjunto fixo de categorias "sistema" (`is_system=true`, `user_id=NULL`) que todos enxergam mas não podem editar. Usuário cria suas próprias categorias se quiser customizar (mesmo nome é permitido — namespacing por `user_id`).
- Schema: PK em `categories` com `user_id NULLABLE`; quando `NULL`, é categoria de sistema. Constraint `UNIQUE (user_id, name, parent_id)`.

### P2.2 - Delete + lançamentos vinculados
- **Resposta: Soft-delete + manter vínculo (categoria "arqueológica").**
- Implicação: `deleted_at TIMESTAMP NULL`. Listagens default filtram `deleted_at IS NULL`. Relatórios históricos podem incluir categorias deletadas. Sem necessidade de reatribuição obrigatória — reduz complexidade da feature.
- Trade-off: nomes de categoria podem ser "renomeados" via delete+create no front-end, gerando categorias-fantasma; aceito como custo. Pode ser combinado com endpoint de "arquivar" (PATCH) para nomenclatura clara.

### P2.3 - Restrição dominante
- **Resposta: Consistência com módulos existentes.**
- Implicação: seguir exatamente o "Padrão Obrigatório de Módulo" de `AGENTS.md`: `application/usecases`, `domain/entities|valueobjects|repositories`, `infrastructure/{http,repositories,messaging}`, `module.go` com wiring, outbox.Publisher quando publicar eventos. Sem atalho de "controller fino + service gordo".
- Implicação: módulo deve expor eventos relevantes (CategoryCreated, CategoryUpdated, CategoryDeleted) via outbox para ser consumido por futuros módulos (transações, orçamento).

### P2.4 - Adapter + módulo
- **Resposta: REST em módulo dedicado `internal/categories`.**
- Implicação: novo bounded context. Rotas `/v1/categories` (com `subcategories` aninhadas ou flat com `parent_id`). Não acopla a `identity` nem cria `internal/finance` macro (que poderia virar god-context).
- Nome do módulo: `internal/categories`. Eventos no namespace `categories.v1.*`.

## Rodada 3 - Alternativas

Foram comparadas 4 alternativas para modelo de schema, mais opções para eventos, seed e cascata de delete.

### Alternativas de Schema/Agregado
- **A) Adjacency List (tabela única + `parent_id` self-ref) + validação no domínio.** ✅ Escolhida.
- B) Duas tabelas (`categories` + `subcategories`) com FK rígida — duplica CRUD, mais código.
- C) Closure Table (suporta N níveis, limita 2 na app) — overkill dado H2.
- D) Adjacency List + projetor de read-model `category_tree_view` — alinha 100% com billing, mas adiciona consumer e tabela de projeção sem ganho proporcional para profundidade=2 e cardinalidade baixa (~50 categorias por usuário no pior caso).

**Racional A:** simplicidade de schema (1 tabela), CRUD único (POST `/v1/categories` aceita `parent_id` opcional), validação `R5/R7` da skill `go-implementation` no agregado para impedir profundidade>2 e ciclos. Listagem em árvore é trivial (SELECT + agrupar em memória ou CTE recursivo limitado a 1 nível).

### P3.2 - Eventos de domínio
- **Resposta: Outbox completo (publisher + consumer).**
- Implicação: `outbox.Publisher` registrado no `module.go`; eventos `categories.v1.category_created`, `categories.v1.category_updated`, `categories.v1.category_deleted` publicados em mesma transação do write. Idempotência por `event_id` (regra obrigatória de outbox em `AGENTS.md`). Futuro consumer em `internal/transactions` ou `internal/budget` reage sem polling.

### P3.3 - Aplicação do seed
- **Resposta: Migration SQL idempotente (`INSERT ... ON CONFLICT DO NOTHING`).**
- Implicação: arquivo `migrations/00XX_seed_system_categories.up.sql` com lista versionada. Auditavel via git. Mesmo padrão das migrations de billing/identity. Constraint UNIQUE para idempotência.

### P3.4 - Cascata em soft-delete
- **Resposta: Bloquear delete se houver filhos ativos.**
- Implicação: tentativa de deletar `Custos Fixos` com `Aluguel` ativa retorna `409 Conflict` listando filhos. Usuário precisa deletar filhos primeiro. Mais explícito, sem mágica de cascata. UX paga 1 round-trip a mais, mas previne deletes acidentais em árvore.
- Combinado com H8 (audit log): cada delete gera evento auditado.

### Pesquisa de Mercado para Seed (referências consultadas)
Categorias recorrentes nos apps brasileiros (Mobills, Organizze, Minhas Economias, Meu Dinheiro Plus, PayPal BR, BB) e literatura de método 50/30/20 — consolidação para o seed:

**DESPESAS — categorias principais sugeridas:**
- Moradia: Aluguel, Financiamento Imobiliário, Condomínio, IPTU, Energia, Água, Gás, Internet, Telefone Fixo, Manutenção da Casa, Móveis e Decoração.
- Alimentação: Supermercado, Feira, Padaria, Açougue, Restaurantes, Delivery, Lanches.
- Transporte: Combustível, Transporte Público, Aplicativos (Uber/99), Estacionamento, Pedágio, Manutenção Veicular, Lavagem Automotiva, IPVA, Licenciamento, Seguro Veicular, Multas.
- Saúde: Plano de Saúde, Consultas Médicas, Exames, Medicamentos, Odontologia, Terapia, Academia, Suplementos.
- Educação: Mensalidade Escolar, Faculdade, Cursos, Livros, Material Escolar, Idiomas.
- Lazer: Cinema, Streaming (Netflix/Spotify/Disney+), Viagens, Hobbies, Bares, Eventos, Jogos.
- Vestuário: Roupas, Calçados, Acessórios.
- Cuidados Pessoais: Cabelereiro, Estética, Cosméticos, Perfumaria.
- Pets: Ração, Veterinário, Banho e Tosa, Acessórios.
- Família: Filhos, Mesada, Presentes Familiares.
- Tarifas e Impostos: Tarifas Bancárias, Anuidade Cartão, Imposto de Renda, Outros Impostos.
- Dívidas: Empréstimo Pessoal, Cartão de Crédito (juros), Cheque Especial, Financiamentos Diversos.
- Investimentos e Poupança: Reserva de Emergência, Investimentos, Previdência.
- Doações: Caridade, Igreja, ONGs.
- Outros: Diversos, Sem Categoria (sistema).

**RECEITAS — categorias principais sugeridas:**
- Salário: Salário, 13º, Férias, PLR/Bônus, Vale-Alimentação, Vale-Refeição.
- Renda Variável: Freelance, Trabalho Extra, Consultoria.
- Investimentos: Rendimentos, Dividendos, Juros, Resgates.
- Aluguel Recebido.
- Restituição/Estornos: Restituição de IR, Estorno, Cashback, Reembolsos.
- Presentes/Mesada Recebida.
- Vendas: Vendas Diversas, Marketplace.
- Outros: Outras Receitas.

> O seed concreto e fechado será materializado no PRD seguinte; aqui registra-se a fonte e o esqueleto.

**Fontes consultadas:**
- https://www.mobills.com.br/blog/financas-pessoais/controle-de-gastos/
- https://blog.bb.com.br/controle-de-gastos-por-categoria-descubra-para-onde-vai-seu-dinheiro/
- https://www.embracon.com.br/blog/quais-sao-os-tipos-de-despesas-pessoais
- https://www.paypal.com/br/webapps/mpp/spendsmarter/financial-education/types-of-household-expenses
- https://www.infomoney.com.br/minhas-financas/regra-50-30-20-conheca-um-metodo-para-organizar-suas-financas/
- https://sites.google.com/site/meudinheiroplus/definicoes-de-categorias
- https://mobills.zendesk.com/hc/en-us/articles/360052868553-What-are-categories
- https://www.emcash.com.br/financas-pessoais/como-criar-categorias-de-gastos/

## Rodada 4 - Trade-offs

### P4.1 - Customização do seed (renomear)
- **Resposta: Endpoint dedicado de clone (POST /v1/categories/{id}/clone).**
- Implicação: usuário pode "clonar" qualquer categoria de sistema para o próprio namespace (`user_id`) e editar a cópia. Listagem precisa de regra de "ocultar sistema quando há clone correspondente" (via flag `cloned_from_category_id`). Mais código, mas evita duplicação visual ao mesmo tempo que mantém seed imutável.

### P4.2 - kind (receita/despesa) + hierarquia
- **Resposta: `kind` na raiz; subcategoria herda do pai (constraint).**
- Implicação: schema com `kind` NOT NULL, mas trigger ou validação de domínio garante `subcategoria.kind = pai.kind`. Migrations podem criar `CHECK (kind IN ('income','expense'))` + função de validação para `parent_id`. Domínio rejeita tentativa de criar subcategoria de tipo divergente.

### P4.3 - Visual (cor/ícone) no MVP
- **Resposta do usuário: "uso exclusivo no WhatsApp".**
- Interpretação: o canal de consumo principal é **WhatsApp** (bot). Não há front-end gráfico próprio que justifique `color`/`icon` no MVP.
- Implicação derivada:
  - Cor/ícone ficam fora do schema do MVP.
  - Listagem em WhatsApp é texto numerado/listado; precisa de nomes curtos e claros.
  - Pode ser necessário endpoint de busca fuzzy (`GET /v1/categories?q=alug`) para o bot resolver intenção do usuário.
  - Audit log precisa registrar canal de origem (`source=whatsapp|api|seed`) para suportar análise de adoção.
  - Identificador do usuário no audit log: `user_id` (id interno de `internal/identity`), independentemente do canal.

### P4.4 - MVP vs robusto
- **Resposta: Entregar tudo na primeira onda (CRUD + outbox + audit log + seed + clone).**
- Implicação: escopo único, mais tempo de implementação (~2 a 3 semanas estimado), porém entrega contrato estável para consumidores futuros (transactions, budget, e o próprio bot WhatsApp).
- Risco aceito: prazo maior. Mitigação: divisão interna em "waves" (igual padrão usado em billing E2): wave-1 domínio+repo, wave-2 use cases, wave-3 HTTP+outbox, wave-4 seed+audit+clone.

## Rodada 5 - Seleção de Direção

### Síntese apresentada ao usuário
Novo módulo `internal/categories` com:
- Adjacency List (tabela única `categories` com `parent_id` self-ref).
- `kind` (`income` | `expense`) na raiz; subcategorias herdam por constraint.
- Profundidade máxima 2 validada no domínio (R5 da skill `go-implementation`).
- Soft-delete bloqueado quando há filhos ativos (409 Conflict).
- Seed via migration SQL idempotente (`ON CONFLICT DO NOTHING`).
- Customização do seed via endpoint dedicado `POST /v1/categories/{id}/clone`.
- Outbox completo (publisher + idempotência por `event_id`).
- Audit log dedicado registrando `who`, `when`, `what` e `source` (`whatsapp`, `api`, `seed`).
- Canal MVP: WhatsApp (sem cor/ícone; busca fuzzy considerada).
- Wave única com sub-waves internas (domínio → use cases → HTTP+outbox → seed+audit+clone).

### P5.1 - Confirmação da direção
- **Resposta: Confirmo a direção recomendada.**

### P5.2 - Próximo passo
- **Resposta: `create-prd` (gerar PRD numerado com requisitos funcionais e seed fechado).**

## Decisões Registradas
| ID | Decisão | Justificativa |
| --- | --- | --- |
| D1 | Criar módulo `internal/categories` seguindo o "Padrão Obrigatório de Módulo" (AGENTS.md). | Restrição dominante = consistência com módulos existentes (P2.3). |
| D2 | Schema Adjacency List: tabela única `categories` com `parent_id` self-ref. | Maior pontuação no scorecard (40); profundidade=2 garantida via domínio. |
| D3 | Profundidade hierárquica fixa em 2. | Resposta P1.4; valida no agregado, não permite "subcategoria de subcategoria". |
| D4 | `kind` (income/expense) na raiz; subcategoria herda do pai (constraint). | Resposta P4.2; modelagem inequívoca. |
| D5 | Soft-delete com `deleted_at`; bloqueia delete se houver filhos ativos. | Respostas P2.2 + P3.4; previne deletes acidentais em árvore. |
| D6 | Seed via migration SQL idempotente (`INSERT ... ON CONFLICT DO NOTHING`). | Resposta P3.3; auditável via git, mesmo padrão de billing/identity. |
| D7 | Customização de seed via endpoint `POST /v1/categories/{id}/clone`. | Resposta P4.1; evita duplicação visual sem mutar dados de sistema. |
| D8 | Outbox completo (publisher + consumer-ready) para eventos `categories.v1.*`. | Resposta P3.2; contrato estável para futuras integrações; idempotência por `event_id`. |
| D9 | Audit log dedicado entregue na primeira onda. | Resposta P4.4 + H3. |
| D10 | Sem cor/ícone no MVP. | Resposta P4.3: canal exclusivo WhatsApp não usa visual gráfico. |
| D11 | Endpoint de busca fuzzy (`GET /v1/categories?q=<termo>`) considerado essencial para UX WhatsApp. | Decorrência da decisão D10 + canal WhatsApp. |
| D12 | Próxima skill: `create-prd`. | Material suficiente; não há incerteza arquitetural pendente que justifique discovery técnico adicional. |
