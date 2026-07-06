<!-- spec-version: 3 -->

# Cenários de Teste — Conversa Agentiva Fluida

Fonte canônica para harness determinístico (CA-12 / RF-33). Cada cenário nomeia a pendência esperada, a sequência de mensagens, as tool calls obrigatórias, o desfecho e as asserções de Run auditável. IDs de categoria são os do baseline `000001_initial_schema.up.sql`.

Abreviações de pagamento: **PIX** = pix, **DEB** = débito, **DIN** = dinheiro, **BOL** = boleto, **CC** = cartão de crédito.

## Convenção Global de Confirmação (spec-version 3) — OBRIGATÓRIA

A partir da `spec-version 3` (PRD D-10/RF-38, ADR-004), **toda escrita financeira exige confirmação humana explícita antes de persistir**. Para evitar duplicação em 134 tabelas, esta convenção é normativa e o harness DEVE aplicá-la a todo cenário:

- Toda linha `write:` de qualquer cenário é precedida por um **turno de confirmação implícito**: `Agent: "Confirma? <resumo: valor, categoria raiz > folha, data, pagamento>"` seguido de `User: <aceite explícito: "sim"/"confirmar"/"ok"/"pode">`. A escrita só ocorre APÓS o aceite.
- O estado imediatamente anterior à escrita é sempre `AwaitingSlot=confirmation`; o harness DEVE assertar a ordem `confirmation → write` no Run auditável e `M-07=0` (nenhuma escrita sem aceite).
- Cenários com `no-write` (cancelamento, expiração, bloqueio, substituição, erro de ledger) NÃO recebem o turno implícito de confirmação: já terminam sem escrita.
- Cenários que exibem confirmação de forma explícita na tabela (ex.: Grupo 9, Grupo 12) são a forma canônica literal; os demais herdam o turno implícito acima.
- Cancelamento no turno de confirmação (`"não"`/`"cancela"`) fecha `PendingStatus=cancelled` sem escrita; resposta ambígua na confirmação gera reprompt único (`ConfirmRepromptCount 0→1`) e, persistindo, cancela sem escrita.
- Seleção entre múltiplos candidatos aceita **índice numérico OU nome** (RF-42): ambos resolvem o mesmo par raiz+folha canônico antes da confirmação.

Onde uma tabela antiga mostrar `write:` logo após o fechamento do último slot sem exibir o turno de confirmação, leia-se o turno implícito inserido entre o fechamento do slot e a escrita.

---

## Grupo 1 — Custo Fixo (root: `66cb85a0` / `custo-fixo`)

### G1-01 Supermercado — categoria direta

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 320,00 no supermercado hoje no pix |
| 2 | Agent | (nenhuma pergunta) — categoria `supermercado` resolvida direto |
| — | — | write: Custo Fixo > Supermercado, R$ 320,00, pix |

**Asserções:** `AwaitingSlot=none`; `TransactionsLedger.CreateTransaction` chamado 1x; root=`66cb85a0`/`custo-fixo`, sub=`97fa4b86`/`supermercado`; status=`completed`.

---

### G1-02 Supermercado — resposta curta de categoria

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 320,00 no Carrefour hoje no pix |
| 2 | Agent | Qual categoria para essa compra no Carrefour? |
| 3 | User | supermercado |
| — | — | write: Custo Fixo > Supermercado, R$ 320,00, pix |

**Asserções:** `AwaitingSlot=category` aberta no T2; T3 fecha com sub=`97fa4b86`/`supermercado`; `CategoriesReader.ResolveForWrite` chamado; sem repetição de valor/pagamento.

---

### G1-03 Feira e Hortifruti

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei R$ 85,00 na feira ontem no dinheiro |
| 2 | Agent | Qual categoria? (ambíguo) |
| 3 | User | feira |
| — | — | write: Custo Fixo > Feira e Hortifruti, R$ 85,00, dinheiro |

**Asserções:** sub=`0c004f2d`/`feira-e-hortifruti`.

---

### G1-04 Padaria

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei R$ 28,00 na padaria hoje no débito |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Padaria, R$ 28,00, débito |

**Asserções:** sub=`0b549268`/`padaria`.

---

### G1-05 Açougue

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei carne hoje, R$ 130,00, débito |
| 2 | Agent | Qual categoria? |
| 3 | User | açougue |
| — | — | write: Custo Fixo > Açougue, R$ 130,00, débito |

**Asserções:** sub=`75af9f6b`/`acougue`.

---

### G1-06 Aluguel

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei R$ 1.800,00 de aluguel hoje no boleto |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Aluguel, R$ 1800,00, boleto |

**Asserções:** sub=`c2fda6a3`/`aluguel`.

---

### G1-07 Condomínio

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Boleto do condomínio venceu hoje, R$ 620,00 |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Condomínio, R$ 620,00, boleto |

**Asserções:** sub=`d0b1fa13`/`condominio`.

---

### G1-08 Energia

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Conta de luz veio R$ 210,00, paguei no pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Energia, R$ 210,00, pix |

**Asserções:** sub=`36916fab`/`energia`.

---

### G1-09 Água

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei R$ 95,00 de água hoje, débito |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Água, R$ 95,00, débito |

**Asserções:** sub=`fa93273d`/`agua`.

---

### G1-10 Gás

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 120,00 no gás hoje, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Gás, R$ 120,00, pix |

**Asserções:** sub=`4e6f8b6b`/`gas`.

---

### G1-11 Internet

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Mensalidade da internet R$ 130,00, debitou hoje |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Internet, R$ 130,00, débito |

**Asserções:** sub=`9391ac38`/`internet`.

---

### G1-12 Telefonia

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei R$ 80,00 de celular, boleto |
| 2 | Agent | Qual categoria? |
| 3 | User | telefonia |
| — | — | write: Custo Fixo > Telefonia, R$ 80,00, boleto |

**Asserções:** sub=`7319ba14`/`telefonia`.

---

### G1-13 TV por Assinatura

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei R$ 220,00 da Sky hoje, boleto |
| 2 | Agent | Qual categoria? |
| 3 | User | tv por assinatura |
| — | — | write: Custo Fixo > TV por Assinatura, R$ 220,00, boleto |

**Asserções:** sub=`2e90fdd3`/`tv-por-assinatura`.

---

### G1-14 Transporte Público

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Coloquei R$ 100,00 no Bilhete Único hoje, débito |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Transporte Público, R$ 100,00, débito |

**Asserções:** sub=`007c090e`/`transporte-publico`.

---

### G1-15 Combustível

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Abasteci hoje R$ 280,00 no pix |
| 2 | Agent | Qual categoria? |
| 3 | User | combustível |
| — | — | write: Custo Fixo > Combustível, R$ 280,00, pix |

**Asserções:** sub=`cb13d50d`/`combustivel`.

---

### G1-16 Estacionamento Mensal

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei R$ 350,00 de estacionamento mensal, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Estacionamento Mensal, R$ 350,00, pix |

**Asserções:** sub=`7e647851`/`estacionamento-mensal`.

---

### G1-17 Pedágio

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei R$ 12,00 de pedágio hoje, dinheiro |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Pedágio, R$ 12,00, dinheiro |

**Asserções:** sub=`9dc2ed94`/`pedagio`.

---

### G1-18 Manutenção Veicular

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Fiz revisão do carro hoje, R$ 850,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | manutenção do carro |
| — | — | write: Custo Fixo > Manutenção Veicular, R$ 850,00, pix |

**Asserções:** sub=`bf2fcca0`/`manutencao-veicular`.

---

### G1-19 IPVA e Licenciamento

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei IPVA hoje R$ 1.200,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > IPVA e Licenciamento, R$ 1200,00, pix |

**Asserções:** sub=`311c7b7f`/`ipva-e-licenciamento`.

---

### G1-20 Seguro Veicular

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Renovei seguro do carro hoje, R$ 3.500,00, boleto |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Seguro Veicular, R$ 3500,00, boleto |

**Asserções:** sub=`75e7909d`/`seguro-veicular`.

---

### G1-21 Plano de Saúde

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Debito do plano de saúde caiu hoje, R$ 680,00 |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Plano de Saúde, R$ 680,00, débito |

**Asserções:** sub=`c8f579ea`/`plano-de-saude`.

---

### G1-22 Plano Odontológico

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei R$ 120,00 do plano odontológico, boleto |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Plano Odontológico, R$ 120,00, boleto |

**Asserções:** sub=`1af66343`/`plano-odontologico`.

---

### G1-23 Consultas e Exames

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Fiz exame de sangue hoje, R$ 180,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | consultas |
| — | — | write: Custo Fixo > Consultas e Exames, R$ 180,00, pix |

**Asserções:** sub=`af5619e0`/`consultas-e-exames`.

---

### G1-24 Medicamentos Contínuos

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei remédio de pressão esse mês, R$ 145,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | medicamentos contínuos |
| — | — | write: Custo Fixo > Medicamentos Contínuos, R$ 145,00, pix |

**Asserções:** sub=`157b18fe`/`medicamentos-continuos`.

---

### G1-25 Medicamentos e Farmácia

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 150,00 na farmácia hoje, no pix |
| 2 | Agent | Qual categoria? |
| 3 | User | farmácia |
| — | — | write: Custo Fixo > Medicamentos e Farmácia, R$ 150,00, pix |

**Asserções:** sub=`3ca95dd5`/`medicamentos-e-farmacia`; valor/pagamento NÃO reSolicitados.

---

### G1-26 Odontologia

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei consulta dentista hoje, R$ 250,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | dentista |
| — | — | write: Custo Fixo > Odontologia, R$ 250,00, pix |

**Asserções:** sub=`4ded7fd4`/`odontologia`.

---

### G1-27 Terapia e Saúde Mental

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei sessão de terapia hoje, R$ 220,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Terapia e Saúde Mental, R$ 220,00, pix |

**Asserções:** sub=`a15cba16`/`terapia-e-saude-mental`.

---

### G1-28 Escola e Creche

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Mensalidade da creche venceu hoje, R$ 1.100,00, boleto |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Escola e Creche, R$ 1100,00, boleto |

**Asserções:** sub=`cab69263`/`escola-e-creche`.

---

### G1-29 Faculdade e Pós-graduação

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei mensalidade da faculdade hoje, R$ 1.400,00, boleto |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Faculdade e Pós-graduação, R$ 1400,00, boleto |

**Asserções:** sub=`46e492a0`/`faculdade-e-pos-graduacao`.

---

### G1-30 Pensão Alimentícia

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei pensão esse mês, R$ 2.200,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | pensão |
| — | — | write: Custo Fixo > Pensão Alimentícia, R$ 2200,00, pix |

**Asserções:** sub=`5828e634`/`pensao-alimenticia`.

---

### G1-31 Assinaturas Essenciais

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Renovou Adobe hoje, R$ 79,00, cartão de crédito |
| 2 | Agent | Qual categoria? |
| 3 | User | assinatura |
| — | — | write: Custo Fixo > Assinaturas Essenciais |

**Asserções:** sub=`178d590e`/`assinaturas-essenciais`.

---

### G1-32 Tarifas Bancárias

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Descontaram R$ 35,00 de tarifa do banco hoje |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Tarifas Bancárias, R$ 35,00, débito |

**Asserções:** sub=`347e0488`/`tarifas-bancarias`.

---

### G1-33 Impostos e Tributos

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei DAS hoje, R$ 460,00, boleto |
| 2 | Agent | Qual categoria? |
| 3 | User | imposto |
| — | — | write: Custo Fixo > Impostos e Tributos, R$ 460,00, boleto |

**Asserções:** sub=`7d56377d`/`impostos-e-tributos`.

---

### G1-34 Empréstimos e Financiamentos

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei parcela do financiamento hoje, R$ 1.100,00, débito |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Empréstimos e Financiamentos, R$ 1100,00, débito |

**Asserções:** sub=`b29895dd`/`emprestimos-e-financiamentos`.

---

### G1-35 Dívidas e Juros

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei juros do cartão hoje, R$ 230,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | dívidas e juros |
| — | — | write: Custo Fixo > Dívidas e Juros, R$ 230,00, pix |

**Asserções:** sub=`5b9a1cba`/`dividas-e-juros`.

---

### G1-36 Manutenção da Casa

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei encanador hoje, R$ 400,00, dinheiro |
| 2 | Agent | Qual categoria? |
| 3 | User | manutenção da casa |
| — | — | write: Custo Fixo > Manutenção da Casa, R$ 400,00, dinheiro |

**Asserções:** sub=`3f7c80e0`/`manutencao-da-casa`.

---

### G1-37 Serviços Domésticos

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei diarista hoje, R$ 200,00, dinheiro |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Serviços Domésticos, R$ 200,00, dinheiro |

**Asserções:** sub=`ca8e4a6c`/`servicos-domesticos`.

---

### G1-38 Pets Recorrentes

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei mensalidade pet shop, R$ 180,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | pets |
| — | — | write: Custo Fixo > Pets Recorrentes, R$ 180,00, pix |

**Asserções:** sub=`d4b74050`/`pets-recorrentes`.

---

### G1-39 Financiamento Imobiliário

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei parcela do apê hoje, R$ 2.800,00, débito |
| 2 | Agent | Qual categoria? |
| 3 | User | financiamento imobiliário |
| — | — | write: Custo Fixo > Financiamento Imobiliário, R$ 2800,00, débito |

**Asserções:** sub=`f9d9e5b6`/`financiamento-imobiliario`.

---

### G1-40 Seguro Residencial

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei seguro do apartamento hoje, R$ 900,00, boleto |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > Seguro Residencial, R$ 900,00, boleto |

**Asserções:** sub=`0abec125`/`seguro-residencial`.

---

### G1-41 Transporte por Aplicativo Recorrente

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Assinou plano mensal Uber, R$ 79,90, cartão de crédito |
| 2 | Agent | Qual categoria? |
| 3 | User | transporte por aplicativo |
| — | — | write: Custo Fixo > Transporte por Aplicativo Recorrente |

**Asserções:** sub=`c13dcc6e`/`transporte-por-aplicativo-recorrente`.

---

### G1-42 Seguros Pessoais

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei seguro de vida hoje, R$ 350,00, boleto |
| 2 | Agent | Qual categoria? |
| 3 | User | seguros pessoais |
| — | — | write: Custo Fixo > Seguros Pessoais, R$ 350,00, boleto |

**Asserções:** sub=`6a0d56cc`/`seguros-pessoais`.

---

### G1-43 IPTU

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei parcela do IPTU hoje, R$ 480,00, boleto |
| 2 | Agent | (resolve direto) |
| — | — | write: Custo Fixo > IPTU, R$ 480,00, boleto |

**Asserções:** sub=`80a870e9`/`iptu`.

---

### G1-44 Taxas Residenciais

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei taxa de lixo hoje, R$ 60,00, boleto |
| 2 | Agent | Qual categoria? |
| 3 | User | taxa residencial |
| — | — | write: Custo Fixo > Taxas Residenciais, R$ 60,00, boleto |

**Asserções:** sub=`8eaa0160`/`taxas-residenciais`.

---

## Grupo 2 — Conhecimento (root: `8314f021` / `conhecimento`)

### G2-01 Cursos e Treinamentos

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei R$ 890,00 num curso de programação hoje, cartão |
| 2 | Agent | Qual categoria? |
| 3 | User | cursos |
| — | — | write: Conhecimento > Cursos e Treinamentos |

**Asserções:** root=`8314f021`/`conhecimento`, sub=`b3a4824f`/`cursos-e-treinamentos`.

---

### G2-02 Plataformas de Ensino

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Assinei Alura hoje, R$ 139,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | plataforma de ensino |
| — | — | write: Conhecimento > Plataformas de Ensino, R$ 139,00, pix |

**Asserções:** sub=`01b51d39`/`plataformas-de-ensino`.

---

### G2-03 Livros e E-books

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei 3 livros na Amazon, R$ 180,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | livros |
| — | — | write: Conhecimento > Livros e E-books, R$ 180,00, pix |

**Asserções:** sub=`bac52783`/`livros-e-ebooks`.

---

### G2-04 Certificações

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei prova AWS hoje, R$ 1.200,00, cartão de crédito |
| 2 | Agent | Qual categoria? |
| 3 | User | certificação |
| — | — | write: Conhecimento > Certificações |

**Asserções:** sub=`654552ab`/`certificacoes`.

---

### G2-05 Idiomas

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei mensalidade inglês hoje, R$ 320,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | inglês |
| — | — | write: Conhecimento > Idiomas, R$ 320,00, pix |

**Asserções:** sub=`fec9aed9`/`idiomas`.

---

### G2-06 Mentoria e Coaching

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei sessão de mentoria hoje, R$ 500,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | mentoria |
| — | — | write: Conhecimento > Mentoria e Coaching, R$ 500,00, pix |

**Asserções:** sub=`8d114d26`/`mentoria-e-coaching`.

---

### G2-07 Congressos e Workshops

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Inscrição no evento de tecnologia, R$ 350,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | workshop |
| — | — | write: Conhecimento > Congressos e Workshops, R$ 350,00, pix |

**Asserções:** sub=`3c5e9972`/`congressos-e-workshops`.

---

### G2-08 Aulas Particulares

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei aula particular de matemática hoje, R$ 120,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | aulas particulares |
| — | — | write: Conhecimento > Aulas Particulares, R$ 120,00, pix |

**Asserções:** sub=`ce2850ad`/`aulas-particulares`.

---

### G2-09 Material de Estudo

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei apostila concurso, R$ 95,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | material de estudo |
| — | — | write: Conhecimento > Material de Estudo, R$ 95,00, pix |

**Asserções:** sub=`6f70f7d5`/`material-de-estudo`.

---

### G2-10 Software e Ferramentas de Estudo

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Assinei Notion Pro hoje, R$ 49,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | ferramenta de estudo |
| — | — | write: Conhecimento > Software e Ferramentas de Estudo, R$ 49,00, pix |

**Asserções:** sub=`4850d076`/`software-e-ferramentas-de-estudo`.

---

## Grupo 3 — Prazeres (root: `ac535261` / `prazeres`)

### G3-01 Delivery

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Pedi comida no iFood hoje, R$ 95,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Prazeres > Delivery, R$ 95,00, pix |

**Asserções:** root=`ac535261`/`prazeres`, sub=`ddbb0dc7`/`delivery`.

---

### G3-02 Restaurantes

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Fui almoçar fora hoje, R$ 140,00, débito |
| 2 | Agent | Qual categoria? |
| 3 | User | restaurante |
| — | — | write: Prazeres > Restaurantes, R$ 140,00, débito |

**Asserções:** sub=`d539672d`/`restaurantes`.

---

### G3-03 Bares e Lanches

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Saí com os amigos hoje, gastei R$ 80,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | bares |
| — | — | write: Prazeres > Bares e Lanches, R$ 80,00, pix |

**Asserções:** sub=`a371851d`/`bares-e-lanches`.

---

### G3-04 Cafeterias

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Tomei café com amiga hoje, R$ 45,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | cafeteria |
| — | — | write: Prazeres > Cafeterias, R$ 45,00, pix |

**Asserções:** sub=`a20b4072`/`cafeterias`.

---

### G3-05 Streaming de Vídeo

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei Netflix hoje, R$ 55,90, cartão de crédito |
| 2 | Agent | (resolve direto) |
| — | — | write: Prazeres > Streaming de Vídeo |

**Asserções:** sub=`85e56497`/`streaming-de-video`.

---

### G3-06 Música e Áudio

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Assinei Spotify hoje, R$ 21,90, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Prazeres > Música e Áudio |

**Asserções:** sub=`8580a31d`/`musica-e-audio`.

---

### G3-07 Games e Assinaturas de Jogos

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei jogo no PS Store, R$ 299,00, cartão |
| 2 | Agent | Qual categoria? |
| 3 | User | games |
| — | — | write: Prazeres > Games e Assinaturas de Jogos |

**Asserções:** sub=`514c00a0`/`games-e-assinaturas-de-jogos`.

---

### G3-08 Cinema e Teatro

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Fui ao cinema hoje, R$ 80,00, débito |
| 2 | Agent | (resolve direto) |
| — | — | write: Prazeres > Cinema e Teatro, R$ 80,00, débito |

**Asserções:** sub=`5190df3d`/`cinema-e-teatro`.

---

### G3-09 Shows e Eventos

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei ingresso show hoje, R$ 360,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Prazeres > Shows e Eventos, R$ 360,00, pix |

**Asserções:** sub=`09073cdd`/`shows-e-eventos`.

---

### G3-10 Passeios e Parques

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Fui ao parque hoje com a família, R$ 60,00, dinheiro |
| 2 | Agent | Qual categoria? |
| 3 | User | passeio |
| — | — | write: Prazeres > Passeios e Parques, R$ 60,00, dinheiro |

**Asserções:** sub=`aed45dcf`/`passeios-e-parques`.

---

### G3-11 Viagens de Lazer

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei passagem aérea, R$ 1.200,00, cartão de crédito |
| 2 | Agent | Qual categoria? |
| 3 | User | viagem de lazer |
| — | — | write: Prazeres > Viagens de Lazer |

**Asserções:** sub=`0134668f`/`viagens-de-lazer`.

---

### G3-12 Hospedagem de Lazer

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Reservei hotel hoje, R$ 800,00, cartão de crédito |
| 2 | Agent | Qual categoria? |
| 3 | User | hospedagem |
| — | — | write: Prazeres > Hospedagem de Lazer |

**Asserções:** sub=`7a69762f`/`hospedagem-de-lazer`.

---

### G3-13 Roupas e Calçados

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei tênis hoje, R$ 450,00, cartão de crédito |
| 2 | Agent | Qual categoria? |
| 3 | User | roupas |
| — | — | write: Prazeres > Roupas e Calçados |

**Asserções:** sub=`14416063`/`roupas-e-calcados`.

---

### G3-14 Beleza e Estética

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Fui ao salão hoje, R$ 180,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Prazeres > Beleza e Estética, R$ 180,00, pix |

**Asserções:** sub=`f9656739`/`beleza-e-estetica`.

---

### G3-15 Esportes e Academia

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Mensalidade da academia, R$ 160,00, débito |
| 2 | Agent | (resolve direto) |
| — | — | write: Prazeres > Esportes e Academia, R$ 160,00, débito |

**Asserções:** sub=`c0e10d9f`/`esportes-e-academia`.

---

### G3-16 Presentes

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei presente de aniversário hoje, R$ 220,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Prazeres > Presentes, R$ 220,00, pix |

**Asserções:** sub=`09e7cd05`/`presentes`.

---

### G3-17 Hobbies

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei material de pintura hoje, R$ 190,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | hobby |
| — | — | write: Prazeres > Hobbies, R$ 190,00, pix |

**Asserções:** sub=`671873dc`/`hobbies`.

---

### G3-18 Doações

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Fiz doação de R$ 100,00 para ONG hoje, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Prazeres > Doações, R$ 100,00, pix |

**Asserções:** sub=`c2470946`/`doacoes`.

---

### G3-19 Pets Não Recorrentes

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Levei o cachorro ao vet hoje, R$ 320,00, cartão |
| 2 | Agent | Qual categoria? |
| 3 | User | veterinário |
| — | — | write: Prazeres > Pets Não Recorrentes |

**Asserções:** sub=`481d2d82`/`pets-nao-recorrentes`.

---

### G3-20 Transporte de Lazer

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei Uber para sair hoje à noite, R$ 45,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | uber lazer |
| — | — | write: Prazeres > Transporte de Lazer, R$ 45,00, pix |

**Asserções:** sub=`6feeb8fd`/`transporte-de-lazer`.

---

### G3-21 Compras Pessoais

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei perfume hoje, R$ 350,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | compras pessoais |
| — | — | write: Prazeres > Compras Pessoais, R$ 350,00, pix |

**Asserções:** sub=`a2af4429`/`compras-pessoais`.

---

## Grupo 4 — Metas (root: `f133508e` / `metas`)

### G4-01 Tecnologia

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei notebook hoje, R$ 5.500,00, cartão 10x |
| 2 | Agent | Qual categoria? |
| 3 | User | tecnologia |
| — | — | write: Metas > Tecnologia, R$ 5500,00, cartão 10x |

**Asserções:** root=`f133508e`/`metas`, sub=`3ff5e6b5`/`tecnologia`; `installments=10`.

---

### G4-02 Veículo

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Dei entrada no carro hoje, R$ 15.000,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | veículo |
| — | — | write: Metas > Veículo, R$ 15000,00, pix |

**Asserções:** sub=`ef1a26ec`/`veiculo`.

---

### G4-03 Casa e Reforma

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei parte da reforma hoje, R$ 8.000,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | reforma |
| — | — | write: Metas > Casa e Reforma, R$ 8000,00, pix |

**Asserções:** sub=`61698c19`/`casa-e-reforma`.

---

### G4-04 Viagem Planejada

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei pacote de viagem Europa, R$ 12.000,00, cartão 12x |
| 2 | Agent | Qual categoria? |
| 3 | User | viagem planejada |
| — | — | write: Metas > Viagem Planejada, cartão 12x |

**Asserções:** sub=`8a4228f0`/`viagem-planejada`; `installments=12`.

---

### G4-05 Casamento e Festa

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei buffet do casamento, R$ 25.000,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | casamento |
| — | — | write: Metas > Casamento e Festa, R$ 25000,00, pix |

**Asserções:** sub=`6752f218`/`casamento-e-festa`.

---

### G4-06 Empreendedorismo

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Investei no meu negócio hoje, R$ 3.000,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | empreendedorismo |
| — | — | write: Metas > Empreendedorismo, R$ 3000,00, pix |

**Asserções:** sub=`480b8f7d`/`empreendedorismo`.

---

### G4-07 Quitação de Dívidas

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Quitei dívida do cartão hoje, R$ 7.800,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | quitação de dívida |
| — | — | write: Metas > Quitação de Dívidas, R$ 7800,00, pix |

**Asserções:** sub=`946643a8`/`quitacao-de-dividas`.

---

## Grupo 5 — Liberdade Financeira (root: `35ced21e` / `liberdade-financeira`)

### G5-01 Reserva de Emergência

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Aportei R$ 500,00 na reserva de emergência hoje, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Liberdade Financeira > Reserva de Emergência, R$ 500,00, pix |

**Asserções:** root=`35ced21e`/`liberdade-financeira`, sub=`45c7e533`/`reserva-de-emergencia`.

---

### G5-02 Tesouro Direto

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei Tesouro Selic hoje, R$ 1.000,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Liberdade Financeira > Tesouro Direto, R$ 1000,00, pix |

**Asserções:** sub=`9103a0e6`/`tesouro-direto`.

---

### G5-03 CDB e RDB

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Apliquei R$ 5.000,00 no CDB do Nubank hoje, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Liberdade Financeira > CDB e RDB, R$ 5000,00, pix |

**Asserções:** sub=`ee26c4d9`/`cdb-e-rdb`.

---

### G5-04 Ações

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei ações hoje, R$ 2.000,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | ações |
| — | — | write: Liberdade Financeira > Ações, R$ 2000,00, pix |

**Asserções:** sub=`e1266272`/`acoes`.

---

### G5-05 Fundos Imobiliários

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei cotas de FII hoje, R$ 1.500,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Liberdade Financeira > Fundos Imobiliários, R$ 1500,00, pix |

**Asserções:** sub=`1e5b4db2`/`fundos-imobiliarios`.

---

### G5-06 Previdência Privada

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Aportei R$ 600,00 na previdência este mês, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Liberdade Financeira > Previdência Privada, R$ 600,00, pix |

**Asserções:** sub=`b1ac9b12`/`previdencia-privada`.

---

### G5-07 Criptoativos

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei Bitcoin hoje, R$ 800,00, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | cripto |
| — | — | write: Liberdade Financeira > Criptoativos, R$ 800,00, pix |

**Asserções:** sub=`9747b1c4`/`criptoativos`.

---

### G5-08 Aportes em Corretora

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Fiz aporte na corretora hoje, R$ 3.000,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: Liberdade Financeira > Aportes em Corretora, R$ 3000,00, pix |

**Asserções:** sub=`866793cb`/`aportes-em-corretora`.

---

## Grupo 6 — Receitas (income)

### G6-01 Salário — receita direta

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi meu salário hoje, R$ 8.500,00 |
| 2 | Agent | (resolve direto) |
| — | — | write: income, root=`86dd34b0`/`salario` — subcategoria: bloqueado sem folha |

**Asserções:** root=`86dd34b0` não possui subcategoria folha → BLOQUEIA; agente pede subcategoria.

---

### G6-02 Salário — com subcategoria

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi salário hoje, R$ 8.500,00 |
| 2 | Agent | Qual subcategoria? (ex: Salário, Décimo Terceiro, Férias…) |
| 3 | User | salário |
| — | — | write: income, root=`86dd34b0`/`salario`, sub=`86dd34b0` folha não existe → Agent deve apresentar folhas válidas |

**Notas:** `salario` como root não tem UUID de folha própria — o MVP trata a folha como a própria `salario` (se for raiz folha combinada) ou exige que o usuário escolha uma das folhas reais. Verificar contrato de `ResolveForWrite` para esse caso.

---

### G6-03 Décimo Terceiro

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi 13º salário hoje, R$ 4.200,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: income, root=`86dd34b0`/`salario`, sub=`98455e74`/`decimo-terceiro` |

**Asserções:** sub=`98455e74`/`decimo-terceiro`.

---

### G6-04 PLR e Bônus

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi bônus da empresa hoje, R$ 5.000,00, pix |
| 2 | Agent | Qual categoria para essa entrada? |
| 3 | User | PLR |
| — | — | write: income, root=`86dd34b0`/`salario`, sub=`4b61504d`/`plr-e-bonus` |

---

### G6-05 Vale-Alimentação

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Caiu vale alimentação hoje, R$ 800,00 |
| 2 | Agent | (resolve direto) |
| — | — | write: income, sub=`bbc5809c`/`vale-alimentacao` |

---

### G6-06 Vale-Refeição

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi vale refeição hoje, R$ 650,00 |
| 2 | Agent | (resolve direto) |
| — | — | write: income, sub=`6e896533`/`vale-refeicao` |

---

### G6-07 Freelance

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi pagamento de freela hoje, R$ 2.000,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: income, root=`275ef473`/`renda-variavel`, sub=`403192d5`/`freelance` |

---

### G6-08 Consultoria

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi R$ 3.500,00 de consultoria hoje, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: income, root=`275ef473`/`renda-variavel`, sub=`0d613676`/`consultoria` |

---

### G6-09 Dividendos

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi dividendos de FII esse mês, R$ 420,00 |
| 2 | Agent | (resolve direto) |
| — | — | write: income, root=`1c801292`/`investimentos`, sub=`5b983987`/`dividendos` |

---

### G6-10 Rendimentos

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Rendimento do CDB caiu hoje, R$ 180,00 |
| 2 | Agent | (resolve direto) |
| — | — | write: income, root=`1c801292`/`investimentos`, sub=`c8276187`/`rendimentos` |

---

### G6-11 Aluguel Residencial Recebido

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi aluguel do apartamento hoje, R$ 2.100,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: income, root=`6044ffc4`/`aluguel-recebido`, sub=`00b886a7`/`aluguel-residencial-recebido` |

---

### G6-12 Cashback

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi cashback do cartão hoje, R$ 87,00 |
| 2 | Agent | (resolve direto) |
| — | — | write: income, root=`c0c8b110`/`restituicoes-e-cashback`, sub=`3791836d`/`cashback` |

---

### G6-13 Restituição de IR

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Caiu restituição do IR hoje, R$ 1.340,00 |
| 2 | Agent | (resolve direto) |
| — | — | write: income, root=`c0c8b110`/`restituicoes-e-cashback`, sub=`7a17fe1c`/`restituicao-de-ir` |

---

### G6-14 Vendas Diversas

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Vendi um celular hoje, R$ 900,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: income, root=`8dba4d69`/`vendas`, sub=`6650a195`/`vendas-diversas` |

---

### G6-15 Marketplace

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi pagamento do Mercado Livre hoje, R$ 450,00, pix |
| 2 | Agent | (resolve direto) |
| — | — | write: income, root=`8dba4d69`/`vendas`, sub=`52ded4b8`/`marketplace` |

---

## Grupo 7 — Fluxos de Pendência (Controle de Estado)

### G7-01 Substituição de pendência por nova frase completa (CA-02 / RF-31)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 150,00 no mercado hoje, no pix |
| 2 | Agent | Qual categoria para essa compra no mercado? |
| 3 | User | Gastei R$ 150,00 na farmácia hoje, no pix |
| 4 | Agent | (processa farmácia como nova operação; pendência mercado = `replaced`) |
| — | — | pendência mercado: status=`replaced`, no-write; farmácia: write Custo Fixo > Medicamentos e Farmácia |

**Asserções:** pendência anterior status=`replaced`; T3 contém valor+verbo+pagamento → detectado como nova operação; sub=`3ca95dd5`/`medicamentos-e-farmacia`; zero escrita sobre lançamento de mercado.

---

### G7-02 Substituição seguida de resposta compatível com pendência antiga (CA-11 / RF-32)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 150,00 no mercado hoje, no pix |
| 2 | Agent | Qual categoria? |
| 3 | User | Gastei R$ 150,00 na farmácia hoje, no pix |
| 4 | Agent | (farmácia processada; mercado = `replaced`) |
| 5 | User | custo fixo |
| 6 | Agent | (ignora pendência substituída; trata T5 como nova intenção ou pergunta o que deseja registrar) |
| — | — | zero write de mercado; pendência antiga não reativada |

**Asserções:** status pendência mercado=`replaced` permanece; T5 não reativa pendência; zero duplicidade.

---

### G7-03 Resposta "custo fixo" sem subcategoria folha → bloqueia (CA-09 / RF-30)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 200,00 numa loja hoje, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | custo fixo |
| 4 | Agent | Custo fixo tem várias subcategorias. Qual delas se encaixa melhor: Aluguel, Supermercado, Energia...? |
| — | — | pendência continua ativa; nenhuma escrita até folha válida informada |

**Asserções:** raiz sem folha bloqueada; `AwaitingSlot=category` permanece; `TransactionsLedger` não chamado.

---

### G7-04 Cancelamento explícito (CA-05 / RF-08)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 300,00 em material de construção, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | cancela |
| 4 | Agent | Tudo certo, o registro foi cancelado. |
| — | — | pendência status=`cancelled`; zero write |

**Asserções:** `TransactionsLedger` não chamado; status=`cancelled`; resposta sem mencionar valor ou categoria.

---

### G7-05 Cancelamento variação "deixa pra lá" (RF-08)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei R$ 90,00 num bar, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | deixa pra lá |
| 4 | Agent | Ok, não registrei nada. |
| — | — | status=`cancelled`; zero write |

---

### G7-06 Cancelamento variação "não registra" (RF-08)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 50,00 de uber hoje |
| 2 | Agent | Qual categoria? |
| 3 | User | não registra |
| — | — | status=`cancelled`; zero write |

---

### G7-07 Resposta "sim e pix" em pendência de categoria (CA-03)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 150,00 no mercado hoje, no pix |
| 2 | Agent | Qual categoria para essa compra no mercado? |
| 3 | User | sim e pix |
| 4 | Agent | (não interpreta "sim" como categoria; não confirma como bem-sucedido; pede esclarecimento de categoria) |
| — | — | `AwaitingSlot=category` permanece; zero write; reprompt único |

**Asserções:** "sim" não é slot de categoria; pagamento (pix) já estava preservado e não deve ser solicitado novamente; `RepromptCount=1`.

---

### G7-08 Expiração de pendência (CA-08 / RF-09)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 200,00 no supermercado, débito |
| 2 | Agent | Qual categoria? |
| — (31+ minutos depois) | — | |
| 3 | User | supermercado |
| 4 | Agent | O registro expirou. Para registrar, envie a informação completa novamente. |
| — | — | status=`expired`; zero write |

**Asserções:** `SuspendedAt + 30min < now`; `DecidePendingResume` retorna `expired`; `TransactionsLedger` não chamado.

---

### G7-09 Replay idempotente (CA-07 / RF-20)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 80,00 na farmácia, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | farmácia |
| 4 | Agent | Despesa de R$ 80,00 registrada em *Custo Fixo > Medicamentos e Farmácia* ✅ |
| 5 | User | farmácia (mesmo messageId) |
| — | — | replay detectado; zero escrita duplicada |

**Asserções:** `OriginWamid` igual; `TransactionsLedger.CreateTransaction` chamado exatamente 1x; resposta de replay sem confirmar nova escrita.

---

### G7-10 Múltiplos candidatos — apresentação e escolha (CA-04 / RF-27)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 200,00 com saúde hoje, pix |
| 2 | Agent | Encontrei mais de uma categoria possível. Qual se encaixa melhor? 1. Plano de Saúde 2. Consultas e Exames 3. Terapia e Saúde Mental |
| 3 | User | consultas |
| — | — | ResolveForWrite(root=`66cb85a0`, sub=`af5619e0`, kind=expense); write |

**Asserções:** cada opção carrega `rootCategoryId`, `rootSlug`, `subcategoryId`, `subcategorySlug`; escolha valida par raiz+folha antes de write.

---

### G7-11 Múltiplos candidatos — candidato rejeitado por `ResolveForWrite` (RF-35)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei medicamento hoje, R$ 85,00, pix |
| 2 | Agent | Apresenta opções válidas |
| 3 | User | escolhe candidato cujo `ResolveForWrite` falha |
| 4 | Agent | (não persiste; pede nova escolha) |
| — | — | `TransactionsLedger` não chamado; pendência mantida |

**Asserções:** zero fallback para categoria genérica ou LLM-estimada; `M-04=0`.

---

### G7-12 Correção de descrição durante pendência (RF-07 / US-02)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 150,00 no mercado hoje, no pix |
| 2 | Agent | Qual categoria? |
| 3 | User | na verdade foi farmácia |
| 4 | Agent | Qual subcategoria de farmácia? Ex: Medicamentos e Farmácia ou Medicamentos Contínuos |
| 5 | User | medicamentos e farmácia |
| — | — | description atualizado para farmácia; write Custo Fixo > Medicamentos e Farmácia; R$ 150,00 preservado |

**Asserções:** valor e pagamento NÃO ressolicitados; description=`farmácia`; sub=`3ca95dd5`.

---

### G7-13 Resposta curta ambígua — incompatível com slot (RF-16)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 200,00 na loja, pix |
| 2 | Agent | Qual categoria? (`AwaitingSlot=category`) |
| 3 | User | tudo bem |
| 4 | Agent | Não entendi. Qual categoria: por exemplo Supermercado, Restaurante, Farmácia...? |
| — | — | pendência mantida; reprompt; RepromptCount=1; zero write |

**Asserções:** "tudo bem" não é categoria válida; slot não fechado; `RepromptCount=1`.

---

### G7-14 Resposta ambígua segunda vez — cancela (RF similar a HITL TTL)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 200,00 na loja, pix |
| 2 | Agent | Qual categoria? |
| 3 | User | tudo bem |
| 4 | Agent | Não entendi. Qual categoria? |
| 5 | User | ok sim |
| 6 | Agent | Não consegui identificar a categoria. O registro foi cancelado para evitar dados errados. |
| — | — | `RepromptCount=2`; status=`cancelled`; zero write |

---

### G7-15 Erro de ledger — sem sucesso simulado (CA-06 / RF-22)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 300,00 no supermercado, pix |
| 2 | Agent | (resolve categoria direto) |
| 3 | Ledger | retorna erro 500 |
| 4 | Agent | Não consegui registrar. Tente novamente em breve. |
| — | — | zero write; resposta sem sucesso; pendência pode ser mantida se erro for transitório |

**Asserções:** `M-03=0`; resposta não contém "registrei" ou "anotei".

---

### G7-16 Cartão de crédito sem cartão identificado (CA-10 / RF-25)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei tênis no crédito, R$ 320,00 |
| 2 | Agent | Qual cartão? (`AwaitingSlot=card`) |
| 3 | User | Nubank |
| — | — | `CardManager.ResolveByNickname("Nubank")`; write com cardId; valor/descrição preservados |

**Asserções:** `AwaitingSlot=card`; cardId resolvido; `installments=1` default; zero re-pergunta de valor.

---

### G7-17 Cartão de crédito com parcelas (RF-25)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei geladeira no crédito 10x, R$ 3.200,00 |
| 2 | Agent | Qual cartão? |
| 3 | User | Itaú |
| — | — | write com `installments=10`, `cardId` resolvido, categoria Metas > Tecnologia ou similar |

**Asserções:** `installments=10`; cartão resolvido antes de write; zero sucesso sem cardId.

---

### G7-18 Pendência de pagamento (AwaitingSlot=payment_method)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 180,00 no restaurante hoje |
| 2 | Agent | Qual foi a forma de pagamento? |
| 3 | User | pix |
| — | — | write: Prazeres > Restaurantes, R$ 180,00, pix |

**Asserções:** slot pagamento fechado com T3; zero re-pergunta de valor/descrição.

---

### G7-19 Pendência de data (AwaitingSlot=date)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei academia no pix, R$ 160,00 |
| 2 | Agent | Quando foi o pagamento? (data) |
| 3 | User | ontem |
| — | — | `occurredAt = today - 1 day`; write com data correta |

**Asserções:** `occurredAt` calculado deterministicamente; zero repeat de valor.

---

### G7-20 Fluxo completo confirmado (US-01 / CA-01)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 150,00 no mercado hoje, no pix |
| 2 | Agent | Qual categoria para essa compra no mercado? |
| 3 | User | custo fixo |
| 4 | Agent | Custo fixo tem várias opções. Qual subcategoria? Ex: Supermercado, Feira, Açougue |
| 5 | User | supermercado |
| 6 | Agent | Confirma? Despesa de R$ 150,00 em *Custo Fixo > Supermercado*, hoje, no pix |
| 7 | User | sim |
| 8 | Agent | Despesa de R$ 150,00 registrada em *Custo Fixo > Supermercado* para hoje no pix ✅ |
| — | — | write (após T7): root=`66cb85a0`/`custo-fixo`, sub=`97fa4b86`/`supermercado`, R$ 150,00, pix |

**Asserções:** `AwaitingSlot=confirmation` em T6; escrita SÓ após aceite T7 (M-07=0); M-02=100% (valor e pagamento não re-perguntados); M-03=0 (sucesso após write real); CA-01 validado.

---

## Grupo 8 — Cartão de Crédito (CC) — Despesas com parcelas

### G8-01 Produto eletrônico parcelado

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei celular no crédito 12x, R$ 4.800,00 |
| 2 | Agent | Qual cartão? |
| 3 | User | Nubank |
| — | — | write: Metas > Tecnologia, R$ 4800,00, CC, installments=12, cardId=Nubank |

---

### G8-02 Roupa parcelada

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Comprei roupa em 3x no crédito, R$ 600,00 |
| 2 | Agent | Qual cartão? |
| 3 | User | Itaú |
| — | — | write: Prazeres > Roupas e Calçados, R$ 600,00, CC, installments=3 |

---

### G8-03 Curso parcelado

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Paguei curso em 6x no cartão, R$ 1.800,00 |
| 2 | Agent | Qual categoria? |
| 3 | User | cursos |
| 4 | Agent | Qual cartão? |
| 5 | User | Nubank |
| — | — | write: Conhecimento > Cursos e Treinamentos, R$ 1800,00, CC, installments=6 |

---

## Grupo 9 — Recorrência

### G9-01 Despesa recorrente mensal

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Quero registrar o aluguel toda virada do mês, R$ 1.800,00, boleto |
| 2 | Agent | Confirma recorrência mensal de R$ 1.800,00 em Custo Fixo > Aluguel, boleto, todo dia 1? |
| 3 | User | sim |
| — | — | `TransactionsLedger.CreateRecurringTemplate`: frequency=monthly, amount=180000, sub=`c2fda6a3`/`aluguel` |

**Asserções:** confirmação antes de write de recorrência; zero sucesso sem confirmação; status=`completed`; delega a `internal/transactions` `create_recurring_template.go` (CA-16).

---

### G9-02 Cancelamento de recorrência antes da confirmação

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Quero registrar academia todo mês, R$ 160,00, débito |
| 2 | Agent | Confirma recorrência mensal? |
| 3 | User | não |
| — | — | zero write; pendência status=`cancelled` |

---

## Grupo 10 — Gates de Segurança (M-03 / M-04)

### G10-01 Categoria raiz sem folha — income (RF-30)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Recebi R$ 500,00 de vendas hoje, pix |
| 2 | Agent | Qual subcategoria de Vendas? Ex: Vendas Diversas, Marketplace |
| 3 | User | vendas (raiz) |
| — | — | bloqueio; sub=`8dba4d69` é raiz; exige folha |

**Asserções:** `TransactionsLedger` não chamado; agente não afirma sucesso; M-04=0.

---

### G10-02 Resposta do LLM com ID de categoria inválido (RF-13 / RF-35)

| Turno | Ator | Texto |
|-------|------|-------|
| — | LLM | sugere categoryId=`00000000-0000-0000-0000-000000000000` |
| — | Gate | `ResolveForWrite` retorna erro para ID inválido |
| — | — | zero write; agente pede nova informação ao usuário |

**Asserções:** ID do LLM nunca vira autoridade de escrita; `CategoryWriteGate` bloqueia; M-04=0.

---

### G10-03 Sucesso simulado — proibido (M-03 = 0)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 100,00 no mercado, pix |
| 2 | Agent | (ledger retorna erro ou não é chamado) |
| — | — | FAIL se resposta contém "registrei", "anotei", "salvo" sem write real |

**Asserções:** verificar que `TransactionsLedger.CreateTransaction` foi chamado antes de qualquer confirmação; M-03=0.

---

### G10-04 Resposta curta válida não re-pergunta dados já salvos (M-02 = 100%)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 340,00 no supermercado hoje, débito |
| 2 | Agent | Qual categoria? |
| 3 | User | supermercado |
| — | — | write sem re-perguntar R$ 340,00 nem débito |

**Asserções:** `AmountCents=34000` e `PaymentMethod=debit` preservados no snapshot; M-02=100%.

---

## Grupo 11 — Payloads de Harness Determinístico (CA-12)

Formato mínimo exigido por cenário de harness para validar CA-12 / RF-33:

```json
{
  "scenario": "G7-20",
  "turns": [
    { "actor": "user", "text": "Gastei R$ 150,00 no mercado hoje, no pix", "messageId": "wamid-001" },
    { "actor": "agent", "expectPendingStatus": "active", "expectAwaitingSlot": "category" },
    { "actor": "user", "text": "supermercado", "messageId": "wamid-002" },
    { "actor": "agent", "expectPendingStatus": "active", "expectAwaitingSlot": "confirmation", "expectNoWrite": true },
    { "actor": "user", "text": "sim", "messageId": "wamid-003" },
    { "actor": "agent",
      "expectConfirmationBeforeWrite": true,
      "expectWrite": {
        "amountCents": 15000,
        "paymentMethod": "pix",
        "rootCategoryId": "66cb85a0-3266-5900-b8e3-13cdcd00ab62",
        "rootSlug": "custo-fixo",
        "subcategoryId": "97fa4b86-d43c-5ad5-a99b-c88c8427fb30",
        "subcategorySlug": "supermercado",
        "categorySource": "user_selected_candidate",
        "originWamid": "wamid-001"
      },
      "expectPendingStatus": "completed",
      "expectRunStatus": "succeeded",
      "expectNoRepeat": ["amountCents", "paymentMethod"]
    }
  ]
}
```

Todos os cenários dos Grupos 7, 10 e 12 devem ter payload equivalente no harness, com:

- `expectWrite` presente quando há escrita; ausente (e `expectNoWrite=true`) quando não há.
- `expectPendingStatus` em todo turno do agente.
- `expectToolCalls` listando quais tools foram invocadas e com qual `ToolOutcome`.
- `expectRunStatus` ao final de cada cenário.
- `expectNoRepeat` listando slots que NÃO devem ser re-perguntados ao usuário.
- `expectConfirmationBeforeWrite=true` em todo cenário com escrita: o turno com `AwaitingSlot=confirmation` e o aceite explícito DEVEM preceder o `expectWrite` no Run auditável (M-07=0). Cenários `expectNoWrite` não exigem este campo.

---

## Grupo 12 — Gate de Confirmação, Edição e Seleção por Número (spec-version 3)

### G12-01 Caminho inequívoco exige confirmação (CA-13 / RF-38)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 320,00 no supermercado hoje no pix |
| 2 | Agent | Confirma? Despesa de R$ 320,00 em *Custo Fixo > Supermercado*, hoje, no pix (`AwaitingSlot=confirmation`) |
| 3 | User | sim |
| — | — | write (após T3): root=`66cb85a0`, sub=`97fa4b86`/`supermercado`, R$ 320,00, pix |

**Asserções:** mesmo sem ambiguidade, abre direto em `AwaitingSlot=confirmation`; nenhuma escrita síncrona no T1; `M-07=0`; escrita só após T3.

---

### G12-02 Recusa no turno de confirmação (CA-05 / RF-39)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 320,00 no supermercado hoje no pix |
| 2 | Agent | Confirma? Despesa de R$ 320,00 em *Custo Fixo > Supermercado*, hoje, no pix |
| 3 | User | não |
| 4 | Agent | Tudo certo, não registrei nada. |
| — | — | status=`cancelled`; zero write |

**Asserções:** cancelamento no gate fecha sem efeito; `TransactionsLedger` não chamado.

---

### G12-03 Confirmação ambígua → reprompt único → cancela (CA-14 / RF-39)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 180,00 no restaurante hoje no pix |
| 2 | Agent | Confirma? Despesa de R$ 180,00 em *Prazeres > Restaurantes*, hoje, no pix |
| 3 | User | talvez |
| 4 | Agent | Só para confirmar: posso registrar essa despesa? (responda sim ou não) |
| 5 | User | sei lá |
| 6 | Agent | Não confirmei, então não registrei nada. Se quiser, é só reenviar. |
| — | — | `ConfirmRepromptCount=1` em T4; status=`cancelled` em T6; zero write |

**Asserções:** reprompt único no gate; segunda ambiguidade cancela sem efeito; `M-07=0`.

---

### G12-04 Múltiplos candidatos — escolha por NÚMERO (CA-15 / RF-42)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Gastei R$ 200,00 com saúde hoje, pix |
| 2 | Agent | Encontrei mais de uma categoria. Qual? 1. Plano de Saúde 2. Consultas e Exames 3. Terapia e Saúde Mental |
| 3 | User | 2 |
| 4 | Agent | Confirma? Despesa de R$ 200,00 em *Custo Fixo > Consultas e Exames*, hoje, no pix |
| 5 | User | sim |
| — | — | write: sub=`af5619e0`/`consultas-e-exames`, resolvido a partir do índice 2 |

**Asserções:** `DecideCategoryChoice` resolve índice numérico e nome para o mesmo par canônico (paridade com G7-10 por nome); `ResolveForWrite` revalida antes de write; escrita só após T5.

---

### G12-05 Edição de lançamento com clarificação (CA-17 / RF-25, RF-43)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Editar meu último lançamento do mercado para R$ 175,00 |
| 2 | Agent | (resolve transação alvo; categoria do alvo ainda válida) Confirma? Atualizar *Custo Fixo > Supermercado* para R$ 175,00? |
| 3 | User | sim |
| — | — | `TransactionsLedger.UpdateTransaction` na transação alvo preservada, respeitando `TargetVersion`; sem nova transação |

**Asserções:** `TargetTransactionID`/`TargetVersion` preservados no snapshot; `OperationKind=edit`; zero criação de nova transação; escrita só após aceite.

---

### G12-06 Edição com categoria ambígua (CA-17 / RF-43)

| Turno | Ator | Texto |
|-------|------|-------|
| 1 | User | Muda a categoria do último lançamento para saúde |
| 2 | Agent | Qual? 1. Plano de Saúde 2. Consultas e Exames (`AwaitingSlot=category`, edição) |
| 3 | User | 1 |
| 4 | Agent | Confirma? Atualizar para *Custo Fixo > Plano de Saúde*? |
| 5 | User | sim |
| — | — | `UpdateTransaction` com nova categoria canônica na transação alvo; `TargetVersion` respeitado |

**Asserções:** clarify categorial dentro de edição; par raiz+folha revalidado; escrita só após aceite; sem nova transação.

---

## Resumo de Cobertura

| Grupo | Categorias | Cenários | Gaps cobertos |
|-------|-----------|----------|---------------|
| G1 — Custo Fixo | 44 subcategorias | 44 | M-02, RF-05, RF-14 |
| G2 — Conhecimento | 11 subcategorias | 10 | RF-02, RF-06 |
| G3 — Prazeres | 22 subcategorias | 21 | RF-06, RF-15 |
| G4 — Metas | 12 subcategorias | 7 | RF-02, installments |
| G5 — Liberdade Financeira | 17 subcategorias | 8 | RF-06 |
| G6 — Receitas | 24 subcategorias | 15 | RF-10, RF-30 |
| G7 — Controle de Estado | — | 20 | CA-01..12, RF-07..09, RF-15..17, RF-31..32 |
| G8 — Cartão de Crédito | — | 3 | RF-25, CA-10 |
| G9 — Recorrência | — | 2 | RF-25, CA-16 |
| G10 — Gates de Segurança | — | 4 | M-03=0, M-04=0, RF-13, RF-30, RF-35 |
| G11 — Harness | — | todos G7+G10+G12 | CA-12, RF-33 |
| G12 — Confirmação/Edição/Seleção | — | 6 | CA-13..15, CA-17, RF-38..43, M-07=0 |
| **Total** | **130 subcategorias** | **140** | **0 gaps** |

> Convenção Global de Confirmação (spec-version 3): todos os cenários com escrita herdam o turno de confirmação obrigatório antes do `write:`; o harness assevera a ordem `confirmation → write` e `M-07=0` em cada payload de G11.
