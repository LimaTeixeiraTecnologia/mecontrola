# Prompt Mandatório — Review -> Bugfix Loop até APPROVED real (`prd-refatoracao-agent-canonico`)

> **Uso:** cole o bloco **PROMPT PRONTO PARA USO** como mensagem inicial de uma sessão dedicada.
> **Objetivo:** executar `review`, usar a saída da review como entrada do `bugfix`, repetir quantas rodadas forem necessárias e só encerrar com `APPROVED` real.
> **Fonte da verdade:** working tree atual + `AGENTS.md` + `.specs/prd-refatoracao-agent-canonico/` + `docs/plans/2026_06_25_runbook_jornada_completa_agent_canonico.md`.

---

## Comparativo rápido

| Aspecto | Prompt original | Prompt enriquecido |
|---|---|---|
| Objetivo | Review + bugfix em loop | Loop canônico `review -> bugfix -> review` até `APPROVED` real |
| Escopo | `.specs/prd-refatoracao-agent-canonico` | Spec completo + runbook + working tree + código afetado |
| Critério de parada | Implícito | Somente `APPROVED`; `APPROVED_WITH_REMARKS` não encerra |
| Qualidade | "100% aceite / DoD / 0 gaps" | Gate binário com proibição explícita de falso positivo e flexibilização |
| Interação com usuário | Mencionada | Runbook tratado como fonte das jornadas/interações; perguntar só quando não houver inferência segura |
| Saída esperada | Não definida | Formato obrigatório por rodada + veredito final |

---

## Prompt original

```text
Execute a skill de review e use o output da review como input para a skill de bugfix. Faça quantas rodadas forem necessárias para validar o que foi implementado em `.specs/prd-refatoracao-agent-canonico`, sem flexibilizar. Precisa atender 100% dos critérios de aceite, DoD, 0 gaps, 0 lacunas, 0 falso positivo, MVP robusto, eficiente, production-ready/proof realmente pronto para main, considerando também as iterações com o usuário documentadas em `docs/plans/2026_06_25_runbook_jornada_completa_agent_canonico.md`.
```

---

## Prompt pronto para uso

```text
Execute um loop mandatório de `review -> bugfix -> review` sobre a implementação relacionada a `.specs/prd-refatoracao-agent-canonico/` até obter `APPROVED` real, sem flexibilização.

OBJETIVO INEGOCIÁVEL
- Meta final: 100% dos critérios de aceite e DoD atendidos.
- Tolerância zero: 0 gaps, 0 lacunas, 0 falso positivo.
- O resultado precisa estar realmente pronto para `main`: MVP robusto, eficiente e production-ready/proof.
- O loop só termina com veredito canônico `APPROVED`.
- `APPROVED_WITH_REMARKS` deve ser tratado como não aprovado e obriga nova rodada de bugfix.

FONTES OBRIGATÓRIAS DE VERDADE
1. `AGENTS.md`
2. `.specs/prd-refatoracao-agent-canonico/prd.md`
3. `.specs/prd-refatoracao-agent-canonico/techspec.md`
4. `.specs/prd-refatoracao-agent-canonico/tasks.md`
5. Todas as tasks e ADRs dentro de `.specs/prd-refatoracao-agent-canonico/`
6. `docs/plans/2026_06_25_runbook_jornada_completa_agent_canonico.md`
7. Working tree atual do repositório

REGRAS HARD
- Não flexibilize nada.
- Não invente aceite atendido, evidência, comportamento, wiring ou cobertura inexistente.
- Não marque concluído com "aparentemente ok".
- Se spec, plano e código divergirem, o working tree atual é a fonte da verdade; registre drift explicitamente e siga a opção mais segura.
- O ponto de partida obrigatório da análise de wiring/execução é `cmd/server/server.go` e/ou `cmd/worker/worker.go`; não use `internal/platform/runtime` como ponto de partida.
- Use o runbook como fonte obrigatória das jornadas, interações com o usuário, fluxos verbatim e critérios observáveis de não regressão.
- Pergunte ao usuário apenas se houver decisão de comportamento realmente aberta e sem inferência segura a partir do spec, runbook e código.

PROTOCOLO OBRIGATÓRIO DE EXECUÇÃO
1. Carregue o contexto obrigatório.
2. Execute a skill `review` sobre o escopo completo da iniciativa, confrontando implementação real vs. PRD, techspec, tasks, ADRs e runbook.
3. Colete o veredito canônico da review.
4. Se o veredito for `APPROVED`, só encerre se realmente não houver nenhum gap aberto.
5. Se o veredito for `APPROVED_WITH_REMARKS`, `REJECTED` ou qualquer achado aberto:
   - use a saída da `review` como entrada direta da skill `bugfix`;
   - preserve integralmente contexto, severidade, arquivos, sintomas, critérios violados e evidências apontadas pela review;
   - corrija pela causa raiz, não por remendo;
   - após concluir os bugfixes, execute nova rodada de `review`.
6. Repita o ciclo sem limite de rodadas até `APPROVED` real.
7. Se surgir `BLOCKED`, pare somente com bloqueio explícito, causa concreta e input exato faltante.

COMO USAR A SAÍDA DA REVIEW COMO ENTRADA DO BUGFIX
- Cada achado da review deve virar item de entrada do bugfix sem perda de contexto.
- Se vários achados tiverem a mesma causa raiz, o bugfix pode tratá-los em conjunto, desde que a resposta cite explicitamente todos os achados cobertos.
- Nenhum achado pode ser descartado, suavizado ou reinterpretado para parecer menor do que é.
- Após cada bugfix, a nova review deve verificar tanto o delta quanto os impactos colaterais no escopo inteiro.

ESCOPO MÍNIMO OBRIGATÓRIO DA REVIEW EM TODA RODADA
- Bundle completo `.specs/prd-refatoracao-agent-canonico/`
- Runbook `docs/plans/2026_06_25_runbook_jornada_completa_agent_canonico.md`
- Código alterado e código diretamente impactado
- Entry points `cmd/server/server.go` e `cmd/worker/worker.go`
- Wiring, workflows, tools, bindings, usecases, adapters, migrations, testes e contratos tocados pela iniciativa
- Critérios de aceite, DoD e fluxos observáveis descritos no spec e no runbook

GATES DE QUALIDADE INEGOCIÁVEIS
- 100% dos critérios de aceite atendidos
- 100% do DoD atendido
- 0 gap funcional
- 0 lacuna de arquitetura, wiring, contrato, fluxo ou validação
- 0 falso positivo de conclusão
- nenhuma regressão observável nos fluxos válidos do runbook
- pronto para `main` de verdade, não "quase pronto"

FORMATO OBRIGATÓRIO DE SAÍDA POR RODADA
## Rodada N
### Veredito da review
- `APPROVED`, `APPROVED_WITH_REMARKS`, `REJECTED` ou `BLOCKED`

### Achados
| ID | Severidade | Arquivo/escopo | Gap encontrado | Critério/DoD afetado | Ação exigida |
|---|---|---|---|---|---|

### Entrada encaminhada ao bugfix
- Liste exatamente quais achados da review foram enviados ao bugfix.

### Resultado do bugfix
- Descreva objetivamente o que foi corrigido por causa raiz.

### Revalidação
- Diga se a rodada seguinte é nova `review` ou se houve bloqueio real.

CRITÉRIO FINAL DE ENCERRAMENTO
- Só encerre quando a última review retornar `APPROVED` genuíno.
- `APPROVED_WITH_REMARKS` não encerra.
- Se restar qualquer pendência, observação crítica, risco aberto, evidência faltante ou dúvida sobre aceite/DoD, continue o loop.
- Não declare sucesso parcial.
```

---

## Justificativa objetiva das adições

- **Fontes obrigatórias nomeadas:** elimina ambiguidade de escopo.
- **Protocolo explícito de loop:** garante que review alimente bugfix sem desvio.
- **Critério de parada binário:** evita encerrar com falso positivo.
- **Runbook como fonte das interações:** ancora a validação nas jornadas reais do usuário.
- **Formato de saída por rodada:** força rastreabilidade entre achado, correção e nova validação.
