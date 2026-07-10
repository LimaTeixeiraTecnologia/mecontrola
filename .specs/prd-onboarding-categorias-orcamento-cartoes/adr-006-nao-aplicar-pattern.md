# Registro de Decisão Arquitetural (ADR-006)

## Metadados

- **Título:** Não aplicar design pattern GoF — refactor direto sobre o workflow durável existente
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Autor da feature (skill `design-patterns-mandatory`)
- **Relacionados:** PRD (decisão de pattern), techspec.md, ADR-001, ADR-003, `select_pattern.py`

## Contexto

A skill `design-patterns-mandatory` exige rodar o seletor determinístico antes de recomendar ou
rejeitar qualquer pattern. A feature é um refactor local de um workflow durável já existente
(`internal/agents/application/workflows/onboarding_workflow.go`) que consome o kernel
`internal/platform/workflow` — o qual **já implementa** a máquina de estados durável (suspend/resume,
cursor, snapshot). Não há introdução de nova família de objetos, nova hierarquia ou branching
recorrente que justifique formalizar um pattern.

## Decisão

**Não aplicar padrão** (`Recomendar: nao aplicar padrao`). Implementar como solução direta: reordenar
a `Sequence`, adicionar steps e enums fechados, e fundir o step de review (ADR-003). Evidência do
seletor determinístico:

```json
// scripts/select_pattern.py --input selector-input.json
{ "status": "reject",
  "simpler_alternative": "Usar solucao direta, refactor local ou composicao simples",
  "economy_case": ["prefer_direct_solution + single_variant_only + low_change_frequency → retorno insuficiente para formalizar um pattern"],
  "efficiency_case": ["A opcao simples reduz indirecao e custo cognitivo."],
  "robustness_case": ["Menos tipos e menos acoplamento estrutural diminuem a superficie de falha."] }
```

Sinais canônicos usados: `prefer_direct_solution`, `single_variant_only`, `low_change_frequency`
(regra de recusa da matriz: essa combinação retorna `reject`). Evidência de compatibilidade com o
codebase: `onboarding_workflow.go:941` (`Sequence` existente), `engine.go:296` (máquina de estados do
kernel).

## Alternativas Consideradas

- **State (GoF).** O onboarding tem estados e transições, mas o kernel já provê o mecanismo de estado
  durável; formalizar um State no consumidor duplicaria a máquina de estados. Rejeitado.
- **Template Method / Strategy para os steps.** Os steps já são funções compostas numa `Sequence`; não
  há variação de algoritmo intercambiável nem herança. Rejeitados.
- **Combinator de loop no kernel** (ver ADR-003 opção C). Rejeitado por blast radius no kernel genérico.

## Consequências

### Benefícios Esperados

- Menor indirecão, menor contagem de tipos, menor custo cognitivo (Etapa 7 da skill).
- Menor superfície de falha — alinhado a "economia, eficiência e robustez".

### Trade-offs e Custos

- Nenhum pattern formal a documentar; a robustez vem dos enums fechados e da pureza de `Decide*`.

### Riscos e Mitigações

- **Risco:** crescimento futuro do onboarding pedir estrutura. **Mitigação:** reavaliar o seletor se a
  frequência de mudança aumentar (a matriz então não retornaria `reject`).

## Plano de Implementação

Segue o plano das ADR-001..ADR-005; nenhuma estrutura de pattern adicional é criada.

## Monitoramento e Validação

- Selector output persistido como evidência da decisão.
- Revisão de código confirma ausência de indirecão desnecessária.

## Impacto em Documentação e Operação

- Nenhum artefato de pattern a manter.

## Revisão Futura

Reexecutar `select_pattern.py` se a área passar a exibir branching recorrente, múltiplas variações de
fluxo ou alta frequência de mudança.
