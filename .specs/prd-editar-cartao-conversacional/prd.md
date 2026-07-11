# Documento de Requisitos do Produto (PRD) — Editar Cartão pela Conversa (WhatsApp)

<!-- spec-version: 1 -->

> Origem: `docs/us/2026-07-10-us-editar-cartao-conversacional.md` (US única, validada).
> Data: 2026-07-10.
> Persona confirmada: usuário final via agente WhatsApp (`internal/agents`); módulo `internal/card` como suporte.
> Decisões confirmadas: confirmação universal; versão revalidada no servidor; banco não reconhecido pede fechamento; múltiplos campos por confirmação; alterar vencimento com parcelas em aberto é permitido após aviso; confirmação mostra de-para (atual → novo) por campo; textos determinísticos reusam os padrões existentes.

## Visão Geral

O MeControla já permite **cadastrar** cartões de crédito pela conversa no WhatsApp com um fluxo robusto (workflow dedicado `card-create-confirm`, confirmação humana, TTL, escrita idempotente e mensagens determinísticas). A **edição** de cartões, porém, é frágil e assimétrica: alterar apelido ou banco grava sem confirmação e sem idempotência; alterar o dia de vencimento pede confirmação, mas um defeito de payload pode não persistir o novo valor; e o agente não consegue obter a versão exigida pela ferramenta de edição sem inventá-la, o que é proibido.

Esta funcionalidade entrega a **edição de cartão pela conversa** com o mesmo nível de robustez da criação: identificação segura do cartão, confirmação humana explícita para qualquer alteração, versão gerenciada pelo servidor, recálculo correto do ciclo de fatura, escrita idempotente e ausência total de falso sucesso. É valiosa porque fecha uma lacuna de produto (o usuário hoje não consegue corrigir com segurança dados de um cartão pela conversa) e elimina defeitos que causam mutação silenciosa e possível perda da alteração.

## Objetivos

- Permitir que o usuário edite apelido, banco e dia de vencimento de um cartão existente conversando no WhatsApp, com confirmação antes de gravar.
- Alcançar paridade de robustez com a criação: confirmação humana, TTL, idempotência, no-false-success e mensagens determinísticas.
- Eliminar os defeitos confirmados: perda do novo vencimento por payload incompleto e impossibilidade de obter a versão sem inventá-la.
- Métricas de sucesso:
  - Zero casos de falso sucesso (o agente nunca afirma que atualizou sem gravação real), medido por scorer comportamental e por auditoria de Run.
  - Zero mutação silenciosa: 100% das edições passam por confirmação humana explícita.
  - Zero aplicação duplicada em reenvio da mesma mensagem (idempotência por `wamid`).
  - Gate de avaliação real-LLM de edição de cartão com razão de acerto ≥ 0,90 por categoria, conforme prática do repositório.

## Histórias de Usuário

- Como usuário do MeControla no WhatsApp, quero editar apelido, banco ou dia de vencimento de um cartão já cadastrado, confirmando antes de qualquer alteração, para que meus cartões fiquem corretos com segurança e sem mudanças silenciosas.
- Como usuário do MeControla no WhatsApp, quero que o agente só me diga que atualizou o cartão quando a alteração realmente persistir, para que eu nunca receba uma confirmação inventada.
- Como usuário do MeControla no WhatsApp, quero corrigir vários dados de um cartão de uma vez (por exemplo, apelido e vencimento na mesma frase), para que eu resolva tudo em uma única confirmação.
- Como usuário do MeControla no WhatsApp, quero que, ao trocar o banco por um que o sistema não reconhece, ele me pergunte o dia de fechamento, para que o ciclo de fatura continue correto.

## Funcionalidades Core

1. **Identificação segura do cartão a editar** — resolve o cartão por apelido, ou por lista/detalhe, sempre restrito ao próprio usuário, sem que o agente invente identificadores. Importa porque edições precisam atingir exatamente o cartão certo.
2. **Confirmação humana universal antes de gravar** — qualquer alteração (apelido, banco e/ou vencimento) passa por um gate de confirmação durável, com aceitar, cancelar, resposta ambígua, expiração e bloqueio de pendência. Importa porque remove a mutação silenciosa atual e dá controle ao usuário.
3. **Edição multi-campo com recálculo correto do ciclo** — permite alterar vários campos numa única confirmação; ao mudar banco ou vencimento, o dia de fechamento é recalculado; banco não reconhecido pede o dia de fechamento. Importa porque mantém a fatura consistente.
4. **Versão gerenciada pelo servidor (lock otimista)** — o workflow captura a versão atual ao iniciar a confirmação e a revalida ao efetivar, abortando se houver edição concorrente; o agente nunca lida com versão. Importa porque fecha o gap que hoje impede a edição e preserva a detecção de concorrência.
5. **Escrita idempotente e sem falso sucesso** — a gravação é idempotente por identificador da mensagem, com mensagens determinísticas de sucesso, replay e erro; falha não-domínio nunca é reportada como sucesso. Importa porque garante confiança no que o agente informa.

## Requisitos Funcionais

Identificação do cartão:
- RF-01: O sistema deve resolver o cartão a editar pelo apelido informado pelo usuário, retornando o identificador quando encontrado.
- RF-02: Quando o apelido não corresponder a nenhum cartão ativo do usuário, o sistema deve indicar que não encontrou e oferecer a lista de cartões para escolha.
- RF-03: O sistema deve permitir listar os cartões do usuário e detalhar um cartão específico para apoiar a escolha antes de editar.
- RF-04: O agente nunca deve inventar identificador de cartão; a identificação deve sempre vir de uma ferramenta de consulta restrita ao próprio usuário.

Gatilho e campos de edição:
- RF-05: O sistema deve permitir editar apelido, banco e/ou dia de vencimento, aceitando a alteração de múltiplos campos em uma única solicitação e confirmação.
- RF-06: A versão do cartão para o controle otimista deve ser obtida e gerenciada pelo servidor; o agente não deve fornecer nem depender de a versão vir do usuário.
- RF-07: O dia de fechamento não deve ser editável isoladamente pela conversa, permanecendo um valor derivado; não deve existir conceito de limite de crédito.

Confirmação humana universal:
- RF-08: Toda edição, independentemente do campo alterado, deve exigir confirmação humana explícita antes de qualquer gravação, eliminando a gravação direta de apelido/banco existente hoje.
- RF-09: O estado de espera da confirmação deve ser persistido de forma durável antes de a pergunta ser enviada ao usuário, sendo a fonte única de verdade para a retomada.
- RF-10: A pergunta de confirmação deve ser determinística e apresentar, para cada campo alterado, o de-para do valor atual para o novo valor (por exemplo, "Apelido: Nubank → Roxinho"); quando a alteração incluir o dia de vencimento, deve conter também a nota de impacto sobre parcelas em aberto.
- RF-11: Uma resposta de aceite explícita ("sim", "confirmar", "confirmo", "ok", "pode", "yes", "s") deve efetivar a edição.
- RF-12: Uma resposta de cancelamento explícita ("não", "nao", "cancelar", "cancelo", "no", "n") deve descartar a edição sem efeito, com mensagem determinística de cancelamento.
- RF-13: Uma resposta ambígua deve gerar uma única repergunta pedindo "sim" ou "não"; uma segunda resposta ambígua deve cancelar a edição sem efeito.
- RF-14: A confirmação deve expirar após 15 minutos de inatividade, encerrando sem efeito e devolvendo o controle ao fluxo normal do agente; o TTL deve ser alinhado ao da criação.
- RF-15: Enquanto houver uma confirmação de operação pendente, um novo pedido de operação deve ser bloqueado com mensagem determinística instruindo o usuário a responder sim ou não antes.

Banco e ciclo de fatura:
- RF-16: Quando o banco for alterado para um banco reconhecido, o dia de fechamento deve ser recalculado automaticamente a partir das regras do banco.
- RF-17: Quando o banco for alterado para um banco não reconhecido e o dia de fechamento não tiver sido informado, o sistema deve perguntar o dia de fechamento (espelhando a criação) e usar o valor informado.
- RF-18: Quando o dia de vencimento for alterado, o novo valor deve ser efetivamente persistido e o dia de fechamento recalculado de acordo com o banco.
- RF-19: Alterar o dia de vencimento de um cartão que possui parcelas em aberto deve ser permitido após a confirmação, com a nota de impacto exibida; o sistema não deve bloquear a alteração por existirem parcelas em aberto.

Controle de versão e concorrência:
- RF-20: Ao iniciar a confirmação, o sistema deve capturar a versão atual do cartão; ao efetivar, deve revalidar essa versão e, se o cartão tiver sido alterado nesse intervalo, abortar a edição com mensagem determinística orientando o usuário a tentar novamente com os dados atuais.

Idempotência, mensagens e no-false-success:
- RF-21: A gravação da edição deve ser idempotente pelo identificador da mensagem original; um reenvio da mesma confirmação não deve aplicar a alteração duas vezes e deve responder de forma determinística que o cartão já estava atualizado.
- RF-22: Uma edição bem-sucedida deve responder com mensagem determinística de sucesso.
- RF-23: Erros de regra de negócio devem ser classificados e respondidos com mensagem determinística específica, cobrindo pelo menos: apelido já em uso ao renomear, dia de vencimento inválido, cartão não encontrado e versão divergente.
- RF-24: Uma falha não relacionada a regra de negócio deve marcar a execução como falha e nunca ser reportada como sucesso; a resposta deve orientar o usuário a tentar novamente.
- RF-25: O agente deve repassar os textos determinísticos do sistema sem reescrita e nunca deve mencionar termos de infraestrutura ("workflow", "pendência", "sistema interno") ao usuário.
- RF-26: As mensagens determinísticas de sucesso, cancelamento, replay e erro devem reusar os padrões já existentes na criação e no gate de confirmação, adaptados ao contexto de edição, preservando tom e emojis; a especificação técnica fixa os textos exatos sem alterar a semântica definida neste PRD.

Escopo, propriedade e ciclo de vida:
- RF-27: Toda leitura e gravação de edição deve ser restrita aos cartões do próprio usuário, com a identidade derivada do contexto de execução.
- RF-28: Um cartão marcado como excluído (soft delete) não deve ser editável e deve ser tratado como não encontrado.

Observabilidade e operação:
- RF-29: Toda execução de edição deve ser observável como um Run auditável contendo, no mínimo, identificador de thread, identificador de run, nome do workflow, status, duração e erro quando houver.
- RF-30: As métricas de edição devem usar cardinalidade controlada, sem `user_id` nem identificador de cartão como rótulo.
- RF-31: Runs de confirmação abandonados devem ser purgados por rotina de housekeeping, sem deixar estado órfão.

Qualidade e aceite:
- RF-32: A edição deve ter cobertura de teste comportamental (avaliação real-LLM/golden) além dos testes unitários e de integração, com gate de razão de acerto ≥ 0,90 por categoria, conforme prática do repositório.

## Experiência do Usuário

- Persona primária: usuário final do MeControla no WhatsApp, que já possui pelo menos um cartão cadastrado e quer corrigir dados dele.
- Fluxo principal: o usuário pede para alterar dados de um cartão ("muda o apelido do Nubank para Roxinho"); o agente identifica o cartão, devolve uma pergunta de confirmação com os dados novos; o usuário responde "sim"; o sistema grava e confirma com mensagem determinística.
- Fluxo com vencimento: ao mudar o vencimento, a confirmação exibe a nota de impacto sobre parcelas em aberto; após o aceite, o novo vencimento é gravado e o fechamento recalculado.
- Fluxo com banco desconhecido: ao trocar para um banco fora da lista, o agente pergunta o dia de fechamento antes de confirmar.
- Variações e erros: apelido não encontrado leva à listagem; resposta ambígua gera uma repergunta e depois cancela; inatividade por 15 minutos expira sem efeito; apelido duplicado, versão divergente e falha transitória retornam mensagens determinísticas específicas.
- Tom e forma: linguagem de parceiro financeiro, sem termos técnicos; todas as mensagens finais de sucesso/erro são determinísticas do sistema e repassadas verbatim pelo agente.

## Restrições Técnicas de Alto Nível

- Canal-alvo é o agente conversacional WhatsApp (`internal/agents`); o contrato REST de edição do módulo `internal/card` já existe e não faz parte desta entrega.
- A solução deve consumir o substrato de plataforma (`internal/platform/workflow` e primitivos de agent/memory), sem recriar o kernel de workflow nem embutir regra de domínio ou LLM no kernel.
- Estados de espera e de operação devem ser modelados como tipos fechados (state-as-type), sem string livre em fronteira pública.
- A escrita financeira deve ser idempotente pela identidade da mensagem original, reusando o mecanismo de escrita idempotente já adotado na criação.
- A confirmação humana e a retomada devem ser duráveis, com o estado persistido antes de qualquer pergunta e retomada aplicada antes de qualquer interpretação da resposta.
- Observabilidade com cardinalidade controlada de métricas, herdando as regras de governança do repositório.
- Privacidade: nenhuma métrica ou log deve expor dados sensíveis do usuário como rótulo de alta cardinalidade.

## Fora de Escopo

- Cadastro (criação) de cartão, que já está implementado e entra apenas como padrão de referência.
- Exclusão de cartão, que já é atendida por outro fluxo de confirmação.
- Tornar o dia de fechamento editável isoladamente e introduzir qualquer conceito de limite de crédito.
- Edição via API REST (`internal/card`), que permanece como adapter existente e não é o canal desta funcionalidade.
- Suporte a edição em massa de múltiplos cartões numa única operação.

## Premissas Confirmadas e Questões em Aberto

Não há questões materiais em aberto. Todas as decisões de produto que afetavam requisitos foram confirmadas com o solicitante em duas rodadas de múltipla escolha: escopo da confirmação (universal), tratamento de versão (revalidação no servidor), banco não reconhecido (perguntar fechamento), edição multi-campo (permitida numa única confirmação), alteração de vencimento com parcelas em aberto (permitida após aviso), conteúdo da confirmação (de-para atual → novo) e padrão dos textos determinísticos (reuso dos padrões existentes).

Premissas confirmadas (não são questões em aberto, são restrições assumidas):
- O recálculo do dia de fechamento ao mudar banco ou vencimento reusa o serviço de dia de compra do módulo `internal/card` já utilizado na criação (`internal/card/domain/services/purchase_day.go`).
- O gate de avaliação real-LLM por categoria adota o limiar ≥ 0,90 e o formato já praticados no repositório para fluxos conversacionais.
- Os textos exatos das mensagens determinísticas serão fixados na especificação técnica seguindo RF-26, sem alterar a semântica definida neste PRD.
