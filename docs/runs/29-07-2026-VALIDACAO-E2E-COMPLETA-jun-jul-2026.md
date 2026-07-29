# Validação E2E completa — Agente WhatsApp mecontrola (dados reais jun/jul 2026)

Documento único que consolida TODO o material desta validação: dados brutos
extraídos do legado, listagem completa de lançamentos reais, o de-para com
frase pronta para o WhatsApp de cada um, a tabela de referência rápida dos
cenários de regressão, e o roteiro E2E completo (pré-condições, evidências
SQL/Prometheus/logs, mapeamento de commits). Nada foi resumido ou deixado de
fora dos arquivos anteriores — este arquivo é a fusão literal de todos eles.

Fonte dos dados reais: FinancialControlDB (SQL Server, `sqlcmd -S
SQL5053.site4now.net,1433 -d DB_A453C8_FinancialControl`). Extraído em
2026-07-29. Usuário de teste no mecontrola: `9bbbbbcd-2081-40a8-9780-2b5d818f1580`
(+5511986896322). VPS: `ssh mecontrola-vps` | DB: `mecontrola_db` (schema
`mecontrola`) | Stack: Swarm.

## Índice

1. [Dados brutos extraídos do FinancialControlDB](#1-dados-brutos-extraídos-do-financialcontroldb)
2. [Todos os lançamentos reais encontrados (146, um por um)](#2-todos-os-lançamentos-reais-encontrados)
3. [De-para completo — legado → frase pronta para o WhatsApp (155 itens)](#3-de-para-completo--legado--frase-pronta-para-o-whatsapp)
4. [Tabela de referência rápida — frases dos cenários de regressão](#4-tabela-de-referência-rápida--frases-dos-cenários-de-regressão)
5. [Roteiro completo de cenários E2E (S0 + C1-C11)](#5-roteiro-completo-de-cenários-e2e)

---

## 1. Dados brutos extraídos do FinancialControlDB

```text
================================================================================
DADOS REAIS — FinancialControlDB (SQL Server, site4now) — Junho e Julho de 2026
Fonte: sqlcmd -S SQL5053.site4now.net,1433 -d DB_A453C8_FinancialControl
Extraído em: 2026-07-29T09:52:57Z
================================================================================

--- CONTAGENS ---
[Transaction]+TransactionItem em jun/jul 2026: 0 linhas (tabela legada; MAX(Date)=2025-12-01 — sem dados de 2026, app real migrou para o Postgres do mecontrola antes desse período)
Bill+BillItem em jun/jul 2026: 10 linhas
Invoice+InvoiceItem em jun/jul 2026: 136 linhas

--- Bill + BillItem (jun/jul 2026) ---
2026-06-01 00:00:00.0000000|1724.39|1035.00|690.00|Condominio [Casa]|660.07
2026-06-01 00:00:00.0000000|1724.39|1035.00|690.00|Energia [Casa]|317.46
2026-06-01 00:00:00.0000000|1724.39|1035.00|690.00|Faxina [Casa]|400.00
2026-06-01 00:00:00.0000000|1724.39|1035.00|690.00|Internet [Celular]|225.62
2026-06-01 00:00:00.0000000|1724.39|1035.00|690.00|Internet [Central Fiber]|121.24
2026-07-01 00:00:00.0000000|1724.39|1035.00|690.00|Condominio [Casa]|660.07
2026-07-01 00:00:00.0000000|1724.39|1035.00|690.00|Energia [Casa]|317.46
2026-07-01 00:00:00.0000000|1724.39|1035.00|690.00|Faxina [Casa]|400.00
2026-07-01 00:00:00.0000000|1724.39|1035.00|690.00|Internet [Celular]|225.62
2026-07-01 00:00:00.0000000|1724.39|1035.00|690.00|Internet [Central Fiber]|121.24


--- Invoice + InvoiceItem (jun/jul 2026, ordenado por PurchaseDate) ---
XP Visa|2026-06-01 00:00:00.0000000|Netflix|44.9|1|Streamings
XP Visa|2026-06-01 00:00:00.0000000|Open.AI|99.9|1|Serviços
XP Visa|2026-06-01 00:00:00.0000000|Busterfit|254.83|1|Academia
XP Visa|2026-06-03 00:00:00.0000000|Drogaria São Luis|98.9|1|Saúde
XP Visa|2026-06-03 00:00:00.0000000|Zona Azul [Barueri]|10|1|Transporte
XP Visa|2026-06-03 00:00:00.0000000|Sem Parar|215.75|1|Transporte
XP Visa|2026-06-04 00:00:00.0000000|Amazon [Prime Video]|39.41|1|Streamings
XP Visa|2026-06-05 00:00:00.0000000|Compras do mês [Atacadão]|1002.29|1|Supermercado
XP Visa|2026-06-07 00:00:00.0000000|Itens da casa [Banheiro]|119.99|1|Casa
XP Visa|2026-06-08 00:00:00.0000000|Testo Black|108.04|1|Saúde
XP Visa|2026-06-09 00:00:00.0000000|Moonshot [IA]|102.34|1|Serviços
XP Visa|2026-06-09 00:00:00.0000000|Microsoft [Github]|11.63|1|Serviços
XP Visa|2026-06-09 00:00:00.0000000|Moonshot [IA]|107.8|1|Serviços
XP Visa|2026-06-10 00:00:00.0000000|Ebanx [PSN]|619.9|1|Lazer
XP Visa|2026-06-10 00:00:00.0000000|Nasoar|133.54|1|Saúde
XP Visa|2026-06-10 00:00:00.0000000|Itens da Casa|24.8|1|Casa
XP Visa|2026-06-11 00:00:00.0000000|Whey|194.9|1|Suplementos
XP Visa|2026-06-13 00:00:00.0000000|Abastecimento [Tracker]|293.15|1|Abastecimento
XP Visa|2026-06-13 00:00:00.0000000|hevy.com|12.9|1|Serviços
XP Visa|2026-06-14 00:00:00.0000000|São Vicente|105.71|1|Supermercado
XP Visa|2026-06-14 00:00:00.0000000|Apple|5.9|1|Serviços
XP Visa|2026-06-14 00:00:00.0000000|Abastecimento [Tracker]|172.33|1|Abastecimento
XP Visa|2026-06-15 00:00:00.0000000|Figurinha da Copa|49|1|Outros
XP Visa|2026-06-15 00:00:00.0000000|Energético|29.3|1|Restaurante
XP Visa|2026-06-16 00:00:00.0000000|Porto Mercadinho|49.9|1|Supermercado
XP Visa|2026-06-16 00:00:00.0000000|Gemini|12.5|1|Serviços
XP Visa|2026-06-18 00:00:00.0000000|YouTube [PasquaDev]|23.99|1|Streamings
XP Visa|2026-06-18 00:00:00.0000000|Wellhub|36.7|1|Academia
XP Visa|2026-06-21 00:00:00.0000000|Bom Demais [Mistura]|289.67|1|Supermercado
XP Visa|2026-06-21 00:00:00.0000000|Varejão Paraná|157.28|1|Supermercado
XP Visa|2026-06-21 00:00:00.0000000|Armazém Paraná|32.88|1|Hortifruti
XP Visa|2026-06-22 00:00:00.0000000|Nasoar [Farmárcia]|113.18|1|Saúde
XP Visa|2026-06-23 00:00:00.0000000|Mercadinho Porto|45.37|1|Supermercado
XP Visa|2026-06-27 00:00:00.0000000|Água de Coco|11|1|Feira [Alimentação]
XP Visa|2026-06-27 00:00:00.0000000|Pastel Do Guri|17|1|Feira [Alimentação]
XP Visa|2026-06-27 00:00:00.0000000|Mercado Bom Demais|73.37|1|Supermercado
XP Visa|2026-06-27 00:00:00.0000000|Pastel Do Guri|13|1|Feira [Alimentação]
XP Visa|2026-06-27 00:00:00.0000000|Mercadinho Porto|62.36|1|Supermercado
XP Visa|2026-06-27 00:00:00.0000000|Abastecimento [Tracker]|224.19|1|Abastecimento
XP Visa|2026-06-27 00:00:00.0000000|Outros|27.95|1|Outros
XP Visa|2026-06-27 00:00:00.0000000|Padaria Do Aurora|8.5|1|Restaurante
XP Visa|2026-06-27 00:00:00.0000000|Mercadinho Porto|26.98|1|Supermercado
XP Visa|2026-06-27 00:00:00.0000000|Pizzaria Bonna Notte|124|1|Restaurante
XP Visa|2026-06-27 00:00:00.0000000|Lavagem [Tracker]|60|1|Lavagem Automotiva
XP Visa|2026-06-27 00:00:00.0000000|Github|55|1|Serviços
XP Visa|2026-06-28 00:00:00.0000000|Casa de Carnes|181.55|1|Supermercado
XP Visa|2026-06-28 00:00:00.0000000|Churros|14|1|Feira [Alimentação]
XP Visa|2026-06-28 00:00:00.0000000|Churrasquinho|50|1|Feira [Alimentação]
XP Visa|2026-06-28 00:00:00.0000000|iFood [Nasoar]|179.37|1|Saúde
XP Visa|2026-06-28 00:00:00.0000000|Doces|70.1|1|Supermercado
XP Visa|2026-06-28 00:00:00.0000000|Minibola Copa Do Mundo Da Fifa 26|95.99|1|Lazer
XP Visa|2026-06-28 00:00:00.0000000|Energético|34|1|Bebidas
XP Visa|2026-06-28 00:00:00.0000000|Doces|36|1|Feira [Alimentação]
XP Visa|2026-06-28 00:00:00.0000000|Mercadinho Porto|23.97|1|Supermercado
XP Visa|2026-06-28 00:00:00.0000000|Tylenol Sisu|24.79|1|Saúde
XP Visa|2026-06-28 00:00:00.0000000|Motel|209|1|Outros
XP Visa|2026-06-29 00:00:00.0000000|Armazém Paraná|60.32|1|Hortifruti
XP Visa|2026-06-30 00:00:00.0000000|Sabão Lavar Louças|124.58|1|Casa
XP Visa|2026-06-30 00:00:00.0000000|Vitaminas|127.7|1|Suplementos
XP Visa|2026-07-01 00:00:00.0000000|Netflix|44.9|1|Streamings
XP Visa|2026-07-01 00:00:00.0000000|Mercadinho [Energético]|29.47|1|Supermercado
XP Visa|2026-07-01 00:00:00.0000000|Armazém Paraná|60.32|1|Hortifruti
XP Visa|2026-07-01 00:00:00.0000000|Corte de cabelo|77|1|Serviços
XP Visa|2026-07-01 00:00:00.0000000|Microsoft Azure|76.45|1|Serviços
XP Visa|2026-07-01 00:00:00.0000000|Oficial Farma|194.7|1|Suplementos
XP Visa|2026-07-01 00:00:00.0000000|Pizzaria Bonna Notte|131|1|Prazeres
XP Visa|2026-07-01 00:00:00.0000000|Bolo [Festa Junina JJ]|26|1|Lazer
XP Visa|2026-07-01 00:00:00.0000000|Academia [JJ + Stefany]|254.83|1|Academia
XP Visa|2026-07-01 00:00:00.0000000|Mala de Viagem|198.39|1|Viagem
XP Visa|2026-07-02 00:00:00.0000000|Oral-B Refil|70|1|Saúde
XP Visa|2026-07-02 00:00:00.0000000|Lavagem [Tracker]|60|1|Lavagem Automotiva
XP Visa|2026-07-02 00:00:00.0000000|Mordedor Helena|25.8|1|Bebê
XP Visa|2026-07-03 00:00:00.0000000|Espetinho|50|1|Feira [Alimentação]
XP Visa|2026-07-03 00:00:00.0000000|Sorvete|7|1|Feira [Alimentação]
XP Visa|2026-07-03 00:00:00.0000000|Algodão Doce + Pipoca|25|1|Feira [Alimentação]
XP Visa|2026-07-03 00:00:00.0000000|Pamonha|30|1|Feira [Alimentação]
XP Visa|2026-07-03 00:00:00.0000000|Sem Parar|168.99|1|Transporte
XP Visa|2026-07-03 00:00:00.0000000|Livros|60|1|Educação
XP Visa|2026-07-03 00:00:00.0000000|Drogaria São Luis|63.6|1|Saúde
XP Visa|2026-07-03 00:00:00.0000000|Doces|34|1|Feira [Alimentação]
XP Visa|2026-07-03 00:00:00.0000000|Lojas Americanas|54.43|1|Supérfluo
XP Visa|2026-07-04 00:00:00.0000000|Agua de coco|35|1|Feira [Alimentação]
XP Visa|2026-07-04 00:00:00.0000000|Pastel Do Guri|26|1|Feira [Alimentação]
XP Visa|2026-07-05 00:00:00.0000000|Estacionamento|30|1|Transporte
XP Visa|2026-07-07 00:00:00.0000000|Carlinhos [Corte do Tony]|57|1|Serviços
XP Visa|2026-07-07 00:00:00.0000000|Figurinhas da Copa|70|1|Lazer
XP Visa|2026-07-07 00:00:00.0000000|Mercadinho [Almoço]|59.59|1|Supermercado
XP Visa|2026-07-07 00:00:00.0000000|Mercadinho|27.95|1|Supermercado
XP Visa|2026-07-08 00:00:00.0000000|São Vicente|82.02|1|Supermercado
XP Visa|2026-07-08 00:00:00.0000000|Figurinhas|77.99|1|Outros
XP Visa|2026-07-08 00:00:00.0000000|Suplementos|52.55|1|Suplementos
XP Visa|2026-07-09 00:00:00.0000000|Compras do mês|551.68|1|Supermercado
XP Visa|2026-07-09 00:00:00.0000000|Microsoft Azure|70.55|1|Serviços
XP Visa|2026-07-10 00:00:00.0000000|Chopp|30|1|Feira [Alimentação]
XP Visa|2026-07-10 00:00:00.0000000|Churros|14|1|Feira [Alimentação]
XP Visa|2026-07-10 00:00:00.0000000|Hb Solutions [Chaveiro]|15|1|Outros
XP Visa|2026-07-10 00:00:00.0000000|Suplementos [Pré Treino]|89.9|1|Suplementos
XP Visa|2026-07-10 00:00:00.0000000|Doces|31|1|Feira [Alimentação]
XP Visa|2026-07-10 00:00:00.0000000|Ração do Peixe|19.9|1|Supermercado
XP Visa|2026-07-11 00:00:00.0000000|Pão|36.47|1|Supermercado
XP Visa|2026-07-11 00:00:00.0000000|Abastecimento [Tracker]|237.11|1|Abastecimento
XP Visa|2026-07-11 00:00:00.0000000|Armarinhos Fernandes|94.19|1|Bebê
XP Visa|2026-07-11 00:00:00.0000000|Pharma Nutry|32.5|1|Suplementos
XP Visa|2026-07-11 00:00:00.0000000|Madero|144|1|Restaurante
XP Visa|2026-07-11 00:00:00.0000000|McDonald's|38|1|Restaurante
XP Visa|2026-07-12 00:00:00.0000000|Doces|36|1|Feira [Alimentação]
XP Visa|2026-07-12 00:00:00.0000000|Energético|36.79|1|Supérfluo
XP Visa|2026-07-12 00:00:00.0000000|Maquininha|8|1|Outros
XP Visa|2026-07-13 00:00:00.0000000|Estacionamento|45|1|Transporte
XP Visa|2026-07-13 00:00:00.0000000|Doces Santa Rita|23.2|1|Outros
XP Visa|2026-07-13 00:00:00.0000000|Cookies|18|1|Supérfluo
XP Visa|2026-07-13 00:00:00.0000000|Almoço|64|1|Restaurante
XP Visa|2026-07-13 00:00:00.0000000|Doces|13.9|1|Supermercado
XP Visa|2026-07-13 00:00:00.0000000|Doces|5.9|1|Restaurante
XP Visa|2026-07-14 00:00:00.0000000|Compras do mês|340.64|1|Supermercado
XP Visa|2026-07-14 00:00:00.0000000|Apple|5.9|1|Serviços
XP Visa|2026-07-16 00:00:00.0000000|Google|12.5|1|Serviços
XP Visa|2026-07-16 00:00:00.0000000|Almoço|39.89|1|Restaurante
XP Visa|2026-07-17 00:00:00.0000000|Lacake|15|1|Feira [Alimentação]
XP Visa|2026-07-17 00:00:00.0000000|Abastecimento [Tracker]|127.21|1|Abastecimento
XP Visa|2026-07-17 00:00:00.0000000|Github|27.01|1|Serviços
XP Visa|2026-07-18 00:00:00.0000000|Anthropic|110|1|Serviços
XP Visa|2026-07-18 00:00:00.0000000|Nasoar|111.05|1|Saúde
XP Visa|2026-07-27 00:00:00.0000000|Github|69.65|1|Serviços
XP Visa|2026-07-27 00:00:00.0000000|Kimi [Moonshot AI]|100.54|1|Serviços
XP Visa|2026-07-27 00:00:00.0000000|Porto Mercadinho|47|1|Supermercado
XP Visa|2026-07-27 00:00:00.0000000|Bom demais [Proteínas]|194.23|1|Supermercado
XP Visa|2026-07-27 00:00:00.0000000|Leve Mais|79.88|1|Supermercado
XP Visa|2026-07-27 00:00:00.0000000|Whey|229.9|1|Suplementos
XP Visa|2026-07-27 00:00:00.0000000|Varejão Paraná|121.66|1|Supermercado
XP Visa|2026-07-27 00:00:00.0000000|Whey [Tony]|338.92|1|Suplementos
XP Visa|2026-07-27 00:00:00.0000000|Chatgpt|99.9|1|Serviços
XP Visa|2026-07-27 00:00:00.0000000|Armazém Paraná|47.71|1|Hortifruti
XP Visa|2026-07-28 00:00:00.0000000|Nasoar|178.08|1|Saúde
XP Visa|2026-07-28 00:00:00.0000000|Energético|42.49|1|Supermercado (Supérfluo)
XP Visa|2026-07-28 00:00:00.0000000|Moonshot AI|207.66|1|Serviços

--- Category (catálogo completo, 30 categorias ativas) ---
Eletrônicos, Bebidas, Lazer, Casa, Educação, Restaurante, Saúde, Serviços,
Supermercado, Transporte, Vestuário, Viagem, Amortização, Bebê, Streamings,
Feira [Alimentação], Supermercado (Supérfluo), Hortifruti, Abastecimento,
Lavagem Automotiva, Estacionamento, Academia, Suplementos, Supérfluo,
Presentes, Conhecimento, Prazeres, Conforto, Custos fixos, Outros

--- JOIN Category + Invoice + InvoiceItem (jun/jul 2026) — 136/136 linhas com CategoryId resolvido (0 nulas) ---
--- Colunas: CategoryName|Tags|CardName|PurchaseDate|Description|TotalAmount ---
--- NOTA: Tags (Custos fixos/Conforto/Prazeres) é dimensão de classificação
    orçamentária (60/40) do legado, INDEPENDENTE de Category — a mesma
    Category aparece com Tags diferentes (ex.: Supermercado tem os 3
    valores). NÃO é subcategoria: usar CategoryName como categoria real,
    Tags só como contexto de classificação, nunca como categoria alternativa. ---
Abastecimento|Custos fixos|XP Visa|2026-06-13 00:00:00.0000000|Abastecimento [Tracker]|293.15
Abastecimento|Custos fixos|XP Visa|2026-06-14 00:00:00.0000000|Abastecimento [Tracker]|172.33
Abastecimento|Custos fixos|XP Visa|2026-06-27 00:00:00.0000000|Abastecimento [Tracker]|224.19
Abastecimento|Custos fixos|XP Visa|2026-07-11 00:00:00.0000000|Abastecimento [Tracker]|237.11
Abastecimento|Custos fixos|XP Visa|2026-07-17 00:00:00.0000000|Abastecimento [Tracker]|127.21
Academia|Custos fixos|XP Visa|2026-06-01 00:00:00.0000000|Busterfit|254.83
Academia|Custos fixos|XP Visa|2026-06-18 00:00:00.0000000|Wellhub|36.7
Academia|Custos fixos|XP Visa|2026-07-01 00:00:00.0000000|Academia [JJ + Stefany]|254.83
Bebê|Conforto|XP Visa|2026-07-02 00:00:00.0000000|Mordedor Helena|25.8
Bebê|Conforto|XP Visa|2026-07-11 00:00:00.0000000|Armarinhos Fernandes|94.19
Bebidas|Conforto|XP Visa|2026-06-28 00:00:00.0000000|Energético|34
Casa|Custos fixos|XP Visa|2026-06-07 00:00:00.0000000|Itens da casa [Banheiro]|119.99
Casa|Custos fixos|XP Visa|2026-06-10 00:00:00.0000000|Itens da Casa|24.8
Casa|Custos fixos|XP Visa|2026-06-30 00:00:00.0000000|Sabão Lavar Louças|124.58
Educação|Conforto|XP Visa|2026-07-03 00:00:00.0000000|Livros|60
Feira [Alimentação]|Prazeres|XP Visa|2026-06-27 00:00:00.0000000|Água de Coco|11
Feira [Alimentação]|Prazeres|XP Visa|2026-06-27 00:00:00.0000000|Pastel Do Guri|17
Feira [Alimentação]|Prazeres|XP Visa|2026-06-27 00:00:00.0000000|Pastel Do Guri|13
Feira [Alimentação]|Prazeres|XP Visa|2026-06-28 00:00:00.0000000|Doces|36
Feira [Alimentação]|Prazeres|XP Visa|2026-06-28 00:00:00.0000000|Churros|14
Feira [Alimentação]|Prazeres|XP Visa|2026-06-28 00:00:00.0000000|Churrasquinho|50
Feira [Alimentação]|Prazeres|XP Visa|2026-07-03 00:00:00.0000000|Pamonha|30
Feira [Alimentação]|Prazeres|XP Visa|2026-07-03 00:00:00.0000000|Doces|34
Feira [Alimentação]|Prazeres|XP Visa|2026-07-03 00:00:00.0000000|Espetinho|50
Feira [Alimentação]|Prazeres|XP Visa|2026-07-03 00:00:00.0000000|Sorvete|7
Feira [Alimentação]|Prazeres|XP Visa|2026-07-03 00:00:00.0000000|Algodão Doce + Pipoca|25
Feira [Alimentação]|Prazeres|XP Visa|2026-07-04 00:00:00.0000000|Pastel Do Guri|26
Feira [Alimentação]|Prazeres|XP Visa|2026-07-04 00:00:00.0000000|Agua de coco|35
Feira [Alimentação]|Prazeres|XP Visa|2026-07-10 00:00:00.0000000|Chopp|30
Feira [Alimentação]|Prazeres|XP Visa|2026-07-10 00:00:00.0000000|Churros|14
Feira [Alimentação]|Prazeres|XP Visa|2026-07-10 00:00:00.0000000|Doces|31
Feira [Alimentação]|Prazeres|XP Visa|2026-07-12 00:00:00.0000000|Doces|36
Feira [Alimentação]|Conforto|XP Visa|2026-07-17 00:00:00.0000000|Lacake|15
Hortifruti|Custos fixos|XP Visa|2026-06-21 00:00:00.0000000|Armazém Paraná|32.88
Hortifruti|Custos fixos|XP Visa|2026-06-29 00:00:00.0000000|Armazém Paraná|60.32
Hortifruti|Custos fixos|XP Visa|2026-07-01 00:00:00.0000000|Armazém Paraná|60.32
Hortifruti|Custos fixos|XP Visa|2026-07-27 00:00:00.0000000|Armazém Paraná|47.71
Lavagem Automotiva|Conforto|XP Visa|2026-06-27 00:00:00.0000000|Lavagem [Tracker]|60
Lavagem Automotiva|Conforto|XP Visa|2026-07-02 00:00:00.0000000|Lavagem [Tracker]|60
Lazer|Prazeres|XP Visa|2026-06-10 00:00:00.0000000|Ebanx [PSN]|619.9
Lazer|Prazeres|XP Visa|2026-06-28 00:00:00.0000000|Minibola Copa Do Mundo Da Fifa 26|95.99
Lazer|Prazeres|XP Visa|2026-07-01 00:00:00.0000000|Bolo [Festa Junina JJ]|26
Lazer|Prazeres|XP Visa|2026-07-07 00:00:00.0000000|Figurinhas da Copa|70
Outros|Prazeres|XP Visa|2026-06-15 00:00:00.0000000|Figurinha da Copa|49
Outros|Prazeres|XP Visa|2026-06-27 00:00:00.0000000|Outros|27.95
Outros|Prazeres|XP Visa|2026-06-28 00:00:00.0000000|Motel|209
Outros|Conforto|XP Visa|2026-07-08 00:00:00.0000000|Figurinhas|77.99
Outros|Conforto|XP Visa|2026-07-10 00:00:00.0000000|Hb Solutions [Chaveiro]|15
Outros|Prazeres|XP Visa|2026-07-12 00:00:00.0000000|Maquininha|8
Outros|Prazeres|XP Visa|2026-07-13 00:00:00.0000000|Doces Santa Rita|23.2
Prazeres|Prazeres|XP Visa|2026-07-01 00:00:00.0000000|Pizzaria Bonna Notte|131
Restaurante|Prazeres|XP Visa|2026-06-15 00:00:00.0000000|Energético|29.3
Restaurante|Prazeres|XP Visa|2026-06-27 00:00:00.0000000|Pizzaria Bonna Notte|124
Restaurante|Prazeres|XP Visa|2026-06-27 00:00:00.0000000|Padaria Do Aurora|8.5
Restaurante|Prazeres|XP Visa|2026-07-11 00:00:00.0000000|McDonald's|38
Restaurante|Conforto|XP Visa|2026-07-11 00:00:00.0000000|Madero|144
Restaurante|Custos fixos|XP Visa|2026-07-13 00:00:00.0000000|Almoço|64
Restaurante|Prazeres|XP Visa|2026-07-13 00:00:00.0000000|Doces|5.9
Restaurante|Custos fixos|XP Visa|2026-07-16 00:00:00.0000000|Almoço|39.89
Saúde|Custos fixos|XP Visa|2026-06-03 00:00:00.0000000|Drogaria São Luis|98.9
Saúde|Custos fixos|XP Visa|2026-06-08 00:00:00.0000000|Testo Black|108.04
Saúde|Custos fixos|XP Visa|2026-06-10 00:00:00.0000000|Nasoar|133.54
Saúde|Custos fixos|XP Visa|2026-06-22 00:00:00.0000000|Nasoar [Farmárcia]|113.18
Saúde|Custos fixos|XP Visa|2026-06-28 00:00:00.0000000|Tylenol Sisu|24.79
Saúde|Custos fixos|XP Visa|2026-06-28 00:00:00.0000000|iFood [Nasoar]|179.37
Saúde|Custos fixos|XP Visa|2026-07-02 00:00:00.0000000|Oral-B Refil|70
Saúde|Custos fixos|XP Visa|2026-07-03 00:00:00.0000000|Drogaria São Luis|63.6
Saúde|Custos fixos|XP Visa|2026-07-18 00:00:00.0000000|Nasoar|111.05
Saúde|Custos fixos|XP Visa|2026-07-28 00:00:00.0000000|Nasoar|178.08
Serviços|Custos fixos|XP Visa|2026-06-01 00:00:00.0000000|Open.AI|99.9
Serviços|Custos fixos|XP Visa|2026-06-09 00:00:00.0000000|Moonshot [IA]|102.34
Serviços|Custos fixos|XP Visa|2026-06-09 00:00:00.0000000|Moonshot [IA]|107.8
Serviços|Custos fixos|XP Visa|2026-06-09 00:00:00.0000000|Microsoft [Github]|11.63
Serviços|Custos fixos|XP Visa|2026-06-13 00:00:00.0000000|hevy.com|12.9
Serviços|Conforto|XP Visa|2026-06-14 00:00:00.0000000|Apple|5.9
Serviços|Custos fixos|XP Visa|2026-06-16 00:00:00.0000000|Gemini|12.5
Serviços|Custos fixos|XP Visa|2026-06-27 00:00:00.0000000|Github|55
Serviços|Custos fixos|XP Visa|2026-07-01 00:00:00.0000000|Corte de cabelo|77
Serviços|Custos fixos|XP Visa|2026-07-01 00:00:00.0000000|Microsoft Azure|76.45
Serviços|Custos fixos|XP Visa|2026-07-07 00:00:00.0000000|Carlinhos [Corte do Tony]|57
Serviços|Custos fixos|XP Visa|2026-07-09 00:00:00.0000000|Microsoft Azure|70.55
Serviços|Custos fixos|XP Visa|2026-07-14 00:00:00.0000000|Apple|5.9
Serviços|Custos fixos|XP Visa|2026-07-16 00:00:00.0000000|Google|12.5
Serviços|Custos fixos|XP Visa|2026-07-17 00:00:00.0000000|Github|27.01
Serviços|Custos fixos|XP Visa|2026-07-18 00:00:00.0000000|Anthropic|110
Serviços|Custos fixos|XP Visa|2026-07-27 00:00:00.0000000|Github|69.65
Serviços|Custos fixos|XP Visa|2026-07-27 00:00:00.0000000|Kimi [Moonshot AI]|100.54
Serviços|Custos fixos|XP Visa|2026-07-27 00:00:00.0000000|Chatgpt|99.9
Serviços|Custos fixos|XP Visa|2026-07-28 00:00:00.0000000|Moonshot AI|207.66
Streamings|Conforto|XP Visa|2026-06-01 00:00:00.0000000|Netflix|44.9
Streamings|Conforto|XP Visa|2026-06-04 00:00:00.0000000|Amazon [Prime Video]|39.41
Streamings|Conforto|XP Visa|2026-06-18 00:00:00.0000000|YouTube [PasquaDev]|23.99
Streamings|Conforto|XP Visa|2026-07-01 00:00:00.0000000|Netflix|44.9
Supérfluo|Prazeres|XP Visa|2026-07-03 00:00:00.0000000|Lojas Americanas|54.43
Supérfluo|Conforto|XP Visa|2026-07-12 00:00:00.0000000|Energético|36.79
Supérfluo|Prazeres|XP Visa|2026-07-13 00:00:00.0000000|Cookies|18
Supermercado|Custos fixos|XP Visa|2026-06-05 00:00:00.0000000|Compras do mês [Atacadão]|1002.29
Supermercado|Custos fixos|XP Visa|2026-06-14 00:00:00.0000000|São Vicente|105.71
Supermercado|Custos fixos|XP Visa|2026-06-16 00:00:00.0000000|Porto Mercadinho|49.9
Supermercado|Custos fixos|XP Visa|2026-06-21 00:00:00.0000000|Bom Demais [Mistura]|289.67
Supermercado|Custos fixos|XP Visa|2026-06-21 00:00:00.0000000|Varejão Paraná|157.28
Supermercado|Custos fixos|XP Visa|2026-06-23 00:00:00.0000000|Mercadinho Porto|45.37
Supermercado|Custos fixos|XP Visa|2026-06-27 00:00:00.0000000|Mercado Bom Demais|73.37
Supermercado|Custos fixos|XP Visa|2026-06-27 00:00:00.0000000|Mercadinho Porto|62.36
Supermercado|Custos fixos|XP Visa|2026-06-27 00:00:00.0000000|Mercadinho Porto|26.98
Supermercado|Prazeres|XP Visa|2026-06-28 00:00:00.0000000|Casa de Carnes|181.55
Supermercado|Prazeres|XP Visa|2026-06-28 00:00:00.0000000|Doces|70.1
Supermercado|Custos fixos|XP Visa|2026-06-28 00:00:00.0000000|Mercadinho Porto|23.97
Supermercado|Prazeres|XP Visa|2026-07-01 00:00:00.0000000|Mercadinho [Energético]|29.47
Supermercado|Conforto|XP Visa|2026-07-07 00:00:00.0000000|Mercadinho|27.95
Supermercado|Conforto|XP Visa|2026-07-07 00:00:00.0000000|Mercadinho [Almoço]|59.59
Supermercado|Custos fixos|XP Visa|2026-07-08 00:00:00.0000000|São Vicente|82.02
Supermercado|Custos fixos|XP Visa|2026-07-09 00:00:00.0000000|Compras do mês|551.68
Supermercado|Custos fixos|XP Visa|2026-07-10 00:00:00.0000000|Ração do Peixe|19.9
Supermercado|Custos fixos|XP Visa|2026-07-11 00:00:00.0000000|Pão|36.47
Supermercado|Prazeres|XP Visa|2026-07-13 00:00:00.0000000|Doces|13.9
Supermercado|Custos fixos|XP Visa|2026-07-14 00:00:00.0000000|Compras do mês|340.64
Supermercado|Custos fixos|XP Visa|2026-07-27 00:00:00.0000000|Leve Mais|79.88
Supermercado|Custos fixos|XP Visa|2026-07-27 00:00:00.0000000|Varejão Paraná|121.66
Supermercado|Custos fixos|XP Visa|2026-07-27 00:00:00.0000000|Porto Mercadinho|47
Supermercado|Custos fixos|XP Visa|2026-07-27 00:00:00.0000000|Bom demais [Proteínas]|194.23
Supermercado (Supérfluo)|Prazeres|XP Visa|2026-07-28 00:00:00.0000000|Energético|42.49
Suplementos|Custos fixos|XP Visa|2026-06-11 00:00:00.0000000|Whey|194.9
Suplementos|Custos fixos|XP Visa|2026-06-30 00:00:00.0000000|Vitaminas|127.7
Suplementos|Custos fixos|XP Visa|2026-07-01 00:00:00.0000000|Oficial Farma|194.7
Suplementos|Custos fixos|XP Visa|2026-07-08 00:00:00.0000000|Suplementos|52.55
Suplementos|Custos fixos|XP Visa|2026-07-10 00:00:00.0000000|Suplementos [Pré Treino]|89.9
Suplementos|Prazeres|XP Visa|2026-07-11 00:00:00.0000000|Pharma Nutry|32.5
Suplementos|Custos fixos|XP Visa|2026-07-27 00:00:00.0000000|Whey|229.9
Suplementos|Custos fixos|XP Visa|2026-07-27 00:00:00.0000000|Whey [Tony]|338.92
Transporte|Custos fixos|XP Visa|2026-06-03 00:00:00.0000000|Sem Parar|215.75
Transporte|Custos fixos|XP Visa|2026-06-03 00:00:00.0000000|Zona Azul [Barueri]|10
Transporte|Custos fixos|XP Visa|2026-07-03 00:00:00.0000000|Sem Parar|168.99
Transporte|Custos fixos|XP Visa|2026-07-05 00:00:00.0000000|Estacionamento|30
Transporte|Custos fixos|XP Visa|2026-07-13 00:00:00.0000000|Estacionamento|45
Viagem|Conforto|XP Visa|2026-07-01 00:00:00.0000000|Mala de Viagem|198.39

```

---

## 2. Todos os lançamentos reais encontrados

### Todos os lançamentos reais encontrados — FinancialControlDB (jun/jul 2026)

Fonte: `sqlcmd` em `SQL5053.site4now.net` (`DB_A453C8_FinancialControl`).
Extraído em 2026-07-29. Listagem completa, um lançamento por linha — nada
resumido, nada inventado. Query bruta em
`docs/runs/29-07-2026-dados-reais-financialcontroldb.txt`.

#### Contagens

| Tabela | Linhas em jun/jul 2026 |
|---|---|
| `[Transaction]` + `TransactionItem` | **0** (tabela legada; `MAX(Date)` = 2025-12-01 — sem dados de 2026, app já migrou para o Postgres do mecontrola) |
| `Bill` + `BillItem` | 10 |
| `Invoice` + `InvoiceItem` | 136 |
| **Total de lançamentos listados abaixo** | **146** |

#### Invoice + InvoiceItem — 136 lançamentos (cartão de crédito)

Join: `Invoice i INNER JOIN InvoiceItem ii ON i.Id = ii.InvoiceId INNER JOIN Category c ON ii.CategoryId = c.Id LEFT JOIN Card cd ON i.CardId = cd.Id`.
`CategoryId` resolvido em 136/136 linhas (0 nulas) — categoria vem sempre do Id, nunca inferida por nome.
`Tags` é dimensão independente de classificação orçamentária (60/40 do legado: Custos fixos / Conforto / Prazeres), **não** é subcategoria.

| # | Data | Descrição | Valor | Categoria | Tags | Cartão | InvoiceItem.Id |
|---|---|---|---|---|---|---|---|
| 1 | 2026-06-01 | Busterfit | R$ 254,83 | Academia | Custos fixos | XP Visa | `d958550a-f32c-4aa8-89b7-8f8e68f519ed` |
| 2 | 2026-06-01 | Netflix | R$ 44,90 | Streamings | Conforto | XP Visa | `3907eedc-e23a-4f6c-b4df-15196a704528` |
| 3 | 2026-06-01 | Open.AI | R$ 99,90 | Serviços | Custos fixos | XP Visa | `19e5fb60-8549-4b07-a0f6-5a0e7c8a589a` |
| 4 | 2026-06-03 | Drogaria São Luis | R$ 98,90 | Saúde | Custos fixos | XP Visa | `a5cecbf8-a3ea-49a1-8676-d333b3a46710` |
| 5 | 2026-06-03 | Sem Parar | R$ 215,75 | Transporte | Custos fixos | XP Visa | `aed5f24b-0778-4f37-b16e-4558bd823871` |
| 6 | 2026-06-03 | Zona Azul [Barueri] | R$ 10,00 | Transporte | Custos fixos | XP Visa | `398ba6e4-9e88-4099-9a88-edb3893bcb46` |
| 7 | 2026-06-04 | Amazon [Prime Video] | R$ 39,41 | Streamings | Conforto | XP Visa | `ccf219a4-a2f9-4139-a3d6-0a91c78f7054` |
| 8 | 2026-06-05 | Compras do mês [Atacadão] | R$ 1.002,29 | Supermercado | Custos fixos | XP Visa | `ffb2a254-63c5-41fc-880e-9afa6e2d6814` |
| 9 | 2026-06-07 | Itens da casa [Banheiro] | R$ 119,99 | Casa | Custos fixos | XP Visa | `44cfd4d0-5d4a-40f6-865e-0ad018025f3b` |
| 10 | 2026-06-08 | Testo Black | R$ 108,04 | Saúde | Custos fixos | XP Visa | `4a00e28b-6d6c-4105-aa24-644435df9607` |
| 11 | 2026-06-09 | Microsoft [Github] | R$ 11,63 | Serviços | Custos fixos | XP Visa | `abb8924e-246b-4db9-b61c-969edf5920f9` |
| 12 | 2026-06-09 | Moonshot [IA] | R$ 107,80 | Serviços | Custos fixos | XP Visa | `92dd5b07-2d6c-463d-9198-aa12f595e35b` |
| 13 | 2026-06-09 | Moonshot [IA] | R$ 102,34 | Serviços | Custos fixos | XP Visa | `45192b12-f739-4ed9-b986-33bc4e02ee45` |
| 14 | 2026-06-10 | Ebanx [PSN] | R$ 619,90 | Lazer | Prazeres | XP Visa | `c2450982-a4e5-4b59-9aaa-a8369520edb4` |
| 15 | 2026-06-10 | Itens da Casa | R$ 24,80 | Casa | Custos fixos | XP Visa | `1b1aab0c-39d4-43f5-af12-de58fa01c890` |
| 16 | 2026-06-10 | Nasoar | R$ 133,54 | Saúde | Custos fixos | XP Visa | `76b98594-2f7f-4bfa-89c0-ebbfec569c31` |
| 17 | 2026-06-11 | Whey | R$ 194,90 | Suplementos | Custos fixos | XP Visa | `8fee23ed-4b4b-45a6-a46b-009dd2544a7e` |
| 18 | 2026-06-13 | Abastecimento [Tracker] | R$ 293,15 | Abastecimento | Custos fixos | XP Visa | `f8d52877-5f23-426a-a1e7-49da05092518` |
| 19 | 2026-06-13 | hevy.com | R$ 12,90 | Serviços | Custos fixos | XP Visa | `d52453eb-94f6-4210-9503-7d1c58d50b21` |
| 20 | 2026-06-14 | Abastecimento [Tracker] | R$ 172,33 | Abastecimento | Custos fixos | XP Visa | `a3bc789d-aee0-4418-95e6-26025284b554` |
| 21 | 2026-06-14 | Apple | R$ 5,90 | Serviços | Conforto | XP Visa | `127a051c-229d-409f-86b9-52ab988edc69` |
| 22 | 2026-06-14 | São Vicente | R$ 105,71 | Supermercado | Custos fixos | XP Visa | `15228836-8237-4a83-a584-3ff1967b6ac5` |
| 23 | 2026-06-15 | Energético | R$ 29,30 | Restaurante | Prazeres | XP Visa | `4757edc3-953a-4eb6-bdd1-94dcd15436a5` |
| 24 | 2026-06-15 | Figurinha da Copa | R$ 49,00 | Outros | Prazeres | XP Visa | `57376c2f-df30-4a17-8798-470822b32bb2` |
| 25 | 2026-06-16 | Gemini | R$ 12,50 | Serviços | Custos fixos | XP Visa | `b41d7e81-2013-45ea-a079-2aaa25a2502b` |
| 26 | 2026-06-16 | Porto Mercadinho | R$ 49,90 | Supermercado | Custos fixos | XP Visa | `2a3c0c3f-5d23-4e26-a618-c7837626b76a` |
| 27 | 2026-06-18 | Wellhub | R$ 36,70 | Academia | Custos fixos | XP Visa | `6dfba80e-d742-494e-bef9-9f738971696a` |
| 28 | 2026-06-18 | YouTube [PasquaDev] | R$ 23,99 | Streamings | Conforto | XP Visa | `3e200e08-eb76-4d2e-80fd-3117d73f106f` |
| 29 | 2026-06-21 | Armazém Paraná | R$ 32,88 | Hortifruti | Custos fixos | XP Visa | `f2e06945-7bce-4851-bda8-695d799c5e9a` |
| 30 | 2026-06-21 | Bom Demais [Mistura] | R$ 289,67 | Supermercado | Custos fixos | XP Visa | `6b496936-4301-40f2-9d80-8889b67838df` |
| 31 | 2026-06-21 | Varejão Paraná | R$ 157,28 | Supermercado | Custos fixos | XP Visa | `d2ed3cc8-a597-4b99-9902-cbb04dbe29e6` |
| 32 | 2026-06-22 | Nasoar [Farmárcia] | R$ 113,18 | Saúde | Custos fixos | XP Visa | `60957061-a806-4f76-8b09-cb4ce9a8a2fc` |
| 33 | 2026-06-23 | Mercadinho Porto | R$ 45,37 | Supermercado | Custos fixos | XP Visa | `9e7eb45a-89a5-474f-8c10-eef6680a0f1e` |
| 34 | 2026-06-27 | Abastecimento [Tracker] | R$ 224,19 | Abastecimento | Custos fixos | XP Visa | `efa35074-307e-429a-928e-44b414b1564e` |
| 35 | 2026-06-27 | Água de Coco | R$ 11,00 | Feira [Alimentação] | Prazeres | XP Visa | `d16cb2ba-99bd-4189-98c7-f7366f9fccb4` |
| 36 | 2026-06-27 | Github | R$ 55,00 | Serviços | Custos fixos | XP Visa | `c4ababc2-2c7a-462c-9ea3-14a7750a88cb` |
| 37 | 2026-06-27 | Lavagem [Tracker] | R$ 60,00 | Lavagem Automotiva | Conforto | XP Visa | `1c91a532-e1ff-4c4a-841c-0aef87437c93` |
| 38 | 2026-06-27 | Mercadinho Porto | R$ 26,98 | Supermercado | Custos fixos | XP Visa | `85ae8027-bfae-485f-aa1e-3271fd456972` |
| 39 | 2026-06-27 | Mercadinho Porto | R$ 62,36 | Supermercado | Custos fixos | XP Visa | `6810fba9-6d78-48cb-b9fa-9b8cb4a98455` |
| 40 | 2026-06-27 | Mercado Bom Demais | R$ 73,37 | Supermercado | Custos fixos | XP Visa | `d8161f88-eec6-4c1b-9bd8-d8939671dbe7` |
| 41 | 2026-06-27 | Outros | R$ 27,95 | Outros | Prazeres | XP Visa | `f26d08df-d39a-48f8-982a-6163b368dd50` |
| 42 | 2026-06-27 | Padaria Do Aurora | R$ 8,50 | Restaurante | Prazeres | XP Visa | `b2f51ac9-7fa7-4611-ab0d-472a0804556c` |
| 43 | 2026-06-27 | Pastel Do Guri | R$ 17,00 | Feira [Alimentação] | Prazeres | XP Visa | `568fda34-3e10-4c24-abde-fb7095bedd7e` |
| 44 | 2026-06-27 | Pastel Do Guri | R$ 13,00 | Feira [Alimentação] | Prazeres | XP Visa | `22ebc94b-4757-491b-af30-9288792278fb` |
| 45 | 2026-06-27 | Pizzaria Bonna Notte | R$ 124,00 | Restaurante | Prazeres | XP Visa | `9ebdbda6-26dc-4e70-bcd6-2efe9e7057dc` |
| 46 | 2026-06-28 | Casa de Carnes | R$ 181,55 | Supermercado | Prazeres | XP Visa | `51e81e93-bd7c-4dd1-beb1-12b26a85a6e6` |
| 47 | 2026-06-28 | Churrasquinho | R$ 50,00 | Feira [Alimentação] | Prazeres | XP Visa | `18c7d32b-d6ca-4961-bdd6-3d8b0bc5272b` |
| 48 | 2026-06-28 | Churros | R$ 14,00 | Feira [Alimentação] | Prazeres | XP Visa | `c3052670-e73a-488d-b940-2f9e6b0fbc07` |
| 49 | 2026-06-28 | Doces | R$ 70,10 | Supermercado | Prazeres | XP Visa | `32552a90-d0c6-4118-adf4-5c563535a515` |
| 50 | 2026-06-28 | Doces | R$ 36,00 | Feira [Alimentação] | Prazeres | XP Visa | `64616063-a8e9-4467-8a06-ba7770d6eb6b` |
| 51 | 2026-06-28 | Energético | R$ 34,00 | Bebidas | Conforto | XP Visa | `249ce500-75f6-48de-b5e9-a3514848c148` |
| 52 | 2026-06-28 | iFood [Nasoar] | R$ 179,37 | Saúde | Custos fixos | XP Visa | `be1531be-f531-4364-8990-62cf4d7ab869` |
| 53 | 2026-06-28 | Mercadinho Porto | R$ 23,97 | Supermercado | Custos fixos | XP Visa | `7f647a0c-2b2b-4d9b-8658-dbdc4e75e4c1` |
| 54 | 2026-06-28 | Minibola Copa Do Mundo Da Fifa 26 | R$ 95,99 | Lazer | Prazeres | XP Visa | `5ac0c0e7-7ab4-4aa4-ac14-73384e93b4a3` |
| 55 | 2026-06-28 | Motel | R$ 209,00 | Outros | Prazeres | XP Visa | `2d3f03e3-20bd-4b40-922f-f25feff5dc2b` |
| 56 | 2026-06-28 | Tylenol Sisu | R$ 24,79 | Saúde | Custos fixos | XP Visa | `7ccbbc53-2ed7-460b-a642-c223dfebcc50` |
| 57 | 2026-06-29 | Armazém Paraná | R$ 60,32 | Hortifruti | Custos fixos | XP Visa | `36a7cbc9-d14c-448b-a5d4-2a719a9a172a` |
| 58 | 2026-06-30 | Sabão Lavar Louças | R$ 124,58 | Casa | Custos fixos | XP Visa | `ba7c2483-10a4-4cb1-8646-52a93b6a7f96` |
| 59 | 2026-06-30 | Vitaminas | R$ 127,70 | Suplementos | Custos fixos | XP Visa | `f0197acb-7e81-4f25-9f92-c0adafc5426e` |
| 60 | 2026-07-01 | Academia [JJ + Stefany] | R$ 254,83 | Academia | Custos fixos | XP Visa | `cab46d7d-f077-46fc-86a0-419abeb04d6a` |
| 61 | 2026-07-01 | Armazém Paraná | R$ 60,32 | Hortifruti | Custos fixos | XP Visa | `0a1539fb-5101-4b60-96ae-a13a86f035fe` |
| 62 | 2026-07-01 | Bolo [Festa Junina JJ] | R$ 26,00 | Lazer | Prazeres | XP Visa | `900bc691-d186-4ce7-8da6-375987c6633f` |
| 63 | 2026-07-01 | Corte de cabelo | R$ 77,00 | Serviços | Custos fixos | XP Visa | `707dbe87-a9d4-41a0-a044-9357fe83893d` |
| 64 | 2026-07-01 | Mala de Viagem | R$ 198,39 | Viagem | Conforto | XP Visa | `e4fa5d08-c984-4b1c-b93c-028948462a8e` |
| 65 | 2026-07-01 | Mercadinho [Energético] | R$ 29,47 | Supermercado | Prazeres | XP Visa | `a18a473d-5cca-40a8-b460-9f755417c7e1` |
| 66 | 2026-07-01 | Microsoft Azure | R$ 76,45 | Serviços | Custos fixos | XP Visa | `8eeca181-9002-464e-ad18-83cc85c054fd` |
| 67 | 2026-07-01 | Netflix | R$ 44,90 | Streamings | Conforto | XP Visa | `7b074100-6a8e-48c9-9cbc-aa42dc5c782f` |
| 68 | 2026-07-01 | Oficial Farma | R$ 194,70 | Suplementos | Custos fixos | XP Visa | `1961faa6-4e0c-4d2c-bb2d-84851d9ac366` |
| 69 | 2026-07-01 | Pizzaria Bonna Notte | R$ 131,00 | Prazeres | Prazeres | XP Visa | `ed47a860-5c29-45e9-a823-f35b9e7050af` |
| 70 | 2026-07-02 | Lavagem [Tracker] | R$ 60,00 | Lavagem Automotiva | Conforto | XP Visa | `6191a5a9-ac7d-40b1-942a-92831adf59ae` |
| 71 | 2026-07-02 | Mordedor Helena | R$ 25,80 | Bebê | Conforto | XP Visa | `6967c864-4b11-489b-a420-79bcc0d25fe0` |
| 72 | 2026-07-02 | Oral-B Refil | R$ 70,00 | Saúde | Custos fixos | XP Visa | `18fdafe3-9654-4ee4-b506-60f177456592` |
| 73 | 2026-07-03 | Algodão Doce + Pipoca | R$ 25,00 | Feira [Alimentação] | Prazeres | XP Visa | `07a45df1-cc04-4cb8-9f9b-d580644f9345` |
| 74 | 2026-07-03 | Doces | R$ 34,00 | Feira [Alimentação] | Prazeres | XP Visa | `e156b4fe-2c99-4169-ac2c-2c39d9683834` |
| 75 | 2026-07-03 | Drogaria São Luis | R$ 63,60 | Saúde | Custos fixos | XP Visa | `6dda0394-6d2c-4eb4-acd2-365358dff9b4` |
| 76 | 2026-07-03 | Espetinho | R$ 50,00 | Feira [Alimentação] | Prazeres | XP Visa | `120d96fb-2af5-4136-a8f1-a20b23a615bc` |
| 77 | 2026-07-03 | Livros | R$ 60,00 | Educação | Conforto | XP Visa | `8153343e-2c95-4780-b2bc-1770c41658a6` |
| 78 | 2026-07-03 | Lojas Americanas | R$ 54,43 | Supérfluo | Prazeres | XP Visa | `d8719b5d-132e-4fcf-8822-2d87bba56a48` |
| 79 | 2026-07-03 | Pamonha | R$ 30,00 | Feira [Alimentação] | Prazeres | XP Visa | `fdd9496a-935c-4fb8-aa74-4c4c804ef44a` |
| 80 | 2026-07-03 | Sem Parar | R$ 168,99 | Transporte | Custos fixos | XP Visa | `96b24f9e-38f4-40a0-b79b-1130616a13aa` |
| 81 | 2026-07-03 | Sorvete | R$ 7,00 | Feira [Alimentação] | Prazeres | XP Visa | `a06d4110-d835-4f0b-b271-f3f0a72b1bf8` |
| 82 | 2026-07-04 | Agua de coco | R$ 35,00 | Feira [Alimentação] | Prazeres | XP Visa | `7fda96e1-4925-46bb-8da0-6565b4e3225a` |
| 83 | 2026-07-04 | Pastel Do Guri | R$ 26,00 | Feira [Alimentação] | Prazeres | XP Visa | `d0d1939d-bf46-4f76-80b0-86e72c78cc36` |
| 84 | 2026-07-05 | Estacionamento | R$ 30,00 | Transporte | Custos fixos | XP Visa | `559cb145-8db0-42ed-a465-7e1c8c2fefc7` |
| 85 | 2026-07-07 | Carlinhos [Corte do Tony] | R$ 57,00 | Serviços | Custos fixos | XP Visa | `431daa81-a2d4-4b5d-ae50-3e00b6716a14` |
| 86 | 2026-07-07 | Figurinhas da Copa | R$ 70,00 | Lazer | Prazeres | XP Visa | `66be376d-d66f-4b60-842b-2da0cdea1e87` |
| 87 | 2026-07-07 | Mercadinho | R$ 27,95 | Supermercado | Conforto | XP Visa | `0da8f8d9-4c2b-4d94-b3ff-07c0d6e735f7` |
| 88 | 2026-07-07 | Mercadinho [Almoço] | R$ 59,59 | Supermercado | Conforto | XP Visa | `1850e830-f636-4b34-a1b4-1c111f911fab` |
| 89 | 2026-07-08 | Figurinhas | R$ 77,99 | Outros | Conforto | XP Visa | `2f340531-9761-4468-a494-4d6b2881281e` |
| 90 | 2026-07-08 | São Vicente | R$ 82,02 | Supermercado | Custos fixos | XP Visa | `5ac04fe9-3418-4a0d-aab7-041d4f1380d0` |
| 91 | 2026-07-08 | Suplementos | R$ 52,55 | Suplementos | Custos fixos | XP Visa | `02cd1bca-6ebb-4223-af1c-c70d92216e63` |
| 92 | 2026-07-09 | Compras do mês | R$ 551,68 | Supermercado | Custos fixos | XP Visa | `4adfa39b-dd9c-42f1-b807-dd6e46f1964d` |
| 93 | 2026-07-09 | Microsoft Azure | R$ 70,55 | Serviços | Custos fixos | XP Visa | `e296f0cb-fa8b-410b-9a44-5b29cbefbb60` |
| 94 | 2026-07-10 | Chopp | R$ 30,00 | Feira [Alimentação] | Prazeres | XP Visa | `d22099d9-8277-4504-a67e-4e674fdecb38` |
| 95 | 2026-07-10 | Churros | R$ 14,00 | Feira [Alimentação] | Prazeres | XP Visa | `b30dacc4-d859-4b2b-97f7-12d7b879fef4` |
| 96 | 2026-07-10 | Doces | R$ 31,00 | Feira [Alimentação] | Prazeres | XP Visa | `1ec19826-3556-4711-a964-ca00d6ab956e` |
| 97 | 2026-07-10 | Hb Solutions [Chaveiro] | R$ 15,00 | Outros | Conforto | XP Visa | `c53d251e-813e-4c37-ad09-d59e36a8a2ac` |
| 98 | 2026-07-10 | Ração do Peixe | R$ 19,90 | Supermercado | Custos fixos | XP Visa | `c9ff8db6-74a1-4ce8-9407-ef6e0c7a8bc0` |
| 99 | 2026-07-10 | Suplementos [Pré Treino] | R$ 89,90 | Suplementos | Custos fixos | XP Visa | `b53d7ed4-e8fb-4513-aaab-c6c7cf063704` |
| 100 | 2026-07-11 | Abastecimento [Tracker] | R$ 237,11 | Abastecimento | Custos fixos | XP Visa | `785da6b1-f3a9-44f3-8621-cc92889f3ea8` |
| 101 | 2026-07-11 | Armarinhos Fernandes | R$ 94,19 | Bebê | Conforto | XP Visa | `6f7d97ba-f43f-4464-942e-ce329668dd43` |
| 102 | 2026-07-11 | Madero | R$ 144,00 | Restaurante | Conforto | XP Visa | `bb9c7452-282d-4d99-acce-207a43c698f9` |
| 103 | 2026-07-11 | McDonald's | R$ 38,00 | Restaurante | Prazeres | XP Visa | `412c15fc-c0b8-4d08-8530-4fd665533bde` |
| 104 | 2026-07-11 | Pão | R$ 36,47 | Supermercado | Custos fixos | XP Visa | `0ab01000-085a-40d0-a37b-fa05876ad015` |
| 105 | 2026-07-11 | Pharma Nutry | R$ 32,50 | Suplementos | Prazeres | XP Visa | `27239945-c516-49fa-9f04-bcf6ccf4886f` |
| 106 | 2026-07-12 | Doces | R$ 36,00 | Feira [Alimentação] | Prazeres | XP Visa | `3c994b30-a284-4280-9572-6e410be3ee7f` |
| 107 | 2026-07-12 | Energético | R$ 36,79 | Supérfluo | Conforto | XP Visa | `e4ccffea-a706-4cb3-88ef-02d4ef5649fa` |
| 108 | 2026-07-12 | Maquininha | R$ 8,00 | Outros | Prazeres | XP Visa | `8d38af90-674a-46aa-ab96-0d0527ca1c00` |
| 109 | 2026-07-13 | Almoço | R$ 64,00 | Restaurante | Custos fixos | XP Visa | `8316fbb9-93e5-4f8b-b50a-99f8590ca8ee` |
| 110 | 2026-07-13 | Cookies | R$ 18,00 | Supérfluo | Prazeres | XP Visa | `3c1f3c5b-0cb1-4bb3-90a4-98fa961c58f4` |
| 111 | 2026-07-13 | Doces | R$ 13,90 | Supermercado | Prazeres | XP Visa | `df3ec41d-a987-46d2-a37b-c600ba2419e6` |
| 112 | 2026-07-13 | Doces | R$ 5,90 | Restaurante | Prazeres | XP Visa | `204bf48e-4121-4492-b850-e55538226380` |
| 113 | 2026-07-13 | Doces Santa Rita | R$ 23,20 | Outros | Prazeres | XP Visa | `ea613013-c9e4-47d7-8b24-1a964f8eef10` |
| 114 | 2026-07-13 | Estacionamento | R$ 45,00 | Transporte | Custos fixos | XP Visa | `1aca7ea6-cc64-4d06-9ae8-507e60bd4ea4` |
| 115 | 2026-07-14 | Apple | R$ 5,90 | Serviços | Custos fixos | XP Visa | `62241bde-c03b-4b37-b9bd-3a629dc49761` |
| 116 | 2026-07-14 | Compras do mês | R$ 340,64 | Supermercado | Custos fixos | XP Visa | `72f1d946-0705-43dc-be85-abb4be1833be` |
| 117 | 2026-07-16 | Almoço | R$ 39,89 | Restaurante | Custos fixos | XP Visa | `e0f4df49-c424-4ac7-9f09-9bbd24bc8894` |
| 118 | 2026-07-16 | Google | R$ 12,50 | Serviços | Custos fixos | XP Visa | `07f6e79c-0882-4a2e-ad1b-4760e15b5f6c` |
| 119 | 2026-07-17 | Abastecimento [Tracker] | R$ 127,21 | Abastecimento | Custos fixos | XP Visa | `b90e2ff2-cb24-42ab-a2e0-1a9468375aeb` |
| 120 | 2026-07-17 | Github | R$ 27,01 | Serviços | Custos fixos | XP Visa | `5374d9da-64a7-4e54-ae72-219e49ae37cf` |
| 121 | 2026-07-17 | Lacake | R$ 15,00 | Feira [Alimentação] | Conforto | XP Visa | `0332a7bf-444e-49d4-b28e-61541f7b9a05` |
| 122 | 2026-07-18 | Anthropic | R$ 110,00 | Serviços | Custos fixos | XP Visa | `350fd300-02f0-4df1-bdcd-ea0bdf9e1346` |
| 123 | 2026-07-18 | Nasoar | R$ 111,05 | Saúde | Custos fixos | XP Visa | `2ff73576-a572-4687-ad40-d5de28c1cf45` |
| 124 | 2026-07-27 | Armazém Paraná | R$ 47,71 | Hortifruti | Custos fixos | XP Visa | `83c1d638-7f47-4b63-a3ab-3dec49cb125d` |
| 125 | 2026-07-27 | Bom demais [Proteínas] | R$ 194,23 | Supermercado | Custos fixos | XP Visa | `3739a66b-2944-4a8c-b784-829d2cee8a04` |
| 126 | 2026-07-27 | Chatgpt | R$ 99,90 | Serviços | Custos fixos | XP Visa | `a5a837d8-ff16-4d37-a1ab-6afd78916c1b` |
| 127 | 2026-07-27 | Github | R$ 69,65 | Serviços | Custos fixos | XP Visa | `d21c44d2-48a4-4b93-ab34-f8ade2b11bcf` |
| 128 | 2026-07-27 | Kimi [Moonshot AI] | R$ 100,54 | Serviços | Custos fixos | XP Visa | `cad21536-4c92-4cbc-9f03-9fb60be93ff6` |
| 129 | 2026-07-27 | Leve Mais | R$ 79,88 | Supermercado | Custos fixos | XP Visa | `fb6231af-4956-4539-bd41-84c00a968460` |
| 130 | 2026-07-27 | Porto Mercadinho | R$ 47,00 | Supermercado | Custos fixos | XP Visa | `a5aa59d3-3e79-4a0a-8504-7ecc302c5793` |
| 131 | 2026-07-27 | Varejão Paraná | R$ 121,66 | Supermercado | Custos fixos | XP Visa | `ae19b1fa-ca41-43c1-809b-16b2e27d8bb1` |
| 132 | 2026-07-27 | Whey | R$ 229,90 | Suplementos | Custos fixos | XP Visa | `e8267375-97b2-411d-b9ab-1ece34b3d92f` |
| 133 | 2026-07-27 | Whey [Tony] | R$ 338,92 | Suplementos | Custos fixos | XP Visa | `f6e1a1c6-d99d-4b51-a5c0-739c84a8f947` |
| 134 | 2026-07-28 | Energético | R$ 42,49 | Supermercado (Supérfluo) | Prazeres | XP Visa | `c0d45585-fc12-4ac7-b297-8500f0567338` |
| 135 | 2026-07-28 | Moonshot AI | R$ 207,66 | Serviços | Custos fixos | XP Visa | `5735b976-86b0-4384-8540-c202ee2f5b67` |
| 136 | 2026-07-28 | Nasoar | R$ 178,08 | Saúde | Custos fixos | XP Visa | `20fb3b12-ad53-4f71-9044-74b12fe583e3` |

#### Bill + BillItem — 10 lançamentos (contas fixas de casa)

Join: `Bill b INNER JOIN BillItem bi ON b.Id = bi.BillId`. Sem `CategoryId`
nesta tabela (schema não tem essa FK) — os 5 itens se repetem em junho e
julho, sempre com o mesmo total de fatura (`R$ 1.724,39`, split 60/40:
`R$ 1.035,00` / `R$ 690,00`).

| # | Data | Título | Valor |
|---|---|---|---|
| 1 | 2026-06-01 | Condominio [Casa] | R$ 660,07 |
| 2 | 2026-06-01 | Energia [Casa] | R$ 317,46 |
| 3 | 2026-06-01 | Faxina [Casa] | R$ 400,00 |
| 4 | 2026-06-01 | Internet [Celular] | R$ 225,62 |
| 5 | 2026-06-01 | Internet [Central Fiber] | R$ 121,24 |
| 6 | 2026-07-01 | Condominio [Casa] | R$ 660,07 |
| 7 | 2026-07-01 | Energia [Casa] | R$ 317,46 |
| 8 | 2026-07-01 | Faxina [Casa] | R$ 400,00 |
| 9 | 2026-07-01 | Internet [Celular] | R$ 225,62 |
| 10 | 2026-07-01 | Internet [Central Fiber] | R$ 121,24 |

#### Transaction + TransactionItem — 0 lançamentos

```sql
SELECT * FROM [Transaction] t
INNER JOIN TransactionItem ti ON t.Id = ti.TransactionId
WHERE t.Date >= '2026-06-01' AND t.Date < '2026-08-01';
```

Retorna vazio. Verificado: `SELECT MIN(Date), MAX(Date) FROM [Transaction]`
= `2021-01-01` a `2025-12-01` — a tabela simplesmente não tem registros de
2026. Não há lançamentos deste bloco para listar (não inventado — ausência
real confirmada por contagem).

---

## 3. De-para completo — legado → frase pronta para o WhatsApp

### De-para completo — lançamentos reais do legado → frase pronta para o WhatsApp (mecontrola)

Fonte: FinancialControlDB (SQL Server), jun/jul 2026. Cada frase segue
literalmente o regex de um guard determinístico já verificado no código
(`internal/agents/application/agents/guards/card_expense_shortcut.go` para
compras no cartão, `register_expense_shortcut.go` para despesas fora do
cartão) — não é frase "natural" aproximada, é a frase que o parser aceita.

Fórmula usada (constante, aplicada a todos os itens abaixo):
- Cartão (Invoice/InvoiceItem, à vista): `gastei <valor total> no <descrição limpa> no crédito <cartão>`
- Cartão parcelado (Installment > 1): `gastei <valor TOTAL da compra> no <descrição limpa> em <N>x no crédito <cartão>`
  (confirmado em `internal/transactions/domain/services/transaction_workflow.go:83-88`:
  o valor informado é o TOTAL da compra; o `InstallmentSplitter` divide
  pelas parcelas internamente — nunca informar o valor da parcela).
- Conta fixa (Bill/BillItem, sem cartão): `gastei <valor> na/no <título limpo> em dinheiro`
  (Bill não tem forma de pagamento no legado; "em dinheiro" é só uma opção
  válida e determinística — ajuste se preferir pix/débito, mudando o sufixo
  reconhecido em `register_expense_shortcut.go:33`).

"Descrição limpa" = texto original sem anotações entre colchetes/parênteses
(ex.: "Compras do mês [Atacadão]" → "Compras do mês"). O texto ORIGINAL
completo (com a anotação) fica sempre na coluna "Legado", nada é perdido.

#### 1) Compras no cartão à vista — 136 lançamentos (Invoice + InvoiceItem, Installment = 1)

| # | Data | Legado (descrição, valor, categoria) | Frase para o WhatsApp |
|---|---|---|---|
| 1 | 2026-06-01 | Busterfit (R$ 254,83, Academia) | `gastei 254,83 no Busterfit no crédito XP` |
| 2 | 2026-06-01 | Netflix (R$ 44,90, Streamings) | `gastei 44,90 no Netflix no crédito XP` |
| 3 | 2026-06-01 | Open.AI (R$ 99,90, Serviços) | `gastei 99,90 no Open.AI no crédito XP` |
| 4 | 2026-06-03 | Drogaria São Luis (R$ 98,90, Saúde) | `gastei 98,90 no Drogaria São Luis no crédito XP` |
| 5 | 2026-06-03 | Sem Parar (R$ 215,75, Transporte) | `gastei 215,75 no Sem Parar no crédito XP` |
| 6 | 2026-06-03 | Zona Azul [Barueri] (R$ 10,00, Transporte) | `gastei 10,00 no Zona Azul no crédito XP` |
| 7 | 2026-06-04 | Amazon [Prime Video] (R$ 39,41, Streamings) | `gastei 39,41 no Amazon no crédito XP` |
| 8 | 2026-06-05 | Compras do mês [Atacadão] (R$ 1.002,29, Supermercado) | `gastei 1.002,29 no Compras do mês no crédito XP` |
| 9 | 2026-06-07 | Itens da casa [Banheiro] (R$ 119,99, Casa) | `gastei 119,99 no Itens da casa no crédito XP` |
| 10 | 2026-06-08 | Testo Black (R$ 108,04, Saúde) | `gastei 108,04 no Testo Black no crédito XP` |
| 11 | 2026-06-09 | Microsoft [Github] (R$ 11,63, Serviços) | `gastei 11,63 no Microsoft no crédito XP` |
| 12 | 2026-06-09 | Moonshot [IA] (R$ 107,80, Serviços) | `gastei 107,80 no Moonshot no crédito XP` |
| 13 | 2026-06-09 | Moonshot [IA] (R$ 102,34, Serviços) | `gastei 102,34 no Moonshot no crédito XP` |
| 14 | 2026-06-10 | Ebanx [PSN] (R$ 619,90, Lazer) | `gastei 619,90 no Ebanx no crédito XP` |
| 15 | 2026-06-10 | Itens da Casa (R$ 24,80, Casa) | `gastei 24,80 no Itens da Casa no crédito XP` |
| 16 | 2026-06-10 | Nasoar (R$ 133,54, Saúde) | `gastei 133,54 no Nasoar no crédito XP` |
| 17 | 2026-06-11 | Whey (R$ 194,90, Suplementos) | `gastei 194,90 no Whey no crédito XP` |
| 18 | 2026-06-13 | Abastecimento [Tracker] (R$ 293,15, Abastecimento) | `gastei 293,15 no Abastecimento no crédito XP` |
| 19 | 2026-06-13 | hevy.com (R$ 12,90, Serviços) | `gastei 12,90 no hevy.com no crédito XP` |
| 20 | 2026-06-14 | Abastecimento [Tracker] (R$ 172,33, Abastecimento) | `gastei 172,33 no Abastecimento no crédito XP` |
| 21 | 2026-06-14 | Apple (R$ 5,90, Serviços) | `gastei 5,90 no Apple no crédito XP` |
| 22 | 2026-06-14 | São Vicente (R$ 105,71, Supermercado) | `gastei 105,71 no São Vicente no crédito XP` |
| 23 | 2026-06-15 | Energético (R$ 29,30, Restaurante) | `gastei 29,30 no Energético no crédito XP` |
| 24 | 2026-06-15 | Figurinha da Copa (R$ 49,00, Outros) | `gastei 49,00 no Figurinha da Copa no crédito XP` |
| 25 | 2026-06-16 | Gemini (R$ 12,50, Serviços) | `gastei 12,50 no Gemini no crédito XP` |
| 26 | 2026-06-16 | Porto Mercadinho (R$ 49,90, Supermercado) | `gastei 49,90 no Porto Mercadinho no crédito XP` |
| 27 | 2026-06-18 | Wellhub (R$ 36,70, Academia) | `gastei 36,70 no Wellhub no crédito XP` |
| 28 | 2026-06-18 | YouTube [PasquaDev] (R$ 23,99, Streamings) | `gastei 23,99 no YouTube no crédito XP` |
| 29 | 2026-06-21 | Armazém Paraná (R$ 32,88, Hortifruti) | `gastei 32,88 no Armazém Paraná no crédito XP` |
| 30 | 2026-06-21 | Bom Demais [Mistura] (R$ 289,67, Supermercado) | `gastei 289,67 no Bom Demais no crédito XP` |
| 31 | 2026-06-21 | Varejão Paraná (R$ 157,28, Supermercado) | `gastei 157,28 no Varejão Paraná no crédito XP` |
| 32 | 2026-06-22 | Nasoar [Farmárcia] (R$ 113,18, Saúde) | `gastei 113,18 no Nasoar no crédito XP` |
| 33 | 2026-06-23 | Mercadinho Porto (R$ 45,37, Supermercado) | `gastei 45,37 no Mercadinho Porto no crédito XP` |
| 34 | 2026-06-27 | Abastecimento [Tracker] (R$ 224,19, Abastecimento) | `gastei 224,19 no Abastecimento no crédito XP` |
| 35 | 2026-06-27 | Água de Coco (R$ 11,00, Feira [Alimentação]) | `gastei 11,00 no Água de Coco no crédito XP` |
| 36 | 2026-06-27 | Github (R$ 55,00, Serviços) | `gastei 55,00 no Github no crédito XP` |
| 37 | 2026-06-27 | Lavagem [Tracker] (R$ 60,00, Lavagem Automotiva) | `gastei 60,00 no Lavagem no crédito XP` |
| 38 | 2026-06-27 | Mercadinho Porto (R$ 26,98, Supermercado) | `gastei 26,98 no Mercadinho Porto no crédito XP` |
| 39 | 2026-06-27 | Mercadinho Porto (R$ 62,36, Supermercado) | `gastei 62,36 no Mercadinho Porto no crédito XP` |
| 40 | 2026-06-27 | Mercado Bom Demais (R$ 73,37, Supermercado) | `gastei 73,37 no Mercado Bom Demais no crédito XP` |
| 41 | 2026-06-27 | Outros (R$ 27,95, Outros) | `gastei 27,95 no Outros no crédito XP` |
| 42 | 2026-06-27 | Padaria Do Aurora (R$ 8,50, Restaurante) | `gastei 8,50 no Padaria Do Aurora no crédito XP` |
| 43 | 2026-06-27 | Pastel Do Guri (R$ 17,00, Feira [Alimentação]) | `gastei 17,00 no Pastel Do Guri no crédito XP` |
| 44 | 2026-06-27 | Pastel Do Guri (R$ 13,00, Feira [Alimentação]) | `gastei 13,00 no Pastel Do Guri no crédito XP` |
| 45 | 2026-06-27 | Pizzaria Bonna Notte (R$ 124,00, Restaurante) | `gastei 124,00 no Pizzaria Bonna Notte no crédito XP` |
| 46 | 2026-06-28 | Casa de Carnes (R$ 181,55, Supermercado) | `gastei 181,55 no Casa de Carnes no crédito XP` |
| 47 | 2026-06-28 | Churrasquinho (R$ 50,00, Feira [Alimentação]) | `gastei 50,00 no Churrasquinho no crédito XP` |
| 48 | 2026-06-28 | Churros (R$ 14,00, Feira [Alimentação]) | `gastei 14,00 no Churros no crédito XP` |
| 49 | 2026-06-28 | Doces (R$ 70,10, Supermercado) | `gastei 70,10 no Doces no crédito XP` |
| 50 | 2026-06-28 | Doces (R$ 36,00, Feira [Alimentação]) | `gastei 36,00 no Doces no crédito XP` |
| 51 | 2026-06-28 | Energético (R$ 34,00, Bebidas) | `gastei 34,00 no Energético no crédito XP` |
| 52 | 2026-06-28 | iFood [Nasoar] (R$ 179,37, Saúde) | `gastei 179,37 no iFood no crédito XP` |
| 53 | 2026-06-28 | Mercadinho Porto (R$ 23,97, Supermercado) | `gastei 23,97 no Mercadinho Porto no crédito XP` |
| 54 | 2026-06-28 | Minibola Copa Do Mundo Da Fifa 26 (R$ 95,99, Lazer) | `gastei 95,99 no Minibola Copa Do Mundo Da Fifa 26 no crédito XP` |
| 55 | 2026-06-28 | Motel (R$ 209,00, Outros) | `gastei 209,00 no Motel no crédito XP` |
| 56 | 2026-06-28 | Tylenol Sisu (R$ 24,79, Saúde) | `gastei 24,79 no Tylenol Sisu no crédito XP` |
| 57 | 2026-06-29 | Armazém Paraná (R$ 60,32, Hortifruti) | `gastei 60,32 no Armazém Paraná no crédito XP` |
| 58 | 2026-06-30 | Sabão Lavar Louças (R$ 124,58, Casa) | `gastei 124,58 no Sabão Lavar Louças no crédito XP` |
| 59 | 2026-06-30 | Vitaminas (R$ 127,70, Suplementos) | `gastei 127,70 no Vitaminas no crédito XP` |
| 60 | 2026-07-01 | Academia [JJ + Stefany] (R$ 254,83, Academia) | `gastei 254,83 no Academia no crédito XP` |
| 61 | 2026-07-01 | Armazém Paraná (R$ 60,32, Hortifruti) | `gastei 60,32 no Armazém Paraná no crédito XP` |
| 62 | 2026-07-01 | Bolo [Festa Junina JJ] (R$ 26,00, Lazer) | `gastei 26,00 no Bolo no crédito XP` |
| 63 | 2026-07-01 | Corte de cabelo (R$ 77,00, Serviços) | `gastei 77,00 no Corte de cabelo no crédito XP` |
| 64 | 2026-07-01 | Mala de Viagem (R$ 198,39, Viagem) | `gastei 198,39 no Mala de Viagem no crédito XP` |
| 65 | 2026-07-01 | Mercadinho [Energético] (R$ 29,47, Supermercado) | `gastei 29,47 no Mercadinho no crédito XP` |
| 66 | 2026-07-01 | Microsoft Azure (R$ 76,45, Serviços) | `gastei 76,45 no Microsoft Azure no crédito XP` |
| 67 | 2026-07-01 | Netflix (R$ 44,90, Streamings) | `gastei 44,90 no Netflix no crédito XP` |
| 68 | 2026-07-01 | Oficial Farma (R$ 194,70, Suplementos) | `gastei 194,70 no Oficial Farma no crédito XP` |
| 69 | 2026-07-01 | Pizzaria Bonna Notte (R$ 131,00, Prazeres) | `gastei 131,00 no Pizzaria Bonna Notte no crédito XP` |
| 70 | 2026-07-02 | Lavagem [Tracker] (R$ 60,00, Lavagem Automotiva) | `gastei 60,00 no Lavagem no crédito XP` |
| 71 | 2026-07-02 | Mordedor Helena (R$ 25,80, Bebê) | `gastei 25,80 no Mordedor Helena no crédito XP` |
| 72 | 2026-07-02 | Oral-B Refil (R$ 70,00, Saúde) | `gastei 70,00 no Oral-B Refil no crédito XP` |
| 73 | 2026-07-03 | Algodão Doce + Pipoca (R$ 25,00, Feira [Alimentação]) | `gastei 25,00 no Algodão Doce + Pipoca no crédito XP` |
| 74 | 2026-07-03 | Doces (R$ 34,00, Feira [Alimentação]) | `gastei 34,00 no Doces no crédito XP` |
| 75 | 2026-07-03 | Drogaria São Luis (R$ 63,60, Saúde) | `gastei 63,60 no Drogaria São Luis no crédito XP` |
| 76 | 2026-07-03 | Espetinho (R$ 50,00, Feira [Alimentação]) | `gastei 50,00 no Espetinho no crédito XP` |
| 77 | 2026-07-03 | Livros (R$ 60,00, Educação) | `gastei 60,00 no Livros no crédito XP` |
| 78 | 2026-07-03 | Lojas Americanas (R$ 54,43, Supérfluo) | `gastei 54,43 no Lojas Americanas no crédito XP` |
| 79 | 2026-07-03 | Pamonha (R$ 30,00, Feira [Alimentação]) | `gastei 30,00 no Pamonha no crédito XP` |
| 80 | 2026-07-03 | Sem Parar (R$ 168,99, Transporte) | `gastei 168,99 no Sem Parar no crédito XP` |
| 81 | 2026-07-03 | Sorvete (R$ 7,00, Feira [Alimentação]) | `gastei 7,00 no Sorvete no crédito XP` |
| 82 | 2026-07-04 | Agua de coco (R$ 35,00, Feira [Alimentação]) | `gastei 35,00 no Agua de coco no crédito XP` |
| 83 | 2026-07-04 | Pastel Do Guri (R$ 26,00, Feira [Alimentação]) | `gastei 26,00 no Pastel Do Guri no crédito XP` |
| 84 | 2026-07-05 | Estacionamento (R$ 30,00, Transporte) | `gastei 30,00 no Estacionamento no crédito XP` |
| 85 | 2026-07-07 | Carlinhos [Corte do Tony] (R$ 57,00, Serviços) | `gastei 57,00 no Carlinhos no crédito XP` |
| 86 | 2026-07-07 | Figurinhas da Copa (R$ 70,00, Lazer) | `gastei 70,00 no Figurinhas da Copa no crédito XP` |
| 87 | 2026-07-07 | Mercadinho (R$ 27,95, Supermercado) | `gastei 27,95 no Mercadinho no crédito XP` |
| 88 | 2026-07-07 | Mercadinho [Almoço] (R$ 59,59, Supermercado) | `gastei 59,59 no Mercadinho no crédito XP` |
| 89 | 2026-07-08 | Figurinhas (R$ 77,99, Outros) | `gastei 77,99 no Figurinhas no crédito XP` |
| 90 | 2026-07-08 | São Vicente (R$ 82,02, Supermercado) | `gastei 82,02 no São Vicente no crédito XP` |
| 91 | 2026-07-08 | Suplementos (R$ 52,55, Suplementos) | `gastei 52,55 no Suplementos no crédito XP` |
| 92 | 2026-07-09 | Compras do mês (R$ 551,68, Supermercado) | `gastei 551,68 no Compras do mês no crédito XP` |
| 93 | 2026-07-09 | Microsoft Azure (R$ 70,55, Serviços) | `gastei 70,55 no Microsoft Azure no crédito XP` |
| 94 | 2026-07-10 | Chopp (R$ 30,00, Feira [Alimentação]) | `gastei 30,00 no Chopp no crédito XP` |
| 95 | 2026-07-10 | Churros (R$ 14,00, Feira [Alimentação]) | `gastei 14,00 no Churros no crédito XP` |
| 96 | 2026-07-10 | Doces (R$ 31,00, Feira [Alimentação]) | `gastei 31,00 no Doces no crédito XP` |
| 97 | 2026-07-10 | Hb Solutions [Chaveiro] (R$ 15,00, Outros) | `gastei 15,00 no Hb Solutions no crédito XP` |
| 98 | 2026-07-10 | Ração do Peixe (R$ 19,90, Supermercado) | `gastei 19,90 no Ração do Peixe no crédito XP` |
| 99 | 2026-07-10 | Suplementos [Pré Treino] (R$ 89,90, Suplementos) | `gastei 89,90 no Suplementos no crédito XP` |
| 100 | 2026-07-11 | Abastecimento [Tracker] (R$ 237,11, Abastecimento) | `gastei 237,11 no Abastecimento no crédito XP` |
| 101 | 2026-07-11 | Armarinhos Fernandes (R$ 94,19, Bebê) | `gastei 94,19 no Armarinhos Fernandes no crédito XP` |
| 102 | 2026-07-11 | Madero (R$ 144,00, Restaurante) | `gastei 144,00 no Madero no crédito XP` |
| 103 | 2026-07-11 | McDonald's (R$ 38,00, Restaurante) | `gastei 38,00 no McDonald's no crédito XP` |
| 104 | 2026-07-11 | Pão (R$ 36,47, Supermercado) | `gastei 36,47 no Pão no crédito XP` |
| 105 | 2026-07-11 | Pharma Nutry (R$ 32,50, Suplementos) | `gastei 32,50 no Pharma Nutry no crédito XP` |
| 106 | 2026-07-12 | Doces (R$ 36,00, Feira [Alimentação]) | `gastei 36,00 no Doces no crédito XP` |
| 107 | 2026-07-12 | Energético (R$ 36,79, Supérfluo) | `gastei 36,79 no Energético no crédito XP` |
| 108 | 2026-07-12 | Maquininha (R$ 8,00, Outros) | `gastei 8,00 no Maquininha no crédito XP` |
| 109 | 2026-07-13 | Almoço (R$ 64,00, Restaurante) | `gastei 64,00 no Almoço no crédito XP` |
| 110 | 2026-07-13 | Cookies (R$ 18,00, Supérfluo) | `gastei 18,00 no Cookies no crédito XP` |
| 111 | 2026-07-13 | Doces (R$ 13,90, Supermercado) | `gastei 13,90 no Doces no crédito XP` |
| 112 | 2026-07-13 | Doces (R$ 5,90, Restaurante) | `gastei 5,90 no Doces no crédito XP` |
| 113 | 2026-07-13 | Doces Santa Rita (R$ 23,20, Outros) | `gastei 23,20 no Doces Santa Rita no crédito XP` |
| 114 | 2026-07-13 | Estacionamento (R$ 45,00, Transporte) | `gastei 45,00 no Estacionamento no crédito XP` |
| 115 | 2026-07-14 | Apple (R$ 5,90, Serviços) | `gastei 5,90 no Apple no crédito XP` |
| 116 | 2026-07-14 | Compras do mês (R$ 340,64, Supermercado) | `gastei 340,64 no Compras do mês no crédito XP` |
| 117 | 2026-07-16 | Almoço (R$ 39,89, Restaurante) | `gastei 39,89 no Almoço no crédito XP` |
| 118 | 2026-07-16 | Google (R$ 12,50, Serviços) | `gastei 12,50 no Google no crédito XP` |
| 119 | 2026-07-17 | Abastecimento [Tracker] (R$ 127,21, Abastecimento) | `gastei 127,21 no Abastecimento no crédito XP` |
| 120 | 2026-07-17 | Github (R$ 27,01, Serviços) | `gastei 27,01 no Github no crédito XP` |
| 121 | 2026-07-17 | Lacake (R$ 15,00, Feira [Alimentação]) | `gastei 15,00 no Lacake no crédito XP` |
| 122 | 2026-07-18 | Anthropic (R$ 110,00, Serviços) | `gastei 110,00 no Anthropic no crédito XP` |
| 123 | 2026-07-18 | Nasoar (R$ 111,05, Saúde) | `gastei 111,05 no Nasoar no crédito XP` |
| 124 | 2026-07-27 | Armazém Paraná (R$ 47,71, Hortifruti) | `gastei 47,71 no Armazém Paraná no crédito XP` |
| 125 | 2026-07-27 | Bom demais [Proteínas] (R$ 194,23, Supermercado) | `gastei 194,23 no Bom demais no crédito XP` |
| 126 | 2026-07-27 | Chatgpt (R$ 99,90, Serviços) | `gastei 99,90 no Chatgpt no crédito XP` |
| 127 | 2026-07-27 | Github (R$ 69,65, Serviços) | `gastei 69,65 no Github no crédito XP` |
| 128 | 2026-07-27 | Kimi [Moonshot AI] (R$ 100,54, Serviços) | `gastei 100,54 no Kimi no crédito XP` |
| 129 | 2026-07-27 | Leve Mais (R$ 79,88, Supermercado) | `gastei 79,88 no Leve Mais no crédito XP` |
| 130 | 2026-07-27 | Porto Mercadinho (R$ 47,00, Supermercado) | `gastei 47,00 no Porto Mercadinho no crédito XP` |
| 131 | 2026-07-27 | Varejão Paraná (R$ 121,66, Supermercado) | `gastei 121,66 no Varejão Paraná no crédito XP` |
| 132 | 2026-07-27 | Whey (R$ 229,90, Suplementos) | `gastei 229,90 no Whey no crédito XP` |
| 133 | 2026-07-27 | Whey [Tony] (R$ 338,92, Suplementos) | `gastei 338,92 no Whey no crédito XP` |
| 134 | 2026-07-28 | Energético (R$ 42,49, Supermercado (Supérfluo)) | `gastei 42,49 no Energético no crédito XP` |
| 135 | 2026-07-28 | Moonshot AI (R$ 207,66, Serviços) | `gastei 207,66 no Moonshot AI no crédito XP` |
| 136 | 2026-07-28 | Nasoar (R$ 178,08, Saúde) | `gastei 178,08 no Nasoar no crédito XP` |

#### 2) Compras parceladas — 9 lançamentos reais com fatura em jun/jul 2026

Estas são compras que já existiam parceladas no legado ANTES de jun/jul
(algumas desde 2025) e cuja parcela caiu na fatura de junho e/ou julho de
2026 (`Invoice.Date`). Para reproduzir esse padrão no mecontrola como teste
novo, a frase abaixo registra a compra completa (valor total + Nx) — não dá
para "recriar" uma parcela isolada no meio de um parcelamento já em
andamento, então o teste é: registrar a compra do zero com o mesmo valor
total e número de parcelas real.

| InvoiceControl | Legado (descrição, valor total, parcelas, categoria, compra original) | Frase para o WhatsApp |
|---|---|---|
| InvoiceControl 5440 | Team Cruz [JJ] (total R$ 900,00, 12x, Academia, compra original 2025-08-05, fatura em jun+jul/2026) | `gastei 900,00 no Team Cruz em 12x no crédito XP` |
| InvoiceControl 5786 | Viagem [Maio 2026] (total R$ 3.715,32, 10x, Lazer, compra original 2025-12-01, fatura em jun+jul/2026) | `gastei 3.715,32 no Viagem em 10x no crédito XP` |
| InvoiceControl 5787 | Aleatório (total R$ 12,00, 12x, Eletrônicos, compra original 2025-12-01, fatura em jun+jul/2026) | `gastei 12,00 no Aleatório em 12x no crédito Nubank` |
| InvoiceControl 5790 | Festa do Tony (total R$ 1.622,50, 6x, Outros, compra original 2025-12-03, fatura em jun+jul/2026) | `gastei 1.622,50 no Festa do Tony em 6x no crédito XP` |
| InvoiceControl 5835 | Decolar [Viagem Maceió] (total R$ 12.932,12, 10x, Lazer, compra original 2025-12-19, fatura em jun+jul/2026) | `gastei 12.932,12 no Decolar em 10x no crédito XP` |
| InvoiceControl 5997 | IA para Devs (total R$ 999,00, 10x, Educação, compra original 2026-03-04, fatura em jun+jul/2026) | `gastei 999,00 no IA para Devs em 10x no crédito XP` |
| InvoiceControl 6031 | Tech Leads [Club] (total R$ 1.115,73, 12x, Educação, compra original 2026-03-09, fatura em jun+jul/2026) | `gastei 1.115,73 no Tech Leads em 12x no crédito XP` |
| InvoiceControl 6064 | Arquitetura de Soluções com IA (total R$ 1.499,25, 12x, Educação, compra original 2026-04-02, fatura em jun+jul/2026) | `gastei 1.499,25 no Arquitetura de Soluções com IA em 12x no crédito XP` |
| InvoiceControl 6079 | HBO Max (total R$ 418,80, 12x, Streamings, compra original 2026-04-14, fatura em jun+jul/2026) | `gastei 418,80 no HBO Max em 12x no crédito XP` |

#### 3) Contas fixas de casa — 10 lançamentos (Bill + BillItem)

Sem cartão nem CategoryId no legado (schema de `Bill` não tem essa FK).
Fatura total do mês: R$ 1.724,39 (split 60/40: R$ 1.035,00 / R$ 690,00).

| # | Data | Legado (título, valor) | Frase para o WhatsApp |
|---|---|---|---|
| 1 | 2026-06-01 | Condominio [Casa] (R$ 660,07) | `gastei 660,07 no Condominio em dinheiro` |
| 2 | 2026-06-01 | Energia [Casa] (R$ 317,46) | `gastei 317,46 na Energia em dinheiro` |
| 3 | 2026-06-01 | Faxina [Casa] (R$ 400,00) | `gastei 400,00 na Faxina em dinheiro` |
| 4 | 2026-06-01 | Internet [Celular] (R$ 225,62) | `gastei 225,62 na Internet em dinheiro` |
| 5 | 2026-06-01 | Internet [Central Fiber] (R$ 121,24) | `gastei 121,24 na Internet em dinheiro` |
| 6 | 2026-07-01 | Condominio [Casa] (R$ 660,07) | `gastei 660,07 no Condominio em dinheiro` |
| 7 | 2026-07-01 | Energia [Casa] (R$ 317,46) | `gastei 317,46 na Energia em dinheiro` |
| 8 | 2026-07-01 | Faxina [Casa] (R$ 400,00) | `gastei 400,00 na Faxina em dinheiro` |
| 9 | 2026-07-01 | Internet [Celular] (R$ 225,62) | `gastei 225,62 na Internet em dinheiro` |
| 10 | 2026-07-01 | Internet [Central Fiber] (R$ 121,24) | `gastei 121,24 na Internet em dinheiro` |

---

## 4. Tabela de referência rápida — frases dos cenários de regressão

### Frases prontas para validação E2E no WhatsApp — jun/jul 2026

Roteiro completo (pré-condições, evidências SQL/Prometheus/logs, mapeamento
de commits): `docs/runs/29-07-2026-cenarios-e2e-agente-jun-jul-2026.txt`.
Cada frase abaixo casa literalmente com o regex/guard citado — copiar
exatamente como está; qualquer palavra extra antes/depois pode mudar de
guard ou quebrar o parse.

| Cenário | Frase exata para enviar no WhatsApp | Guard/regex (file:line) |
|---|---|---|
| C1-A | `gastei 69,65 no Github no crédito XP` | `card_expense_shortcut.go:16` (head) + `:20` (marker) |
| C1-B | `no Github eu gastei 69,90 e não 69,65` | `edit_entry_correction_shortcut.go:17` (2 valores, "gastei") |
| C2 | `lançamento do whey, pode mudar o valor para 234,90` | `edit_entry_correction_shortcut.go:18` (só valor novo) |
| C3 | `lançamento do porto mercadinho, o valor certo é 47,50 e não 47` | `edit_entry_correction_shortcut.go:16` (2 valores, "lançamento") |
| C4-1 | `gastei 194,23 no bom demais no crédito XP` | `card_expense_shortcut.go` (cartão citado) |
| C4-2 (a) | `gastei 338,92 no whey tony no crédito` | `card_expense_shortcut.go` (sem apelido → pergunta cartão) |
| C4-2 (b) | `XP` | resposta ao prompt de cartão |
| C5 | `paguei 3775,08 na fatura do cartão XP` | `register_expense_shortcut.go:16` (valor antes de "fatura") |
| C6 | `quanto foi a fatura do XP de janeiro` | sem guard — roteado por LLM (`query_card_invoice`) |
| C7 (a) | `quanto eu gastei hoje` | `query_day_shortcut.go:19` (ancorado `^...$`) |
| C7 (b) | `quanto eu gastei ontem` | `query_day_shortcut.go:19` (ancorado `^...$`) |
| C8 | `detalhe da categoria supermercado` | sem guard — roteado por LLM (detalhe de categoria) |
| C10 | `gastei 400 na faxina em dinheiro` | `register_expense_shortcut.go:33` (sufixo "em dinheiro" = cash) |
| C11 (abrir) | `gastei 30 na padaria no pix` (NÃO confirmar; esperar TTL expirar) | `expired_without_tool.go` (PostGuard sobre o texto do bot) |
| C11 (depois) | `oi` | não deve repetir aviso de expiração |

#### Setup necessário antes de C2/C3

| Setup | Frase | Depois |
|---|---|---|
| Para C2 | `gastei 229,90 no Whey no crédito XP` | responder `sim` na confirmação |
| Para C3 | `gastei 47 no porto mercadinho no pix` | responder `sim` na confirmação |

#### Notas de armadilha (não fazer)

- **C1-A/C4**: nunca colar texto (ex.: data) depois do apelido do cartão —
  o regex captura tudo até o fim da string como nickname e quebra o
  `resolveCard`.
- **C2 vs C3**: frases parecidas mas regexes diferentes — C2 tem só o
  valor novo ("mudar o valor para X"), C3 tem os dois valores ("valor
  certo é X e não Y"). Não são intercambiáveis.
- **C7**: a frase precisa ser só isso (regex ancorado `^...$`) — não
  prefixar com "e ontem," nem sufixar com pontuação extra fora do previsto.
- **C10**: usar "na faxina" (conector "na"), não "com a faxina" — "com"
  deixa o artigo "a" dentro da descrição salva.

---

## 5. Roteiro completo de cenários E2E

Roteiro original em texto plano, preservado integralmente (pré-condições,
frases ENVIAR, comportamento esperado, queries de evidência SQL/Prometheus/
logs, mapeamento commit→cenário). Embutido em bloco de código para manter a
formatação exata (a tabela de referência rápida da seção 4 acima já resume
as frases; esta seção traz todo o resto: setup, evidências, cobertura).

```text
================================================================================
ROTEIRO E2E PRODUCTION-PROOF — AGENTE WHATSAPP — DADOS REAIS JUN/JUL 2026
Data: 2026-07-29
Usuário sob teste: 9bbbbbcd-2081-40a8-9780-2b5d818f1580 (+5511986896322)
VPS: ssh mecontrola-vps | DB: mecontrola_db (schema mecontrola) | Stack: Swarm
Escopo de commits (não cobertos em docs/runs/cenarios-e2e-agente-2026-07-28.txt,
que cobriu até e16dd2c/3a4d06a): 32fd0f1..6393fd6 (29 commits, HEAD atual)
Fonte de dados reais: FinancialControlDB (SQL Server, site4now.net) — extração
bruta em docs/runs/29-07-2026-dados-reais-financialcontroldb.txt
================================================================================

REGRAS DESTE ROTEIRO (herdadas do roteiro de 07-28, inegociáveis)
- Nenhum PASS sem evidência colada (linha de SELECT, log, métrica ou trace).
- Nenhuma resposta inventada: se o bot divergir do esperado, registrar o texto
  REAL em platform_messages (role=assistant) e marcar FAIL/DIVERGÊNCIA.
- Fonte de verdade das respostas do bot: tabela mecontrola.platform_messages,
  não apenas o relato visual do WhatsApp.
- Entre cenários: nenhum workflow_runs ativo/suspenso do usuário. Se sobrar,
  enviar "cancelar" no WhatsApp antes de prosseguir.
- Toda frase ENVIAR abaixo usa descrição/valor/data REAIS extraídos do
  FinancialControlDB (nunca inventados) — o Id de origem é citado como
  rastreio em cada cenário.
- Este arquivo é o ROTEIRO (script). A execução das mensagens no WhatsApp é
  manual (eu não tenho acesso ao WhatsApp). Depois de rodar cada cenário,
  colar as evidências em docs/runs/29-07-2026-resultados-e2e-agente-jun-jul-2026.txt
  seguindo exatamente o padrão de resultados-e2e-agente-2026-07-28.txt.

NOTA IMPORTANTE SOBRE OS DADOS DE ORIGEM
- [Transaction]+TransactionItem do FinancialControlDB: 0 linhas em jun/jul 2026
  (MAX(Date) da tabela = 2025-12-01 — tabela legada, sem dados de 2026; o app
  real já não grava mais ali). Não há frases desse bloco para este roteiro.
- Bill+BillItem: 10 linhas (contas fixas de casa, jun e jul 2026, mesmos 5
  itens repetidos: Energia, Faxina, Condomínio, Internet Central Fiber,
  Internet Celular). Usadas nos cenários de despesa fixa (C10).
- Invoice+InvoiceItem: 136 linhas (compras reais no cartão "XP Visa" jun/jul
  2026). Usadas na maioria dos cenários abaixo — priorizei itens de
  27-28/07/2026 (próximos da data de execução, 2026-07-29) para exercitar
  "hoje"/"ontem" nas consultas diárias com dados verdadeiros.
- Toda categoria usada em "SE PEDIR CATEGORIA" vem do JOIN real
  InvoiceItem.CategoryId = Category.Id (136/136 linhas com CategoryId
  resolvido, 0 nulas — ver docs/runs/29-07-2026-dados-reais-financialcontroldb.txt).
  Confirmado: Github/Whey[Tony]→Serviços/Suplementos, Bom demais/Porto
  Mercadinho→Supermercado (nenhuma inferida por nome, todas via Id).
  A coluna Tags (Custos fixos/Conforto/Prazeres) é uma dimensão de
  classificação orçamentária 60/40 do legado, INDEPENDENTE da categoria —
  não usada como categoria porque não existe mapeamento 1:1 (a mesma
  Category aparece com múltiplos Tags no dataset).

COMANDOS-BASE
  DB:    ssh mecontrola-vps 'docker exec $(docker ps -qf name=mecontrola_postgres.1) psql -U mecontrola -d mecontrola_db -c "<SQL>"'
  LOGS:  ssh mecontrola-vps 'docker service logs mecontrola_server --since 15m 2>&1 | grep -F "<str>"'
         ssh mecontrola-vps 'docker service logs mecontrola_worker --since 15m 2>&1 | grep -F "<str>"'
  PROM:  ssh mecontrola-vps 'docker exec $(docker ps -qf name=mecontrola_otel-lgtm) wget -qO- "http://localhost:9090/api/v1/query?query=<promql>"'
  TRACE: ssh mecontrola-vps 'docker exec $(docker ps -qf name=mecontrola_otel-lgtm) wget -qO- "http://localhost:3200/api/search?tags=..."'

UID = 9bbbbbcd-2081-40a8-9780-2b5d818f1580 (usar nas queries abaixo)

================================================================================
TABELA DE REFERÊNCIA RÁPIDA — FRASE EXATA POR CENÁRIO
================================================================================
Cada frase abaixo casa literalmente com o regex/guard citado no código
(file:line). Copiar exatamente como está — qualquer palavra extra antes/
depois pode mudar de guard ou quebrar o parse (detalhes e evidências de
cada cenário no corpo do roteiro, mais abaixo).

--------------------------------------------------------------------------------------------------------------------------------
Cenário | Guard/regex (file:line)                                          | Frase exata para enviar no WhatsApp
--------|-------------------------------------------------------------------|----------------------------------------------------
C1-A    | card_expense_shortcut.go:16 (head) + :20 (marker)                 | gastei 69,65 no Github no crédito XP
C1-B    | edit_entry_correction_shortcut.go:17 (2 valores, "gastei")        | no Github eu gastei 69,90 e não 69,65
C2      | edit_entry_correction_shortcut.go:18 (só valor novo)              | lançamento do whey, pode mudar o valor para 234,90
C3      | edit_entry_correction_shortcut.go:16 (2 valores, "lançamento")    | lançamento do porto mercadinho, o valor certo é 47,50 e não 47
C4-1    | card_expense_shortcut.go (cartão citado)                         | gastei 194,23 no bom demais no crédito XP
C4-2 (a)| card_expense_shortcut.go (sem apelido → pergunta cartão)          | gastei 338,92 no whey tony no crédito
C4-2 (b)| resposta ao prompt de cartão                                      | XP
C5      | register_expense_shortcut.go:16 (valor antes de "fatura")        | paguei 3775,08 na fatura do cartão XP
C6      | sem guard — roteado por LLM (query_card_invoice)                 | quanto foi a fatura do XP de janeiro
C7 (a)  | query_day_shortcut.go:19 (ancorado ^...$)                        | quanto eu gastei hoje
C7 (b)  | query_day_shortcut.go:19 (ancorado ^...$)                        | quanto eu gastei ontem
C8      | sem guard — roteado por LLM (detalhe de categoria)               | detalhe da categoria supermercado
C10     | register_expense_shortcut.go:33 (sufixo "em dinheiro" = cash)     | gastei 400 na faxina em dinheiro
C11     | expired_without_tool.go (PostGuard sobre o texto do bot, não      | gastei 30 na padaria no pix (abrir e NÃO confirmar,
        | regex de entrada do usuário)                                     | esperar TTL expirar) → depois: oi
--------------------------------------------------------------------------------------------------------------------------------

Notas de armadilha (não fazer):
- C1-A/C4: NUNCA colar texto (ex.: data) depois do apelido do cartão — o
  regex captura tudo até o fim da string como nickname e quebra resolveCard.
- C2 vs C3: frases parecidas mas regexes diferentes — C2 tem só o valor
  novo ("mudar o valor para X"), C3 tem os dois valores ("valor certo é X
  e não Y"). Não são intercambiáveis.
- C7: a frase precisa ser SÓ isso (regex ancorado ^...$) — não prefixar
  com "e ontem," nem sufixar com pontuação extra fora do previsto.
- C10: usar "na faxina" (conector "na"), não "com a faxina" — "com" deixa
  o artigo "a" dentro da descrição salva.

================================================================================
[S0] SETUP E BASELINE (bloqueante antes de qualquer mensagem)
================================================================================

S0.1 — CONFIRMAR QUE O DEPLOY CONTÉM OS COMMITS DESTE ROTEIRO
  Verificar que o HEAD em produção é 6393fd6 (ou posterior):
  a) docker inspect do container server → OTEL_SERVICE_VERSION deve começar
     com "6393fd6".
  b) Evidência comportamental alternativa: métrica
     agents_query_day_total (existe só a partir de db59ea7/570cf3e) e
     agent_guard_decisions_total{tool="query_card_invoice"} respondendo —
     se ausentes, o binário é anterior a este lote.
  PASS: deploy confirmado. FAIL: parar execução e reportar.

S0.2 — BASELINE DB (anotar valores antes de qualquer mensagem)
  SELECT count(*) AS tx_total, max(created_at) FROM mecontrola.transactions
   WHERE user_id='UID';
  SELECT id, bank, nickname, closing_day, due_day FROM mecontrola.cards
   WHERE user_id='UID' AND deleted_at IS NULL;
  SELECT version FROM mecontrola.category_editorial_version;
  SELECT status, count(*) FROM mecontrola.workflow_runs
   WHERE correlation_key LIKE 'UID:%' GROUP BY status;
  SELECT count(*) FROM mecontrola.platform_runs WHERE resource_id='UID';
  SELECT id, name, allocated_cents FROM mecontrola.budget_allocations
   WHERE user_id='UID' AND category_name ILIKE '%Supermercado%';

S0.3 — BASELINE MÉTRICAS (anotar valores de todas as séries)
  agent_runs_total
  agent_empty_completion_retries_total
  agent_guard_decisions_total
  agents_write_total
  agents_query_day_total
  agents_whatsapp_inbound_total
  agents_transaction_write_false_success_total
  workflow_runs_total
  workflow_stale_suspended_reaped_total
  workflow_version_conflict_total

S0.4 — PRÉ-CONDIÇÃO: cartão "XP" ativo (necessário para C1-C6, C8, C9).
  Se não houver com esse apelido exato, criar/renomear via agente antes de
  iniciar (SETUP, fora da contagem de cenários).

S0.5 — PRÉ-CONDIÇÃO: nenhum workflow_runs ativo/suspenso do usuário.
  Se houver: enviar "cancelar" no WhatsApp e confirmar status terminal.

================================================================================
[C1] DESPESA NO CRÉDITO + EDIÇÃO SEM TROCAR CATEGORIA
     Commits: 4ad802d, 7a21182, 8300022
     Fonte real: InvoiceItem d21c44d2-48a4-4b93-ab34-f8ade2b11bcf
       ("Github", XP Visa, PurchaseDate 2026-07-27, TotalAmount 69.65,
       Categoria "Serviços")
================================================================================
PRÉ-CONDIÇÃO: cartão "XP" ativo (S0.4); nenhum rascunho aberto (S0.5).

PASSO A — REGISTRAR
ENVIAR (frase exata; NÃO acrescentar data/texto após o apelido do cartão —
  guards/card_expense_shortcut.go:16-19 captura tudo após "no crédito" como
  apelido do cartão até o fim da string; qualquer palavra extra quebra o
  resolveCard):
  "gastei 69,65 no Github no crédito XP"
  (regex guards/card_expense_shortcut.go:16 head + :20 marker — description
  capturado = "Github", nickname = "XP")
SE PEDIR CATEGORIA: responder "Serviços" (ou índice correspondente)
QUANDO PEDIR CONFIRMAÇÃO: responder "sim"
ESPERADO: confirmação única e determinística (sem bloco duplicado — ver
  achado F-01/regra anti-simulação já corrigida), texto sem 💳 como
  substantivo, sem markdown incompatível (ver C9).
EVIDÊNCIA:
  SELECT id, description, amount_cents, category_snapshot, payment_method,
    card_id, origin_operation, version
  FROM mecontrola.transactions
  WHERE user_id='UID' AND description ILIKE '%github%'
  ORDER BY created_at DESC LIMIT 1;
  PROM: agent_guard_decisions_total{guard="card_expense_shortcut"}
  incrementado 1 (confirma que passou pelo atalho, não pelo LLM).

PASSO B — EDITAR SEM TROCAR CATEGORIA (valida 4ad802d + 7a21182 + 8300022)
ENVIAR (frase exata; aciona guards/edit_entry_correction_shortcut.go:17
  editEntryTwoAmountsGasteiRe — NÃO usar "errei"/"foi", o regex exige
  literalmente "no <desc> eu gastei <novo> e não <antigo>"):
  "no Github eu gastei 69,90 e não 69,65"
ESPERADO: bot NÃO pergunta categoria de novo (edição sem troca de categoria
  busca a versão editorial correta — commit 4ad802d), category_snapshot
  permanece a mesma evidência da criação (7a21182), card_id e
  installments preservados (8300022). Como o guard nunca envia entryId
  (só searchTerm="Github" + searchAmountCents=6965 + amountCents=6990,
  ver tools/edit_entry.go:24-27), esta mesma frase também exercita o
  fallback de busca por critério corrigido em 35f3388/a025f41/b1f1da7.
EVIDÊNCIA ADICIONAL:
  PROM: agent_guard_decisions_total{guard="edit_entry_correction_shortcut"}
  incrementado 1.
EVIDÊNCIA:
  SELECT id, amount_cents, category_snapshot, card_id, installments,
    category_editorial_version, version
  FROM mecontrola.transactions
  WHERE user_id='UID' AND description ILIKE '%github%'
  ORDER BY created_at DESC LIMIT 1;
  -- comparar category_snapshot e card_id ANTES/DEPOIS: devem ser idênticos.
  -- version deve ter incrementado exatamente 1.

================================================================================
[C2] EDIT_ENTRY — FALLBACK PARA BUSCA POR CRITÉRIO (entryId inválido/ausente)
     Commits: 35f3388, a025f41, b1f1da7
     Fonte real: InvoiceItem e8267375-97b2-411d-b9ab-1ece34b3d92f
       ("Whey", XP Visa, PurchaseDate 2026-07-27, TotalAmount 229.90,
       Categoria "Suplementos")
================================================================================
PRÉ-CONDIÇÃO: registrar antes (SETUP, fora da contagem), frase exata
  (guards/card_expense_shortcut.go — mesmo padrão do C1 passo A):
  "gastei 229,90 no Whey no crédito XP" → confirmar como no C1
  (categoria "Suplementos").

CENÁRIO — variante SEM valor antigo (só descrição como critério de busca;
  editEntryNewValueOnlyRe em guards/edit_entry_correction_shortcut.go:18):
ENVIAR (frase exata; regex exige literalmente "lançamento do/da <desc>,
  ... mudar/alterar/atualizar o valor para/pra <novo>"):
  "lançamento do whey, pode mudar o valor para 234,90"
ESPERADO: bot NÃO exige um identificador explícito; a tool edit_entry
  recebe searchTerm="whey" e amountCents=23490 SEM entryId e SEM
  searchAmountCents (tools/edit_entry.go:24-27 cai no ramo `else` que só
  usa SearchAmountCents/SearchTerm) — localiza o lançamento só pela
  descrição, exercitando o fallback por critério mais "raso" possível
  (35f3388 + a025f41 + b1f1da7): sem a separação entre campo de busca e
  campo de valor novo, 234,90 vazaria como critério de busca e o
  lançamento não seria encontrado.
EVIDÊNCIA:
  SELECT id, description, amount_cents, updated_at, version
  FROM mecontrola.transactions
  WHERE user_id='UID' AND description ILIKE '%whey%'
  ORDER BY created_at DESC LIMIT 1;
  -- amount_cents deve ser 23490; version incrementada.
  PROM: agent_guard_decisions_total{guard="edit_entry_correction_shortcut"}
  incrementado 1 (2º incremento do roteiro, após C1 passo B).
  LOGS: grep -F "edit_entry" no worker/server no intervalo — conferir que
  o tool caiu no ramo de busca por critério (não erro de entryId inválido).

================================================================================
[C3] GUARD DETERMINÍSTICO DE CORREÇÃO DE LANÇAMENTO (atalho sem LLM)
     Commit: 203c138
     Fonte real: InvoiceItem a5aa59d3-3e79-4a0a-8504-7ecc302c5793
       ("Porto Mercadinho", XP Visa, PurchaseDate 2026-07-27,
       TotalAmount 47.00, Categoria "Supermercado")
================================================================================
PRÉ-CONDIÇÃO: registrar antes (SETUP), frase exata (register_expense_shortcut
  — guards/register_expense_shortcut.go:16, conector "no" + pagamento "pix"
  reconhecido em :44):
  "gastei 47 no porto mercadinho no pix" → confirmar (categoria
  "Supermercado").

CENÁRIO — variante COM valor antigo e novo na mesma frase (3ª forma do
  guard: editEntryTwoAmountsLancamentoRe, guards/edit_entry_correction_shortcut.go:16):
ENVIAR (frase exata; regex exige literalmente "lançamento do/da <desc>, o
  valor certo é <novo> e não <antigo>" — diferente da frase do C2, que
  omite o valor antigo):
  "lançamento do porto mercadinho, o valor certo é 47,50 e não 47"
ESPERADO: correção aplicada diretamente (guard edit_entry_correction_shortcut,
  args searchTerm="porto mercadinho", amountCents=4750,
  searchAmountCents=4700), sem pedir categoria/forma de pagamento de novo,
  sem round-trip de LLM.
EVIDÊNCIA:
  SELECT id, amount_cents, version FROM mecontrola.transactions
  WHERE user_id='UID' AND description ILIKE '%porto mercadinho%'
  ORDER BY created_at DESC LIMIT 1; -- amount_cents = 4750
  PROM: agent_guard_decisions_total{guard="edit_entry_correction_shortcut"}
  deve ter incrementado 1 desde o baseline S0.3.

================================================================================
[C4] DESPESA NO CARTÃO SEM LLM (atalho determinístico) + FORCE REGISTRO
     APÓS RESOLVER CARTÃO
     Commits: acd2de9, 8d68018
     Fonte real: InvoiceItem 3739a66b-2944-4a8c-b784-829d2cee8a04
       ("Bom demais [Proteínas]", XP Visa, PurchaseDate 2026-07-27,
       TotalAmount 194.23, Categoria "Supermercado")
================================================================================
PRÉ-CONDIÇÃO: cartão "XP" ativo; nenhum rascunho aberto.

ENVIAR (frase exata, mesmo padrão de card_expense_shortcut do C1):
  "gastei 194,23 no bom demais no crédito XP"
ESPERADO: como o cartão já foi citado explicitamente por apelido exato no
  mesmo turno, o guard card_expense_shortcut registra sem round-trip de
  LLM (acd2de9) — resposta rápida pedindo só a categoria/confirmação.
SE PEDIR CATEGORIA: "Supermercado". CONFIRMAR: "sim".
EVIDÊNCIA:
  PROM: agent_guard_decisions_total{guard="card_expense_shortcut"}
  incrementado (2º incremento do roteiro, após C1 passo A).
  TRACE: span do turno deve ter ZERO chamada a llm.Provider.Complete antes
  do registro (ou 1 chamada só de confirmação determinística) — comparar
  com C1 que tem 1+ chamadas de LLM para resolver categoria livre.

CENÁRIO 2 (força registro após resolver cartão pendente — 8d68018):
PRÉ-CONDIÇÃO: registrar SEM apelido de cartão para forçar a pergunta —
  frase exata (guards/card_expense_shortcut.go:20 marker casa "no crédito"
  no fim da frase SEM texto de apelido depois, então nickname fica vazio
  e cardID="" — ver extractResolvedCardID retornando "" quando não há
  nickname; description = "whey tony"):
  "gastei 338,92 no whey tony no crédito"
ESPERADO: bot pergunta o apelido do cartão (sem 💳 como substantivo).
ENVIAR: "XP"
ESPERADO: assim que o cartão é resolvido, o registro é FORÇADO no mesmo
  fluxo (sem exigir nova confirmação redundante além da determinística
  padrão) — commit 8d68018.
EVIDÊNCIA:
  SELECT id, description, amount_cents, card_id, origin_operation
  FROM mecontrola.transactions
  WHERE user_id='UID' AND description ILIKE '%whey tony%'
  ORDER BY created_at DESC LIMIT 1;
  -- card_id deve ser o id do cartão XP; amount_cents=33892.
  (Fonte real: InvoiceItem f6e1a1c6-d99d-4b51-a5c0-739c84a8f947, "Whey
  [Tony]", XP Visa, 2026-07-27, R$ 338,92, categoria Suplementos.)

================================================================================
[C5] PAGAMENTO DE FATURA TRATADO COMO DESPESA
     Commit: 8d495f2
     Fonte real: Invoice 65ab4af1-3139-4688-b6f1-120eea78a261
       (cartão XP Visa, Total 3775.079, vencimento 2026-09-01 — fatura
       referente às compras de julho/2026)
================================================================================
PRÉ-CONDIÇÃO: nenhum rascunho aberto.

ENVIAR (frase exata; register_expense_shortcut.go:16 exige o VALOR logo
  após "paguei" para o atalho disparar — "paguei a fatura..." sem valor
  cai no roteamento por LLM em vez do atalho; a frase abaixo já traz o
  valor real da fatura e ainda assim inclui "cartão", cujo bloqueio é
  liberado justamente pela exceção introduzida em 8d495f2 quando a frase
  contém "fatura" — ver isExpenseCardContextBlocker em
  register_expense_shortcut.go:154):
  "paguei 3775,08 na fatura do cartão XP"
ESPERADO: o guard register_expense_shortcut reconhece "paguei <valor> na
  fatura..." como uma DESPESA (não pergunta de consulta) e registra direto
  (sem pedir forma de pagamento, pois nenhuma foi citada — paymentMethod
  fica vazio nesse caso e o tool deve perguntar/assumir conforme regra
  determinística de pagamento).
SE PEDIR FORMA DE PAGAMENTO: "pix".
EVIDÊNCIA:
  SELECT id, description, amount_cents, payment_method FROM
  mecontrola.transactions WHERE user_id='UID' AND description ILIKE '%fatura%'
  ORDER BY created_at DESC LIMIT 1;

================================================================================
[C6] FATURA INEXISTENTE COM FALLBACK
     Commit: 6393fd6
================================================================================
PRÉ-CONDIÇÃO: cartão "XP" ativo.
NOTA: diferente de C1-C5/C7/C10, este fluxo NÃO tem um guard determinístico
  dedicado (nenhum arquivo em guards/ faz shortcut de consulta de fatura) —
  a chamada à tool query_card_invoice acontece via function-calling do LLM.
  A frase abaixo é suficientemente natural para o LLM chamar a tool, mas
  não é uma string travada por regex como as demais; qualquer variação
  razoável também é válida para este cenário.

ENVIAR: "quanto foi a fatura do XP de janeiro"
ESPERADO (fallback determinístico, sem alucinar valor): bot informa que
  não encontrou fatura do cartão XP para o período pedido — texto real
  claro de "não encontrado", NUNCA um valor inventado.
EVIDÊNCIA:
  platform_messages do turno (texto exato do assistant).
  LOGS: grep -F "query_card_invoice" — deve aparecer o branch de fallback
  (ErrInvoiceNotFound ou equivalente introduzido em 6393fd6), NÃO um
  panic/erro genérico "Não consegui registrar. Tente novamente.".

================================================================================
[C7] CONSULTA DIÁRIA (query_day) — ROTEAMENTO VIA GUARD + FORMATO EM REAIS
     Commits: db59ea7, 570cf3e, 482941d, 3e1c1a6, a0fafa6
     Fonte real: InvoiceItems de 2026-07-27 e 2026-07-28 (ver lista completa
       em docs/runs/29-07-2026-dados-reais-financialcontroldb.txt), ex.:
       27/07 — Github 69,65 / Whey 229,90 / Porto Mercadinho 47,00 /
               Bom demais [Proteínas] 194,23 / Leve Mais 79,88 /
               Varejão Paraná 121,66 / Whey [Tony] 338,92 / Chatgpt 99,90 /
               Armazém Paraná 47,71 / Kimi [Moonshot AI] 100,54
       28/07 — Nasoar 178,08 / Energético 42,49 / Moonshot AI 207,66
================================================================================
PRÉ-CONDIÇÃO: os lançamentos de C1-C4 já criam massa própria; este cenário
  usa o dia da EXECUÇÃO REAL (hoje) — os lançamentos criados em C1-C6 no
  dia do teste devem aparecer na consulta de "hoje".

ENVIAR (frase exata; guards/query_day_shortcut.go:19 queryDayAmountRe é
  ancorado com ^...$ — a frase precisa ser só isso, sem prefixo/sufixo):
  "quanto eu gastei hoje"
ESPERADO: resposta roteada deterministicamente pelo guard query_day_shortcut
  (570cf3e), sem round-trip de LLM para decidir a tool; valores formatados
  em reais com vírgula (ex.: "R$ 194,23", NUNCA "194.23" ou "19423") —
  commit 482941d; soma e itens do dia batendo com os lançamentos reais
  criados nos cenários anteriores.
EVIDÊNCIA:
  SELECT sum(amount_cents), count(*) FROM mecontrola.transactions
  WHERE user_id='UID' AND created_at::date = current_date;
  -- comparar com o total dito pelo bot (mesma soma, formatação em R$).
  PROM: agents_query_day_total incrementado 1;
  agent_guard_decisions_total{guard="query_day_shortcut"} incrementado 1.

ENVIAR (frase exata; NÃO usar "e ontem, ..." — o regex é ancorado e não
  tolera prefixo antes de "quanto"):
  "quanto eu gastei ontem"
ESPERADO: mesma formatação em reais, escopo do dia anterior corretamente
  resolvido a partir da data injetada no runtime (112d428) — se ontem não
  teve lançamento do usuário de teste, resposta deve dizer explicitamente
  "nenhum gasto" (nunca inventar valor).
EVIDÊNCIA: platform_messages do turno + SELECT análogo com
  created_at::date = current_date - 1.

================================================================================
[C8] ALOCAÇÃO DE ORÇAMENTO NO DETALHE DA CATEGORIA
     Commit: bf68166
     Fonte real: categoria "Supermercado" concentra o maior volume de
       InvoiceItem em jun/jul 2026 (Compras do mês 1002,29 em 05/06;
       551,68 em 09/07; 340,64 em 14/07; entre outros).
================================================================================
PRÉ-CONDIÇÃO: orçamento ativo do mês com alocação para "Supermercado"
  (verificar em S0.2 budget_allocations; se ausente, configurar via
  agente antes como SETUP).

ENVIAR: "detalhe da categoria supermercado"
ESPERADO: resposta traz o valor de alocação de orçamento correto para a
  categoria (bug corrigido em bf68166 fazia o detalhe não resolver a
  alocação corretamente), gasto acumulado do mês e percentual — SEM que o
  percentual seja CALCULADO pelo LLM (ver C9, deve vir de campo
  determinístico).
EVIDÊNCIA:
  SELECT allocated_cents, spent_cents FROM mecontrola.budget_allocations
  WHERE user_id='UID' AND category_name ILIKE '%Supermercado%';
  -- comparar com os números ditos pelo bot (idênticos, sem arredondar
  diferente).

================================================================================
[C9] MARKDOWN INCOMPATÍVEL E PERCENTUAL PROIBIDOS + NORMALIZAÇÃO WHATSAPP
     Commits: 500af26, 93c734f
================================================================================
Este cenário é uma checagem TRANSVERSAL sobre as respostas já capturadas em
C1-C8 (não precisa de novo envio):

VERIFICAR em CADA texto de platform_messages (role=assistant) capturado:
  a) SEM markdown incompatível com WhatsApp (nada de `**negrito**` estilo
     Markdown puro sem conversão, headers `#`, tabelas) — commit 500af26.
  b) NENHUM percentual calculado pelo LLM: todo "%" que aparecer no texto
     deve vir de um campo determinístico (ex.: budget_allocations), nunca
     de uma conta feita livremente pelo modelo — commit 500af26.
  c) Texto passou pelo whatsapp_format_sanitizer (93c734f): sem sequências
     que quebrem a renderização do WhatsApp (ex.: itálico mal fechado).
EVIDÊNCIA: colar o texto exato de cada resposta relevante + marcar
  PASS/FAIL por item (a)/(b)/(c).

================================================================================
[C10] PAGAMENTO EXPLÍCITO PRESERVADO EM DESPESAS (não pergunta de novo)
      Commit: 886853c
      Fonte real: BillItem 03a52470-8125-49d0-b9e7-5dc74fe32b51
        ("Faxina [Casa]", Bill 2026-07-01, Value 400,00)
================================================================================
PRÉ-CONDIÇÃO: nenhum rascunho aberto.

ENVIAR (frase exata; register_expense_shortcut.go:33 reconhece "em
  dinheiro" como sufixo de pagamento (cash) e o remove do corpo antes do
  regex principal — usar "na faxina", não "com a faxina": o conector
  "com" antes do artigo "a" faria o regex capturar "a faxina" como
  descrição literal em vez de "faxina"):
  "gastei 400 na faxina em dinheiro"
ESPERADO: forma de pagamento "dinheiro" já foi dita explicitamente no
  mesmo turno — bot NÃO deve perguntar "qual foi a forma de pagamento?"
  de novo (regressão corrigida em 886853c); deve pedir só a categoria se
  não for óbvia, e então confirmar.
SE PEDIR CATEGORIA: "Casa" (ou equivalente).
EVIDÊNCIA:
  SELECT payment_method, description FROM mecontrola.transactions
  WHERE user_id='UID' AND description ILIKE '%faxina%'
  ORDER BY created_at DESC LIMIT 1; -- deve refletir "dinheiro" (cash),
  não um valor default/pix por engano; description = "faxina" (limpa,
  sem o artigo).
  platform_messages do turno: NÃO deve haver pergunta de forma de
  pagamento após o texto já a ter citado.

================================================================================
[C11] GUARD expired_without_tool — TEXTO RESTRITO AO TURNO REAL DE EXPIRAÇÃO
      Commits: 32fd0f1, b3fde48
================================================================================
PRÉ-CONDIÇÃO: nenhum rascunho aberto; TTL de confirmação padrão (consultar
  BUDGETS_PENDING_TTL_HORS / TTL do workflow de confirmação de despesa no
  .env de produção para saber quanto esperar, ou usar cenário acelerado se
  houver flag de teste).

PASSO A — abrir uma confirmação pendente e NÃO responder:
ENVIAR: "gastei 30 na padaria no pix"
SE PEDIR CATEGORIA: responder a categoria, mas NÃO responder a confirmação
  final ("sim"/"não") — deixar o workflow suspenso até expirar.

PASSO B — aguardar expiração do TTL, então enviar QUALQUER outra mensagem
  não relacionada:
ENVIAR: "oi"
ESPERADO (b3fde48): o texto de expiração SÓ deve aparecer no turno em que
  a expiração realmente ocorre (o próprio turno que dispara o resume
  expirado), nunca em turnos seguintes sem relação — commit b3fde48
  restringiu o guard expired_without_tool a isso. Enviar uma 2ª mensagem
  neutra depois ("tudo bem?") NÃO deve repetir o texto de expiração.
EVIDÊNCIA:
  SELECT status, suspend_reason, resumed_at FROM mecontrola.workflow_runs
  WHERE correlation_key LIKE 'UID:%' ORDER BY created_at DESC LIMIT 3;
  -- o run da padaria deve estar RunStatusFailed/cancelado por TTL, sem
  ficar suspenso indefinidamente.
  platform_messages: confirmar que o texto de expiração aparece 1x só,
  no turno correto, e a mensagem seguinte ("tudo bem?") tem resposta
  normal (sem repetir aviso de expiração).

================================================================================
[C12] RETRY AUTOMÁTICO EM CONCLUSÃO VAZIA DO LLM (checagem de observabilidade)
      Commit: e16dd2c (já coberto como baseline S0.1; aqui é checagem
      contínua ao longo de TODO o roteiro)
================================================================================
Este cenário não tem ENVIAR próprio — é uma auditoria da métrica ao final
de toda a execução:
  PROM: agent_empty_completion_retries_total (delta do baseline S0.3 até o
  fim do roteiro).
REGISTRAR: se a métrica incrementou em algum momento, qual cenário
  coincide (por timestamp), e se o resultado final do turno ainda assim
  foi correto (retry funcionando) ou se vazou uma resposta vazia/genérica
  para o usuário (regressão).

================================================================================
RESUMO DE COBERTURA — commits 32fd0f1..6393fd6
================================================================================
32fd0f1, b3fde48 ......... C11
35f3388, a025f41, b1f1da7  C2
8300022 .................. C1 (passo B)
4ad802d .................. C1 (passo B)
7a21182 .................. C1 (passo B)
203c138 .................. C3
e16dd2c .................. C12 (já coberto no S0.1 do roteiro 07-28; aqui
                            auditado como checagem contínua)
112d428 .................. C7 (segunda pergunta "ontem")
500af26 .................. C9
bf68166 .................. C8
ee77e1c .................. docs/evidência, sem superfície conversacional
                            própria (registro de resultados do roteiro
                            anterior) — sem cenário dedicado.
93c734f .................. C9
76e48d9 .................. teste (golden), sem superfície conversacional
                            própria — coberto indiretamente em C4/C1
                            (cartão sem emoji já é checado no texto).
886853c .................. C10
ae803ea .................. docs/evidência — sem cenário dedicado.
3cafc13, 0526ef0 ......... observabilidade de falha de guard — auditar via
                            PROM agent_guard_decisions_total{outcome="error"}
                            ao longo de todo o roteiro (sem ENVIAR próprio).
8d68018, acd2de9 ......... C4
8d495f2 .................. C5
db59ea7, 3e1c1a6, a0fafa6  C7
482941d .................. C7
ff0d8ef .................. teste (golden, suporte) — sem cenário dedicado
                            de fluxo (fluxo de suporte já é estático/tool
                            fixa; se quiser cobrir, enviar "ajuda" e
                            conferir resposta determinística de suporte).
570cf3e .................. C7
6393fd6 .................. C6

================================================================================
PRÓXIMO PASSO (fora deste arquivo)
================================================================================
Depois de rodar manualmente cada ENVIAR no WhatsApp com o número de teste,
colar as evidências reais (psql/logs/métricas/trace) em:
  docs/runs/29-07-2026-resultados-e2e-agente-jun-jul-2026.txt
no mesmo padrão de docs/runs/resultados-e2e-agente-2026-07-28.txt — PASS só
com evidência colada, qualquer divergência de texto vira FAIL/registrado,
zero resposta inventada.
```
