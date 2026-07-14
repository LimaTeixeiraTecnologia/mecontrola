# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Tolerância de arredondamento com absorção do resto na maior categoria
- **Data:** 2026-07-13
- **Status:** Aceita
- **Decisores:** Solicitante do produto (jailton.junior94), engenharia da plataforma agentiva
- **Relacionados:** `.specs/prd-distribuicao-personalizada-onboarding/prd.md` (RF-09, RF-11), `docs/us/us-distribuicao-personalizada-onboarding.md` (RN-12), `techspec.md`, ADR-001

## Contexto

A distribuição precisa fechar exatamente o orçamento: a soma dos basis points deve dar 10000 (`DecideDistribution`, `onboarding_workflow.go:219-240`). Hoje o caminho percentual exige `totalPct == 100` sobre valores arredondados para inteiro (`onboarding_workflow.go:277-285`) e o caminho reais exige soma exata igual ao orçamento (`:288-303`). Isso trava o usuário em casos legítimos de arredondamento — por exemplo, percentuais `33,3 + 33,3 + 33,4` cuja soma bruta é 100 mas o arredondamento ingênuo por categoria produz 99, ou centavos residuais na conversão de reais. O PRD (RF-09) pede aceitar diferenças mínimas por arredondamento e absorver o resto, sem afrouxar o invariante de fechamento (RF-11).

## Decisão

Aplicar uma tolerância pequena sobre a **soma bruta informada** (antes do arredondamento para inteiros), avaliada em `DecideDistributionBalance` (ADR-001):

- Percentual: soma bruta em `[99,5; 100,5]` → considerada `balanced`.
- Reais: soma bruta a até `R$ 0,05` do orçamento mensal → considerada `balanced`.
- Fora dessas bandas → `over`/`under` com mensagem de delta (ADR-001).

Quando `balanced`, `DecideAllocationsBP` fecha exatamente 10000 basis points distribuindo o resto por **maior-resto**, reaproveitando a lógica já existente em `centsToBasisPoints` (`onboarding_workflow.go:318-344`), aplicada tanto ao caminho percentual quanto ao de reais. O invariante `DecideDistribution` (sum=10000) permanece como verificação final.

## Alternativas Consideradas

- **Só perda de arredondamento (epsilon de ponto flutuante).** Vantagens: mais estrito. Desvantagens: trava `33,3+33,3+33,4=99` (soma inteira) e `99,6%` legítimo. Rejeitada por punir arredondamentos reais.
- **Banda larga (±1% / ±R$0,50).** Vantagens: mais permissivo. Desvantagens: mascara erro real de distribuição do usuário. Rejeitada por risco de falso "balanced".
- **Ajuste proporcional entre categorias não-zeradas.** Vantagens: distribui o resto suavemente. Desvantagens: mais complexo e menos previsível que maior-resto, que já é o padrão do código. Rejeitada por custo/consistência.

## Consequências

### Benefícios Esperados

- Não trava o usuário por centavos/pontos de arredondamento; fecha o orçamento com exatidão.
- Reaproveita `centsToBasisPoints` (menor superfície nova).
- Vale para os dois fluxos (RF-15).

### Trade-offs e Custos

- Introduz constantes de tolerância a manter (`0,5` pp e `R$ 0,05`).
- A absorção altera levemente a maior categoria em relação ao valor informado (documentado no resumo, que mostra os valores finais).

### Riscos e Mitigações

- Risco: banda escolhida mascarar erro. Mitigação: banda estreita sobre a soma bruta; acima cai em over/under. Rollback: endurecer para epsilon.

## Plano de Implementação

1. Definir as constantes de tolerância (percentual e reais).
2. Avaliar tolerância em `DecideDistributionBalance`.
3. Generalizar `DecideAllocationsBP` para fechar 10000 por maior-resto em ambos os caminhos.
4. Testes: `33,3/33,3/33,4→100`, reais a R$0,04/R$0,06 de distância, e verificação de que `DecideDistribution` continua vendo 10000.

Concluído quando: casos de tolerância passam e o invariante 10000 nunca é violado.

## Monitoramento e Validação

- Sinal: `agents_onboarding_distribution_total{outcome="tolerance_absorbed"}` (ADR-004) para medir frequência de absorção.
- Critério: nenhuma soma dentro da banda é rejeitada; nenhuma soma fora da banda é aceita. Revisar a banda se o outcome `tolerance_absorbed` for anormalmente alto (indício de banda larga demais).

## Impacto em Documentação e Operação

- Documentar as constantes de tolerância no código (via nome de constante, sem comentário — R-ADAPTER-001.1) e no runbook de onboarding.

## Revisão Futura

Revisitar as bandas se o produto passar a aceitar entrada fracionária ou moedas com subunidade diferente.
