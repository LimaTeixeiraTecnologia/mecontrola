# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Alerta dedicado para a taxa de duration_exceeded
- **Data:** 2026-08-07
- **Status:** Aceita
- **Decisores:** Usuário (dono do produto), agente de engenharia
- **Relacionados:** `.specs/prd-limite-audio-20-segundos-whatsapp/prd.md` (Objetivos), `techspec.md`, ADR-001

## Contexto

O critério de sucesso do PRD é guardrail com monitoramento ativo da taxa de rejeição por `duration_exceeded`. Hoje a métrica `agents_audio_inbound_total` já é emitida com labels `outcome` e `reason` e aparece genericamente no dashboard `deployment/dashboards/agent-audio-whatsapp.json:98`, mas não existe nenhum alerta sobre taxa de rejeição por duração no grupo `audio` de `deployment/telemetry/grafana/provisioning/alerting/rules.yaml:1279`. Sem alerta, o acompanhamento prometido no PRD ficaria reativo e dependente de disciplina manual.

## Decisão

Adicionar o alerta `mc-audio-duration-exceeded-rate` ao grupo `audio` existente: razão entre `agents_audio_inbound_total{outcome="rejected",reason="duration_exceeded"}` e o total de áudios inbound, em janela de 1h, com threshold inicial de 0.30 (30%) e severidade informativa. Nenhuma instrumentação nova é criada; o alerta consome a métrica já emitida. O threshold é declaradamente provisório, pois não existe baseline prévio da taxa, e será ajustado após o primeiro ciclo de observação.

## Alternativas Consideradas

1. Somente dashboard existente com revisão manual. Vantagem: zero artefato novo. Desvantagem: monitoramento reativo, não atende o critério de sucesso do PRD. Rejeitada.
2. Painel dedicado sem alerta. Vantagem: visibilidade permanente. Desvantagem: continua exigindo que alguém olhe o painel; notificação é o diferencial do guardrail. Rejeitada.
3. Alerta com threshold agressivo baixo (ex.: 5%) desde o início. Desvantagem: sem baseline, dispararia falso positivo logo após o deploy, quando usuários de áudio longo descobrem o limite. Rejeitada.

## Consequências

### Benefícios Esperados

- Detecção ativa de impacto anômalo da redução do limite, fechando o critério de sucesso do PRD com evidência.
- Zero código novo de instrumentação e zero aumento de cardinalidade de métricas.

### Trade-offs e Custos

- Threshold inicial sem baseline pode exigir ajuste; é um custo aceito e documentado, com severidade informativa para não acionar plantão indevidamente.
- Mais uma regra no arquivo de alertas para manter.

### Riscos e Mitigações

- Risco: falso positivo na primeira semana por rejeições de usuários que hoje enviam áudios entre 20s e 60s. Mitigação: severidade informativa, janela de 1h e threshold de 30%; revisão agendada.
- Risco: a expressão PromQL divergir dos labels reais. Mitigação: validar a query contra o painel genérico existente (`agent-audio-whatsapp.json:98`), que já usa os mesmos labels `outcome` e `reason`.
- Plano de rollback: remover a regra do arquivo de alertas; nenhum impacto em runtime da aplicação.

## Plano de Implementação

1. Adicionar a regra `mc-audio-duration-exceeded-rate` ao grupo `audio` em `rules.yaml`, seguindo o formato das regras irmãs (`mc-audio-stt-error-rate` e demais em `:1283-1411`).
2. Documentar o alerta e o significado da taxa no runbook `audio-whatsapp-stt.md`.
3. Critério de conclusão: regra carregada pelo provisionamento do Grafana e query validada contra os labels existentes.

## Monitoramento e Validação

- Sucesso: alerta avaliando sem erro de query e refletindo rejeições reais após o deploy.
- Critério de revisão: ao fim do primeiro ciclo de observação (30 dias, junto com a ADR-001), ajustar threshold com a taxa real medida.

## Impacto em Documentação e Operação

- `deployment/runbooks/audio-whatsapp-stt.md`: seção de triagem passa a referenciar o alerta e a interpretação da taxa de `duration_exceeded`.

## Revisão Futura

Revisão obrigatória em 30 dias para calibrar o threshold com baseline real; eventos que invalidam a premissa: mudança do limite via env (ADR-001) ou alteração dos labels da métrica.
