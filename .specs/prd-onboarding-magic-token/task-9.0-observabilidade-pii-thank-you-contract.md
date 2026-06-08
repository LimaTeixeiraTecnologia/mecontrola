# Tarefa 9.0: Observabilidade, seguranca PII e contrato da thank-you page externa

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar os requisitos transversais de metricas, logs PII-safe, seguranca operacional e contrato documentado para a thank-you page Astro externa, incluindo criterios de acessibilidade WCAG 2.1 AA.

<requirements>
- Cobrir `RF-04`, `RF-05`, `RF-13`, `RF-14`, `RF-17`, `RF-19`.
- Emitir o conjunto minimo de metricas definido no PRD e techspec.
- Logs estruturados devem mascarar telefone e email e nunca logar token em claro.
- Documentar contrato JSON da thank-you page e requisitos de renderizacao generica sem oracle.
- Registrar criterios de validacao WCAG 2.1 AA para o PR coordenado da landing.
- A execucao posterior deve carregar obrigatoriamente `go-implementation` para qualquer edicao Go, carregar exemplos apenas sob demanda, verificar `go.mod` antes de usar recursos da linguagem, partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nao usar `internal/platform/runtime` como ponto de partida e nao adicionar comentarios em arquivos Go.
</requirements>

## Subtarefas

- [ ] 9.1 Consolidar metricas Prometheus listadas na techspec nos use cases, handlers e jobs.
- [ ] 9.2 Revisar logs `slog` para garantir mascaramento de PII e token redacted.
- [ ] 9.3 Documentar contrato da landing em artefato local ou secao operacional existente.
- [ ] 9.4 Especificar criterios de aceite da pagina `/obrigado/[token]`: CTA, fallback, auto-redirect mobile e mensagem generica.
- [ ] 9.5 Especificar validacao WCAG 2.1 AA com contraste, foco, teclado, semantica e axe-core no repo da landing.
- [ ] 9.6 Validar que nenhuma metrica/log contradiz a defesa contra enumeracao de `RF-17`.

## Detalhes de Implementação

Referenciar `techspec.md` secoes 6.6, 8.1, 8.5, 8.6, 9.1, 9.2, 9.5, 11.2 e ADR-007. Nao implementar a pagina Astro neste repositorio se o repositorio da landing nao estiver presente no workspace.

## Critérios de Sucesso

- Todas as metricas de `RF-13` existem nos pontos de origem apropriados.
- Logs de fluxo nao expoem email, telefone, token claro ou texto inbound contendo token.
- Contrato externo da thank-you page esta claro para PR coordenado na landing.
- Mensagem publica de token invalido permanece unica e generica.
- Criterios WCAG 2.1 AA estao explicitamente testaveis para a equipe da landing.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes smoke de metricas quando houver registry local.
- [ ] Testes unitarios para redaction/mascara quando novo helper for criado.
- [ ] Validacao manual do contrato externo contra `techspec.md`.
- [ ] `go test -race -count=1 ./internal/onboarding/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/`
- `internal/identity/domain/pii/mask.go`
- `.specs/prd-onboarding-magic-token/techspec.md`
- `docs/`
