# Documento de Requisitos do Produto (PRD) — Nome de Tratamento do Usuário

<!-- spec-version: 1 -->

## Visão Geral

O MeControla conversa com o usuário pelo WhatsApp, mas hoje não sabe como o usuário gostaria de ser chamado: o primeiro contato do onboarding já emenda boas-vindas e objetivo, e não há campo de nome de tratamento em lugar algum da plataforma conversacional. O único "nome" existente é o `display_name` cadastral do módulo `identity`, que é dado civil/de cobrança e não deve ser usado como tratamento afetivo.

Esta funcionalidade permite que o agente pergunte, no início do onboarding, como o usuário gostaria de ser chamado, extraia o nome/apelido a partir de linguagem natural, armazene esse valor de forma estruturada e passe a usá-lo nas interações para tornar a experiência mais próxima, humana e personalizada. O usuário pode alterar o nome de tratamento a qualquer momento por linguagem natural, com efeito imediato. Toda mensagem segue o Tom de Voz oficial do MeControla.

É valioso porque personalização por nome aumenta proximidade e engajamento sem tocar dados cadastrais/cobrança, reutilizando os primitivos de memória de plataforma já existentes.

Fonte de produto: `docs/us/2026-07-15-us-nome-de-tratamento-do-usuario.md` (US única, validada) derivada de `US_Nome_de_Tratamento_do_Usuario_MeControla.md`, confrontada com a base de código.

## Objetivos

- Personalizar a conversa pelo nome escolhido pelo usuário, no onboarding e no dia a dia, sem alterar dados cadastrais ou de cobrança.
- Sucesso é: novos usuários informam o nome no onboarding e o agente passa a tratá-los por esse nome de forma natural e fiel ao Tom de Voz.
- Métricas-chave (KPIs):
  - **Taxa de captura no onboarding**: % de novos usuários que informam um nome de tratamento utilizável no passo inicial.
  - **Aderência de uso do nome**: aderência das respostas ao Tom de Voz e ao uso natural do nome, medida pelos scorers existentes (`tone_adherence` LLM-judged e scorer comportamental determinístico), sem repetição excessiva.
  - **Taxa de edição bem-sucedida**: % de intenções de alteração de nome concluídas com atualização efetiva e confirmação.
- Meta de qualidade: os fluxos de captura, uso e edição passam no gate de aceite real-LLM (golden) com score ≥ 0,90 e **0 falso-sucesso**, alinhado ao padrão consolidado do projeto para fluxos conversacionais.

## Histórias de Usuário

- Como novo usuário do MeControla no WhatsApp, quero informar como gostaria de ser chamado logo no início do onboarding, para que o agente fale comigo de forma próxima e pessoal.
- Como usuário ativo do MeControla, quero que o agente use o nome que escolhi ao registrar lançamentos e mostrar resumos, para sentir a conversa mais humana.
- Como usuário ativo do MeControla, quero trocar por linguagem natural, a qualquer momento, o nome pelo qual sou chamado, para ajustar a personalização sem mexer nos meus dados cadastrais.
- Como usuário que não quer informar um nome, quero seguir o onboarding normalmente sem ser bloqueado, para não ter fricção.

Persona primária: usuário final do MeControla no canal WhatsApp (em onboarding e pós-onboarding). Não há persona administrativa envolvida.

## Funcionalidades Core

1. **Captura do nome no onboarding** — o agente pergunta, integrado à mensagem de boas-vindas, "Antes da gente começar, como você gostaria que eu te chamasse? 💚". Importante porque é o momento natural de personalizar a relação. Em alto nível: um passo inicial dedicado, anterior ao passo de objetivo, coleta e extrai o nome.
2. **Extração por linguagem natural** — o agente entende diferentes formas de informar o nome ("Stefany", "Pode me chamar de Stef", "Prefiro Stef", "Só Stef mesmo"). Importante para não exigir formato fixo. Em alto nível: interpretação por linguagem natural com saída estruturada.
3. **Persistência estruturada e utilizável** — o nome fica gravado de forma estruturada e passa a alimentar as respostas do agente. Importante porque personalização exige um valor confiável e vigente. Em alto nível: um único valor vigente por usuário, gravado de forma estruturada e disponibilizado ao agente para uso nas conversas.
4. **Uso nas interações** — quando há nome vigente, o agente o usa de forma natural e fiel ao Tom de Voz, sem repetição excessiva. Importante para gerar proximidade sem soar artificial.
5. **Alteração por linguagem natural** — o usuário pode pedir para trocar o nome a qualquer momento, com ou sem informar o novo nome na mesma mensagem, com efeito imediato e confirmação. Importante para manter a personalização sempre atual.

## Requisitos Funcionais

- RF-01: No início do onboarding, antes da coleta de objetivo, o agente apresenta a mensagem de boas-vindas e pergunta ao usuário como gostaria de ser chamado, usando o texto oficial "Antes da gente começar, como você gostaria que eu te chamasse? 💚", sem duplicar a saudação de boas-vindas nas etapas seguintes.
- RF-02: O agente identifica e extrai a forma de tratamento pretendida pelo usuário a partir de linguagem natural, cobrindo variações como nome direto, "Pode me chamar de X", "Me chama de X", "Prefiro X", "Meu apelido é X", "Só X mesmo", "X tá bom". Quando o usuário indica explicitamente como quer ser chamado (ex.: "me chama de Stef"), prevalece esse apelido; quando informa apenas o nome completo/composto sem indicar tratamento, prevalece o primeiro nome como nome de tratamento.
- RF-03: O nome de tratamento é persistido de forma estruturada em `platform_resources` (chave dedicada em `metadata`) e também disponibilizado de forma que o agente consiga utilizá-lo nas conversas; existe no máximo um nome de tratamento vigente por usuário.
- RF-04: Se o usuário não informar um nome utilizável (ex.: "não", "tanto faz" ou responde diretamente sobre o objetivo), o onboarding prossegue sem nome de tratamento e nunca é bloqueado; o agente passa a tratar o usuário de forma neutra, sem nome.
- RF-05: Quando houver nome de tratamento vigente, o agente o utiliza nas interações de forma natural e coerente com o contexto, sem inserir o nome de forma excessiva ou artificial na mesma interação.
- RF-06: O agente reconhece a intenção de alterar o nome de tratamento por linguagem natural, mesmo sem comando específico (ex.: "Quero trocar como você me chama", "Muda como você me chama", "Quero mudar meu apelido", "A partir de agora quero que me chame de outro nome").
- RF-07: Quando a mensagem de alteração já contém o novo nome (ex.: "Agora me chama de Stef", "Troca meu nome para Stef"), o agente não pergunta novamente, aplica a alteração e confirma com o novo nome (ex.: "Combinado, Stef! 💚 Vou te chamar assim daqui pra frente.").
- RF-08: Quando a intenção de alteração não traz o novo nome, o agente pergunta uma vez "Claro! Como você gostaria que eu te chamasse a partir de agora? 💚", aplica ao receber a resposta e confirma; não há confirmação adicional do tipo sim/não.
- RF-09: Após qualquer atualização, o novo nome de tratamento é considerado imediatamente e todas as interações seguintes usam o valor vigente.
- RF-10: A alteração do nome de tratamento não modifica o `display_name` cadastral do módulo `identity` nem quaisquer dados cadastrais ou de cobrança do usuário.
- RF-11: O valor informado é normalizado (remoção de espaços nas bordas) e limitado a 40 caracteres, aceitando letras, números, espaços e emojis; valor vazio após normalização, valor composto apenas por símbolos/pontuação ou resposta de recusa não é persistido e equivale a "sem nome utilizável" (RF-04); valor acima de 40 caracteres leva o agente a reperguntar pedindo uma forma mais curta; apenas um valor vigente é mantido, substituindo o anterior.
- RF-12: Todas as mensagens dos fluxos de captura, uso e alteração seguem obrigatoriamente o Tom de Voz oficial do MeControla, cuja fonte de verdade verificável são os scorers já codificados no projeto: `tone_adherence` (LLM-judged) e o scorer comportamental determinístico (negrito com asterisco simples, emojis oficiais); nenhuma nova definição de tom é criada.
- RF-13: Se a gravação do nome de tratamento falhar, o passo falha de forma explícita, sem confirmar sucesso ao usuário, e o nome de tratamento anterior permanece inalterado.
- RF-14: Os cenários de captura, uso e alteração são validados por gate de aceite real-LLM (golden) com score ≥ 0,90 e 0 falso-sucesso, além dos testes determinísticos aplicáveis.
- RF-15: A funcionalidade é liberada para todos os usuários, sem feature flag, mantendo o comportamento seguro por padrão (nome opcional) para quem não informa nome.
- RF-16: São instrumentadas as métricas de produto: taxa de captura do nome no onboarding, aderência de uso do nome (via scorers) e taxa de edição bem-sucedida, com cardinalidade controlada de labels (sem `user_id`).

## Experiência do Usuário

- Persona: usuário final do MeControla no WhatsApp, em onboarding (novo) e pós-onboarding (ativo).
- Fluxo de captura: primeira mensagem = boas-vindas + pergunta do nome → usuário responde em linguagem natural → agente confirma implicitamente ao seguir o fluxo (ou trata sem nome se não informado) → segue para objetivo.
- Fluxo de uso: em respostas do dia a dia (ex.: registro de lançamento, resumo de orçamento), o agente insere o nome de forma pontual e natural ("Prontinho, Stefany! Seu lançamento foi registrado. ✅").
- Fluxo de alteração com nome informado: usuário envia "Agora me chama de Stef" → agente atualiza e responde "Combinado, Stef! 💚 Vou te chamar assim daqui pra frente.".
- Fluxo de alteração sem nome informado: usuário envia "Quero trocar como você me chama" → agente pergunta "Claro! Como você gostaria que eu te chamasse a partir de agora? 💚" → usuário responde → agente atualiza e confirma.
- Acessibilidade/consistência: toda copy segue o Tom de Voz oficial (negrito com asterisco simples, emojis oficiais, tom motivacional e acolhedor).

## Restrições Técnicas de Alto Nível

- O nome de tratamento vive na camada de memória de plataforma (`platform_resources`), separado do agregado `User` do módulo `identity`; nenhum dado cadastral/de cobrança é afetado (privacidade e separação de fronteiras de dado).
- A utilização do nome pelo agente depende do canal de contexto que efetivamente alcança o modelo (memória de trabalho injetada no system prompt); a persistência estruturada em `metadata` por si só não é suficiente para uso conversacional — o valor precisa estar disponível ao agente.
- Provider de LLM único (OpenRouter) para extração e conversa; não há fallback chain nem circuit breaker.
- Governança obrigatória do repositório: roteamento por registry (sem `switch case intent.Kind`), estados de fronteira como tipos fechados (DMMF state-as-type), zero comentários em Go de produção, adaptadores finos; o fluxo de alteração é personalização não-destrutiva e não entra no gate HITL de operações sensíveis.
- Reuso obrigatório de primitivos e padrões existentes: interface de memória de trabalho (`Get`/`Upsert`/`UpsertMetadata`), padrão de passo durável do onboarding e padrão de edição pós-onboarding já consolidado, evitando novas abstrações concorrentes.
- Instrumentação com cardinalidade controlada (labels permitidos: enums fechados; proibido `user_id` como label).

## Fora de Escopo

- Alterar `display_name` cadastral ou quaisquer dados de cadastro/cobrança do módulo `identity`.
- Alterar o mecanismo genérico de plataforma para injetar `metadata` JSONB diretamente no system prompt do runtime; a utilização pelo agente se dá pelo canal de memória de trabalho já existente.
- Histórico ou versionamento de nomes de tratamento anteriores.
- Nome de tratamento distinto por canal ou por conversa/thread (o valor é único por usuário).
- Inferência de gênero, flexão gramatical ou tradução do nome informado.
- Fluxo administrativo ou de suporte para editar o nome de tratamento de terceiros.

## Decisões Confirmadas

Todas as questões de produto foram resolvidas; não há suposições nem questões em aberto.

- Escopo: captura + uso + edição em entrega única.
- KPIs: taxa de captura no onboarding + aderência de uso do nome (via scorers) + taxa de edição bem-sucedida.
- Gate de aceite: real-LLM (golden) com score ≥ 0,90 e 0 falso-sucesso.
- Rollout: todos os usuários, sem feature flag.
- Validação do nome (RF-11): até 40 caracteres, aceita letras/números/espaços/emojis, normaliza bordas, rejeita vazio/só-símbolo/recusa, repergunta se exceder.
- Nome composto (RF-02): prevalece o apelido pretendido quando indicado; senão, o primeiro nome como tratamento.
- Fonte do Tom de Voz (RF-12): os scorers já codificados no projeto (`tone_adherence` + scorer comportamental) são a fonte de verdade; nenhuma nova definição de tom é criada.

Detalhe deferido à Especificação Técnica por ser decisão de implementação, não de produto (não altera comportamento observável): o identificador exato da chave estruturada de `metadata` e o cabeçalho exato da seção de memória de trabalho usada para disponibilizar o nome ao agente.
