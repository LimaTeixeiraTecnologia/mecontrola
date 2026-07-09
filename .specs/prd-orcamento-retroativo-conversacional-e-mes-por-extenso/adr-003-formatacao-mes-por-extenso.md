# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Formatação de mês por extenso como função pura em `internal/budgets`, sem alterar armazenamento ISO
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Time de plataforma / agente financeiro
- **Relacionados:** PRD (RF-18, RF-19), techspec.md, design-patterns-mandatory, ADR-002

## Contexto

`Competence.String()` retorna ISO `YYYY-MM` (`competence.go:51`). Não há formatação por extenso em `internal/budgets` (busca por "janeiro"/"junho"/"MonthName" sem correspondência). A instrução tem mapa slug→nome apenas para categorias (`mecontrola_agent.go:199-204`) e injeta `{competência}` cru na mensagem de orçamento não encontrado (`mecontrola_agent.go:223`), exibindo `2026-06`. O usuário pediu explicitamente meses por extenso ("junho de 2026", "janeiro de 2025"). O armazenamento e as constraints (`budgets_competence_chk`) permanecem `YYYY-MM` (fora de escopo mudar).

## Decisão

Adicionar função pura `FormatCompetencePtBR(c Competence) string` (ou método `Competence.Humanize()`) em `internal/budgets/domain/valueobjects/competence.go`, mapeando `YYYY-MM` → "<mês> de <ano>" em português do Brasil (tabela fixa de 12 meses). Toda saída do agente que cite competência usa essa formatação; `YYYY-MM` permanece contrato interno e formato de armazenamento. A mensagem de orçamento não encontrado e as confirmações/retrospectiva passam a citar o mês por extenso.

## Alternativas Consideradas

- **Strategy pattern para formatação:** over-engineering para um mapeamento fixo determinístico; a skill design-patterns recomenda explicitamente evitar Strategy aqui. Rejeitada.
- **Formatar via LLM (instrução apenas):** não determinístico, arrisca inconsistência ("2026-06" vazando). Rejeitada como fonte única; a função pura é a autoridade, a instrução apenas a consome.
- **`time.Month.String()` + i18n lib:** retornaria inglês ou adicionaria dependência; pt-BR fixo é suficiente (i18n fora de escopo). Rejeitada.

## Consequências

### Benefícios Esperados

- Consistência: toda menção de mês por extenso, sem tocar o dado persistido.
- Determinístico e testável por tabela pura (12 meses).

### Trade-offs e Custos

- Tabela de meses pt-BR fixa (aceitável; i18n fora de escopo).

### Riscos e Mitigações

- **Vazamento de ISO em alguma saída:** cobertura por cenário E2E que valida "junho de 2026" na resposta; instrução reforça uso da função. Rollback trivial (função isolada).

## Plano de Implementação

1. `FormatCompetencePtBR` + testes puros (12 meses, "junho de 2026", "janeiro de 2025").
2. Consumir na instrução do agente (mensagens de orçamento, confirmação, retrospectiva) e nas respostas do workflow.

## Monitoramento e Validação

- Cenário E2E valida ausência de `YYYY-MM` visível ao usuário e presença do extenso; parte do gate ≥0.90.

## Impacto em Documentação e Operação

- Instrução do agente documenta uso da formatação por extenso.

## Revisão Futura

- Revisar apenas se i18n além de pt-BR entrar em escopo.
