# Tarefa 1.0: Config: chave WA_MSG_AUDIO_DURATION_EXCEEDED, default 20s e validações

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Registrar a nova configuração `WA_MSG_AUDIO_DURATION_EXCEEDED` em todos os pontos obrigatórios de `configs/config.go` e mudar o default de `AGENT_AUDIO_MAX_DURATION` de 60s para 20s, mantendo a faixa de validação `[1s..60s]` intacta (ADR-001 e ADR-002 da techspec).

<requirements>
- RF-05: limite configurável por variável de ambiente, default 20s, faixa `[1s..60s]` mantida para rollback sem deploy.
- Fundação de RF-01, RF-02 e RF-03: o use case só recebe a nova configuração se ela existir e for lida do ambiente.
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar campo `AudioDurationExceededReply string` com tag `mapstructure:"WA_MSG_AUDIO_DURATION_EXCEEDED"` em `AgentConfig`, após `AudioRejectedReply` (`configs/config.go:198`), sem renomear nem reordenar campos existentes.
- [ ] 1.2 Registrar `"WA_MSG_AUDIO_DURATION_EXCEEDED"` em `envKeys()` (`configs/config.go:561-567`), após as duas chaves `WA_MSG_AUDIO_*` existentes; sem esta entrada a env var é silenciosamente ignorada.
- [ ] 1.3 Espelhar a validação de não vazio das mensagens existentes (`configs/config.go:1076-1082`) para a nova chave em `validateAgentAudio`, e em `validateProductionAudio` (`configs/config.go:1135-1140`) quando `AudioEnabled=true`.
- [ ] 1.4 Alterar `configs/config.go:1441` de `60*time.Second` para `20*time.Second` e adicionar `SetDefault("WA_MSG_AUDIO_DURATION_EXCEEDED", ...)` após `:1444` com o texto exato `esse áudio passou de 20 segundos 🎙️ me manda um mais curtinho, de até 20 segundos?` (decisão do usuário, ADR-002).
- [ ] 1.5 Não alterar a faixa de validação `[1s..60s]` em `configs/config.go:1062-1067`.
- [ ] 1.6 Testes em `configs/config_test.go`: default 20s quando env ausente; default da mensagem quando env ausente; override `AGENT_AUDIO_MAX_DURATION=30s` aceito; rejeição fora da faixa mantida (`:949-959`); rejeição de `WA_MSG_AUDIO_DURATION_EXCEEDED` vazia espelhando `:981-1001`; fixture `newBaseConfig` (`:1861-1864`) atualizada com o campo novo.

## Detalhes de Implementação

Ver `techspec.md` seções `Arquitetura do Sistema` (componentes modificados, item `configs/config.go`), `Design de Implementação` (Interfaces Chave) e ADR-001/ADR-002. A constante de fallback `defaultAudioDurationExceededReply` vive no use case (tarefa 2.0) e deve ter o mesmo texto do `SetDefault` para não divergir.

## Critérios de Sucesso

- `go build ./configs/...` e `go vet ./configs/...` sem erros.
- `go test -race -count=1 ./configs/...` verde, incluindo os testes novos de default, override e obrigatoriedade.
- Boot com env ausente resulta em `AudioMaxDuration == 20s` e mensagem default preenchida.
- Boot em produção com `AGENT_AUDIO_ENABLED=true` falha com erro claro se a nova chave estiver vazia.
- Nenhum diff fora de `configs/config.go` e `configs/config_test.go`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `design-patterns-mandatory` — gate de desenho obrigatório para mudança Go; expectativa de `não aplicar padrão` já registrada na ADR-003, reexecutar apenas se surgir sinal novo.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `configs/config.go`
- `configs/config_test.go`
- `.specs/prd-limite-audio-20-segundos-whatsapp/techspec.md`
- `.specs/prd-limite-audio-20-segundos-whatsapp/adr-001-limite-20s-via-config-com-default-sem-endurecer-faixa.md`
- `.specs/prd-limite-audio-20-segundos-whatsapp/adr-002-selecao-de-mensagem-por-reason-no-usecase.md`
