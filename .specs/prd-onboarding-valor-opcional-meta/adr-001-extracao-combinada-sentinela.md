# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Extração combinada meta+valor com par sentinela `hasAmount`+`amountBRL` sob strict schema
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Jailton (owner), técnica via subagentes go-implementation/mastra
- **Relacionados:** [prd.md](prd.md) (RF-01, RF-03.1, RF-03.2, RF-09), [techspec.md](techspec.md), R-AGENT-WF-001.4

## Contexto

O `step-goal` precisa extrair, numa única chamada ao parser LLM, a meta textual e, opcionalmente, um valor monetário (RF-01). O padrão vigente (`goalSchema`, `incomeSchema` em `onboarding_workflow.go:359-375`) usa `llm.Schema{Strict:true}` com `additionalProperties:false` e todos os campos em `required`. Sob strict, um campo `required` não pode ser omitido nem nulo — logo não há como representar "valor ausente" por omissão ou `null`. Além disso, o modelo de produção é `openai/gpt-4o-mini`, e memória de projeto registra que ele é sensível a ambiguidade de schema (falso-verde C4) e responde bem a instruction-by-example.

Há ainda a repergunta específica de valor (RF-03.2), na qual a meta já é válida e o texto do usuário fala só do valor — reusar o schema combinado nesse ponto forçaria o modelo a re-extrair a meta de um texto que não a menciona.

## Decisão

1. **Dois schemas distintos**: `goalWithValueSchema` (`goal` + `hasAmount` + `amountBRL`) para a extração combinada; `goalValueSchema` (`hasAmount` + `amountBRL`) para a repergunta value-only. Cada call-site tem responsabilidade única.
2. **Par sentinela `hasAmount` (boolean) + `amountBRL` (number)**, ambos `required`: o LLM sinaliza ausência explicitamente com `hasAmount:false` (e `amountBRL:0`), em vez de omitir campo. Isso dá um alvo claro de instruction-by-example e evita a ambiguidade "0 = valor literal vs. 0 = ausência" na fronteira LLM.
3. **Conversão de formato no LLM** (RF-09): "R$ 400.000,00", "400000", "10 mil reais", "400 mil", "1,5 milhão" são convertidos para `amountBRL float64` pelo modelo, como `incomeSchema` já faz com `amountBRL number`. Nenhum parser Go de linguagem natural.
4. O smart constructor `DecideGoalValueCents(hasAmount, amountBRL)` trata `hasAmount:false` **OU** `amountBRL<=0` como ausência (defesa dupla contra alucinação do booleano). Escopo em `internal/agents/application/workflows/onboarding_workflow.go`.

## Alternativas Consideradas

- **Estender `goalSchema` com campo `amountBRL` opcional/nullable (um schema só)**: Vantagem: um único schema. Desvantagem: sob strict + `additionalProperties:false`, "opcional" exige tirar de `required`, e a ausência vira omissão — comportamento frágil no gpt-4o-mini; e a repergunta value-only reusaria um schema que exige `goal`. Rejeitada por fragilidade de sinal e responsabilidade misturada.
- **Sentinela só por `amountBRL<=0`, sem `hasAmount`**: Vantagem: schema menor. Desvantagem: sem um campo booleano dedicado, a instrução perde a âncora de exemplo que ajuda o modelo a distinguir "sem valor" de "valor zero"; maior risco de `amountBRL` fabricado. Rejeitada por robustez inferior no modelo do gate (o `int64` sentinela permanece, porém, na camada de **estado** — ver ADR-002).
- **Strategy pattern para extratores plugáveis**: rejeitada pelo gate `design-patterns-mandatory` (`reject`): só existe um caminho concreto por situação, sem troca de algoritmo em runtime; indireção sem ganho.

## Consequências

### Benefícios Esperados

- Extração combinada numa chamada (RF-01) com sinal robusto no gpt-4o-mini (suporta o gate ≥0.90 da ADR-003).
- Responsabilidade única por call-site; repergunta value-only não re-extrai meta.
- Defesa dupla (`hasAmount` + `amountBRL<=0`) reduz falso-positivo de valor.

### Trade-offs e Custos

- Dois schemas e dois system prompts a manter (vs. um). Custo baixo e localizado.
- Um campo extra (`hasAmount`) no payload do LLM.

### Riscos e Mitigações

- **Modelo fabrica `amountBRL>0` sem valor no texto** → constructor já colapsa para ausência se `hasAmount:false`; se o modelo também mentir o booleano, reforçar instruction-by-example no prompt (mitigação comprovada em C4). Validação final: harness real-LLM (ADR-003). Rollback: sem impacto em dados (valor é aditivo e opcional).

## Plano de Implementação

1. Adicionar `goalWithValueSchema`, `goalValueSchema`, structs `goalWithValueExtract`/`goalValueExtract` junto aos schemas existentes.
2. Adicionar `_goalWithValueSystemPrompt`, `_goalValueSystemPrompt` com exemplos de conversão RF-09.
3. Wire nas duas call-sites de `BuildGoalStep` (ver techspec).
4. Validar com harness real-LLM (ADR-003).

## Monitoramento e Validação

- Critério de sucesso: gate real-LLM ≥ 0.90 em `openai/gpt-4o-mini` (ADR-003).
- Sinal de revisão: se o harness ficar cronicamente marginal (~0.90) por instabilidade de `hasAmount`, revisar prompt/schema.

## Impacto em Documentação e Operação

- Nenhum runbook novo. A techspec referencia esta ADR.

## Revisão Futura

- Revisitar se o provider/modelo de produção mudar (o design de schema é calibrado para gpt-4o-mini), ou se um segundo consumidor precisar do mesmo padrão de extração opcional (candidato a helper reutilizável).
