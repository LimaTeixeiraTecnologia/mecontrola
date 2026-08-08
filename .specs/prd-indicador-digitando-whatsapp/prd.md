# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 1 -->

Funcionalidade: Indicador de digitação no WhatsApp durante o processamento da resposta
Origem: `docs/us/US-001-indicador-digitando-whatsapp.md` (validada por `validar-historias-usuario.py`)

## Visão Geral

O assistente financeiro mecontrola conversa com o usuário pelo WhatsApp. Hoje, entre o envio da mensagem e a resposta final, o usuário vê silêncio total: sem ticks azuis de leitura e sem indicador de digitação. Essa espera é estrutural, pois o processamento é assíncrono via outbox e envolve STT de áudio, chamada de LLM e execução de tools, durando segundos no caso normal. A funcionalidade exibe o indicador de "digitando" da WhatsApp Cloud API logo no início do processamento, sinalizando que a mensagem foi recebida e que a resposta está a caminho. O recurso é oficial da Meta, inseparável do read receipt (ticks azuis), dura até 25 segundos ou até a resposta ser enviada, e será entregue atrás de feature flag desligada por default para garantir zero regressão.

## Objetivos

- Eliminar a percepção de mensagem ignorada durante a espera pela resposta do assistente.
- Entregar a melhoria sem nenhuma alteração de comportamento para quem operar com a flag desligada, incluindo latência, taxa de erro e contagem de mensagens.
- Permitir rollback instantâneo sem deploy, apenas desligando a flag.

Métricas chave (saúde técnica, medidas com a stack de observabilidade existente, sem instrumentação nova de produto):

- Taxa de sucesso da chamada de indicador de digitação: meta acima de 99% das emissões tentadas.
- Latência p95 da resposta final ao usuário: variação zero dentro da margem de erro após a ativação.
- Taxa de erro da resposta final (`outcome=send_error` no contador existente de inbound): variação zero após a ativação.
- Incidentes atribuíveis à funcionalidade: zero.

## Histórias de Usuário

- Como usuário do mecontrola que conversa com o assistente financeiro pelo WhatsApp, quero ver o indicador de "digitando" logo após enviar minha mensagem, para saber que ela foi recebida e que uma resposta está sendo preparada em vez de ficar no silêncio. (US-001, persona primária)
- Como usuário que envia áudio, quero o mesmo sinal de recebimento, pois o processamento de voz é o caminho mais lento e o mais sujeito à percepção de falha. (persona primária, caso de borda de latência)
- Como operador da plataforma, quero ligar e desligar o recurso por ambiente sem deploy, para reverter instantaneamente se a Meta apresentar instabilidade. (persona secundária: operação)

## Funcionalidades Core

1. Emissão do indicador de digitação no início do processamento
   - O que faz: ao consumir uma mensagem inbound válida e não duplicada, emite uma chamada à WhatsApp Cloud API que marca a mensagem como lida e exibe "digitando" ao usuário.
   - Por que importa: é o único sinal de recebimento visível ao usuário antes da resposta final; hoje esse sinal não existe.
   - Como funciona em alto nível: ponto único de emissão no consumer do worker, cobrindo texto, áudio, retomada de workflow e onboarding, pois todo fluxo aceito nesse consumer termina em resposta ao usuário.
2. Controle por feature flag
   - O que faz: liga e desliga a emissão por configuração de ambiente, com default desligado.
   - Por que importa: é o mecanismo de zero regressão e de rollback instantâneo.
   - Como funciona em alto nível: com a flag desligada, nenhuma chamada nova é feita e o fluxo permanece idêntico ao atual.
3. Falha silenciosa e observável
   - O que faz: erro na chamada do indicador é registrado em log estruturado e métrica, sem retry, sem bloqueio e sem impacto na resposta final.
   - Por que importa: o indicador é melhoria de percepção, nunca dependência funcional; a resposta ao usuário não pode depender dele.
   - Como funciona em alto nível: tratamento best-effort coerente com o client Meta existente, que já opera sem retry.

## Requisitos Funcionais

- RF-01: Para toda mensagem inbound aceita pelo consumer do worker (payload válido e não duplicada), o sistema deve emitir uma única chamada à WhatsApp Cloud API contendo `status: "read"`, o `message_id` da mensagem inbound e `typing_indicator` do tipo `text`, antes de iniciar o processamento de áudio, retomada de workflow, onboarding ou agente.
- RF-02: A emissão deve ocorrer exatamente uma vez por mensagem. Não haverá renovação periódica do indicador; a expiração automática de 25 segundos definida pela Meta é aceita nesta versão.
- RF-03: Não deve haver emissão para mensagens sem fluxo de resposta: duplicadas pelo dedup de WAMID, payloads inválidos rejeitados na validação, e rotas fora do consumer do worker, incluindo a rota de ativação de números desconhecidos no dispatcher.
- RF-04: Falha da chamada do indicador (timeout, 4xx, 5xx) deve ser tratada como best-effort: log estruturado com identificadores da mensagem, incremento de métrica de erro, sem retry, sem bloqueio e sem alteração da resposta final.
- RF-05: A funcionalidade deve ser controlada por configuração de ambiente (feature flag) com default desligado, permitindo ativação e desativação sem deploy e sem mudança de código.
- RF-06: Com a flag desligada, o comportamento deve ser idêntico ao atual de produção: nenhuma chamada adicional à Meta, nenhuma alteração de latência, de taxa de erro, de contadores de outcome ou de contagem de mensagens observada por testes unitários, de integração e e2e existentes.
- RF-07: A ativação em qualquer ambiente exige evidência prévia de que a versão da Graph API em uso pelo client Meta aceita o campo `typing_indicator`, demonstrada por chamada real em ambiente de teste com resposta de sucesso e indicador observado no aparelho. Se a versão em uso não aceitar o campo, a ativação fica bloqueada até decisão de bump de versão, que deve cobrir em regressão o client de mensagens e o client de mídia, pois ambos compartilham a versão pinada.
- RF-08: A emissão deve ser observável por métrica de contador com labels de canal e outcome (sucesso ou erro), seguindo o padrão do contador de inbound já existente, sem labels de alta cardinalidade como identificadores de usuário ou de mensagem.
- RF-09: O indicador não pode interferir em mecanismos existentes: dedup de WAMID e sua compensação, outbox, persistência de mensagens, e contagem de mensagens enviadas usada por mocks e testes e2e. A operação de indicador deve ser distinta da operação de envio de texto.
- RF-10: Mensagens cujo processamento exceda 25 segundos (por exemplo, áudio com STT e LLM) seguem com emissão única; a resposta final deve ser entregue normalmente mesmo após a expiração da bolha.

## Experiência do Usuário

- Persona primária: usuário do mecontrola no WhatsApp, mobile, em conversas de texto e áudio.
- Fluxo principal: o usuário envia mensagem, vê ticks azuis segundos depois junto com a bolha de "digitando", e a bolha desaparece quando a resposta chega.
- Caso de borda conhecido e aceito: em processamentos acima de 25 segundos a bolha expira antes da resposta; o usuário vê digitando, depois silêncio, depois a resposta. Essa limitação é da plataforma Meta, está documentada e foi aceita na decisão de escopo.
- Considerações de UX: o indicador é inseparável dos ticks azuis (contrato da Meta), portanto a leitura passa a ser exibida no início do processamento, não na entrega da resposta. Esse efeito foi avaliado e aceito.
- Acessibilidade: não se aplica alteração de interface própria; o indicador é renderizado pelo cliente WhatsApp do usuário.

## Restrições Técnicas de Alto Nível

- Integração externa: WhatsApp Business Cloud API oficial da Meta, no endpoint de mensagens já utilizado hoje, com as credenciais `META_ACCESS_TOKEN` e `META_PHONE_NUMBER_ID` já existentes; nenhuma credencial ou permissão nova.
- Contrato da plataforma Meta: o indicador dura no máximo 25 segundos ou até o envio da resposta; a chamada marca a mensagem como lida de forma inseparável; a Meta recomenda não exibir o indicador quando não haverá resposta, o que fundamenta o RF-03.
- Versão da Graph API: o client está pinado em versão específica hoje; a compatibilidade do campo `typing_indicator` com essa versão não foi confirmada na documentação oficial (leitura automatizada bloqueada) e por isso o RF-07 é gate obrigatório de ativação, não suposição.
- Zero regressão: flag desligada por default, operação separada do envio de texto, e suíte existente (unitária, integração, e2e) como evidência obrigatória de paridade de comportamento.
- Performance: uma chamada HTTP adicional à Meta por mensagem processada; sem impacto mensurável permitido na latência da resposta final; falha do indicador nunca pode adicionar latência ao fluxo de resposta.
- Privacidade: nenhum dado novo é coletado ou enviado; a chamada usa apenas o identificador da mensagem já recebido via webhook.
- Governança do repositório: implementação sujeita às skills `go-implementation` (entrypoint Go), `mastra` (fluxo Thread-Run no consumer, sem reimplementar primitivos de plataforma), `domain-modeling-production` (princípios DMMF) e `design-patterns-mandatory` (gate de desenho; expectativa de `não aplicar padrão` dada a simplicidade da chamada).

## Fora de Escopo

- Renovação periódica do indicador além dos 25 segundos em processamentos longos (candidato a iteração futura, decidido após medição de latência real).
- Indicador na rota de ativação de números desconhecidos, que roda no dispatcher e responde por template (decisão registrada: resposta rápida, fora do ponto de emissão aprovado).
- Indicador em mensagens proativas (lembretes, alertas de orçamento, aviso de fatura, templates de onboarding), pois não são resposta a mensagem inbound.
- Mark-as-read isolado, sem indicador de digitação.
- Reações de emoji, status de "online" ou qualquer outra forma de presença.
- Bump de versão da Graph API (só entra em pauta se o gate do RF-07 reprovar a versão atual, como decisão separada).
- Métricas de comportamento do usuário (mensagens repetidas, reenvios), adiadas como medição de valor para fase futura; o sucesso desta entrega é medido por saúde técnica.

## Suposições e Questões em Aberto

Não restam questões em aberto nem suposições não verificadas neste documento. As decisões de escopo foram confirmadas pelo usuário em duas rodadas de múltipla escolha: ponto de emissão no worker, emissão única, rollout por flag desligada por default, cobertura de todos os fluxos com resposta, exclusão da rota de ativação e sucesso medido por saúde técnica. O único fato externo não verificável neste momento (suporte do campo `typing_indicator` na versão pinada da Graph API) não é tratado como suposição: está convertido no gate obrigatório RF-07, que bloqueia a ativação até haver evidência real em ambiente de teste.
