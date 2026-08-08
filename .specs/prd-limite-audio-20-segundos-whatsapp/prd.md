# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->

## Visão Geral

O mecontrola já aceita e transcreve mensagens de voz do WhatsApp em produção, com pipeline completo de download, validação de duração, formato, tamanho, custo e confiança, seguido de transcrição via OpenRouter. Hoje o limite de duração aceito é de 60 segundos. Esta funcionalidade reduz o limite para 20 segundos e passa a informar o usuário de forma explícita quando o áudio excede o limite, transformando uma regra técnica silenciosa em um guardrail de produto visível, medido e reversível.

Problema que resolve: áudios longos elevam o custo de transcrição, aumentam a latência até a resposta do assistente e ampliam a janela de falhas de download e processamento, sem benefício proporcional para o registro de gastos, que é uma interação curta por natureza. Para quem: para o usuário que registra gastos por voz (resposta mais rápida e previsível, com aviso claro do limite) e para a operação do produto (custo de STT limitado por mensagem e comportamento auditável). Por que é valioso: o mecanismo de enforcement já existe e está testado, portanto a mudança é cirúrgica, com rollback instantâneo por configuração e risco de regressão próximo de zero.

Origem dos requisitos: história de usuário `docs/us/US-002-limite-audio-20-segundos-whatsapp.md`, validada pela skill `user-stories` com evidências de código em `path:linha` e decisões confirmadas em rodadas de múltipla escolha.

## Objetivos

- Sucesso primário (guardrail com monitoramento, decisão confirmada pelo usuário):
  - 100% dos áudios com duração acima de 20 segundos são rejeitados antes de qualquer chamada de transcrição, com registro de auditoria contendo outcome, reason e duração medida.
  - Zero regressão funcional: fluxo de texto, formatos aceitos, limites de tamanho, custo e confiança, feature flag e demais mensagens de rejeição permanecem idênticos ao comportamento atual, verificado pela suíte de testes existente e pelos gates de governança.
  - Áudios na fronteira de exatamente 20 segundos continuam aceitos, sem rejeição por arredondamento.
- Métricas chave a acompanhar pós-deploy:
  - Taxa de rejeição por `duration_exceeded` sobre o total de áudios recebidos, para medir o impacto da mudança nos usuários que hoje enviam áudios entre 20 e 60 segundos.
  - Ausência de variação nas demais taxas de rejeição (formato, tamanho, custo, confiança) e na taxa de aprovação de áudios dentro do limite.
  - Custo de STT por áudio aprovado, que passa a ter teto proporcional a 20 segundos.
- Metas de negócio:
  - Tornar o custo de transcrição por mensagem previsível e limitado, sem promessa numérica de redução percentual, pois não existe baseline de custo por áudio medido hoje (decisão confirmada: não inventar meta sem baseline).
  - Manter capacidade de rollback operacional do limite por variável de ambiente, sem necessidade de deploy, dentro da faixa já validada de 1 a 60 segundos.

## Histórias de Usuário

- Primária (detalhada e validada em `docs/us/US-002-limite-audio-20-segundos-whatsapp.md`): como usuário do mecontrola que registra gastos por mensagens de voz no WhatsApp, quero que áudios de até 20 segundos sejam aceitos e transcritos, recebendo um aviso claro quando o áudio passar do limite, para ter resposta rápida do assistente e saber exatamente como corrigir quando exceder a duração permitida.
- Secundária (operador do produto): como responsável pela operação e pelo custo do mecontrola, quero que o limite de duração seja configurável por ambiente dentro de uma faixa segura e que toda rejeição por duração seja auditada, para ajustar ou reverter a regra sem deploy e medir o impacto da mudança com dados reais.
- Casos de borda cobertos: áudio com duração exatamente igual a 20 segundos é aceito; áudio acima de 20 segundos é rejeitado antes da transcrição com mensagem específica; ambiente sem a variável de configuração explícita aplica o novo default de 20 segundos; mensagens de texto não passam por nenhuma validação de duração; usuário que reenvia áudio dentro do limite após rejeição tem o novo áudio processado normalmente, pois a idempotência por identificador de mensagem trata cada envio de forma independente.

## Funcionalidades Core

1. Limite de aceite de 20 segundos para áudio de voz
   - O que faz: restringe a 20 segundos a duração máxima de áudio aceita para transcrição, usando o mecanismo de enforcement de duração já existente no pipeline de áudio.
   - Por que é importante: limita custo de STT e latência por mensagem sem criar mecanismo novo, reduzindo risco de regressão.
   - Como funciona em alto nível: a duração máxima passa a ser 20 segundos por configuração, com o mesmo ponto de verificação e o mesmo outcome de rejeição auditado que já existem hoje.
2. Aviso explícito do limite na rejeição por duração
   - O que faz: quando a rejeição é por duração excedida, o usuário recebe mensagem específica informando que o limite é de 20 segundos, em vez da mensagem genérica atual.
   - Por que é importante: sem aviso do motivo, o usuário tende a reenviar outro áudio longo e sofrer nova rejeição silenciosa; a mensagem educa no momento do erro.
   - Como funciona em alto nível: a resposta de rejeição passa a ser escolhida também pelo motivo da rejeição, e não apenas pelo outcome, mantendo as mensagens atuais para todos os demais motivos.
3. Default seguro e rollback operacional
   - O que faz: ambientes sem configuração explícita passam a aplicar 20 segundos como default, e o limite pode ser ajustado ou revertido por variável de ambiente dentro da faixa de 1 a 60 segundos já validada no boot.
   - Por que é importante: garante o comportamento novo em qualquer ambiente e preserva reversibilidade instantânea sem deploy.
4. Observabilidade da mudança
   - O que faz: toda rejeição por duração continua registrada em auditoria com duração medida, permitindo acompanhar a taxa de `duration_exceeded` desde o primeiro deploy.
   - Por que é importante: é a única fonte de verdade para medir o impacto da redução do limite sobre os usuários e decidir ajustes futuros com evidência.

## Requisitos Funcionais

- RF-01: o sistema deve aceitar para transcrição somente áudios de voz com duração de até 20 segundos, inclusive a fronteira exata de 20 segundos.
- RF-02: o sistema deve rejeitar áudios com duração estritamente acima de 20 segundos antes de qualquer chamada de transcrição, reutilizando o ponto de enforcement de duração já existente no pipeline.
- RF-03: ao rejeitar por duração excedida, o sistema deve responder ao usuário com mensagem específica que informa o limite de 20 segundos, configurável por ambiente.
- RF-04: as rejeições pelos demais motivos (formato não suportado, tamanho excedido, custo excedido, baixa confiança, texto vazio, incoerência, idioma não suportado e falhas de mídia) devem manter exatamente as mensagens e os comportamentos atuais.
- RF-05: o limite de duração deve permanecer configurável por variável de ambiente, com default de 20 segundos e faixa de validação de 1 a 60 segundos no boot, preservando rollback operacional sem deploy.
- RF-06: toda rejeição por duração excedida deve ser registrada na auditoria de áudio existente com outcome de rejeição, reason `duration_exceeded` e a duração medida em milissegundos, mantendo a idempotência por identificador de mensagem.
- RF-07: o fluxo de mensagens de texto e as demais validações do pipeline de áudio (formatos OGG/Opus e M4A/AAC, limites de tamanho, custo e confiança, feature flag e reinjeção do texto transcrito no fluxo do assistente) devem permanecer funcionalmente inalterados.
- RF-08: a documentação operacional de áudio (runbook e exemplos de configuração de ambiente) deve ser atualizada no mesmo ciclo para refletir o novo default de 20 segundos, evitando divergência entre documentação e comportamento.
- RF-09: nenhuma comunicação proativa (broadcast ou template) deve ser enviada aos usuários sobre a mudança; a comunicação ocorre exclusivamente pela mensagem específica no momento da rejeição por duração (decisão confirmada pelo usuário).

## Experiência do Usuário

- Persona primária: usuário do mecontrola que envia mensagens de voz curtas pelo WhatsApp para registrar gastos e consultar finanças.
- Jornada feliz: o usuário envia um áudio de até 20 segundos e recebe a resposta normal do assistente, sem perceber nenhuma mudança em relação ao comportamento atual.
- Jornada de erro recuperável: o usuário envia um áudio acima de 20 segundos e recebe uma mensagem clara informando que o limite é de 20 segundos; ao regravar mais curto e reenviar, o áudio é processado normalmente.
- Jornada inalterada: usuários de mensagem de texto não são afetados em nenhum aspecto.
- Considerações de UX: a mensagem de rejeição por duração deve ser curta, em português, informar o limite em segundos e orientar o reenvio; o tom deve seguir o das mensagens de áudio já existentes no produto.
- Acessibilidade: não se aplica além do canal WhatsApp já existente; não há interface visual nova.

## Restrições Técnicas de Alto Nível

- Integrações existentes que devem ser preservadas: WhatsApp Cloud API da Meta para inbound, download de mídia e resposta; OpenRouter como único provedor de STT e LLM; pipeline assíncrono via outbox e worker já em produção.
- Restrição do provedor de transcrição: a documentação oficial da OpenAI (modelo servido via OpenRouter) define limite de 25 MB por arquivo e nenhum teto de duração, portanto o limite de 20 segundos é regra de produto do mecontrola, não restrição externa, e pode ser revertido sem negociação com provedor.
- Restrição da integração Meta: o payload do webhook não carrega a duração do áudio; a duração só é conhecida após o download e o parse dos bytes, portanto a rejeição antes do download é tecnicamente inviável nesta integração e está fora de escopo.
- Zero regressão como restrição não negociável: a mudança deve se limitar a configuração, default, seleção de mensagem por motivo e documentação, sem alterar estruturas de dados, contratos de eventos, migrations ou o fluxo canônico Thread-Run do assistente.
- Privacidade e dados: nenhum dado novo é coletado; os bytes de áudio e a transcrição seguem o tratamento já existente no pipeline, sem retenção adicional.
- Performance: a mudança não adiciona chamadas externas nem etapas novas ao pipeline; a latência de áudios aprovados tende a reduzir pelo teto de duração menor.
- Conformidade com governança do repositório: implementação regida obrigatoriamente pelas skills `go-implementation`, `mastra`, `domain-modeling-production` e `design-patterns-mandatory`, com consumer e handlers mantidos como adapters finos e decisão por motivo dentro do use case.

## Fora de Escopo

- Suporte a novos formatos de áudio como MP3 ou WAV.
- Endurecimento da faixa de validação para impedir ajuste operacional acima de 20 segundos sem deploy.
- Alteração dos limites de tamanho, custo ou confiança, da feature flag de áudio ou das mensagens dos demais motivos de rejeição.
- Rejeição de áudio longo antes do download da mídia.
- Envio de áudio outbound pelo assistente.
- Suporte a imagem, vídeo ou documento no inbound.
- Comunicação proativa da mudança por broadcast, template ou campanha.
- Instrumentação de funil de reenvio após rejeição e definição de metas percentuais de custo ou latência, ambas dependentes de baseline que não existe hoje.

## Suposições e Questões em Aberto

Não restam suposições não suportadas nem questões em aberto. Todas as decisões materiais foram confirmadas pelo usuário em rodadas de múltipla escolha durante a elaboração da US-002 e deste PRD:

- Forma de aplicação do limite: configuração de ambiente mais novo default de 20 segundos no código, mantendo a faixa de validação de 1 a 60 segundos.
- Mensagem de rejeição: específica para o motivo de duração excedida, informando o limite de 20 segundos, com os demais motivos inalterados.
- Critério de sucesso: guardrail com monitoramento da taxa de `duration_exceeded` e zero regressão, sem meta numérica de negócio por ausência de baseline.
- Comunicação: silenciosa, com educação do usuário exclusivamente pela mensagem no momento da rejeição.

Registro de evidência: as afirmações sobre o comportamento atual do sistema derivam da confrontação de código documentada em `docs/us/US-002-limite-audio-20-segundos-whatsapp.md` (seção Evidências, com `path:linha`), incluindo a lista do que foi buscado e não encontrado, sem falso positivo de evidência.
