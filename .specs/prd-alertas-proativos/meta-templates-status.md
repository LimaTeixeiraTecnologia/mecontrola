# Status dos templates Meta - 2026-08-05

Fonte de verdade desta consolidacao:

- consulta real na Meta Graph API em 2026-08-05 com `META_WABA_ID` e `META_ACCESS_TOKEN` do `.env` local;
- detalhamento funcional em [docs/refin/2026-08-05-meta-templates-alertas-proativos.md](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md);
- regras de produto em [.specs/prd-alertas-proativos/techspec.md](/Users/jailtonjunior/Git/mecontrola/.specs/prd-alertas-proativos/techspec.md:77) e [.specs/prd-alertas-proativos/adr-003-threshold-90-condicionado.md](/Users/jailtonjunior/Git/mecontrola/.specs/prd-alertas-proativos/adr-003-threshold-90-condicionado.md:35).

Regra operacional: `APPROVED` na Meta nao significa liberado no produto. Gates de release, opt-in `MARKETING` e bloqueios de dominio continuam valendo.

## Quadro consolidado

| Kind | Template | Meta ID | Categoria Meta atual | Idioma | Status Meta | Situacao de produto | Observacoes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `activation` | `mecontrola_ativacao` | `976808612134618` | `MARKETING` | `pt_BR` | `APPROVED` | legado, fora do escopo dos alertas proativos | template ja existente no fluxo de ativacao |
| `budget_missing_month_start` | `mecontrola_budget_missing_month_start` | `1561236175466476` | `UTILITY` | `pt_BR` | `APPROVED` | liberavel no Release 1 por gate de template | sem parametros |
| `budget_not_reviewed_day_3` | `mecontrola_budget_not_reviewed_day_3` | `1053591550739935` | `MARKETING` | `pt_BR` | `APPROVED` | liberavel no Release 1 so com opt-in | documento de templates descreve categoria sugerida `UTILITY`, mas a Meta hoje retorna `MARKETING` |
| `category_threshold_80` | `mecontrola_category_threshold_80` | `1710238193529722` | `UTILITY` | `pt_BR` | `APPROVED` | liberavel no Release 1 por gate de template | exige 4 parametros |
| `category_threshold_90` | `mecontrola_category_threshold_90` | `1062335899595161` | `UTILITY` | `pt_BR` | `APPROVED` | bloqueado no produto | a spec do Release 1 nao permite emissao de 90 |
| `category_threshold_100` | `mecontrola_category_threshold_100` | `1376417771353202` | `UTILITY` | `pt_BR` | `APPROVED` | liberavel no Release 1 por gate de template | exige 4 parametros |
| `month_closing` | `mecontrola_month_closing` | `1059680423220718` | `UTILITY` | `pt_BR` | `APPROVED` | fora do Release 1 | template de Release 2 com alta carga de parametros |
| `weekly_motivation` | `mecontrola_weekly_motivation` | `1970815953590167` | `MARKETING` | `pt_BR` | `APPROVED` | fora do Release 1 e exige opt-in | sem parametros |
| `usage_reactivation_3d` | `mecontrola_usage_reactivation_3d` | `1746833263117443` | `MARKETING` | `pt_BR` | `APPROVED` | fora do Release 1 e exige opt-in | sem parametros |
| `abandonment_risk_7d` | `mecontrola_abandonment_risk_7d` | `871497755818159` | `MARKETING` | `pt_BR` | `PENDING` | bloqueado enquanto nao houver `APPROVED` | sem parametros |
| `goal_achieved` | `mecontrola_goal_achieved` | `892397930196406` | `MARKETING` | `pt_BR` | `APPROVED` | bloqueado por dominio | depende de meta individual provada no codebase; documento local sugere `UTILITY`, mas a Meta hoje retorna `MARKETING` |

## Templates aprovados

### `mecontrola_budget_missing_month_start`

- Kind: `budget_missing_month_start`
- Status Meta: `APPROVED`
- Categoria Meta atual: `UTILITY`
- Uso: inicio de mes sem orcamento vigente
- Parametros: nenhum
- Follow-up esperado: iniciar criacao de orcamento
- Referencia funcional: [docs/refin/2026-08-05-meta-templates-alertas-proativos.md:39](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md:39)

### `mecontrola_budget_not_reviewed_day_3`

- Kind: `budget_not_reviewed_day_3`
- Status Meta: `APPROVED`
- Categoria Meta atual: `MARKETING`
- Uso: terceiro dia do mes sem orcamento cadastrado ou revisado
- Parametros: nenhum
- Follow-up esperado: iniciar criacao de orcamento
- Referencia funcional: [docs/refin/2026-08-05-meta-templates-alertas-proativos.md:152](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md:152)
- Observacao: a categoria atual na Meta exige opt-in antes de envio, mesmo que o texto funcional do documento o trate como `UTILITY`

### `mecontrola_category_threshold_80`

- Kind: `category_threshold_80`
- Status Meta: `APPROVED`
- Categoria Meta atual: `UTILITY`
- Uso: categoria atingiu 80% do planejado
- Parametros:
  - `{{1}}` categoria
  - `{{2}}` valor planejado
  - `{{3}}` valor gasto
  - `{{4}}` saldo restante
- Follow-up esperado: detalhar categoria por subcategoria
- Referencia funcional: [docs/refin/2026-08-05-meta-templates-alertas-proativos.md:59](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md:59)

### `mecontrola_category_threshold_90`

- Kind: `category_threshold_90`
- Status Meta: `APPROVED`
- Categoria Meta atual: `UTILITY`
- Uso: categoria atingiu 90% do planejado
- Parametros:
  - `{{1}}` categoria
  - `{{2}}` valor planejado
  - `{{3}}` valor gasto
  - `{{4}}` saldo restante
- Follow-up esperado: panorama completo do orcamento
- Referencia funcional: [docs/refin/2026-08-05-meta-templates-alertas-proativos.md:90](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md:90)
- Bloqueio de produto:
  - `REQ-08` do techspec restringe o Release 1 a `80` e `100` e mantem `90` como estado futuro [techspec]( /Users/jailtonjunior/Git/mecontrola/.specs/prd-alertas-proativos/techspec.md:79 )
  - o ADR condiciona a liberacao a VO, migration, constraints e testes [adr-003]( /Users/jailtonjunior/Git/mecontrola/.specs/prd-alertas-proativos/adr-003-threshold-90-condicionado.md:43 )

### `mecontrola_category_threshold_100`

- Kind: `category_threshold_100`
- Status Meta: `APPROVED`
- Categoria Meta atual: `UTILITY`
- Uso: categoria atingiu ou ultrapassou 100% do planejado
- Parametros:
  - `{{1}}` categoria
  - `{{2}}` valor planejado
  - `{{3}}` valor gasto
  - `{{4}}` excedente
- Follow-up esperado: panorama completo do orcamento
- Referencia funcional: [docs/refin/2026-08-05-meta-templates-alertas-proativos.md:121](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md:121)

### `mecontrola_month_closing`

- Kind: `month_closing`
- Status Meta: `APPROVED`
- Categoria Meta atual: `UTILITY`
- Uso: fechamento da competencia financeira
- Parametros: 15 campos de planejado, realizado e status por categoria
- Referencia funcional: [docs/refin/2026-08-05-meta-templates-alertas-proativos.md:174](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md:174)
- Observacao: documento local o posiciona como Release 2; nao tratar como liberado no Release 1

### `mecontrola_weekly_motivation`

- Kind: `weekly_motivation`
- Status Meta: `APPROVED`
- Categoria Meta atual: `MARKETING`
- Uso: reforco semanal de constancia
- Parametros: nenhum
- Referencia funcional: [docs/refin/2026-08-05-meta-templates-alertas-proativos.md:237](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md:237)
- Observacao: exige opt-in e controle de frequencia

### `mecontrola_usage_reactivation_3d`

- Kind: `usage_reactivation_3d`
- Status Meta: `APPROVED`
- Categoria Meta atual: `MARKETING`
- Uso: usuario ativo com assinatura ativa e 3 dias sem registrar ou interagir
- Parametros: nenhum
- Follow-up esperado: panorama de categorias
- Referencia funcional: [docs/refin/2026-08-05-meta-templates-alertas-proativos.md:260](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md:260)
- Observacao: exige opt-in

### `mecontrola_goal_achieved`

- Kind: `goal_achieved`
- Status Meta: `APPROVED`
- Categoria Meta atual: `MARKETING`
- Uso pretendido: meta individual atingida
- Parametros:
  - `{{1}}` nome da meta
  - `{{2}}` valor da meta
  - `{{3}}` valor acumulado
- Referencia funcional: [docs/refin/2026-08-05-meta-templates-alertas-proativos.md:306](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md:306)
- Bloqueio de produto: usar somente quando existir entidade de meta individual com prova no dominio; a categoria orcamentaria `Metas` nao substitui esse requisito
- Observacao: o documento local sugere categoria `UTILITY`, mas a Meta hoje retorna `MARKETING`, entao qualquer futuro uso tambem exigira opt-in

### `mecontrola_ativacao`

- Kind: `activation`
- Status Meta: `APPROVED`
- Categoria Meta atual: `MARKETING`
- Uso: template legado de ativacao e outreach, preservado fora do escopo de alertas proativos
- Parametros: nao consolidados neste anexo
- Observacao: mantido aqui apenas para contexto operacional, nao como template novo da spec de alertas

## Templates pendentes

### `mecontrola_abandonment_risk_7d`

- Kind: `abandonment_risk_7d`
- Status Meta: `PENDING`
- Categoria Meta atual: `MARKETING`
- Uso: usuario com 7 dias ou mais sem interacao
- Parametros: nenhum
- Follow-up esperado: panorama de categorias
- Referencia funcional: [docs/refin/2026-08-05-meta-templates-alertas-proativos.md:281](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md:281)
- Gate: nao ha caminho de envio real enquanto a Meta nao retornar `APPROVED`

## Divergencias documentais

- O quadro inicial em [docs/refin/2026-08-05-meta-templates-alertas-proativos.md](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-meta-templates-alertas-proativos.md:11) esta desatualizado: varios templates que ali constam como `PENDING` agora estao `APPROVED`.
- O resumo em [docs/refin/2026-08-05-sdd-alertas-proativos.md](/Users/jailtonjunior/Git/mecontrola/docs/refin/2026-08-05-sdd-alertas-proativos.md:80) tambem nao reflete mais o retorno atual da Meta.
- Divergencias de categoria entre documento local e Meta atual:
  - `mecontrola_budget_not_reviewed_day_3`: documento sugere `UTILITY`; Meta retorna `MARKETING`
  - `mecontrola_goal_achieved`: documento sugere `UTILITY`; Meta retorna `MARKETING`

## Leitura operacional recomendada

- `APPROVED` e liberavel no Release 1:
  - `mecontrola_budget_missing_month_start`
  - `mecontrola_category_threshold_80`
  - `mecontrola_category_threshold_100`
  - `mecontrola_budget_not_reviewed_day_3`, desde que o gate de opt-in `MARKETING` seja respeitado
- `APPROVED`, mas bloqueado por regra de produto:
  - `mecontrola_category_threshold_90`
  - `mecontrola_goal_achieved`
- `APPROVED`, mas fora do Release 1:
  - `mecontrola_month_closing`
  - `mecontrola_weekly_motivation`
  - `mecontrola_usage_reactivation_3d`
- `PENDING`:
  - `mecontrola_abandonment_risk_7d`
