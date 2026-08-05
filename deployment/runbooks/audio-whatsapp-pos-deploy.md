# Pos-deploy: validar e acompanhar audio no WhatsApp (`ssh mecontrola-vps`)

Complementa `deployment/runbooks/audio-whatsapp-stt.md`. Este arquivo e a sequencia operacional a
executar **depois** do deploy, na ordem. Nada aqui presume que a feature funciona: cada passo tem um
criterio objetivo de passa/falha e o que fazer quando falha.

Premissa de rollout: subir com `AGENT_AUDIO_ENABLED=false`, validar as fases 0-2, so entao ligar.

---

## Fase 0 — Deploy no ar e migration aplicada (flag ainda desligada)

```bash
ssh mecontrola-vps
```

```bash
docker service ls
docker service ps mecontrola_server --no-trunc | head
docker service ps mecontrola_worker --no-trunc | head
```

**Passa:** replicas `2/2` em server e worker, sem tasks em `Rejected`/`Failed` recentes.

Migration (a de audio e a `15`; `16` amplia o CHECK de `reason`; `17` adiciona o estado
nao-terminal `processing` do ADR-002):

```bash
docker exec -i $(docker ps -qf name=mecontrola_postgres) \
  psql -U mecontrola -d mecontrola -c \
  "SELECT version, dirty FROM schema_migrations;"
```

**Passa:** `version >= 17` e `dirty = false`.
**Falha:** `dirty = true` ⇒ nao ligue a flag; investigar a migration antes de qualquer coisa.

Confirmar que a tabela de auditoria nao guarda midia bruta (invariante de privacidade RF-24):

```bash
docker exec -i $(docker ps -qf name=mecontrola_postgres) \
  psql -U mecontrola -d mecontrola -c \
  "\d agents_whatsapp_audio_messages"
```

**Passa:** nenhuma coluna `bytea`/`blob`/`audio_data`/`media_url`. Devem existir `media_sha256`,
`transcription_sha256`, `outcome`, `reason`.

Config efetivamente carregada:

```bash
docker exec $(docker ps -qf name=mecontrola_worker | head -1) env | grep -E 'AGENT_AUDIO|AGENT_STT'
```

**Passa:** `AGENT_AUDIO_ENABLED=false` nesta fase, `AGENT_STT_MODEL` preenchido,
`AGENT_AUDIO_MAX_BYTES` e `AGENT_AUDIO_MAX_COST_MICROUSD` maiores que zero.

---

## Fase 1 — Regressao do fluxo textual (obrigatoria, flag desligada)

O risco numero um de um deploy de audio e quebrar quem digita. Valide isso **antes** de ligar audio.

Envie por WhatsApp, do seu numero real, uma mensagem **de texto**: `gastei 25 reais no mercado`.

```bash
docker exec -i $(docker ps -qf name=mecontrola_postgres) \
  psql -U mecontrola -d mecontrola -c \
  "SELECT status, attempts, left(error,120) AS erro, created_at
     FROM outbox_events
    WHERE event_type = 'agents.whatsapp.inbound.v1'
    ORDER BY created_at DESC LIMIT 5;"
```

**Passa:** ultimo evento com `status=3` (processado) e `attempts` baixo; voce recebeu resposta no
WhatsApp e o lancamento apareceu.
**Falha:** `status=4` (dead-letter) ⇒ **rollback imediato**, o deploy quebrou o fluxo textual.

---

## Fase 2 — Comportamento com a flag desligada (regressao do defeito corrigido)

Este passo existe porque, antes desta correcao, audio com a flag desligada ia para dead-letter sem
resposta nenhuma ao usuario.

Envie um **audio** pelo WhatsApp com `AGENT_AUDIO_ENABLED=false`.

**Passa:** voce recebe no WhatsApp
`"entrada por áudio ainda não está disponível por aqui, pode digitar sua mensagem?"`
e o evento fica `status=3` (nao dead-letter):

```bash
docker exec -i $(docker ps -qf name=mecontrola_postgres) \
  psql -U mecontrola -d mecontrola -c \
  "SELECT status, attempts, left(error,160) AS erro
     FROM outbox_events
    WHERE event_type='agents.whatsapp.inbound.v1'
    ORDER BY created_at DESC LIMIT 3;"
```

**Falha:** silencio no WhatsApp, ou `status=4` com `payload incompleto` ⇒ o binario no ar nao contem
a correcao de roteamento. Nao prossiga.

Confirme tambem que **nada** foi para Meta Media API nem OpenRouter nesta fase: a tabela de
auditoria de audio deve continuar vazia.

```bash
docker exec -i $(docker ps -qf name=mecontrola_postgres) \
  psql -U mecontrola -d mecontrola -c \
  "SELECT count(*) FROM agents_whatsapp_audio_messages;"
```

**Passa:** `0`.

---

## Fase 3 — Ligar a flag (canario) e validar o caminho feliz

```bash
docker service update --env-add AGENT_AUDIO_ENABLED=true mecontrola_worker
docker service update --env-add AGENT_AUDIO_ENABLED=true mecontrola_server
docker service ps mecontrola_worker | head
```

Aguarde `2/2` novamente antes de testar.

### Cenario 3.1 — Despesa por audio (caminho feliz)

Envie um audio dizendo: **"gastei cinquenta reais no mercado hoje no debito"**.

**Passa:**
- resposta textual no WhatsApp confirmando o lancamento, equivalente ao que o texto faria;
- auditoria com outcome terminal `dispatched`:

```bash
docker exec -i $(docker ps -qf name=mecontrola_postgres) \
  psql -U mecontrola -d mecontrola -c \
  "SELECT outcome, reason, duration_ms, size_bytes, stt_model, cost_microusd,
          (transcription IS NOT NULL) AS tem_transcricao, error_code
     FROM agents_whatsapp_audio_messages
    ORDER BY created_at DESC LIMIT 5;"
```

Esperado: `outcome=dispatched`, `reason=approved`, `duration_ms` preenchido, `cost_microusd` na casa
de centenas (audio curto), `tem_transcricao=t`, `error_code` nulo.

**Falha critica a vigiar:** se aparecer `outcome=transcription_uncertain` com
`reason=language_unsupported` em **todo** audio, o binario no ar **nao** tem o fallback de idioma —
a feature esta inoperante. Desligue a flag e investigue.

### Cenario 3.2 — Nenhum audio incerto vira lancamento

Envie um audio **ruim de proposito**: sussurro/ruido, ou fale 1 segundo e corte.

**Passa:** resposta pedindo reenvio, e nenhuma transacao criada. Verifique que nao houve run:

```bash
docker exec -i $(docker ps -qf name=mecontrola_postgres) \
  psql -U mecontrola -d mecontrola -c \
  "SELECT a.wamid, a.outcome, a.reason,
          (SELECT count(*) FROM platform_runs r WHERE r.correlation_key = a.wamid) AS runs
     FROM agents_whatsapp_audio_messages a
    ORDER BY a.created_at DESC LIMIT 5;"
```

**Passa:** para linhas com `outcome` em (`transcription_uncertain`, `transcription_failed`,
`rejected`), a coluna `runs` deve ser `0`. Qualquer valor maior que zero e violacao de RF-15/RF-35 e
exige desligar a flag imediatamente.

### Cenario 3.3 — Idempotencia por WAMID

Encaminhe o **mesmo** audio novamente (forward no WhatsApp gera novo WAMID; para testar de fato,
reenvie e compare). Cada WAMID deve ter exatamente uma linha:

```bash
docker exec -i $(docker ps -qf name=mecontrola_postgres) \
  psql -U mecontrola -d mecontrola -c \
  "SELECT wamid, count(*) FROM agents_whatsapp_audio_messages
    GROUP BY wamid HAVING count(*) > 1;"
```

**Passa:** zero linhas. Duplicidade aqui significaria dupla mutacao financeira.

### Cenario 3.4 — Audio longo (regressao do teto de custo)

Grave um audio de **~55-60 segundos** falando um lancamento no fim.

**Passa:** processado normalmente. **Falha:** `outcome=rejected, reason=cost_exceeded` ⇒ o binario
no ar ainda tem a taxa de preflight antiga (`34` microusd/s), que rejeitava 59-60s.

### Cenario 3.5 — Privacidade nos logs (RF-27)

```bash
docker service logs mecontrola_worker --since 30m 2>&1 | grep -i "process_audio_inbound" | tail -20
```

**Passa:** linhas com `outcome`, `reason`, `mime_family`, `duration_ms`, `size_bytes`, `stt_model`.
**Falha:** qualquer log contendo a transcricao, base64, ou a URL temporaria da Meta.

### Cenario 3.6 — Nenhuma linha presa em `processing` (ADR-002)

A linha de auditoria e aberta com `outcome='processing'` **antes** do download e finalizada no fim.
Uma linha que fique nesse estado por muito tempo indica worker morto no meio do processamento.

```bash
docker exec -i $(docker ps -qf name=mecontrola_postgres) \
  psql -U mecontrola -d mecontrola -c \
  "SELECT wamid, created_at, now() - created_at AS idade
     FROM agents_whatsapp_audio_messages
    WHERE outcome = 'processing'
      AND created_at < now() - interval '5 minutes'
    ORDER BY created_at;"
```

**Passa:** zero linhas. Se houver, a proxima entrega do mesmo WAMID a finaliza automaticamente como
`transcription_failed / interrupted` e responde ao usuario pedindo reenvio — o audio **nao** e
reprocessado (ADR-002). Investigar por que o worker morreu; a linha em si nao trava nada.

---

## Fase 4 — Janela de observacao (primeiras 24-48h)

### Metricas (Prometheus, dentro da VPS)

```bash
q() { curl -sG --data-urlencode "query=$1" http://localhost:9090/api/v1/query | jq -r '.data.result[] | "\(.metric) => \(.value[1])"'; }

# distribuicao de outcomes de audio
q 'sum by (outcome,reason) (increase(agents_audio_inbound_total[1h]))'

# taxa de falha de STT (alerta dispara > 5%)
q 'sum(increase(agents_audio_inbound_total{outcome="transcription_failed"}[15m])) / clamp_min(sum(increase(agents_audio_inbound_total[15m])),1)'

# taxa de incerteza tecnica (alerta dispara > 20%)
q 'sum(increase(agents_audio_inbound_total{outcome="transcription_uncertain"}[15m])) / clamp_min(sum(increase(agents_audio_inbound_total[15m])),1)'

# p95 de transcricao (alvo <= 8s)
q 'histogram_quantile(0.95, sum by (le) (rate(agents_audio_transcription_latency_seconds_bucket[15m])))'

# custo acumulado em microusd na ultima hora
q 'sum(increase(agents_audio_cost_microusd_total[1h]))'

# falso-sucesso financeiro: precisa ser 0
q 'sum(increase({__name__=~"agents_.+_false_success_total"}[1h]))'
```

Criterios de aceite da janela:

| Sinal | Verde | Acao se vermelho |
|---|---|---|
| `false_success` | exatamente `0` | **reverter flag imediatamente**, sem investigar antes |
| taxa de falha STT | `< 5%` | ver runbook secao 3.2 |
| taxa de incerteza | `< 20%` | ver runbook secao 3.3 (lembrar: idioma **nao** e causa plausivel) |
| p95 transcricao | `<= 8s` | ver runbook secao 3.4 |
| custo/hora | dentro do esperado para o volume | ver runbook secao 3.5 |
| duplicidade por WAMID | `0` | investigar dedup antes de continuar |

### Alertas provisionados

```bash
curl -s http://localhost:3000/api/v1/provisioning/alert-rules \
  -u admin:$GRAFANA_PASS | jq -r '.[] | select(.title|test("audio";"i")) | .title'
```

Esperado: `mc-audio-stt-error-rate`, `mc-audio-transcription-uncertain-rate`,
`mc-audio-transcription-latency-p95`, `mc-audio-cost-microusd-high`, `mc-audio-false-success`.

### Logs por outcome (Loki)

```bash
curl -sG http://localhost:3100/loki/api/v1/query_range \
  --data-urlencode 'query={service_name="mecontrola-worker"} |= "process_audio_inbound"' \
  --data-urlencode "start=$(date -u -d '1 hour ago' +%s)000000000" \
  --data-urlencode "end=$(date -u +%s)000000000" | jq -r '.data.result[].values[][1]' | tail -20
```

---

## Rollback

Do mais barato ao mais caro:

1. **Desligar a flag** (mantem texto intacto, audio volta a responder "indisponivel"):

```bash
docker service update --env-add AGENT_AUDIO_ENABLED=false mecontrola_worker
docker service update --env-add AGENT_AUDIO_ENABLED=false mecontrola_server
```

2. **Rollback da imagem**, se o problema nao for especifico de audio:

```bash
docker service rollback mecontrola_worker
docker service rollback mecontrola_server
```

A migration `000015`/`000016` e aditiva e **nao** precisa ser revertida no rollback de imagem: a
tabela simplesmente para de receber linhas.

Gatilho de reversao imediata, sem investigacao previa: qualquer disparo de `mc-audio-false-success`,
ou qualquer evidencia de mutacao financeira originada de audio classificado como incerto.

---

## Limitacao a ter em mente durante a operacao

Deteccao de idioma **nao e um controle ativo** (ver `audio-whatsapp-stt.md` secao 2.1): nenhum modelo
STT do OpenRouter retorna `language`. Um audio em outro idioma sera transcrito a forca como PT-BR e
so sera barrado se cair nos gates de texto vazio, incoerencia ou truncamento. Ao triar incerteza
alta, os motivos plausiveis sao `empty_text`, `incoherent` e `truncated` — nao `language_unsupported`.
