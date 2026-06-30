# Tarefa 3.0: Config e mensagens da jornada de ativação

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar as novas chaves de configuração e mensagens necessárias à jornada e descontinuar o uso do texto legado de comando `ATIVAR`.

<requirements>
- RF-10: `ONBOARDING_ACTIVATION_WINDOW_HOURS` (default 24).
- RF-24: `WA_MSG_ACTIVATION_NOT_FOUND` (texto de no-match) + janela/housekeeping do throttle.
- RF-29: remover o uso de `WA_MSG_PLEASE_USE_ATIVAR_COMMAND` (depreciar).
- `ONBOARDING_ACTIVATION_PAGE_URL` (default `https://mecontrola.app.br`) para o e-mail.
- Defaults registrados no loader; sem segredo hardcoded.
</requirements>

## Subtarefas

- [ ] 3.1 Adicionar campos em `configs/config.go` (`OnboardingConfig`: `ActivationWindowHours`, `ActivationPageURL`; `WhatsAppConfig`: `ActivationNotFound`; throttle: janela + schedule de housekeeping) com tags `mapstructure` e defaults.
- [ ] 3.2 Registrar defaults no loader (`setOnboardingDefaults`/`setWhatsAppDefaults` equivalentes) e nas `envKeys`.
- [ ] 3.3 Atualizar `.env.example` com as novas chaves e seus comentários de uso.
- [ ] 3.4 Marcar `WA_MSG_PLEASE_USE_ATIVAR_COMMAND` como não utilizado (remover consumo em 7.0; aqui apenas parar de exigi-lo).

## Detalhes de Implementação

Ver techspec.md, seção "Build Order" (passo 12) e "Decisões/Defaults". Mensagens de boas-vindas (`WA_MSG_WELCOME_ACTIVATED`, `WA_MSG_ONBOARDING_INTRO`) já existem — alinhar os textos default às strings exatas do PRD (Experiência do Usuário).

## Critérios de Sucesso

- Novas chaves carregam com defaults corretos e podem ser sobrescritas por env.
- `.env.example` documenta todas as novas variáveis.
- Janela de 24h e base-URL da página disponíveis para as tarefas 4.0/5.0/8.0.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários de carga de config (defaults + override por env) para as novas chaves.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `configs/config.go`
- `.env.example`
