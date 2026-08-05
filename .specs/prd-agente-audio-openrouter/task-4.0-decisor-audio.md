# Tarefa 4.0: Decisor técnico fechado de áudio

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar tipos fechados e decisor puro para aprovar, rejeitar ou marcar transcrição como incerta antes de qualquer entrada no fluxo financeiro. A decisão deve ser determinística, sem IO e sem inferir semântica financeira.

<requirements>
- Cobrir RF-13, RF-14, RF-15, RF-16, RF-17, RF-19, RF-33, RF-35 e RF-42.
- `AudioOutcomeApproved` deve ser o único outcome que permite `HandleInbound`.
- Texto vazio, idioma não PT-BR, truncamento, erro STT, duração inválida, custo excedido e baixa confiança quando disponível devem falhar fechado.
- Ambiguidade financeira com transcrição tecnicamente confiável deve seguir fluxo textual existente.
- Normalização permitida é apenas técnica: trim, espaços e caracteres de controle.
</requirements>

## Subtarefas

- [x] 4.1 Criar tipos fechados de outcome e reason de áudio no pacote de aplicação adequado.
- [x] 4.2 Implementar command/input com `Validate()` e zero value inválido.
- [x] 4.3 Implementar `DecideAudioTranscription` puro, sem `context.Context`, IO, LLM ou repository.
- [x] 4.4 Definir resposta segura de reenvio para `TranscriptionUncertain` e rejeições técnicas.
- [x] 4.5 Criar testes de tabela para todos os outcomes e razões.
- [x] 4.6 Garantir que decisão aprovada não corrige valor, categoria, data, meio de pagamento ou semântica financeira.

## Detalhes de Implementação

Referenciar `techspec.md` nas seções `Estados Fechados`, `Classificacao Tecnica de Transcricao` e `Conformidade com Skills e Regras`.

Evidências de codebase a respeitar:
- `internal/agents/application/usecases/handle_inbound.go:22`
- `internal/agents/application/dtos/input/inbound_input.go:27`
- `internal/platform/agent/runtime.go:116`
- `internal/platform/agent/runtime.go:122`

## Critérios de Sucesso

- Estado ilegal não cabe em contrato público.
- `TranscriptionUncertain` garante 0 chamada a `HandleInbound` quando integrado.
- Decisor é testável sem banco, HTTP, OpenRouter ou WhatsApp.
- Ambiguidade financeira não vira incerteza técnica quando a transcrição é tecnicamente confiável.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — O decisor protege a entrada do `AgentRuntime` e dos workflows existentes.
- `domain-modeling-production` — A tarefa modela outcomes, reasons, invariantes e transições fechadas.
- `design-patterns-mandatory` — A tarefa deve rejeitar pattern formal se função/metodo puro resolver com menor custo.

## Testes da Tarefa

- [x] `go test -race -count=1 ./internal/agents/application/...`
- [x] Teste de todos os reasons de incerteza e rejeição.
- [x] Teste de normalização técnica sem enriquecimento semântico.
- [x] Teste de zero value inválido e `Validate()`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/dtos/input/`
- `internal/agents/application/usecases/`
- `internal/agents/application/*_test.go`
