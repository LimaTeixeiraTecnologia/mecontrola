# Tarefa 1.0: ObjectiveProfile VO + classificação + SplitTemplate

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar, em `internal/onboarding`, a política de recomendação de distribuição por objetivo: o tipo
fechado `ObjectiveProfile`, o smart constructor `ParseObjectiveProfile`, o classificador
determinístico por palavra-chave `classifyByKeyword` e a tabela `SplitTemplate` que retorna **basis
points** por categoria. A **matemática cents** (basis points × renda) NÃO vive aqui — é delegada a
`internal/budgets` (ADR-006, Tarefa 4.0/5.0). Sem IO, sem LLM.

<requirements>
- RF-13: distribuição varia por objetivo (perfis fixos determinísticos).
- RF-13a: objetivo ambíguo/desconhecido → perfil default `ProfileOrganizeSpending`.
- ADR-004 (classificação híbrida — parte determinística) e ADR-006 (regra no onboarding).
- DMMF: discriminated union/state-as-type; `Parse*` é parse-don't-validate; funções puras.
</requirements>

## Subtarefas

- [ ] 1.1 Criar `internal/onboarding/domain/valueobjects/objective_profile.go` com `ObjectiveProfile` (`iota+1`, zero reservado), `ParseObjectiveProfile(raw string) (ObjectiveProfile, bool)`, `String()`.
- [ ] 1.2 Implementar `classifyByKeyword(objective string) (ObjectiveProfile, bool)` (pt-br) e `SplitTemplate(p ObjectiveProfile) []SplitEntryBP` com a tabela do techspec (cada linha soma 10000).
- [ ] 1.3 Testes unitários puros (sem mock) da tabela objetivo→perfil, fallback e soma dos templates.

## Detalhes de Implementação

Ver techspec.md → "Interfaces Chave" (bloco `ObjectiveProfile`), "Modelos de Dados" (tabela de
perfis) e ADR-004. Valores: PayoffDebt 45/5/10/25/15; EmergencyFund 40/5/10/15/30; Invest
40/10/10/10/30; SpecificGoal 40/5/10/30/15; OrganizeSpending (default) 40/10/15/20/15.

## Critérios de Sucesso

- `ObjectiveProfile` é tipo fechado (sem string livre na assinatura pública).
- `SplitTemplate` retorna basis points cuja soma é exatamente 10000 para todos os perfis.
- Nenhuma multiplicação `× incomeCents` (cálculo cents) aqui — pertence a `internal/budgets`.
- `ParseObjectiveProfile` rejeita valor desconhecido (`ok=false`); `classifyByKeyword` retorna default-miss `ok=false` para ambíguo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem). go-implementation (linguagem, auto) aplica-se por ser código Go de domínio.

## Testes da Tarefa

- [ ] Testes unitários (tabela objetivo→perfil; fallback; soma dos templates; valores não divisíveis)
- [ ] Testes de integração (não aplicável — domínio puro)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] `objective_profile.go` criado em `domain/valueobjects/`, sem IO/LLM/`context.Context`.
- [ ] Zero comentários no `.go` de produção (R-ADAPTER-001.1).
- [ ] Cobertura unitária dos 5 perfis + ambíguo + soma 10000.
- [ ] `go build ./internal/onboarding/...` e `go test ./internal/onboarding/domain/...` passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/onboarding/... && \
go test ./internal/onboarding/domain/valueobjects/... -run ObjectiveProfile -count=1
# zero comentários
grep -rn --include="*.go" --exclude="*_test.go" "^[[:space:]]*//" \
  internal/onboarding/domain/valueobjects/objective_profile.go \
  | grep -Ev "(//go:|//nolint:|// Code generated)" && echo FAIL || echo OK
```

## Arquivos Relevantes
- `internal/onboarding/domain/valueobjects/objective_profile.go` (novo)
- `internal/onboarding/domain/valueobjects/objective_profile_test.go` (novo)
