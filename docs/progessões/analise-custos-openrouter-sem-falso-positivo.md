# Prompt — Análise de Custos OpenRouter sem Falso Positivo

> Prompt pronto para uso. Colar inteiro em uma sessão nova. Criado em 2026-08-07
> a partir de fatos verificados em produção (jornada documentada nesta mesma pasta).
> Regra-mãe: **todo número precisa de fonte citada; o que não puder ser medido é declarado, nunca estimado como fato.**

````text
<objetivo>
Analisar os custos reais de OpenRouter do mecontrola (produção + CI) e responder:
quanto foi gasto, onde, quando, e o que mais gastou — com cada número ligado à sua
fonte. Done = relatório em que todo valor tem fonte e toda projeção está rotulada
como projeção.
</objetivo>

<contexto>
- Credenciais em `.env` (raiz do repo): OPENROUTER_BASE_URL, OPENROUTER_API_KEY.
  NUNCA exibir a chave; extrair com grep + cut, usar só em header Authorization.
- Produção roda na VPS: `ssh mecontrola-vps`. Containers swarm:
  mecontrola_postgres, mecontrola_otel-lgtm (Prometheus interno em localhost:9090,
  acessível via docker exec), mecontrola_server, mecontrola_worker.
- Banco: `docker exec <postgres> psql -U mecontrola -d mecontrola_db`, schema `mecontrola`.
- A MESMA chave OpenRouter pode ser usada por produção, CI (GitHub Actions),
  testes locais e máquinas de dev. Gasto fora da VPS só é detectável por diferença
  contra os tokens medidos — declarar isso, não atribuir sem prova.
- Uso de tokens NÃO é persistido por usuário no banco (só métricas agregadas
  OTel). Custo por usuário individual NÃO é computável; média por usuário =
  gasto_total / count(users WHERE status='ACTIVE' AND deleted_at IS NULL).
</contexto>

<fontes_em_ordem_de_verdade>
1. FATURADO (verdade oficial): GET $OPENROUTER_BASE_URL/api/v1/credits
   (total_credits, total_usage) e /api/v1/auth/key (usage_daily, usage_weekly,
   usage_monthly). São os únicos números de dinheiro oficiais.
2. TOKENS (medição da app): Prometheus na VPS —
   `query=sum by (model,type) (agent_llm_tokens_total)` com type ∈ {prompt,
   completion, cached}; chamadas em `sum by (model,status)
   (agent_llm_provider_call_total)`; STT medido pela app no banco:
   `SELECT stt_model, count(*), sum(cost_microusd)/1e6 FROM
   mecontrola.agents_whatsapp_audio_messages GROUP BY 1`.
3. PREÇOS (correntes): GET /api/v1/models — campo pricing.prompt/completion/
   input_cache_read por model. Se o modelo não constar na lista (ex.: whisper,
   text-embedding-3-small não constavam em 2026-08), declarar "preço indisponível
   no endpoint" em vez de citar preço de memória.
4. CÂMBIO: https://economia.awesomeapi.com.br/json/last/USD-BRL — registrar bid,
   fonte e timestamp da cotação.
</fontes_em_ordem_de_verdade>

<regras_anti_falso_positivo>
- PROIBIDO usar increase()/rate() do Prometheus como verdade: validado em
  2026-08-07 que extrapola ~5–10x neste ambiente (STT: 22 chamadas na métrica
  vs. 4 reais no banco). Usar valores brutos de contador e checar resets.
- Contadores zeram a cada deploy: antes de agregar janelas, checar deploys
  (`gh run list --workflow=cd.yml --limit N`) e, se houver deploy no meio da
  janela, somar por instância ou declarar a quebra de série.
- Sempre declarar tamanho da amostra (número de chamadas) junto de qualquer
  taxa ou média. Hit rate com 4 chamadas ≠ fato; é leitura de janela.
- Cache de prompt: economia = cached × (preço_prompt − preço_cache_read).
  Nunca assumir que cache existe sem a série type="cached" ou experimento
  (2 chamadas idênticas, comparar usage.prompt_tokens_details.cached_tokens
  e cost da 2ª).
- Distinguir FATO MEDIDO de PROJEÇÃO ARITMÉTICA: projeção = baseline × taxa,
  e deve vir rotulada "projeção" com a premissa explícita.
- Se uma fonte falhar (endpoint fora do ar, métrica ausente, coluna inexistente),
  declarar "indisponível" e seguir com as demais; proibido preencher lacuna
  com valor plausível.
- Todo relatório termina com seção "Limitações" listando o que não foi possível
  verificar e por quê.
</regras_anti_falso_positivo>

<formulas>
- custo_chat = (prompt − cached) × pricing.prompt + cached × pricing.input_cache_read
  + completion × pricing.completion   (preços por token, /1e6 se vierem por Mtok)
- custo_stt = sum(cost_microusd)/1e6  (banco, medido pela própria app)
- custo_total_BRL = (custo_chat + custo_stt) × USDBRL.bid
- media_por_usuario_BRL = custo_total_BRL / usuarios_ativos
</formulas>

<formato_de_saida>
Tabelas com colunas: item | USD | BRL | fonte (endpoint/query + timestamp).
Seções obrigatórias: (1) faturado oficial; (2) breakdown por modelo/feature;
(3) antes × depois quando houver mudança sendo avaliada; (4) média por usuário
com a fórmula; (5) Limitações. Projeções em tabela separada, rotuladas.
</formato_de_saida>
````

## Armadilhas já validadas (não repita a investigação — cite este histórico)

| Armadilha | Evidência (2026-08-07) |
|---|---|
| `increase()` do Prometheus infla 5–10x | STT: 22 chamadas métrica vs. 4 no banco |
| Golden gate do CI era ~99% da fatura | ~472 execuções/deploy × ~8 deploys/dia; US$ 6,55/dia = 6 gates × ~US$ 1 |
| Chave de prod == chave local/CI | label `sk-or-v1-a25…b6c` confere com `/api/v1/auth/key` |
| Métricas zeram a cada deploy | série `agent_llm_tokens_total` sumiu após deploy `e037189` |
| Whisper/embedding sem preço no `/api/v1/models` | 400 modelos listados, nenhum whisper/embedding |
| Golden harness usa fake metrics | `buildGoldenHarnessProvider` → `fake.NewProvider()` — gate não gera métrica Prometheus |

## Estado de custo após as mudanças de 2026-08-07

- Golden gate: somente `workflow_dispatch` (US$ 0/dia automático).
- Produção: ~US$ 0,02/dia com 1 usuário ativo; cache de prompt ativo e medido
  (`type="cached"`, primeira leitura: 45,6% hit, −22,8% no custo de prompt da janela).
- Zerar 100% do custo automático já é fato; o custo restante é dirigido por
  tráfego real de usuário — só chega a zero desligando o agente.
````

> Nota: este arquivo é documentação operacional; atualize a tabela de armadilhas
> quando novas forem validadas em produção.
