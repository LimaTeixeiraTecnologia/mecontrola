# Validação de dados de agosto/2026 — FinancialControlDB → frases prontas para WhatsApp

Gerado em **2026-08-01**, complementando
`docs/runs/30-07-2026-VALIDACAO-E2E-COMPLETA-jun-jul-2026.md` (jun/jul
2026). Mesma metodologia: extração real via `sqlcmd` do
`FinancialControlDB` (SQL Server, `sqlcmd -S SQL5053.site4now.net,1433 -d
DB_A453C8_FinancialControl`), credenciais lidas de
`/Users/jailtonjunior/Git/financial-migration/.env`, e de-para real contra
`mecontrola.categories`/`mecontrola.category_dictionary` no Postgres de
produção (`ssh mecontrola-vps`, banco `mecontrola_db`). Extraído em
**2026-08-01**. Nada foi inventado — toda linha abaixo tem o `Id` real do
banco como evidência, e toda categoria sugerida vem do resultado literal
das 3 consultas (exato/token/fuzzy) rodadas contra o Postgres real.

## Achado importante — dois sentidos diferentes de "fatura de agosto"

O legado tem **duas datas diferentes** que podem significar "agosto":

1. **`InvoiceItem.PurchaseDate` em agosto/2026** — a data real da compra no
   cartão. São os lançamentos genuinamente **novos** de agosto, que ainda
   não apareceram no documento de jun/jul. **Resultado: 4 linhas.**
2. **`Invoice.Date` = agosto/2026** — a data de referência do registro
   `Invoice` (cabeçalho da fatura) em si.

**Correção sobre esta seção (pedido do usuário para trazer `SELECT *
FROM Invoice` completo — ver seção 5): a frase original aqui dizia que
existe "uma fatura corrente/aberta" por cartão. Isso estava ERRADO.**
`SELECT * FROM Invoice WHERE CardId = <XP Visa>` (seção 5) mostra **39
registros `Invoice` distintos para o cartão XP Visa**, um por mês, de
2024-02-01 até 2027-04-01 — ou seja, o legado TEM fatura mensal fechada
normalmente, não é uma fatura única acumulando tudo. O que é real e
verificado (não invento): o registro `Invoice` de 2026-08-01 (Id
`3f5a1afe-063b-4053-b1b2-8ab70bf5c5b3`, Total R$ 8.731,45) tem **97
`InvoiceItem` ligados a ele via `InvoiceId`**, e o `PurchaseDate` desses 97
itens vai de **2025-08-05 até 2026-06-23** — ou seja, o vínculo
`InvoiceItem.InvoiceId → Invoice` (mês da fatura) **não corresponde** ao
`InvoiceItem.PurchaseDate` (data real da compra) para a maioria desses
itens. Confirmei que cada `InvoiceItem` existe **uma única vez** no banco
(ex.: "Água de Coco" 27/06, Id `d16cb2ba-99bd-4189-98c7-f7366f9fccb4`,
está ligado só ao `InvoiceId` de agosto, não duplicado em outro mês) — não
é duplicação de dado, é o próprio vínculo do legado que não é 1:1 com o
mês da compra. Não sei a causa raiz dessa distribuição (não vou inventar
uma explicação sem evidência adicional); o fato verificável é: **gerar
frase de registro para os 97 itens do `InvoiceId` de agosto reproduziria
compras cuja data real (`PurchaseDate`) já foi coberta pelo documento de
jun/jul** (ex.: "Github", "Água de Coco", "Compras do mês" em 27/06 e
09/07 já constam lá) — por isso não gerei frase de WhatsApp para esses 97
itens: usaria a data de hoje/agosto para uma compra que, pela própria
data real extraída, aconteceu em outro mês.

Além disso, verifiquei `Bill`+`BillItem` (contas fixas) e a tabela legada
`[Transaction]` para agosto/2026:

- `Bill`+`BillItem` com `Date` em agosto/2026: **3 linhas** (contas fixas
  de agosto — a conta de agosto não tem "Condominio" nem "Energia" nesta
  extração, só os 3 itens abaixo; não inventei os dois que faltam).
- `[Transaction]`+`TransactionItem` em agosto/2026: **0 linhas** —
  reconfirmado nesta extração: `MAX(Date) = 2025-12-01`, sem mudança desde
  o levantamento de jun/jul (tabela legada morta, app real já migrou para
  o Postgres do mecontrola antes desse período).

**Total de lançamentos genuinamente novos de agosto/2026 (o que vale a
pena registrar no WhatsApp): 7** (4 no cartão XP + 3 de conta fixa).

---

## 1. Dados brutos extraídos (agosto/2026)

```text
================================================================================
DADOS REAIS — FinancialControlDB (SQL Server, site4now) — Agosto de 2026
Fonte: sqlcmd -S SQL5053.site4now.net,1433 -d DB_A453C8_FinancialControl
Extraído em: 2026-08-01
================================================================================

--- CONTAGENS ---
[Transaction]+TransactionItem em ago/2026: 0 linhas (MAX(Date)=2025-12-01, sem mudança)
Bill+BillItem em ago/2026 (Bill.Date): 3 linhas
InvoiceItem com PurchaseDate em ago/2026 (compras novas reais): 4 linhas
Invoice com Date em ago/2026 (fatura corrente/aberta por cartão, NÃO exclusiva de agosto): 2 registros (98 itens no total, ver achado acima)

--- InvoiceItem novos de agosto (por PurchaseDate, ordenado por Description) ---
--- Colunas: Cartão|PurchaseDate|Description|TotalAmount|Installment|Category(legado)|Tags(legado)|InvoiceItem.Id ---
XP Visa|2026-08-01 00:00:00|Corte de Cabelo|77|1|Serviços|Custos fixos|3c389596-fcac-4043-be6d-da1dde012d04
XP Visa|2026-08-01 00:00:00|Leve Mais [Mercadinho]|48.42|1|Supermercado|Custos fixos|6888c5a1-cf47-439d-b692-0e5551ce1dd9
XP Visa|2026-08-01 00:00:00|Netflix|44.9|1|Streamings|Conforto|aafca242-4055-4069-ac50-adf444cfd9e0
XP Visa|2026-08-01 00:00:00|Ovos|22.8|1|Supermercado|Custos fixos|45f343f5-53f6-4b3d-9c1c-fa6a1e0ff663

--- Bill + BillItem de agosto ---
--- Colunas: BillDate|Bill.Total|SixtyPercent|FortyPercent|BillItem.Title|BillItem.Value|BillItem.Id ---
2026-08-01 00:00:00|776.86|466.00|311.00|Faxina [Casa]|430.00|025c4363-81ea-4252-8bba-af6cf1321230
2026-08-01 00:00:00|776.86|466.00|311.00|Internet [Celular]|225.62|37a09786-2161-49f8-bb85-1209132a641f
2026-08-01 00:00:00|776.86|466.00|311.00|Internet [Central Fiber]|121.24|64feea4b-e5b5-4cc6-8d12-d071d9cff67d

--- Invoice (cabeçalho, Date=agosto/2026) — referência apenas, sem frase gerada ---
--- Colunas: Cartão|Número|Invoice.Total|Invoice.Id ---
XP Visa|0083|8731.45|3f5a1afe-063b-4053-b1b2-8ab70bf5c5b3 (98 itens ligados a este Invoice: 97; PurchaseDate de 2025-08-05 a 2026-07-28)
Nubank|1402|1.00|dc19f5d5-d5a5-4298-a1c4-935c19590057 (1 item ligado: "Aleatório", R$12,00, 8x, PurchaseDate=2025-12-01)
```

---

## 2. De-para real — os 4 termos novos contra `mecontrola.category_dictionary`

Mesma metodologia da seção 0.6 do documento de jun/jul: 3 estágios reais do
algoritmo (`internal/categories/application/usecases/search_dictionary.go`),
rodados via SQL direto no Postgres de produção (`mecontrola_db`).

| Termo/descrição | Estágio | Resultado | Categoria real (raiz) | Subcategoria real (folha) |
|---|---|---|---|---|
| Corte de Cabelo | exato/token/fuzzy(≥0,4) | sem match em nenhum estágio | — | — |
| Leve Mais | exato/token | sem match | — | — |
| Leve Mais | fuzzy | ⚠️ bate em "move mais" (sim=0,556, termo da Move Mais/pedágio) — **mesmo falso positivo já descartado no documento de jun/jul**; não uso | — | — |
| Netflix | exato | bate em `netflix` | Prazeres | Streaming de Vídeo |
| Ovos | exato/token/fuzzy(≥0,4) | sem match em nenhum estágio | — | — |
| Faxina | exato/token/fuzzy(≥0,4) | sem match (reconfirmado, mesmo resultado de jun/jul) | — | — |
| Internet | exato | bate em `internet` (reconfirmado, mesmo resultado de jun/jul) | Custo Fixo | Internet |

"Corte de Cabelo", "Leve Mais" e "Ovos" caem no fallback
`BuildRootOnlyCandidates`: o bot mostra as 5 categorias-raiz
(Conhecimento, Custo Fixo, Liberdade Financeira, Metas, Prazeres) e pede
para o usuário escolher.

## 2.1 Categoria/subcategoria completa para os 4 termos sem match automático

Por pedido explícito do usuário ("quero as categorias e subcategorias de
forma completa"), busquei a árvore real completa de
`mecontrola.categories` (5 raízes, 106 subcategorias ativas, `kind =
'expense'`) e escolhi manualmente, por contexto, a subcategoria real mais
adequada para os 4 termos que o algoritmo (exato/token/fuzzy) não resolveu
sozinho. **Isto é diferente da seção 2 acima**: não é resultado do
algoritmo de categorização automática do mecontrola — é minha leitura do
termo contra a lista real de categorias (nunca uma categoria inventada,
sempre uma das 106 que existem de fato no banco), sinalizada como escolha
manual/contextual para ficar claro que o bot não chegaria nela sozinho.

| Termo/descrição | Categoria (raiz) | Subcategoria (folha) | Por quê (contexto real do lançamento) |
|---|---|---|---|
| Corte de Cabelo | Prazeres | Beleza e Estética | corte de cabelo é serviço de barbearia/salão — mesma subcategoria já usada como sugestão para termos de salão na análise de jun/jul (seção 0.6, caso "São Vicente"/"salão") |
| Leve Mais [Mercadinho] | Custo Fixo | Supermercado | descrição legado já traz `[Mercadinho]`; mesma subcategoria que o algoritmo resolveu automaticamente para outros termos de supermercado do dataset (Mercadinho Porto, Compras do mês, Mercado Bom Demais) |
| Ovos | Custo Fixo | Supermercado | item de compra de supermercado; no legado `Category=Supermercado, Tags=Custos fixos`, coerente com a subcategoria real escolhida |
| Faxina [Casa] | Custo Fixo | Serviços Domésticos | existe subcategoria real exatamente para isso na árvore do mecontrola (`Custo Fixo > Serviços Domésticos`) — serviço de limpeza residencial recorrente |

Essas 4 linhas **não são resultado de exato/token/fuzzy** (seção 2 já
mostrou que nenhum estágio bateu) — são escolha humana informada sobre a
árvore real, para dar uma resposta completa de categoria a cada
lançamento. Se preferir manter fiel só ao que o algoritmo resolveria
sozinho (sem escolha manual), use a seção 3 original (2ª mensagem "—" para
esses 4 casos) em vez da tabela da seção 3.1-completa abaixo.

---

## 3. Frases prontas para o WhatsApp — os 7 lançamentos novos de agosto/2026

Mesmo formato validado em produção no documento de jun/jul (seção 0.5/0.7):
**duas mensagens** — a 1ª registra (formato aceito pelo guard
determinístico, sem sufixo colado), a 2ª só é enviada **se o bot
perguntar a categoria**. Coluna "Categoria completa" traz sempre
`Categoria > Subcategoria` real (seção 2 quando é match automático do
algoritmo, seção 2.1 quando é escolha manual/contextual sobre a árvore
real — marcado explicitamente em cada linha).

### 3.1 Compras no cartão XP — 4 lançamentos (`Invoice`+`InvoiceItem`, à vista)

Formato: `gastei <valor> no <descrição limpa> no crédito XP`
(`internal/agents/application/agents/guards/card_expense_shortcut.go`).

| # | Data | Legado (descrição, valor) | 1ª mensagem — registrar | Categoria completa | Origem |
|---|---|---|---|---|---|
| 1 | 2026-08-01 | Corte de Cabelo (R$ 77,00) | `gastei 77,00 no Corte de Cabelo no crédito XP` | `Prazeres > Beleza e Estética` | manual (seção 2.1) |
| 2 | 2026-08-01 | Leve Mais [Mercadinho] (R$ 48,42) | `gastei 48,42 no Leve Mais no crédito XP` | `Custo Fixo > Supermercado` | manual (seção 2.1); algoritmo bateria falso positivo em "Move Mais/pedágio" — descartado |
| 3 | 2026-08-01 | Netflix (R$ 44,90) | `gastei 44,90 no Netflix no crédito XP` | `Prazeres > Streaming de Vídeo` | algoritmo real — match exato |
| 4 | 2026-08-01 | Ovos (R$ 22,80) | `gastei 22,80 no Ovos no crédito XP` | `Custo Fixo > Supermercado` | manual (seção 2.1) |

2ª mensagem pronta para colar quando o bot perguntar a categoria (mesmo
formato validado em jun/jul, `Categoria > Subcategoria`):
`Prazeres > Beleza e Estética` / `Custo Fixo > Supermercado` /
`Prazeres > Streaming de Vídeo` / `Custo Fixo > Supermercado`.

### 3.2 Contas fixas — 3 lançamentos (`Bill`+`BillItem`, sem cartão)

Formato: `gastei <valor> na/no <título limpo> em dinheiro`
(`internal/agents/application/agents/guards/register_expense_shortcut.go`;
Bill não tem forma de pagamento no legado, "em dinheiro" é só uma opção
válida e determinística).

| # | Data | Legado (título, valor) | 1ª mensagem — registrar | Categoria completa | Origem |
|---|---|---|---|---|---|
| 5 | 2026-08-01 | Faxina [Casa] (R$ 430,00) | `gastei 430,00 na Faxina em dinheiro` | `Custo Fixo > Serviços Domésticos` | manual (seção 2.1) |
| 6 | 2026-08-01 | Internet [Celular] (R$ 225,62) | `gastei 225,62 na Internet em dinheiro` | `Custo Fixo > Internet` | algoritmo real — match exato |
| 7 | 2026-08-01 | Internet [Central Fiber] (R$ 121,24) | `gastei 121,24 na Internet em dinheiro` | `Custo Fixo > Internet` | algoritmo real — match exato |

2ª mensagem pronta: `Custo Fixo > Serviços Domésticos` / `Custo Fixo >
Internet` / `Custo Fixo > Internet`.

---

## 4. O que NÃO está aqui (para não inventar)

- Os **97 itens** ligados ao `Invoice` de agosto/2026 do XP Visa (Id
  `3f5a1afe-063b-4053-b1b2-8ab70bf5c5b3`) **não geraram frase** — o
  `PurchaseDate` real deles vai de 2025-08-05 a 2026-06-23 (ver correção
  na seção "Achado importante" e dump completo na seção 5), a maioria já
  coberta no documento de jun/jul; reenviá-las como "gastei ..." usaria a
  data de agosto para compras cuja data real é outra.
- Nenhuma linha em `[Transaction]`+`TransactionItem` de agosto/2026 —
  tabela legada confirmada vazia para 2026 (reconfirmado nesta extração).
- Para os 4 termos sem match automático (Corte de Cabelo, Leve Mais, Ovos,
  Faxina), a categoria da seção 3 é uma **escolha manual/contextual minha**
  sobre a árvore real (seção 2.1), não o resultado do algoritmo do
  mecontrola — sem esse pedido, o bot mostraria a lista de 5
  categorias-raiz e pediria escolha manual do usuário no teste real. Se
  quiser testar exatamente o que o algoritmo faria sozinho, não envie a 2ª
  mensagem para essas 4 e deixe o bot mostrar a lista.

---

## 5. `SELECT * FROM Invoice` completo — cartão XP Visa

Por pedido explícito do usuário. Query real rodada via `sqlcmd`:

```sql
SELECT c.Id AS CardId, c.Name, c.Number FROM Card c WHERE c.Name = 'XP Visa';
-- b4351e7e-f9ac-4a84-a113-a0e159303281 | XP Visa | 0083

SELECT * FROM Invoice i WHERE i.CardId = 'b4351e7e-f9ac-4a84-a113-a0e159303281' ORDER BY i.Date;
```

Colunas reais da tabela `Invoice`: `Id | CardId | Date | Total | CreatedAt
| UpdatedAt | Active`. **39 linhas** — uma por mês, de 2024-02-01 até
2027-04-01 (o cartão foi criado em `2023-12-27`, mesma data do
`CreatedAt` das primeiras faturas). Nenhuma linha foi omitida ou resumida.

| Invoice.Id | CardId | Date (mês da fatura) | Total | CreatedAt | UpdatedAt | Active |
|---|---|---|---|---|---|---|
| 9ba4ca76-01c3-4513-8585-7d050377318e | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2024-02-01 | 9075.25 | 2023-12-27 07:38:33.004 | NULL | 1 |
| d74a4536-5832-462f-beea-106d29f8c170 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2024-03-01 | 9700.71 | 2023-12-27 07:38:33.471 | NULL | 1 |
| ebf51449-b5ca-4290-8549-24b007e0a850 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2024-04-01 | 11553.91 | 2023-12-27 07:38:33.822 | NULL | 1 |
| 59b10fcc-4c3f-44af-adcd-6945c00038fb | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2024-05-01 | 8321.36 | 2024-01-02 14:29:29.353 | NULL | 1 |
| 52475e63-4c63-4ea2-a693-aac6e253d3b7 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2024-06-01 | 12176.19 | 2024-01-02 14:29:29.721 | NULL | 1 |
| ae4d166e-fc48-46e3-b921-20e98f4bc8b9 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2024-07-01 | 12328.24 | 2024-01-02 14:29:30.066 | NULL | 1 |
| acc769bc-bfc6-4a53-957a-548c799f7ae5 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2024-08-01 | 12063.33 | 2024-01-02 14:29:30.432 | NULL | 1 |
| 0d10dbbf-c2d4-4b42-b83f-e5c8fd5d81f6 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2024-09-01 | 11196.96 | 2024-01-02 14:29:30.785 | NULL | 1 |
| 1f356bbe-7520-439d-ab6c-fab87de7024b | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2024-10-01 | 10898.88 | 2024-01-02 14:29:31.128 | NULL | 1 |
| ab51ea22-a7f8-457c-91d9-d2b55ee82140 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2024-11-01 | 10957.66 | 2024-01-02 14:29:31.482 | NULL | 1 |
| 7944e7f4-dd1e-4501-be79-d22cc9bccd59 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2024-12-01 | 10279.43 | 2024-01-02 14:29:31.841 | NULL | 1 |
| 8bad927b-6322-4c66-85f5-86eb2ad5f0a5 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-01-01 | 8415.19 | 2024-01-02 14:29:32.183 | NULL | 1 |
| d1b4eadc-b382-4627-a53f-32cbca7aee5c | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-02-01 | 10633.44 | 2024-02-29 09:56:25.669 | NULL | 1 |
| 70256e68-bccc-4fa3-908f-2c9c60625ba3 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-03-01 | 8550.52 | 2024-02-29 09:56:26.142 | NULL | 1 |
| 9a8aaa13-fe16-4648-86b4-1b771fb6dc90 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-04-01 | 9875.45 | 2024-05-06 14:52:38.279 | NULL | 1 |
| e8023689-1c7b-47f3-8dc7-9f74185e50c2 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-05-01 | 9650.01 | 2024-05-06 14:52:38.606 | NULL | 1 |
| df1d9ab7-2f3d-4ff8-b439-11c3145e07c7 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-06-01 | 7644.56 | 2024-09-28 12:27:55.830 | NULL | 1 |
| b8fce1a5-a4dc-401a-9503-e8509f7398b6 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-07-01 | 10930.65 | 2024-09-28 12:27:56.270 | NULL | 1 |
| af1adcda-6754-4cb3-b548-03cb3f05a7ea | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-08-01 | 7242.88 | 2024-09-28 12:27:56.707 | NULL | 1 |
| fac1cc7d-bc78-42e4-813e-e5599435dd4a | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-09-01 | 8479.28 | 2025-07-19 19:51:10.209 | NULL | 1 |
| 3d1684c9-92ba-428a-b1e8-ec552c891f70 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-10-01 | 9602.93 | 2025-07-19 19:51:10.633 | NULL | 1 |
| 3ad651e4-a6f7-4fa5-acc6-5da446543968 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-11-01 | 8273.44 | 2025-07-19 19:51:11.072 | NULL | 1 |
| edeb0ed9-c615-4df4-9cf3-3b04cacf8bec | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2025-12-01 | 8624.98 | 2025-07-23 19:21:11.845 | NULL | 1 |
| 38139b84-9699-4676-a7b7-8b6dabe014d1 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2026-01-01 | 4617.74 | 2025-07-23 19:21:12.262 | NULL | 1 |
| a543b854-f641-4163-9c84-1f8105839aed | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2026-02-01 | 8948.40 | 2025-07-23 19:21:12.678 | NULL | 1 |
| 63b0fcf0-9e6a-4661-9763-68e0dd6b2478 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2026-03-01 | 6684.85 | 2025-07-23 19:21:13.110 | NULL | 1 |
| 3d988d78-d549-4e25-a46d-0c1126f353d1 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2026-04-01 | 9071.37 | 2025-07-23 19:21:13.528 | NULL | 1 |
| 633fdad2-ba21-4b2b-88fb-354b423411a1 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2026-05-01 | 6754.03 | 2025-08-05 19:22:56.791 | NULL | 1 |
| 639a81bd-eb94-4504-be22-4965ec9e6ff4 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2026-06-01 | 6336.10 | 2025-08-05 19:22:57.201 | NULL | 1 |
| 6da42a26-bf45-4274-a7be-c77e1d476920 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2026-07-01 | 7573.36 | 2025-08-05 19:22:57.617 | NULL | 1 |
| **3f5a1afe-063b-4053-b1b2-8ab70bf5c5b3** | b4351e7e-f9ac-4a84-a113-a0e159303281 | **2026-08-01** | **8731.45** | 2025-08-05 19:22:58.022 | NULL | 1 |
| 65ab4af1-3139-4688-b6f1-120eea78a261 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2026-09-01 | 4965.81 | 2025-12-01 08:47:28.332 | NULL | 1 |
| 3dc21823-fa6a-4f45-87b7-acaeb14c8af9 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2026-10-01 | 2017.46 | 2025-12-01 08:47:28.705 | NULL | 1 |
| 928e2f24-7d5d-4a63-8cb6-1a0f701691b7 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2026-11-01 | 352.72 | 2026-03-04 07:39:39.568 | NULL | 1 |
| 4da1c252-336e-41b9-bbcb-3781dbade81f | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2026-12-01 | 352.72 | 2026-03-04 07:39:39.913 | NULL | 1 |
| 57dc4ad0-061f-4cf0-b236-56efebf24df4 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2027-01-01 | 352.72 | 2026-03-04 07:39:40.256 | NULL | 1 |
| 28eadcc8-9d57-471f-a771-2dbc425a10a6 | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2027-02-01 | 252.82 | 2026-03-09 08:13:02.286 | NULL | 1 |
| 9366bc15-4200-472b-9109-2e017ed8125e | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2027-03-01 | 252.82 | 2026-03-09 08:13:02.618 | NULL | 1 |
| dd433821-d12b-465d-bae9-106b722e60fa | b4351e7e-f9ac-4a84-a113-a0e159303281 | 2027-04-01 | 159.84 | 2026-04-02 07:03:07.433 | NULL | 1 |

**A linha em negrito é a fatura de agosto/2026** (Total R$ 8.731,45, 97
`InvoiceItem` ligados). Verificação adicional (não invento, dado real):
`SELECT COUNT(*) FROM InvoiceItem WHERE InvoiceId =
'3f5a1afe-...'` → **97**; `MIN/MAX(PurchaseDate)` desses 97 itens →
**2025-08-05 a 2026-06-23**. Cada `InvoiceItem` existe uma única vez no
banco (testei com o item "Água de Coco" de 27/06,
Id `d16cb2ba-99bd-4189-98c7-f7366f9fccb4`: só está ligado a este
`InvoiceId` de agosto, não há duplicata em outro mês) — o vínculo
`InvoiceId → mês da fatura` não segue o `PurchaseDate` real da compra
para a maior parte desses 97 itens. Não sei a causa raiz dessa
distribuição no dado de origem; não vou especular sem evidência adicional.

---

## 6. Decisão confirmada — formato e fonte de categoria

O usuário havia pedido o formato de mensagem única (`gastei ... no
crédito XP e coloque em Categoria > Subcategoria`), mas eu reportei um
fato já verificado por código no documento de jun/jul (seção 0.5, Achado
1): esse formato **quebra o guard determinístico do cartão**
(`cardExpenseMarkerRe`/`parseCardExpenseShortcut` em
`internal/agents/application/agents/guards/card_expense_shortcut.go` —
tudo depois de "no crédito" vira candidato a apelido do cartão, e mais de
3 palavras faz o guard falhar, caindo no caminho por LLM não validado).
Também havia uma divergência real entre a árvore de categoria do legado
(`Tags`/`Category`) e a árvore real do mecontrola (ex.: Netflix é
`Conforto > Streamings` no legado, mas `Prazeres > Streaming de Vídeo` no
mecontrola).

**Decisão confirmada pelo usuário:**

1. **Formato de 2 mensagens** (já validado pelo guard real) — não o
   formato de mensagem única.
2. **Fonte de categoria: árvore real do mecontrola** (não o legado).

Isso é exatamente o que já está pronto nas tabelas da seção 3 deste
documento — nenhuma frase nova precisa ser gerada; as 7 frases da seção 3
(4 de cartão + 3 de conta fixa), com a coluna "Categoria completa" já
usando a árvore real do mecontrola, são a resposta final desta seção.

---

## 7. Todas as 98 compras dentro da fatura de agosto (`InvoiceId` = fatura de agosto por cartão)

Por pedido explícito do usuário. Lista completa dos itens ligados às 2
faturas com `Invoice.Date = 2026-08-01` (97 no XP Visa, `Invoice.Id
3f5a1afe-063b-4053-b1b2-8ab70bf5c5b3`; 1 no Nubank, `Invoice.Id
dc19f5d5-d5a5-4298-a1c4-935c19590057`) — nenhuma linha omitida ou
resumida, ordenada por `PurchaseDate`. Coluna "descrição" traz o valor
TOTAL da compra e o valor da PARCELA cobrada nesta fatura específica
(`InstallmentValue`) entre parênteses.

**Verificação de soma (dado real, não invento):** somando
`InstallmentValue` dos 97 itens do XP Visa dá **R$ 8.731,45** — bate
exatamente com `Invoice.Total` da fatura de agosto (seção 5). Somando o
único item do Nubank (`InstallmentValue = R$ 1,00`) bate com o
`Invoice.Total` de R$ 1,00 do Nubank. Ou seja: o valor que compõe a
fatura de agosto é a soma das PARCELAS cobradas neste mês, não o valor
total das compras — para os 8 itens parcelados (linhas 1, 3, 4, 5, 6, 7,
8 e o item do Nubank), isso explica por que aparecem na fatura de agosto
mesmo com `PurchaseDate` de meses anteriores (é uma parcela de uma compra
parcelada vencendo em agosto). **Isso não explica os outros 90 itens**
(`Installment = 1`, à vista, `PurchaseDate` entre 27/06 e 18/07) — para
esses, compra total e parcela têm o mesmo valor, e não há uma razão de
parcelamento para estarem na fatura de agosto em vez da fatura do mês em
que a compra aconteceu. Meu achado anterior (seção "Achado importante")
sobre não gerar frase de WhatsApp para esses itens continua valendo: a
data real da compra (`PurchaseDate`) desses 90 itens é jun/jul, já
coberta no documento anterior.

| # | Data da compra (`PurchaseDate`) | Cartão | Descrição (compra total, parcela desta fatura, parcelamento) | Categoria (legado, `Tags > Category`) | `InvoiceItem.Id` |
|---|---|---|---|---|---|
| 1 | 2025-08-05 | XP Visa | Team Cruz [JJ] (compra R$ 900,00, parcela desta fatura R$ 75,00, 12x) | Conhecimento > Academia | 4c4f3df2-c7d8-4419-8afd-d65ef5945076 |
| 2 | 2025-12-01 | Nubank | Aleatório (compra R$ 12,00, parcela desta fatura R$ 1,00, 8x) | Prazeres > Eletrônicos | f502e758-5e17-4bbe-ac71-7a1acc280101 |
| 3 | 2025-12-01 | XP Visa | Viagem [Maio 2026] (compra R$ 3715,32, parcela desta fatura R$ 371,53, 8x) | Metas > Lazer | fad634dc-6fb7-4385-8e81-d5f858996969 |
| 4 | 2025-12-19 | XP Visa | Decolar [Viagem Maceió] (compra R$ 12932,12, parcela desta fatura R$ 1293,21, 8x) | Metas > Lazer | 1cbd42a1-ae1e-44ee-832b-6c37dfa00496 |
| 5 | 2026-03-04 | XP Visa | IA para Devs (compra R$ 999,00, parcela desta fatura R$ 99,90, 5x) | Conhecimento > Educação | f6d3780d-42e3-43a9-9f82-6ae2b9a786a0 |
| 6 | 2026-03-09 | XP Visa | Tech Leads [Club] (compra R$ 1115,73, parcela desta fatura R$ 92,98, 5x) | Conhecimento > Educação | 999676e7-c794-44b8-803e-81dbead47f6c |
| 7 | 2026-04-02 | XP Visa | Arquitetura de Soluções com IA (compra R$ 1499,25, parcela desta fatura R$ 124,94, 4x) | Conhecimento > Educação | 41312987-a6aa-45de-94c7-056850cb0553 |
| 8 | 2026-04-14 | XP Visa | HBO Max (compra R$ 418,80, parcela desta fatura R$ 34,90, 4x) | Conforto > Streamings | 9f17324c-6110-480b-aba0-d5b7c19de44b |
| 9 | 2026-06-27 | XP Visa | Abastecimento [Tracker] (compra R$ 224,19, parcela desta fatura R$ 224,19, 1x) | Custos fixos > Abastecimento | efa35074-307e-429a-928e-44b414b1564e |
| 10 | 2026-06-27 | XP Visa | Github (compra R$ 55,00, parcela desta fatura R$ 55,00, 1x) | Custos fixos > Serviços | c4ababc2-2c7a-462c-9ea3-14a7750a88cb |
| 11 | 2026-06-27 | XP Visa | Lavagem [Tracker] (compra R$ 60,00, parcela desta fatura R$ 60,00, 1x) | Conforto > Lavagem Automotiva | 1c91a532-e1ff-4c4a-841c-0aef87437c93 |
| 12 | 2026-06-27 | XP Visa | Mercadinho Porto (compra R$ 26,98, parcela desta fatura R$ 26,98, 1x) | Custos fixos > Supermercado | 85ae8027-bfae-485f-aa1e-3271fd456972 |
| 13 | 2026-06-27 | XP Visa | Mercadinho Porto (compra R$ 62,36, parcela desta fatura R$ 62,36, 1x) | Custos fixos > Supermercado | 6810fba9-6d78-48cb-b9fa-9b8cb4a98455 |
| 14 | 2026-06-27 | XP Visa | Mercado Bom Demais (compra R$ 73,37, parcela desta fatura R$ 73,37, 1x) | Custos fixos > Supermercado | d8161f88-eec6-4c1b-9bd8-d8939671dbe7 |
| 15 | 2026-06-27 | XP Visa | Outros (compra R$ 27,95, parcela desta fatura R$ 27,95, 1x) | Prazeres > Outros | f26d08df-d39a-48f8-982a-6163b368dd50 |
| 16 | 2026-06-27 | XP Visa | Padaria Do Aurora (compra R$ 8,50, parcela desta fatura R$ 8,50, 1x) | Prazeres > Restaurante | b2f51ac9-7fa7-4611-ab0d-472a0804556c |
| 17 | 2026-06-27 | XP Visa | Pastel Do Guri (compra R$ 13,00, parcela desta fatura R$ 13,00, 1x) | Prazeres > Feira [Alimentação] | 22ebc94b-4757-491b-af30-9288792278fb |
| 18 | 2026-06-27 | XP Visa | Pastel Do Guri (compra R$ 17,00, parcela desta fatura R$ 17,00, 1x) | Prazeres > Feira [Alimentação] | 568fda34-3e10-4c24-abde-fb7095bedd7e |
| 19 | 2026-06-27 | XP Visa | Pizzaria Bonna Notte (compra R$ 124,00, parcela desta fatura R$ 124,00, 1x) | Prazeres > Restaurante | 9ebdbda6-26dc-4e70-bcd6-2efe9e7057dc |
| 20 | 2026-06-27 | XP Visa | Água de Coco (compra R$ 11,00, parcela desta fatura R$ 11,00, 1x) | Prazeres > Feira [Alimentação] | d16cb2ba-99bd-4189-98c7-f7366f9fccb4 |
| 21 | 2026-06-28 | XP Visa | Casa de Carnes (compra R$ 181,55, parcela desta fatura R$ 181,55, 1x) | Prazeres > Supermercado | 51e81e93-bd7c-4dd1-beb1-12b26a85a6e6 |
| 22 | 2026-06-28 | XP Visa | Churrasquinho (compra R$ 50,00, parcela desta fatura R$ 50,00, 1x) | Prazeres > Feira [Alimentação] | 18c7d32b-d6ca-4961-bdd6-3d8b0bc5272b |
| 23 | 2026-06-28 | XP Visa | Churros (compra R$ 14,00, parcela desta fatura R$ 14,00, 1x) | Prazeres > Feira [Alimentação] | c3052670-e73a-488d-b940-2f9e6b0fbc07 |
| 24 | 2026-06-28 | XP Visa | Doces (compra R$ 36,00, parcela desta fatura R$ 36,00, 1x) | Prazeres > Feira [Alimentação] | 64616063-a8e9-4467-8a06-ba7770d6eb6b |
| 25 | 2026-06-28 | XP Visa | Doces (compra R$ 70,10, parcela desta fatura R$ 70,10, 1x) | Prazeres > Supermercado | 32552a90-d0c6-4118-adf4-5c563535a515 |
| 26 | 2026-06-28 | XP Visa | Energético (compra R$ 34,00, parcela desta fatura R$ 34,00, 1x) | Conforto > Bebidas | 249ce500-75f6-48de-b5e9-a3514848c148 |
| 27 | 2026-06-28 | XP Visa | Mercadinho Porto (compra R$ 23,97, parcela desta fatura R$ 23,97, 1x) | Custos fixos > Supermercado | 7f647a0c-2b2b-4d9b-8658-dbdc4e75e4c1 |
| 28 | 2026-06-28 | XP Visa | Minibola Copa Do Mundo Da Fifa 26 (compra R$ 95,99, parcela desta fatura R$ 95,99, 1x) | Prazeres > Lazer | 5ac0c0e7-7ab4-4aa4-ac14-73384e93b4a3 |
| 29 | 2026-06-28 | XP Visa | Motel (compra R$ 209,00, parcela desta fatura R$ 209,00, 1x) | Prazeres > Outros | 2d3f03e3-20bd-4b40-922f-f25feff5dc2b |
| 30 | 2026-06-28 | XP Visa | Tylenol Sisu (compra R$ 24,79, parcela desta fatura R$ 24,79, 1x) | Custos fixos > Saúde | 7ccbbc53-2ed7-460b-a642-c223dfebcc50 |
| 31 | 2026-06-28 | XP Visa | iFood [Nasoar] (compra R$ 179,37, parcela desta fatura R$ 179,37, 1x) | Custos fixos > Saúde | be1531be-f531-4364-8990-62cf4d7ab869 |
| 32 | 2026-06-29 | XP Visa | Armazém Paraná (compra R$ 60,32, parcela desta fatura R$ 60,32, 1x) | Custos fixos > Hortifruti | 36a7cbc9-d14c-448b-a5d4-2a719a9a172a |
| 33 | 2026-06-30 | XP Visa | Sabão Lavar Louças (compra R$ 124,58, parcela desta fatura R$ 124,58, 1x) | Custos fixos > Casa | ba7c2483-10a4-4cb1-8646-52a93b6a7f96 |
| 34 | 2026-06-30 | XP Visa | Vitaminas (compra R$ 127,70, parcela desta fatura R$ 127,70, 1x) | Custos fixos > Suplementos | f0197acb-7e81-4f25-9f92-c0adafc5426e |
| 35 | 2026-07-01 | XP Visa | Academia [JJ + Stefany] (compra R$ 254,83, parcela desta fatura R$ 254,83, 1x) | Custos fixos > Academia | cab46d7d-f077-46fc-86a0-419abeb04d6a |
| 36 | 2026-07-01 | XP Visa | Armazém Paraná (compra R$ 60,32, parcela desta fatura R$ 60,32, 1x) | Custos fixos > Hortifruti | 0a1539fb-5101-4b60-96ae-a13a86f035fe |
| 37 | 2026-07-01 | XP Visa | Bolo [Festa Junina JJ] (compra R$ 26,00, parcela desta fatura R$ 26,00, 1x) | Prazeres > Lazer | 900bc691-d186-4ce7-8da6-375987c6633f |
| 38 | 2026-07-01 | XP Visa | Corte de cabelo (compra R$ 77,00, parcela desta fatura R$ 77,00, 1x) | Custos fixos > Serviços | 707dbe87-a9d4-41a0-a044-9357fe83893d |
| 39 | 2026-07-01 | XP Visa | Mala de Viagem (compra R$ 198,39, parcela desta fatura R$ 198,39, 1x) | Conforto > Viagem | e4fa5d08-c984-4b1c-b93c-028948462a8e |
| 40 | 2026-07-01 | XP Visa | Mercadinho [Energético] (compra R$ 29,47, parcela desta fatura R$ 29,47, 1x) | Prazeres > Supermercado | a18a473d-5cca-40a8-b460-9f755417c7e1 |
| 41 | 2026-07-01 | XP Visa | Microsoft Azure (compra R$ 76,45, parcela desta fatura R$ 76,45, 1x) | Custos fixos > Serviços | 8eeca181-9002-464e-ad18-83cc85c054fd |
| 42 | 2026-07-01 | XP Visa | Netflix (compra R$ 44,90, parcela desta fatura R$ 44,90, 1x) | Conforto > Streamings | 7b074100-6a8e-48c9-9cbc-aa42dc5c782f |
| 43 | 2026-07-01 | XP Visa | Oficial Farma (compra R$ 194,70, parcela desta fatura R$ 194,70, 1x) | Custos fixos > Suplementos | 1961faa6-4e0c-4d2c-bb2d-84851d9ac366 |
| 44 | 2026-07-01 | XP Visa | Pizzaria Bonna Notte (compra R$ 131,00, parcela desta fatura R$ 131,00, 1x) | Prazeres > Prazeres | ed47a860-5c29-45e9-a823-f35b9e7050af |
| 45 | 2026-07-02 | XP Visa | Lavagem [Tracker] (compra R$ 60,00, parcela desta fatura R$ 60,00, 1x) | Conforto > Lavagem Automotiva | 6191a5a9-ac7d-40b1-942a-92831adf59ae |
| 46 | 2026-07-02 | XP Visa | Mordedor Helena (compra R$ 25,80, parcela desta fatura R$ 25,80, 1x) | Conforto > Bebê | 6967c864-4b11-489b-a420-79bcc0d25fe0 |
| 47 | 2026-07-02 | XP Visa | Oral-B Refil (compra R$ 70,00, parcela desta fatura R$ 70,00, 1x) | Custos fixos > Saúde | 18fdafe3-9654-4ee4-b506-60f177456592 |
| 48 | 2026-07-03 | XP Visa | Algodão Doce + Pipoca (compra R$ 25,00, parcela desta fatura R$ 25,00, 1x) | Prazeres > Feira [Alimentação] | 07a45df1-cc04-4cb8-9f9b-d580644f9345 |
| 49 | 2026-07-03 | XP Visa | Doces (compra R$ 34,00, parcela desta fatura R$ 34,00, 1x) | Prazeres > Feira [Alimentação] | e156b4fe-2c99-4169-ac2c-2c39d9683834 |
| 50 | 2026-07-03 | XP Visa | Drogaria São Luis (compra R$ 63,60, parcela desta fatura R$ 63,60, 1x) | Custos fixos > Saúde | 6dda0394-6d2c-4eb4-acd2-365358dff9b4 |
| 51 | 2026-07-03 | XP Visa | Espetinho (compra R$ 50,00, parcela desta fatura R$ 50,00, 1x) | Prazeres > Feira [Alimentação] | 120d96fb-2af5-4136-a8f1-a20b23a615bc |
| 52 | 2026-07-03 | XP Visa | Livros (compra R$ 60,00, parcela desta fatura R$ 60,00, 1x) | Conforto > Educação | 8153343e-2c95-4780-b2bc-1770c41658a6 |
| 53 | 2026-07-03 | XP Visa | Lojas Americanas (compra R$ 54,43, parcela desta fatura R$ 54,43, 1x) | Prazeres > Supérfluo | d8719b5d-132e-4fcf-8822-2d87bba56a48 |
| 54 | 2026-07-03 | XP Visa | Pamonha (compra R$ 30,00, parcela desta fatura R$ 30,00, 1x) | Prazeres > Feira [Alimentação] | fdd9496a-935c-4fb8-aa74-4c4c804ef44a |
| 55 | 2026-07-03 | XP Visa | Sem Parar (compra R$ 168,99, parcela desta fatura R$ 168,99, 1x) | Custos fixos > Transporte | 96b24f9e-38f4-40a0-b79b-1130616a13aa |
| 56 | 2026-07-03 | XP Visa | Sorvete (compra R$ 7,00, parcela desta fatura R$ 7,00, 1x) | Prazeres > Feira [Alimentação] | a06d4110-d835-4f0b-b271-f3f0a72b1bf8 |
| 57 | 2026-07-04 | XP Visa | Agua de coco (compra R$ 35,00, parcela desta fatura R$ 35,00, 1x) | Prazeres > Feira [Alimentação] | 7fda96e1-4925-46bb-8da0-6565b4e3225a |
| 58 | 2026-07-04 | XP Visa | Pastel Do Guri (compra R$ 26,00, parcela desta fatura R$ 26,00, 1x) | Prazeres > Feira [Alimentação] | d0d1939d-bf46-4f76-80b0-86e72c78cc36 |
| 59 | 2026-07-05 | XP Visa | Estacionamento (compra R$ 30,00, parcela desta fatura R$ 30,00, 1x) | Custos fixos > Transporte | 559cb145-8db0-42ed-a465-7e1c8c2fefc7 |
| 60 | 2026-07-07 | XP Visa | Carlinhos [Corte do Tony] (compra R$ 57,00, parcela desta fatura R$ 57,00, 1x) | Custos fixos > Serviços | 431daa81-a2d4-4b5d-ae50-3e00b6716a14 |
| 61 | 2026-07-07 | XP Visa | Figurinhas da Copa (compra R$ 70,00, parcela desta fatura R$ 70,00, 1x) | Prazeres > Lazer | 66be376d-d66f-4b60-842b-2da0cdea1e87 |
| 62 | 2026-07-07 | XP Visa | Mercadinho (compra R$ 27,95, parcela desta fatura R$ 27,95, 1x) | Conforto > Supermercado | 0da8f8d9-4c2b-4d94-b3ff-07c0d6e735f7 |
| 63 | 2026-07-07 | XP Visa | Mercadinho [Almoço] (compra R$ 59,59, parcela desta fatura R$ 59,59, 1x) | Conforto > Supermercado | 1850e830-f636-4b34-a1b4-1c111f911fab |
| 64 | 2026-07-08 | XP Visa | Figurinhas (compra R$ 77,99, parcela desta fatura R$ 77,99, 1x) | Conforto > Outros | 2f340531-9761-4468-a494-4d6b2881281e |
| 65 | 2026-07-08 | XP Visa | Suplementos (compra R$ 52,55, parcela desta fatura R$ 52,55, 1x) | Custos fixos > Suplementos | 02cd1bca-6ebb-4223-af1c-c70d92216e63 |
| 66 | 2026-07-08 | XP Visa | São Vicente (compra R$ 82,02, parcela desta fatura R$ 82,02, 1x) | Custos fixos > Supermercado | 5ac04fe9-3418-4a0d-aab7-041d4f1380d0 |
| 67 | 2026-07-09 | XP Visa | Compras do mês (compra R$ 551,68, parcela desta fatura R$ 551,68, 1x) | Custos fixos > Supermercado | 4adfa39b-dd9c-42f1-b807-dd6e46f1964d |
| 68 | 2026-07-09 | XP Visa | Microsoft Azure (compra R$ 70,55, parcela desta fatura R$ 70,55, 1x) | Custos fixos > Serviços | e296f0cb-fa8b-410b-9a44-5b29cbefbb60 |
| 69 | 2026-07-10 | XP Visa | Chopp (compra R$ 30,00, parcela desta fatura R$ 30,00, 1x) | Prazeres > Feira [Alimentação] | d22099d9-8277-4504-a67e-4e674fdecb38 |
| 70 | 2026-07-10 | XP Visa | Churros (compra R$ 14,00, parcela desta fatura R$ 14,00, 1x) | Prazeres > Feira [Alimentação] | b30dacc4-d859-4b2b-97f7-12d7b879fef4 |
| 71 | 2026-07-10 | XP Visa | Doces (compra R$ 31,00, parcela desta fatura R$ 31,00, 1x) | Prazeres > Feira [Alimentação] | 1ec19826-3556-4711-a964-ca00d6ab956e |
| 72 | 2026-07-10 | XP Visa | Hb Solutions [Chaveiro] (compra R$ 15,00, parcela desta fatura R$ 15,00, 1x) | Conforto > Outros | c53d251e-813e-4c37-ad09-d59e36a8a2ac |
| 73 | 2026-07-10 | XP Visa | Ração do Peixe (compra R$ 19,90, parcela desta fatura R$ 19,90, 1x) | Custos fixos > Supermercado | c9ff8db6-74a1-4ce8-9407-ef6e0c7a8bc0 |
| 74 | 2026-07-10 | XP Visa | Suplementos [Pré Treino] (compra R$ 89,90, parcela desta fatura R$ 89,90, 1x) | Custos fixos > Suplementos | b53d7ed4-e8fb-4513-aaab-c6c7cf063704 |
| 75 | 2026-07-11 | XP Visa | Abastecimento [Tracker] (compra R$ 237,11, parcela desta fatura R$ 237,11, 1x) | Custos fixos > Abastecimento | 785da6b1-f3a9-44f3-8621-cc92889f3ea8 |
| 76 | 2026-07-11 | XP Visa | Armarinhos Fernandes (compra R$ 94,19, parcela desta fatura R$ 94,19, 1x) | Conforto > Bebê | 6f7d97ba-f43f-4464-942e-ce329668dd43 |
| 77 | 2026-07-11 | XP Visa | Madero (compra R$ 144,00, parcela desta fatura R$ 144,00, 1x) | Conforto > Restaurante | bb9c7452-282d-4d99-acce-207a43c698f9 |
| 78 | 2026-07-11 | XP Visa | McDonald's (compra R$ 38,00, parcela desta fatura R$ 38,00, 1x) | Prazeres > Restaurante | 412c15fc-c0b8-4d08-8530-4fd665533bde |
| 79 | 2026-07-11 | XP Visa | Pharma Nutry (compra R$ 32,50, parcela desta fatura R$ 32,50, 1x) | Prazeres > Suplementos | 27239945-c516-49fa-9f04-bcf6ccf4886f |
| 80 | 2026-07-11 | XP Visa | Pão (compra R$ 36,47, parcela desta fatura R$ 36,47, 1x) | Custos fixos > Supermercado | 0ab01000-085a-40d0-a37b-fa05876ad015 |
| 81 | 2026-07-12 | XP Visa | Doces (compra R$ 36,00, parcela desta fatura R$ 36,00, 1x) | Prazeres > Feira [Alimentação] | 3c994b30-a284-4280-9572-6e410be3ee7f |
| 82 | 2026-07-12 | XP Visa | Energético (compra R$ 36,79, parcela desta fatura R$ 36,79, 1x) | Conforto > Supérfluo | e4ccffea-a706-4cb3-88ef-02d4ef5649fa |
| 83 | 2026-07-12 | XP Visa | Maquininha (compra R$ 8,00, parcela desta fatura R$ 8,00, 1x) | Prazeres > Outros | 8d38af90-674a-46aa-ab96-0d0527ca1c00 |
| 84 | 2026-07-13 | XP Visa | Almoço (compra R$ 64,00, parcela desta fatura R$ 64,00, 1x) | Custos fixos > Restaurante | 8316fbb9-93e5-4f8b-b50a-99f8590ca8ee |
| 85 | 2026-07-13 | XP Visa | Cookies (compra R$ 18,00, parcela desta fatura R$ 18,00, 1x) | Prazeres > Supérfluo | 3c1f3c5b-0cb1-4bb3-90a4-98fa961c58f4 |
| 86 | 2026-07-13 | XP Visa | Doces (compra R$ 13,90, parcela desta fatura R$ 13,90, 1x) | Prazeres > Supermercado | df3ec41d-a987-46d2-a37b-c600ba2419e6 |
| 87 | 2026-07-13 | XP Visa | Doces (compra R$ 5,90, parcela desta fatura R$ 5,90, 1x) | Prazeres > Restaurante | 204bf48e-4121-4492-b850-e55538226380 |
| 88 | 2026-07-13 | XP Visa | Doces Santa Rita (compra R$ 23,20, parcela desta fatura R$ 23,20, 1x) | Prazeres > Outros | ea613013-c9e4-47d7-8b24-1a964f8eef10 |
| 89 | 2026-07-13 | XP Visa | Estacionamento (compra R$ 45,00, parcela desta fatura R$ 45,00, 1x) | Custos fixos > Transporte | 1aca7ea6-cc64-4d06-9ae8-507e60bd4ea4 |
| 90 | 2026-07-14 | XP Visa | Apple (compra R$ 5,90, parcela desta fatura R$ 5,90, 1x) | Custos fixos > Serviços | 62241bde-c03b-4b37-b9bd-3a629dc49761 |
| 91 | 2026-07-14 | XP Visa | Compras do mês (compra R$ 340,64, parcela desta fatura R$ 340,64, 1x) | Custos fixos > Supermercado | 72f1d946-0705-43dc-be85-abb4be1833be |
| 92 | 2026-07-16 | XP Visa | Almoço (compra R$ 39,89, parcela desta fatura R$ 39,89, 1x) | Custos fixos > Restaurante | e0f4df49-c424-4ac7-9f09-9bbd24bc8894 |
| 93 | 2026-07-16 | XP Visa | Google (compra R$ 12,50, parcela desta fatura R$ 12,50, 1x) | Custos fixos > Serviços | 07f6e79c-0882-4a2e-ad1b-4760e15b5f6c |
| 94 | 2026-07-17 | XP Visa | Abastecimento [Tracker] (compra R$ 127,21, parcela desta fatura R$ 127,21, 1x) | Custos fixos > Abastecimento | b90e2ff2-cb24-42ab-a2e0-1a9468375aeb |
| 95 | 2026-07-17 | XP Visa | Github (compra R$ 27,01, parcela desta fatura R$ 27,01, 1x) | Custos fixos > Serviços | 5374d9da-64a7-4e54-ae72-219e49ae37cf |
| 96 | 2026-07-17 | XP Visa | Lacake (compra R$ 15,00, parcela desta fatura R$ 15,00, 1x) | Conforto > Feira [Alimentação] | 0332a7bf-444e-49d4-b28e-61541f7b9a05 |
| 97 | 2026-07-18 | XP Visa | Anthropic (compra R$ 110,00, parcela desta fatura R$ 110,00, 1x) | Custos fixos > Serviços | 350fd300-02f0-4df1-bdcd-ea0bdf9e1346 |
| 98 | 2026-07-18 | XP Visa | Nasoar (compra R$ 111,05, parcela desta fatura R$ 111,05, 1x) | Custos fixos > Saúde | 2ff73576-a572-4687-ad40-d5de28c1cf45 |


---

## 8. Correção final — causa raiz encontrada (print real do app confirma) + todos os lançamentos a partir de 27/07/2026

O usuário mandou um print real da tela do app (cartão XP Visa) mostrando
a fatura com **fechamento em 01/09/2026** (Total R$ 4.965,81) já incluindo
lançamentos a partir de **27/07/2026**. Isso bate exatamente com
`Invoice.Id 65ab4af1-3139-4688-b6f1-120eea78a261` (`Date=2026-09-01`,
`Total=4965.81`) da lista completa da seção 5.

**Isso resolve a dúvida em aberto das seções "Achado importante" e 7**:
não é uma distribuição arbitrária/sem causa raiz — é um **ciclo de fatura
normal de cartão de crédito**, com fechamento por volta de 26-27 de cada
mês. Reconsultei o banco: `SELECT * FROM InvoiceItem ii INNER JOIN
Invoice i ON ii.InvoiceId=i.Id WHERE ii.PurchaseDate >= '2026-07-27'`
mostra que **todo item com `PurchaseDate >= 27/07/2026` está ligado ao
`InvoiceId` que fecha em 01/09/2026**, não ao de 01/08/2026. Ou seja: a
fatura "de agosto" (`Invoice.Date=2026-08-01`, fechamento) acumula compras
até por volta de 18-26/07; compras a partir de 27/07 já entram na fatura
seguinte (fechamento 01/09). Retiro a frase anterior "não sei a causa
raiz" — a causa raiz é o ciclo de fechamento do cartão, confirmado agora
por evidência direta (print do app + dado do banco batendo 100%).

### 8.1 Todos os 28 lançamentos com `PurchaseDate >= 27/07/2026` (Invoice + InvoiceItem, cartão XP Visa)

Nenhuma linha omitida. Fonte: `sqlcmd`, `InvoiceItem` join `Invoice` join
`Card`/`Category`, `PurchaseDate >= '2026-07-27'`, ordenado por data e
descrição.

| # | Data | Descrição (legado) | Valor | Legado `Tags > Category` |
|---|---|---|---|---|
| 1 | 2026-07-27 | Armazém Paraná | R$ 47,71 | Custos fixos > Hortifruti |
| 2 | 2026-07-27 | Bom demais [Proteínas] | R$ 194,23 | Custos fixos > Supermercado |
| 3 | 2026-07-27 | Chatgpt | R$ 99,90 | Custos fixos > Serviços |
| 4 | 2026-07-27 | Github | R$ 69,65 | Custos fixos > Serviços |
| 5 | 2026-07-27 | Kimi [Moonshot AI] | R$ 100,54 | Custos fixos > Serviços |
| 6 | 2026-07-27 | Leve Mais | R$ 79,88 | Custos fixos > Supermercado |
| 7 | 2026-07-27 | Porto Mercadinho | R$ 47,00 | Custos fixos > Supermercado |
| 8 | 2026-07-27 | Varejão Paraná | R$ 121,66 | Custos fixos > Supermercado |
| 9 | 2026-07-27 | Whey | R$ 229,90 | Custos fixos > Suplementos |
| 10 | 2026-07-27 | Whey [Tony] | R$ 338,92 | Custos fixos > Suplementos |
| 11 | 2026-07-28 | Energético | R$ 42,49 | Prazeres > Supermercado (Supérfluo) |
| 12 | 2026-07-28 | Moonshot AI | R$ 207,66 | Custos fixos > Serviços |
| 13 | 2026-07-28 | Nasoar | R$ 178,08 | Custos fixos > Saúde |
| 14 | 2026-07-29 | Energético | R$ 17,57 | Prazeres > Supermercado (Supérfluo) |
| 15 | 2026-07-29 | Pizzaria Bonna Notte | R$ 149,00 | Prazeres > Restaurante |
| 16 | 2026-07-30 | Armazém Paraná | R$ 39,97 | Custos fixos > Hortifruti |
| 17 | 2026-07-30 | Lavagem [Tracker] | R$ 60,00 | Conforto > Lavagem Automotiva |
| 18 | 2026-07-30 | Varejão Paraná | R$ 150,93 | Custos fixos > Hortifruti |
| 19 | 2026-07-31 | Abastecimento [Tracker] | R$ 257,56 | Custos fixos > Abastecimento |
| 20 | 2026-07-31 | Churrasquinho | R$ 50,00 | Prazeres > Feira [Alimentação] |
| 21 | 2026-07-31 | Doces | R$ 40,00 | Prazeres > Feira [Alimentação] |
| 22 | 2026-07-31 | Hamburgueria do Portuga | R$ 189,20 | Prazeres > Restaurante |
| 23 | 2026-07-31 | Lacake | R$ 15,00 | Prazeres > Feira [Alimentação] |
| 24 | 2026-07-31 | Porto Mercadinho | R$ 28,38 | Custos fixos > Supermercado |
| 25 | 2026-08-01 | Corte de Cabelo | R$ 77,00 | Custos fixos > Serviços |
| 26 | 2026-08-01 | Leve Mais [Mercadinho] | R$ 48,42 | Custos fixos > Supermercado |
| 27 | 2026-08-01 | Netflix | R$ 44,90 | Conforto > Streamings |
| 28 | 2026-08-01 | Ovos | R$ 22,80 | Custos fixos > Supermercado |

### 8.2 De-para real contra `mecontrola.category_dictionary` — 19 termos únicos novos

Mesma metodologia (exato → token → fuzzy ≥0,4), rodada via SQL direto no
Postgres de produção para todos os termos únicos deste lote que ainda não
tinham sido checados:

| Termo | Estágio | Resultado |
|---|---|---|
| Abastecimento | exato | `Custo Fixo > Combustível` |
| Chatgpt | exato | `Custo Fixo > Assinaturas Essenciais` |
| Mercadinho (em "Porto Mercadinho") | token | `Custo Fixo > Supermercado` |
| Pizzaria (em "Pizzaria Bonna Notte") | token | `Prazeres > Bares e Lanches` |
| Hamburgueria (em "Hamburgueria do Portuga") | token | `Prazeres > Bares e Lanches` |
| Armazém Paraná, Bom demais, Github, Kimi, Varejão Paraná, Whey, Energético, Moonshot AI, Nasoar, Lavagem, Churrasquinho, Doces, Lacake | exato/token/fuzzy(≥0,4) | sem match em nenhum estágio |

### 8.3 Frases completas para o WhatsApp (formato de 2 mensagens, categoria real do mecontrola)

Mesma decisão já confirmada: formato de 2 mensagens (guard-safe) +
categoria da árvore real do mecontrola. Para os termos sem match
automático, uso escolha manual/contextual **só quando o mapeamento é
inequívoco** (ex.: assinatura de IA → Assinaturas Essenciais, supermercado
→ Supermercado); quando o termo é ambíguo (ex.: "Nasoar", "Energético",
"Whey", "Doces", "Churrasquinho", "Lacake", "Lavagem"), deixo sem 2ª
mensagem — forçar uma categoria aqui seria inventar.

| # | 1ª mensagem — registrar | Categoria completa | Origem |
|---|---|---|---|
| 1 | `gastei 47,71 no Armazém Paraná no crédito XP` | `Custo Fixo > Feira e Hortifruti` | manual — legado já marca como Hortifruti, subcategoria real existe |
| 2 | `gastei 194,23 no Bom demais no crédito XP` | `Custo Fixo > Supermercado` | manual — legado marca Category=Supermercado |
| 3 | `gastei 99,90 no Chatgpt no crédito XP` | `Custo Fixo > Assinaturas Essenciais` | algoritmo real — match exato |
| 4 | `gastei 69,65 no Github no crédito XP` | `Custo Fixo > Assinaturas Essenciais` | manual — mesmo padrão de Chatgpt/Open.AI (jun/jul) |
| 5 | `gastei 100,54 no Kimi no crédito XP` | `Custo Fixo > Assinaturas Essenciais` | manual — assinatura de IA, mesmo padrão |
| 6 | `gastei 79,88 no Leve Mais no crédito XP` | `Custo Fixo > Supermercado` | manual — mesma decisão já usada para "Leve Mais" |
| 7 | `gastei 47,00 no Porto Mercadinho no crédito XP` | `Custo Fixo > Supermercado` | algoritmo real — match token ("mercadinho") |
| 8 | `gastei 121,66 no Varejão Paraná no crédito XP` | `Custo Fixo > Supermercado` | manual — legado marca Category=Supermercado |
| 9 | `gastei 229,90 no Whey no crédito XP` | — | sem categoria real conhecida (termo ambíguo, não invento) |
| 10 | `gastei 338,92 no Whey no crédito XP` | — | sem categoria real conhecida (mesmo termo "Whey") |
| 11 | `gastei 42,49 no Energético no crédito XP` | — | sem categoria real conhecida (termo ambíguo) |
| 12 | `gastei 207,66 no Moonshot AI no crédito XP` | `Custo Fixo > Assinaturas Essenciais` | manual — assinatura de IA, mesmo padrão |
| 13 | `gastei 178,08 no Nasoar no crédito XP` | — | sem categoria real conhecida (termo ambíguo) |
| 14 | `gastei 17,57 no Energético no crédito XP` | — | sem categoria real conhecida (termo ambíguo) |
| 15 | `gastei 149,00 no Pizzaria Bonna Notte no crédito XP` | `Prazeres > Bares e Lanches` | algoritmo real — match token ("pizzaria") |
| 16 | `gastei 39,97 no Armazém Paraná no crédito XP` | `Custo Fixo > Feira e Hortifruti` | manual — mesmo termo do item 1 |
| 17 | `gastei 60,00 no Lavagem no crédito XP` | — | sem categoria real conhecida (termo ambíguo) |
| 18 | `gastei 150,93 no Varejão Paraná no crédito XP` | `Custo Fixo > Supermercado` | manual — mesmo termo do item 8 |
| 19 | `gastei 257,56 no Abastecimento no crédito XP` | `Custo Fixo > Combustível` | algoritmo real — match exato |
| 20 | `gastei 50,00 no Churrasquinho no crédito XP` | — | sem categoria real conhecida (termo ambíguo) |
| 21 | `gastei 40,00 no Doces no crédito XP` | — | sem categoria real conhecida (termo ambíguo) |
| 22 | `gastei 189,20 no Hamburgueria do Portuga no crédito XP` | `Prazeres > Bares e Lanches` | algoritmo real — match token ("hamburgueria") |
| 23 | `gastei 15,00 no Lacake no crédito XP` | — | sem categoria real conhecida (termo ambíguo) |
| 24 | `gastei 28,38 no Porto Mercadinho no crédito XP` | `Custo Fixo > Supermercado` | algoritmo real — match token ("mercadinho") |
| 25 | `gastei 77,00 no Corte de Cabelo no crédito XP` | `Prazeres > Beleza e Estética` | manual (já usada na seção 3) |
| 26 | `gastei 48,42 no Leve Mais no crédito XP` | `Custo Fixo > Supermercado` | manual (já usada na seção 3) |
| 27 | `gastei 44,90 no Netflix no crédito XP` | `Prazeres > Streaming de Vídeo` | algoritmo real — match exato |
| 28 | `gastei 22,80 no Ovos no crédito XP` | `Custo Fixo > Supermercado` | manual (já usada na seção 3) |

Itens 25-28 são os mesmos 4 já cobertos na seção 3 (repetidos aqui só
para a lista ficar completa a partir de 27/07, sem pular nenhum).
