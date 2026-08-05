# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 0 (bug puramente documental, sem alteracao de codigo)
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: BUG-003
- Severidade: minor (achado original da review, nivel `low` no schema de 4 niveis)
- Origem: finding de review (lacuna de rastreabilidade spec <-> schema real, migrations 000015/000016 do PRD `prd-agente-audio-openrouter`)
- Estado: fixed
- Causa raiz: a task 6.0 precisou ampliar o `CHECK` de `reason` da tabela `agents_whatsapp_audio_messages` (migration 000016) para cobrir 6 razoes pre-STT nao previstas na decisao original da ADR-003/migration 000015 (task 5.0). Essa origem so ficou registrada no relatorio de execucao da task 6.0 (`6.0_execution_report.md`), nunca na ADR nem na techspec, criando lacuna de rastreabilidade entre spec e schema.
- Arquivos alterados:
  - `.specs/prd-agente-audio-openrouter/adr-003-auditoria-sem-audio-bruto.md` (novo adendo "Adendo 2026-08-04 — Ampliacao do CHECK de reason (migration 000016)")
- Teste de regressao: nao aplicavel — correcao e exclusivamente documental (texto de ADR), sem codigo `.go` ou `.sql` alterado. Validacao consistiu em confirmar que o conteudo do adendo bate exatamente com o schema real (migrations 000015/000016) e com o codigo de dominio (`AudioReason`/`IsValid()`).
- Validacao:
  - Leitura de `migrations/000015_agents_whatsapp_audio_messages.up.sql` (CHECK original com 7 razoes) e `migrations/000016_agents_whatsapp_audio_messages_widen_reason.up.sql` (CHECK ampliado com 13 razoes) confirmando os valores citados no adendo.
  - Leitura de `internal/agents/application/usecases/decide_audio_transcription.go` confirmando que as 13 constantes de `AudioReason` e `IsValid()` batem 1:1 com o CHECK pos-000016.
  - Leitura de `.specs/prd-agente-audio-openrouter/6.0_execution_report.md` confirmando a origem real da mudanca (secoes "Arquivos Alterados" e "Suposicoes") e o teste de integracao real `TestInsertTerminalAcceptsPreSTTRejectionReasons`.
  - Verificacao estrutural do Markdown resultante: `grep -n "^#" adr-003-auditoria-sem-audio-bruto.md` mostra a nova secao `## Adendo 2026-08-04 — ...` corretamente aninhada entre `## Impacto em Documentacao e Operacao` e `## Revisao Futura`, sem quebrar headers ou tabelas existentes.

## Comandos Executados
- `sed -n '1,80p' migrations/000015_agents_whatsapp_audio_messages.up.sql` -> confirmou CHECK original com 7 razoes
- `cat migrations/000016_agents_whatsapp_audio_messages_widen_reason.up.sql` -> confirmou CHECK ampliado com 13 razoes
- `grep -n "AudioReason\|Reason.*=.*\"" internal/agents/application/usecases/decide_audio_transcription.go` -> confirmou as 13 constantes fechadas e `IsValid()`
- `grep -n "^#" .specs/prd-agente-audio-openrouter/adr-003-auditoria-sem-audio-bruto.md` -> confirmou integridade estrutural do Markdown pos-edicao

## Riscos Residuais
- Nenhum. Nenhum arquivo `.go`, `.sql` ou `tasks.md` foi alterado; a mudanca e aditiva e nao afeta build, testes ou comportamento em producao.
