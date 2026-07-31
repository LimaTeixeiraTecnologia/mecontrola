# Registro de Decisão Arquitetural (ADR-003)

## Metadados

- **Título:** Gate de aceite golden real-LLM por grupo de intenção de receita mais casos-armadilha
- **Data:** 2026-07-30
- **Status:** Aceita
- **Decisores:** Solicitante do produto (jailton.junior94), engenharia de plataforma
- **Relacionados:** PRD `.specs/prd-lancamento-edicao-receitas/prd.md` (RF-01, RF-03, RF-04, RF-06, RF-25); techspec `.specs/prd-lancamento-edicao-receitas/techspec.md`; feedback de validação real-LLM (memória do projeto)

## Contexto

- A compreensão ampla de intenção de receita e o valor por-extenso são resolvidos pelo LLM (arquitetura LLM-first já vigente). Sem prova dedicada, famílias inteiras (autônomos, comissões, aluguéis, investimentos) ficam sem cobertura e sujeitas a regressão silenciosa.
- O projeto exige validação real-LLM (mocks não bastam) e proíbe falso-sucesso (verde sem a tool/args corretos).
- Cobrir uma frase por verbo listado seria caro de executar e manter; cobrir só os grupos mais comuns deixaria lacunas.

## Decisão

1. O gate de aceite inclui **pelo menos um caso golden real-LLM por grupo de intenção** de receita (nove grupos): recebimentos, trabalho, autônomos, comércio, produção artesanal, comissões/bônus, salário, aluguéis, investimentos.
2. Cobrir também: valor por-extenso simples (`cem`, `dois mil`) e composto (`dois mil e quinhentos`); gíria (`100 pila`, `500 conto`); datas novas (`semana passada`, `mês passado`, `dia 10`); e edição de receita (`corrige aquela receita`, `era 150`, `na verdade foi 180`, `muda a data`).
3. Preservar os casos-armadilha derivados de produção (falso múltiplo lançamento, paráfrase de origem).
4. Limiar: taxa ≥ 0,90 por grupo e **0 falso-sucesso**; execução com `RUN_REAL_LLM=1` e credenciais OpenRouter.
5. Os casos vivem em um arquivo dedicado `cases_income_natural_language.go`, registrados no `registry.go`, para não inchar `cases_expense_income.go`.

## Alternativas Consideradas

- **≥ 1 caso por frase listada (dezenas de casos).** Vantagem: cobertura literal máxima. Desvantagem: custo alto de execução real-LLM e manutenção. Rejeitada.
- **Subconjunto representativo mínimo (salário/freela/venda).** Vantagem: barato. Desvantagem: deixa famílias inteiras sem prova, com risco de regressão silenciosa. Rejeitada.
- **Cobrir só com mocks.** Contraria a política de validação real-LLM do projeto. Rejeitada.

## Consequências

### Benefícios Esperados

- Prova de regressão robusta e proporcional para as nove famílias e para as formas de valor/data.
- Detecção precoce de falso-sucesso e de regressões de roteamento LLM.

### Trade-offs e Custos

- Custo de execução real-LLM no gate; mitigado pela granularidade por grupo (não por frase).
- Manutenção da suíte quando o catálogo de mensagens ou os schemas mudarem.

### Riscos e Mitigações

- Risco: flutuação do LLM derrubar o limiar. Impacto: falso vermelho intermitente. Mitigação: limiar por grupo (não por caso único) e casos com asserts de tool/args além de propriedade de resposta.
- Risco: caso "verde" sem a tool correta (falso-sucesso). Mitigação: asserts de `ExpectedTool`/`ExpectedArgs` e `0 falso-sucesso` como critério bloqueante.

## Plano de Implementação

1. Criar `cases_income_natural_language.go` com os casos por grupo + valor + data + edição.
2. Registrar no `registry.go`.
3. Executar o gate com `RUN_REAL_LLM=1` e ajustar até ≥ 0,90 por grupo, 0 falso-sucesso.

## Monitoramento e Validação

- Critério de sucesso: gate golden ≥ 0,90 por grupo, 0 falso-sucesso, casos-armadilha preservados verdes.
- Sinais: relatório do harness real-LLM.
- Revisão: ao adicionar nova família de intenção ou nova forma de valor/data.

## Impacto em Documentação e Operação

- Sem impacto em runbooks; a suíte golden é artefato de teste versionado.

## Revisão Futura

- Revisar granularidade se o custo de execução real-LLM crescer além do orçamento ou se surgirem novas famílias/formas.
