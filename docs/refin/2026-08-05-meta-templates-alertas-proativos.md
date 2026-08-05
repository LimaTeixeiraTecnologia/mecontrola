# Spec Meta: templates de alertas proativos MeControla

Data: 2026-08-05

Status: templates criados/submetidos via Meta Graph API em 2026-08-05. Aprovacao parcial confirmada; envio em producao permanece condicionado ao status `APPROVED` por template.

## Estado atual verificado

Consulta via Meta Graph API com `.env` local em 2026-08-05:

| Template | ID Meta | Status | Categoria | Idioma |
| --- | --- | --- | --- | --- |
| `mecontrola_ativacao` | `976808612134618` | `APPROVED` | `MARKETING` | `pt_BR` |
| `mecontrola_budget_missing_month_start` | `1561236175466476` | `PENDING` | `UTILITY` | `pt_BR` |
| `mecontrola_budget_not_reviewed_day_3` | `1053591550739935` | `PENDING` | `UTILITY` | `pt_BR` |
| `mecontrola_category_threshold_80` | `1710238193529722` | `PENDING` | `UTILITY` | `pt_BR` |
| `mecontrola_category_threshold_90` | `1062335899595161` | `PENDING` | `UTILITY` | `pt_BR` |
| `mecontrola_category_threshold_100` | `1376417771353202` | `PENDING` | `UTILITY` | `pt_BR` |
| `mecontrola_goal_achieved` | `892397930196406` | `PENDING` | `UTILITY` | `pt_BR` |
| `mecontrola_month_closing` | `1059680423220718` | `PENDING` | `UTILITY` | `pt_BR` |
| `mecontrola_usage_reactivation_3d` | `1746833263117443` | `APPROVED` | `MARKETING` | `pt_BR` |
| `mecontrola_weekly_motivation` | `1970815953590167` | `APPROVED` | `MARKETING` | `pt_BR` |
| `mecontrola_abandonment_risk_7d` | `871497755818159` | `PENDING` | `MARKETING` | `pt_BR` |
| `hello_world` | nao faz parte do escopo | `APPROVED` | `UTILITY` | `en_US` |

Conclusao: os templates de alertas proativos foram criados. Apenas `mecontrola_weekly_motivation` e `mecontrola_usage_reactivation_3d` estavam `APPROVED` na consulta; os demais permanecem `PENDING`.

## Regras de cadastro

- Idioma: `pt_BR`.
- Variaveis devem ser posicionais e ter exemplos reais de formato.
- Nao cadastrar texto com Markdown de WhatsApp que a Meta rejeite; negrito pode ser removido no template se necessario.
- Nao incluir dados sensiveis desnecessarios.
- Nao prometer acao automatica quando a resposta exige confirmacao do usuario.
- Revalidar status `APPROVED` antes de habilitar envio.

## Templates Release 1 recomendado

### 1. `mecontrola_budget_missing_month_start`

Categoria sugerida: `UTILITY`

Uso: inicio de mes sem orcamento vigente.

Body:

```text
📅 Novo mês, novo controle

Pra eu conseguir te avisar quando uma categoria estiver perto do limite, acompanhar sua evolução e fechar seu mês com clareza, primeiro preciso do seu orçamento deste mês.

Qual seu orçamento para este mês?
```

Parametros: nenhum.

Follow-up esperado: iniciar criacao de orcamento.

### 2. `mecontrola_category_threshold_80`

Categoria sugerida: `UTILITY`

Uso: categoria atingiu 80% do planejado.

Body:

```text
⚠️ Atenção em {{1}}

Você já usou 80% do valor planejado para essa categoria neste mês.

Planejado: R$ {{2}}
Gasto até agora: R$ {{3}}
Ainda disponível: R$ {{4}}

Ainda dá tempo de ajustar a rota sem deixar o mês sair do controle.

Quer ver onde você mais gastou nessa categoria?
```

Parametros:

1. categoria: exemplo `Prazeres`
2. valor planejado: exemplo `1.000,00`
3. valor gasto: exemplo `800,00`
4. saldo restante: exemplo `200,00`

Follow-up esperado: detalhar categoria por subcategoria.

### 3. `mecontrola_category_threshold_90`

Categoria sugerida: `UTILITY`

Uso: categoria atingiu 90% do planejado.

Body:

```text
🚨 Sua categoria {{1}} está quase no limite

Você já consumiu 90% do que planejou para ela neste mês.

Planejado: R$ {{2}}
Gasto até agora: R$ {{3}}
Ainda disponível: R$ {{4}}

Vale olhar com carinho os próximos gastos dessa categoria pra não estourar.

Quer ver seu orçamento completo por categoria?
```

Parametros:

1. categoria: exemplo `Custo Fixo`
2. valor planejado: exemplo `3.000,00`
3. valor gasto: exemplo `2.700,00`
4. saldo restante: exemplo `300,00`

Follow-up esperado: panorama completo do orcamento.

### 4. `mecontrola_category_threshold_100`

Categoria sugerida: `UTILITY`

Uso: categoria atingiu ou ultrapassou 100% do planejado.

Body:

```text
❌ A categoria {{1}} atingiu o limite do mês

Você já usou todo o valor planejado para essa categoria.

Planejado: R$ {{2}}
Gasto atual: R$ {{3}}
Excedente: R$ {{4}}

Calma. Isso não significa que o mês está perdido.

Quer ver seu orçamento completo por categoria?
```

Parametros:

1. categoria: exemplo `Conhecimento`
2. valor planejado: exemplo `500,00`
3. valor gasto: exemplo `650,00`
4. excedente: exemplo `150,00`

Follow-up esperado: panorama completo do orcamento.

### 5. `mecontrola_budget_not_reviewed_day_3`

Categoria sugerida: `UTILITY`

Uso: terceiro dia do mes sem orcamento cadastrado/revisado.

Body:

```text
📅 Seu orçamento do mês ainda não foi definido

Sem ele, eu não consigo te avisar quando uma categoria estiver perto do limite, acompanhar seus gastos com clareza e te mostrar se o mês está indo bem ou não.

Vamos cadastrar agora?
```

Parametros: nenhum.

Follow-up esperado: iniciar criacao de orcamento.

## Templates Release 2

### 6. `mecontrola_month_closing`

Categoria sugerida: `UTILITY`

Uso: fechamento da competencia financeira.

Body:

```text
📊 Fechamento do seu mês no MeControla

Fechei o comparativo entre o que você planejou e o que realmente aconteceu em cada categoria:

Custo Fixo
Planejado: R$ {{1}}
Realizado: R$ {{2}}
Status: {{3}}

Conhecimento
Planejado: R$ {{4}}
Realizado: R$ {{5}}
Status: {{6}}

Prazeres
Planejado: R$ {{7}}
Realizado: R$ {{8}}
Status: {{9}}

Metas
Planejado: R$ {{10}}
Realizado: R$ {{11}}
Status: {{12}}

Liberdade Financeira
Planejado: R$ {{13}}
Realizado: R$ {{14}}
Status: {{15}}

Esse resumo mostra onde o dinheiro seguiu o plano e onde vale ajustar no próximo mês.

Quer que eu monte a base do orçamento do próximo mês com esses dados?
```

Parametros:

1. planejado custo fixo
2. realizado custo fixo
3. status custo fixo
4. planejado conhecimento
5. realizado conhecimento
6. status conhecimento
7. planejado prazeres
8. realizado prazeres
9. status prazeres
10. planejado metas
11. realizado metas
12. status metas
13. planejado liberdade financeira
14. realizado liberdade financeira
15. status liberdade financeira

Risco: templates Meta com muitos parametros podem ficar mais dificeis de aprovar/manter. Alternativa: enviar resumo compacto e abrir detalhamento via resposta do usuario.

### 7. `mecontrola_weekly_motivation`

Categoria sugerida: `MARKETING`

Uso: reforco semanal de constancia.

Body:

```text
💬 Só passando pra reforçar uma coisa:

organizar o dinheiro não é sobre perfeição.
É sobre constância.

Cada lançamento que você faz hoje deixa seu mês mais claro amanhã.

Menos caos. Mais conquistas. 💚
```

Parametros: nenhum.

Risco: pode ser classificado como marketing pela Meta. Exige opt-in e cuidado de frequencia.

### 8. `mecontrola_usage_reactivation_3d`

Categoria sugerida: `MARKETING`

Uso: usuario ativo com assinatura ativa e 3 dias sem registrar/interagir.

Body:

```text
👀 Faz alguns dias que você não registra nada por aqui

Se o mês saiu um pouco do trilho, tudo bem.
A gente retoma de onde parou.

Quer que eu te mostre como estão suas categorias até agora?
```

Parametros: nenhum.

Follow-up esperado: panorama de categorias.

### 9. `mecontrola_abandonment_risk_7d`

Categoria sugerida: `MARKETING`

Uso: usuario com 7 dias ou mais sem interacao.

Body:

```text
📌 Seu mês ainda pode voltar pro controle

Você ficou alguns dias sem registrar seus gastos, mas ainda dá tempo de organizar o que aconteceu.

Se quiser, eu te ajudo a fazer uma retomada simples:
primeiro a gente olha suas categorias, depois ajusta o restante do mês.

Quer retomar agora?
```

Parametros: nenhum.

Follow-up esperado: panorama de categorias.

## Template condicionado

### 10. `mecontrola_goal_achieved`

Categoria sugerida: `UTILITY`

Uso: meta individual atingida.

`BLOCKER`: usar somente se houver entidade de meta individual com nome, valor definido e valor acumulado. Nao usar apenas a categoria orcamentaria `Metas` como substituto sem nova decisao de dominio.

Body:

```text
🎉 Você conseguiu. Sua meta {{1}} foi atingida.

Olha o que isso significa na prática:
esse dinheiro não ficou perdido no mês, não foi engolido pela correria e nem saiu sem direção. Ele foi guardado com propósito — e agora virou conquista de verdade.

Valor da meta: R$ {{2}}
Valor acumulado: R$ {{3}}

É exatamente pra isso que o controle serve:
te aproximar da vida que você quer viver, um valor de cada vez. 💚

Quer criar a próxima meta agora?
```

Parametros:

1. nome da meta: exemplo `Reserva de emergencia`
2. valor da meta: exemplo `5.000,00`
3. valor acumulado: exemplo `5.250,00`

## Variaveis de ambiente sugeridas

```text
META_TEMPLATE_BUDGET_MISSING_MONTH_START=mecontrola_budget_missing_month_start
META_TEMPLATE_CATEGORY_THRESHOLD_80=mecontrola_category_threshold_80
META_TEMPLATE_CATEGORY_THRESHOLD_90=mecontrola_category_threshold_90
META_TEMPLATE_CATEGORY_THRESHOLD_100=mecontrola_category_threshold_100
META_TEMPLATE_BUDGET_NOT_REVIEWED_DAY_3=mecontrola_budget_not_reviewed_day_3
META_TEMPLATE_MONTH_CLOSING=mecontrola_month_closing
META_TEMPLATE_WEEKLY_MOTIVATION=mecontrola_weekly_motivation
META_TEMPLATE_USAGE_REACTIVATION_3D=mecontrola_usage_reactivation_3d
META_TEMPLATE_ABANDONMENT_RISK_7D=mecontrola_abandonment_risk_7d
META_TEMPLATE_GOAL_ACHIEVED=mecontrola_goal_achieved
```

## Checklist de aprovacao

- [x] Template criado na Meta.
- [ ] Idioma `pt_BR`.
- [ ] Categoria coerente com conteudo.
- [ ] Exemplos de parametros preenchidos.
- [ ] Status `APPROVED` confirmado via Graph API.
- [ ] Nome configurado no ambiente.
- [ ] Envio testado com fake/contract test antes de producao.
- [ ] Feature flag do alerta ligada somente apos aprovacao.
