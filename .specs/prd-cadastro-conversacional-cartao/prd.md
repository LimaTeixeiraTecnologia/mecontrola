# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->

Origem: `docs/us/2026-07-08-us-cadastro-conversacional-cartao.md` (US-01).

## Visão Geral

O agente conversacional diário do MeControla no WhatsApp não sabe cadastrar cartão de crédito: essa
capacidade só existe no fluxo de onboarding (`internal/agents/application/workflows/onboarding_workflow.go:681`)
e não há guardrail contra o pedido direto na conversa. No incidente de produção de 2026-07-08, o
usuário `f56e1142-...` pediu "Quero cadastrar um cartão, XP, banco XP e o vencimento dia 1"; o LLM
improvisou uma confirmação, o usuário respondeu "Sim" e recebeu "Não consegui cadastrar o cartão.
Tente novamente em breve." — sem nenhuma escrita, sem tool call, sem erro persistido. A triangulação
de quatro sinks de observabilidade (Postgres, Tempo, Prometheus, logs) confirma que o usecase de
cadastro nunca foi invocado: foi uma alucinação de sucesso/falha do LLM.

Esta funcionalidade habilita o cadastro de cartão pela própria conversa, com confirmação humana
explícita antes da escrita, mensagens acionáveis em cada falha, cálculo correto do dia de fechamento
inclusive para bancos fora da lista suportada, e um guardrail arquitetural que impede o agente de
afirmar sucesso ou falha de cadastro sem executar a ferramenta de escrita. É valiosa porque fecha um
gap de capacidade real (o usuário não consegue cadastrar cartão sem sair da conversa) e elimina uma
classe de falha silenciosa que corrói a confiança no produto.

## Objetivos

- Permitir que o usuário ativo no WhatsApp cadastre um cartão inteiramente pela conversa, sem app.
- Eliminar a alucinação de sucesso/falha de cadastro: toda afirmação corresponde a uma execução real.
- Garantir cálculo correto do dia de fechamento também para bancos não reconhecidos, sem fallback
  silencioso de 7 dias.
- Tornar toda tentativa de cadastro um run auditável, com erro real persistido e métrica de escrita.
- **Critério de sucesso mensurável**: harness real-LLM dos cenários conversacionais com gate
  estatístico ≥ 0.90 (padrão do repositório), **mais** um teste determinístico de regressão do
  incidente (o agente nunca responde sucesso/falha sem tool call; falha sempre persiste erro real no
  run e no log estruturado, nunca erro vazio).

## Histórias de Usuário

- Como usuário ativo do MeControla no WhatsApp, quero cadastrar um novo cartão de crédito conversando
  com o agente, para registrar minhas compras no cartão sem sair da conversa nem precisar do app.
- Como usuário, quero que o agente me pergunte apenas o dado que ainda falta (apelido, banco ou dia
  de vencimento), para não repetir informações que já dei.
- Como usuário de um banco fora da lista suportada, quero informar o dia de fechamento da minha
  fatura, para que o cartão seja criado com o fechamento correto em vez de um valor genérico errado.
- Como usuário, quero confirmar explicitamente antes de o cartão ser criado e poder cancelar, para ter
  controle sobre a escrita.
- Como usuário, quero que o agente nunca me diga "cadastrei" ou "não consegui cadastrar" sem de fato
  ter tentado, para não ser enganado por uma resposta improvisada.

## Funcionalidades Core

- **Ferramenta de cadastro `create_card`**: nova ferramenta conversacional (adapter fino no padrão
  `tool.NewTool[I,O]`) que valida o input contra o schema, mapeia para o comando e delega ao binding
  `CardManager.CreateCard`, que já chama o usecase `CreateCard`. Sem regra de negócio, SQL ou
  branching de domínio na ferramenta.
- **Confirmação humana explícita (gate HITL card-scoped)**: estado de espera fechado, próprio do
  cadastro de cartão (não reuso literal do `pending_entry_workflow`, que é acoplado a transações),
  persistido no `Snapshot` do kernel antes de a pergunta de confirmação ser enviada; resume por
  merge-patch antes do parse.
- **Slot-filling do dado pendente**: o agente pergunta somente o dado que falta entre apelido, banco
  e dia de vencimento.
- **Cálculo do dia de fechamento resiliente a banco não reconhecido**: derivação autoritativa para
  banco reconhecido; pergunta explícita do dia de fechamento para banco não reconhecido, sem fallback
  silencioso de 7 dias.
- **Guardrail anti-alucinação**: arquitetura que garante que qualquer afirmação de cadastro
  corresponde a uma execução real da ferramenta.
- **Idempotência, auditoria e métrica de escrita**: replay não duplica cartão; toda execução é run
  auditável com erro real persistido e métrica de escrita com cardinalidade controlada.

## Requisitos Funcionais

- RF-01: O sistema deve oferecer uma ferramenta conversacional `create_card` que permite cadastrar um
  cartão de crédito pela conversa do agente diário, delegando ao caminho de escrita existente
  (`CardManager.CreateCard` → usecase `CreateCard`). A ferramenta é um adapter fino: valida input
  contra o schema, mapeia para o comando e delega; é proibida regra de negócio, SQL direto ou
  branching de domínio na ferramenta.
- RF-02: Antes de qualquer escrita, o sistema deve exigir confirmação humana explícita, usando o
  mecanismo de suspend/resume do kernel (`SuspendAwaitingInput` + resume por merge-patch) e um estado
  de espera **card-scoped** modelado como tipo fechado (state-as-type), persistido no `Snapshot` do
  kernel **antes** de a pergunta de confirmação ser enviada. É proibido reusar literalmente o
  `pending_entry_workflow` (acoplado a ledger/categoria/`IdempotentWriter` de transações).
- RF-03: A semântica da confirmação deve ser: afirmação explícita ("sim", "confirmar", "ok", "pode")
  executa a criação; negação explícita ("não", "cancelar") descarta sem efeito; resposta ambígua
  re-pergunta **uma única vez** e, na segunda ambiguidade, cancela sem efeito.
- RF-04: O estado de espera de confirmação deve ter **TTL de 15 minutos**, avaliado no resume;
  expiração cancela o cadastro sem efeito e devolve o texto do usuário ao fluxo normal do agente.
- RF-05: O sistema deve realizar slot-filling de apelido do cartão, banco e dia de vencimento (todos
  obrigatórios), perguntando apenas o dado que ainda falta, sem repetir dados já informados.
- RF-06: Durante o slot-filling (antes da confirmação), resposta inválida a um slot (ex.: dia de
  vencimento 32 ou não-parseável) deve gerar uma mensagem acionável específica e manter o agente
  perguntando o mesmo slot até obter valor válido; nenhum estado de espera durável é persistido antes
  da confirmação. A semântica de re-pergunta-única/cancelamento aplica-se apenas à fase de
  confirmação (RF-03), não à fase de slot-filling.
- RF-07: Para banco reconhecido na tabela `mecontrola.banks`, o dia de fechamento (`closing_day`)
  deve ser derivado de forma **autoritativa** (`PurchaseDayService.Decide` a partir do `daysBeforeDue`
  do banco). Um dia de fechamento eventualmente informado pelo usuário para banco reconhecido é
  ignorado — a derivação é a fonte única de verdade.
- RF-08: Para banco **não reconhecido**, o sistema deve perguntar o dia de fechamento da fatura ao
  usuário e usar o valor informado. É **proibido** o fallback silencioso de 7 dias hoje aplicado a
  bancos desconhecidos.
- RF-09: O leitor de dias do banco deve expor, de forma **aditiva**, o sinal de "banco não
  reconhecido" para que o cadastro conversacional decida quando perguntar o dia de fechamento. O fluxo
  de onboarding permanece **inalterado** nesta entrega (mesmo comportamento atual); qualquer ajuste do
  onboarding para perguntar fechamento é follow-up fora deste PRD.
- RF-10: As validações de domínio devem ser reaproveitadas dos smart constructors existentes, sem
  duplicação: dia de vencimento entre 1 e 31, apelido entre 1 e 32 caracteres, banco não vazio, dia de
  fechamento entre 1 e 31. Cada falha de validação deve virar uma mensagem acionável específica ao
  usuário, sem criar cartão.
- RF-11: Não deve haver restrição cruzada entre dia de fechamento e dia de vencimento; ambos são
  validados de forma independente no intervalo 1..31 pelos smart constructors existentes. Nenhuma
  invariante de negócio nova é introduzida.
- RF-12: Se já existir um cartão ativo com o mesmo apelido para o usuário, o sistema deve informar que
  o apelido já está em uso e não criar duplicata, acionando o conflito de unicidade garantido pelo
  índice único parcial existente. Esse conflito é surfaçado no momento da execução da confirmação.
- RF-13: O agente nunca deve afirmar "cadastrei" nem "não consegui cadastrar" sem ter invocado a
  ferramenta `create_card`. O guardrail é arquitetural: ferramenta registrada + instruções que proíbem
  afirmar cadastro sem tool call + gate de confirmação (nenhuma escrita sem confirmação afirmativa) +
  run auditável garantindo que toda afirmação corresponde a execução real. Não há roteamento por
  `switch case intent.Kind` (R-AGENT-WF-001.1); o harness real-LLM ≥ 0.90 valida o guardrail
  estatisticamente.
- RF-14: A ferramenta deve injetar o contexto de idempotência (escopo `create_card`, chave derivada
  do `wamid` da mensagem de confirmação) para ativar o bloco de idempotência já existente no usecase
  `CreateCard`; reenvio da confirmação ou repetição do "Sim" não cria um segundo cartão e retorna a
  mesma resposta de sucesso.
- RF-15: Toda execução de cadastro deve ser um run auditável com erro real persistido: em qualquer
  falha, a coluna de erro do run é preenchida e o log estruturado registra o erro — nunca falha
  silenciosa com erro vazio.
- RF-16: A ferramenta deve emitir métrica de escrita seguindo o padrão de `agents_write_total`, com
  labels `operation` (`create_card`) e `outcome`, sem `user_id` nem outros labels de alta
  cardinalidade (R-AGENT-WF-001.5, R-TXN-004).
- RF-17: O cartão deve ser sempre criado para o `user_id` do principal autenticado da conversa; o
  sistema nunca aceita `user_id` vindo do conteúdo da mensagem (isolamento entre usuários, IDOR-safe).
- RF-18: Deve haver **exclusão mútua** de estados de espera por thread: no máximo um estado de espera
  ativo por vez. A cadeia de resume tenta os estados pendentes em ordem determinística
  (pending_entry de transação → confirmação de cartão → parse do inbound) antes de qualquer parse. Um
  novo cadastro de cartão só abre confirmação se não houver outra pendência ativa; havendo, a resposta
  do usuário é tratada como resume da pendência vigente.
- RF-19: A ferramenta `create_card` deve ser registrada no conjunto de ferramentas do agente diário,
  com ajuste das instruções do agente para descrever a capacidade e proibir respostas de cadastro sem
  tool call.
- RF-20: O contrato de entrada do cadastro conversacional deve transportar um dia de fechamento
  explícito quando o banco não é reconhecido (ampliação do tipo de cartão do agente e do input de
  criação), preservando os smart constructors como ponto único de validação.
- RF-21: Após efetivar, cancelar ou expirar, o run deve ser concluído (`succeeded`/`failed`),
  **nunca** permanecendo suspenso; o estado de espera é limpo de forma determinística, sem draft
  órfão. O housekeeping do kernel purga runs concluídos.
- RF-22: A entrega deve incluir (a) harness real-LLM dos cenários conversacionais com gate estatístico
  ≥ 0.90 e (b) teste determinístico de regressão do incidente cobrindo os critérios de aceite Gherkin
  da US e a invariante "nenhuma afirmação de cadastro sem tool call; falha sempre com erro persistido".

## Experiência do Usuário

- **Fluxo feliz (banco reconhecido)**: usuário pede "cadastrar cartão Nu, banco Nubank, vencimento dia
  10" → agente persiste o estado de espera e pergunta a confirmação (apelido, banco, dia de
  vencimento) → usuário responde "Sim" → cartão criado com o dia de fechamento derivado do banco →
  agente confirma sucesso.
- **Banco não reconhecido**: usuário pede "cadastrar cartão XP, banco XP, vencimento dia 1" → agente
  pergunta o dia de fechamento da fatura (sem fallback de 7 dias) → usuário informa → confirmação →
  "Sim" → cartão criado com o `closing_day` informado.
- **Slot-filling**: "cadastrar cartão Nu do Nubank" (sem dia de vencimento) → agente pergunta somente
  o dia de vencimento.
- **Confirmação negada**: "não" → nenhum cartão criado, run concluído, agente informa cancelamento.
- **Ambiguidade na confirmação**: resposta ambígua 1ª vez → re-pergunta uma vez; 2ª vez → cancela sem
  efeito, run concluído.
- **TTL expirado**: nova mensagem após expiração → cadastro cancelado sem efeito, texto segue para o
  fluxo normal.
- **Apelido duplicado**: confirmação de cartão com apelido de um cartão ativo já existente → agente
  informa que o apelido já está em uso, sem duplicata.
- **Dia inválido**: dia de vencimento 32 → mensagem acionável ("o dia deve estar entre 1 e 31"), sem
  criar cartão.
- **Regressão do incidente**: qualquer resposta de sucesso/falha de cadastro só ocorre após a
  invocação da ferramenta; em falha, o erro real fica persistido no run e no log.

## Restrições Técnicas de Alto Nível

- Canal único WhatsApp Meta; interação texto-only pelo agente diário.
- Conformidade obrigatória com as regras hard do repositório: R-ADAPTER-001 (adapter fino, zero
  comentários), R-AGENT-WF-001 (roteamento por registry sem `switch case intent.Kind`, Tool fina,
  estados de fronteira como tipos fechados, LLM só nas call-sites sancionadas, run auditável,
  pending step antes de pedir confirmação), R-WF-KERNEL-001 (kernel genérico, resume por merge-patch),
  R-TXN-004/R-AGENT-WF-001.5 (cardinalidade controlada de métricas).
- Reuso obrigatório do caminho de escrita existente: usecase `CreateCard`, binding
  `cardManagerAdapter.CreateCard`, tabela `mecontrola.cards` e seu índice único parcial de apelido
  ativo — sem reescrever domínio.
- Estado de espera de confirmação modelado como tipo fechado, persistido no `Snapshot` do kernel;
  resume aplica merge-patch antes do parse. O `Snapshot` é a fonte única de verdade, sem side-store.
- Idempotência via contexto injetado (`idempotency.FromContext`) para ativar o bloco existente hoje
  dormente no caminho conversacional.
- Provider LLM único (OpenRouter); nenhuma chamada LLM no kernel nem dentro da Tool.
- Isolamento por usuário (IDOR-safe): `user_id` sempre do principal autenticado.

## Fora de Escopo

- Edição e exclusão de cartão pela conversa (já existem `update_card` e soft delete).
- Gestão de fatura e parcelas do cartão.
- Cadastro de cartão pelo aplicativo (fora do canal WhatsApp).
- Expandir a lista de bancos suportados na tabela `mecontrola.banks`; a decisão é perguntar o dia de
  fechamento para bancos não reconhecidos, não cadastrar novos bancos.
- Ajuste do fluxo de onboarding para perguntar o dia de fechamento de bancos não reconhecidos; nesta
  entrega o onboarding permanece com o comportamento atual (a correção do reader é aditiva). Fica como
  follow-up.
- Sobrescrita, pelo usuário, do dia de fechamento derivado para bancos reconhecidos.
- Backstop determinístico pós-resposta para detectar afirmação de cadastro sem tool call: o guardrail
  é arquitetural (instruções + tool + gate + run auditável + harness), sem router por intent.

## Suposições e Questões em Aberto

- Suposição: o matching de "banco reconhecido" é feito contra a tabela `mecontrola.banks` pelo
  nome/código informado; normalização exata (case, acentos) é detalhe de especificação técnica.
- Suposição: o conflito de apelido duplicado é surfaçado no momento da execução da confirmação (não há
  pré-checagem durante o slot-filling), conforme os critérios de aceite da US.
- Suposição: a mensagem de confirmação exibe os dados coletados relevantes (apelido, banco, dia de
  vencimento e, para banco não reconhecido, o dia de fechamento informado); o conteúdo/tom exato é
  detalhe de implementação.
- Questão em aberto: nenhuma. Todas as decisões materiais de escopo, fronteira e comportamento foram
  confirmadas com o usuário (escopo do onboarding aditivo, critério de sucesso, autoridade da
  derivação, tratamento de slot inválido, TTL de 15 min, exclusão mútua de estados de espera, natureza
  do guardrail e ausência de restrição cruzada fechamento×vencimento).
