# Relatorio de Bugfix

- Total de bugs no escopo: 3
- Corrigidos: 3
- Testes de regressao adicionados: 6
- Pendentes: nenhum
- Estado final: done

## Bugs
- ID: BUG-01
- Severidade: major
- Origem: RF-11 (finding F1 do review 2026-07-15)
- Estado: fixed
- Causa raiz: `DecideTreatmentName` colapsava `>40 caracteres` no mesmo retorno `("",false)` de vazio/recusa, sem distinguir o caso "muito longo". No fluxo de edicao o reprompt caia na copy generica "Nao entendi..." e o caminho de nome fornecido caia na pergunta padrao, nunca "pedindo uma forma mais curta" exigida por RF-11.
- Arquivos alterados: internal/agents/application/workflows/treatment_name_edit_decisions.go (novo predicado puro `DecideTreatmentNameTooLong`); internal/agents/application/messages/catalog.go (novo builder `TreatmentNameTooLong`); internal/agents/application/workflows/treatment_name_edit_workflow.go (ramo de nome fornecido e ramo de reprompt passam a emitir a mensagem de "forma mais curta" quando o valor excede 40).
- Teste de regressao: TestProvidedNameTooLongAsksForShorterForm e TestResumeNameTooLongRepromptsForShorterForm (treatment_name_edit_workflow_test.go); TestDecideTreatmentNameTooLong (treatment_name_edit_decisions_test.go); TestTreatmentNameTooLong (catalog_test.go).
- Validacao: `go test -race ./internal/agents/application/workflows/... ./internal/agents/application/messages/...` -> 781 passed.
- Escopo/reconciliacao: a reperguntar-forma-mais-curta aplica-se ao fluxo de edicao (RF-06/RF-08). No onboarding, um valor >40 segue sem nome (skip), conforme ADR-003 (captura sem loop de reprompt) e RF-04 (nome opcional, nunca bloqueia) — decisao arquitetural aceita que domina a captura.

- ID: BUG-02
- Severidade: major
- Origem: RF-11 (finding F2 do review 2026-07-15)
- Estado: fixed
- Causa raiz: `DecideTreatmentName` so validava vazio-apos-trim e comprimento; um valor composto apenas por simbolos/pontuacao (ex.: "!!!", "@#$ ...") passava como utilizavel, contrariando a clausula de RF-11 que rejeita "valor composto apenas por simbolos/pontuacao".
- Arquivos alterados: internal/agents/application/workflows/treatment_name_edit_decisions.go (helper puro `treatmentNameHasMeaningfulRune`: exige ao menos uma letra, digito ou rune nao-ASCII/emoji; caso contrario rejeita). Correcao vale para os dois consumidores (onboarding e edicao) por serem a mesma funcao pura.
- Teste de regressao: casos "somente simbolos invalido", "somente pontuacao invalido", "numero valido" e "emoji valido" em TestDecideTreatmentName (treatment_name_edit_decisions_test.go).
- Validacao: `go test -race ./internal/agents/application/workflows/...` -> passed. RF-11 preserva aceite de emojis (rune > MaxASCII e considerada significativa) e de numeros (IsDigit).

- ID: BUG-03
- Severidade: minor
- Origem: RF-13 (finding F3 do review 2026-07-15)
- Estado: fixed
- Causa raiz: em `executeTreatmentNameEdit`, a ordem era `Upsert(working_memory)` (fonte de verdade observavel do LLM) antes de `UpsertMetadata` (mirror). Se o metadata falhasse apos o conteudo, o passo retornava `StepStatusFailed` sem confirmar, mas o nome observavel ja havia mudado — violando "o nome de tratamento anterior permanece inalterado" (RF-13).
- Arquivos alterados: internal/agents/application/workflows/treatment_name_edit_workflow.go (inverte a ordem: `UpsertMetadata` primeiro, `Upsert(working_memory)` por ultimo como ponto de commit observavel).
- Teste de regressao: TestMetadataFailureLeavesObservableNameUnchanged (assere `StepStatusFailed`, sem mensagem de confirmacao e `Upsert` de conteudo nao chamado quando o metadata falha); TestUpsertFailureReturnsFailedStatusWithoutConfirming atualizado para a nova ordem.
- Validacao: `go test -race ./internal/agents/application/workflows/...` -> passed.

## Comandos Executados
- `go build ./internal/agents/... ./internal/platform/...` -> exit 0
- `go vet ./internal/agents/application/workflows/... ./internal/agents/application/tools/... ./internal/agents/application/messages/...` -> exit 0
- `go test -race ./internal/agents/application/workflows/... ./internal/agents/application/tools/... ./internal/agents/application/messages/... ./internal/agents/application/golden/... ./internal/agents/application/postdeploy/... ./internal/agents/` -> 781 passed
- `gofmt -l <arquivos alterados>` -> vazio (exit 0)
- `go build -tags integration ./internal/agents/... ./internal/platform/...` -> exit 0
- Gate zero-comentarios (R-ADAPTER-001.1) nos arquivos de producao alterados -> sem ocorrencias
- Gate estado-fechado (`TreatmentNameEditStatus = "..."`) -> sem ocorrencias

## Riscos Residuais
- Janela conteudo<->metadata invertida: se `UpsertMetadata` suceder e o `Upsert` de conteudo falhar, o metadata (mirror analitico) fica adiante do conteudo. Nao e observavel pelo LLM (que le apenas o working_memory) e cicatriza na proxima edicao bem-sucedida; RF-13 (nome observavel inalterado no erro) permanece satisfeito. Documentado em ADR-001 como mirror analitico.
- Gate golden real-LLM (RF-14) permanece nao executado neste ciclo por exigir `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY`; artefatos (categoria `CategoryTreatmentName`, casos, stub de tool, threshold 0,90) presentes e compilando sob `-tags integration`.
