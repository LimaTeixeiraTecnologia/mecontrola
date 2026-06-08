# PRD — Onboarding via Magic Token (E3)

<!-- spec-version: 1 -->

> **Slug:** `onboarding-magic-token`
> **Épico de origem:** `docs/epics/epic-03-onboarding-magic-token.md`
> **Discoveries de origem:** `docs/discoveries/discovery-onboarding-flow.md`, `docs/discoveries/discovery-billing-hotmart-kiwify.md`, `docs/discoveries/discovery-identity-entitlement.md`
> **Dependências de roadmap:** bloqueado por E1 (`identity-foundation`, PRD em `.specs/prd-identity-foundation/prd.md`); co-dependente em runtime de E2 (`billing-pipeline`, PRD em `.specs/prd-billing-pipeline/prd.md`); precede E4 (`reconciliation-hardening`, backlog pós-MVP, **fora de escopo deste PRD**).

## Visão Geral

O MeControla é um produto 100% WhatsApp. A landing `mecontrola.app.br` vende uma promessa simples: o usuário paga e começa a usar **no próprio WhatsApp**, sem app para baixar e sem cadastro. O problema central que este PRD resolve é o **vínculo confiável entre o pagamento aprovado e o número real de WhatsApp** que vai operar a conta.

Hoje, sem este épico, o fluxo está quebrado em dois pontos:

1. O número que o cliente digita no checkout da Kiwify é **input não verificado** — pode ter erro de digitação, formato divergente, ou ser o número de outra pessoa.
2. O backend recebe `order_approved` da Kiwify mas não tem como saber em qual conversa do WhatsApp o cliente vai aparecer, então a conta fica órfã até alguém intervir manualmente.

A solução de produto é introduzir um **magic token** opaco que viaja junto com o checkout, é marcado como pago via webhook, e é resgatado pelo próprio cliente ao enviar `ATIVAR <token>` no WhatsApp. A identidade do número passa a ser garantida pelo canal (Meta autentica o WhatsApp), e o token funciona como ponte entre a venda aprovada e a conta ativada.

O valor entregue: cliente paga e ativa em poucos cliques, suporte para de receber tickets de "paguei e não funciona", e o sistema passa a ter um sinal forte de identidade do WhatsApp em vez de depender de campo digitado no checkout.

## Objetivos

- **Reduzir órfãos pós-compra:** garantir que a maioria dos pagamentos aprovados se converta em conta ativada sem intervenção humana, dentro de uma janela operacional pequena.
- **Estabelecer identidade confiável do WhatsApp:** o número que opera a conta passa a ser o número que efetivamente abriu o `wa.me`, não o digitado no checkout.
- **Eliminar fricção entre pagamento e primeiro uso:** entre a confirmação do pagamento e a primeira mensagem útil do bot deve existir, no máximo, um clique e uma mensagem.
- **Tornar o suporte operacional escalável:** o caminho feliz precisa ser autoatendido; suporte humano só deve aparecer para exceções genuínas (token expirado, número conflitante, fraude).
- **Métricas-chave a acompanhar** (sem fixar metas neste PRD por falta de baseline em produção):
  - taxa de ativação sobre pagamentos aprovados (`activation_consumed / token_paid`)
  - distribuição do tempo entre `PAID` e `CONSUMED`
  - participação dos caminhos de ativação (direto, fallback de match E.164, outreach)
  - volume de tokens em `PAID` não consumidos em janela operacional (gauge para alerta)
  - taxa de falha do caminho feliz (`ATIVAR` que cai em mensagens de erro / total de tentativas)

## Histórias de Usuário

### Persona primária

**Cliente final** — pessoa adulta no Brasil que descobre o MeControla pela landing e decide assinar para controlar gastos via WhatsApp. Usa principalmente celular. Não quer baixar app, criar senha ou preencher cadastro.

### Personas secundárias

- **Suporte operacional** — responde casos onde a ativação automática falhou (token expirado, número diferente, compra duplicada).
- **Operação de produto/marketing** — precisa enxergar funil de conversão entre cliques na landing, pagamentos e ativações.

### Histórias

- **HU-01** — Como **cliente final**, quero pagar diretamente na landing e ativar minha conta no WhatsApp em poucos cliques, para começar a registrar gastos sem fricção de cadastro.
- **HU-02** — Como **cliente final**, quero que o WhatsApp abra com a mensagem de ativação já pronta, para não precisar digitar nem copiar nada.
- **HU-03** — Como **cliente final** no desktop, quero uma alternativa visível para copiar o código de ativação caso o WhatsApp não abra automaticamente, para conseguir ativar manualmente.
- **HU-04** — Como **cliente final** que pagou e esqueceu de ativar, quero ser lembrado via WhatsApp em janela razoável, para retomar a ativação sem precisar abrir o e-mail.
- **HU-05** — Como **cliente final** que pagou mas o WhatsApp não abriu corretamente, quero que o sistema reconheça o meu número e ative minha conta automaticamente quando eu mandar qualquer mensagem, para não ficar preso na primeira tentativa.
- **HU-06** — Como **cliente final**, quero que reenviar acidentalmente o mesmo `ATIVAR` não quebre minha conta, para reduzir ansiedade de "será que apertei direito?".
- **HU-07** — Como **cliente final** que pagou via Pix com confirmação lenta, quero uma mensagem clara dizendo que o pagamento ainda está processando, para saber que devo tentar novamente em vez de presumir falha.
- **HU-08** — Como **cliente final**, quero que meu código de ativação não funcione em outro número que não seja o meu, para ter alguma segurança contra reuso indevido.
- **HU-09** — Como **suporte**, quero receber sinal claro quando um token é tentado em número conflitante, para conseguir investigar potencial fraude sem ler logs crus.
- **HU-10** — Como **operação de produto**, quero ver o funil de cliques → pagamentos → ativações segmentado por caminho de ativação, para entender onde o usuário fica preso.

## Funcionalidades Core

1. **Criação de sessão de checkout com magic token**
   - O que faz: a partir do clique em "Assinar" na landing, o backend gera um token opaco, persiste em estado inicial e devolve a URL de checkout do provedor de pagamento já com o token embutido.
   - Por que importa: o token é a peça que carrega a identidade do pagamento até o WhatsApp; sem ele, não há ponte entre a venda e a conta.
   - Como funciona em alto nível: endpoint público chamado pela landing, protegido por rate limit, com prazo de expiração definido por produto.

2. **Marcação de token como pago a partir do evento de compra aprovada**
   - O que faz: quando o evento de compra aprovada chega pelo pipeline de billing (E2), o token correspondente sai de "pendente" para "pago" e captura o número de WhatsApp digitado no checkout, e-mail e identificador externo da venda.
   - Por que importa: esse é o sinal de que o cliente já está autorizado a ativar; alimenta também o fallback por outreach.
   - Como funciona em alto nível: este PRD descreve o **contrato consumido** desse marcador; a entrega operacional do webhook fica em E2.

3. **Página de agradecimento com deep link para o WhatsApp**
   - O que faz: após o pagamento aprovado, o cliente é levado para uma página própria do MeControla que exibe um botão grande para abrir o WhatsApp com `ATIVAR <token>` pré-preenchido, mais um fallback visível em texto.
   - Por que importa: é o ponto de fricção crítico onde a conversão pode cair; a página precisa ser nossa, não a padrão do provedor.
   - Como funciona em alto nível: a página recebe o token via URL, tenta auto-redirecionar em mobile depois de um tempo curto, e mostra alternativa de copiar e colar como rede de segurança.

4. **Comando `ATIVAR <token>` no canal do WhatsApp**
   - O que faz: o bot reconhece a mensagem `ATIVAR <token>`, resolve o token, valida estado e expiração, e ativa a conta vinculando o número real ao pagamento.
   - Por que importa: é onde a identidade do WhatsApp passa a ser oficial.
   - Como funciona em alto nível: ativação acontece de forma atômica e idempotente, com mensagens distintas para cada estado relevante do token (inexistente, expirado, ainda pendente, pago, já consumido pelo mesmo número, já consumido por outro número, pagamento ainda processando).

5. **Ativação atômica do cliente**
   - O que faz: em uma única operação consistente, o sistema cria/atualiza o usuário pelo número de WhatsApp real, vincula a assinatura, marca o token como consumido e dispara invalidação do cache de direito de uso (entitlement) gerido por E2.
   - Por que importa: garante que não exista estado intermediário onde o cliente pode ser cobrado mas não tenha conta, ou ter conta mas sem assinatura.
   - Como funciona em alto nível: é uma garantia transacional; o detalhamento técnico é da techspec.

6. **Fallback automático por outreach via template do WhatsApp Business**
   - O que faz: para tokens pagos não consumidos após uma janela operacional curta, o sistema dispara mensagem proativa por template aprovado da Meta para o número que o cliente digitou no checkout, com o `ATIVAR <token>` pronto para uso.
   - Por que importa: cliente que pagou e esqueceu não pode virar churn silencioso; o outreach também habilita o fallback de match E.164 (Funcionalidade 7).
   - Como funciona em alto nível: job periódico identifica candidatos, normaliza o número via componente de identidade (E1) e dispara via WhatsApp Business. **Cada token recebe no máximo uma única tentativa de outreach durante toda a sua vida útil** — política conservadora para preservar reputação da conta WhatsApp Business junto à Meta e evitar percepção de assédio. Tentativas adicionais são deliberadamente excluídas do MVP.

7. **Fallback de ativação por match de número (E.164), gated por outreach**
   - O que faz: se o cliente, em vez de abrir o `wa.me`, responder qualquer mensagem ao bot a partir do mesmo número de WhatsApp que digitou no checkout, **e o token correspondente já tiver recebido o outreach da Funcionalidade 6**, o sistema ativa a conta automaticamente e registra que a ativação ocorreu por esse caminho.
   - Por que importa: é a rede de segurança para casos em que o deep link falha (desktop, app antigo, WhatsApp Web). A pré-condição "outreach já enviado" garante que a ativação automática só ocorra após contato intencional do bot — reduz risco de ativar conta da pessoa errada quando o cliente digitou número de terceiro no checkout (o terceiro nunca recebeu mensagem, então sua resposta espontânea não dispara nada).
   - Como funciona em alto nível: é tratado como **mitigação**, não como caminho principal; cada ativação por esse caminho é rastreada para análise de funil.

8. **Expiração de tokens vencidos e sinalização de subscription órfã**
   - O que faz: tokens cuja janela de ativação expira são marcados como expirados por processo de limpeza. Quando o token expirado tiver estado pago em algum momento (cliente pagou e nunca ativou), o sistema emite sinal estruturado de "subscription órfã expirada" para fila consultável por suporte humano.
   - Por que importa: token aberto indefinidamente vira passe livre; janela curta demais aumenta uso de fallback. A assinatura permanece ativa no provedor mesmo após token expirar — decisão sobre reembolso, contato proativo ou reemissão de token novo cabe ao suporte humano, não a automação no MVP.
   - Como funciona em alto nível: rotina periódica de limpeza com cadência de produto definida (diária); a assinatura **não é cancelada automaticamente** por este fluxo.

9. **Telemetria e funil de ativação**
   - O que faz: o sistema emite métricas que permitem desenhar o funil "clique no Assinar → pagamento aprovado → ativação consumida → primeira mensagem útil", com segmentação por caminho de ativação.
   - Por que importa: sem isso, não há como saber se a página de agradecimento está convertendo, se o outreach está salvando carrinhos abandonados, ou se há fraude crescendo.
   - Como funciona em alto nível: contadores, gauge para fila de pagos não consumidos, histograma de tempo entre pago e consumido, todos com segmentação por caminho.

10. **Mascaramento de PII em logs**
    - O que faz: número de WhatsApp digitado no checkout, número real e e-mail aparecem mascarados em logs estruturados.
    - Por que importa: requisito de LGPD; tratamento mínimo para reduzir exposição em incidentes de log e análises de erro.
    - Como funciona em alto nível: política de logging de campos sensíveis, alinhada à governança transversal do repositório.

## Requisitos Funcionais

- **RF-01:** Ao receber requisição autorizada pela landing para criar sessão de checkout, o sistema deve gerar um token opaco e não enumerável, persistir o token em estado inicial "pendente" com prazo de expiração de **7 dias corridos**, e devolver a URL de checkout do provedor com o token embutido no parâmetro `s`. **O endpoint não é idempotente:** cada chamada cria um token distinto, e cliques duplos ou retentativas geram tokens PENDING órfãos que serão tratados pela rotina de limpeza (RF-11). A defesa contra abuso é exclusivamente o rate limit definido em RF-02; este PRD não introduz `Idempotency-Key`, deduplicação por IP+UA ou outras políticas de coalescing no MVP.
- **RF-02:** O endpoint de criação de sessão de checkout deve estar protegido por rate limit de **10 requisições por minuto por IP de origem**, retornando rejeição clara quando o limite for excedido, sem criar token.
- **RF-03:** O sistema deve aceitar e processar a marcação de um token como "pago" disparada pelo pipeline de billing (E2), armazenando, no mínimo: número de WhatsApp digitado no checkout, e-mail do comprador, identificador externo da venda e instante de aprovação. Marcações repetidas para o mesmo token devem ser idempotentes.
- **RF-04:** O sistema deve disponibilizar uma página de agradecimento própria do MeControla, acessível por URL contendo o token, que apresente um botão de abertura do WhatsApp com a mensagem `ATIVAR <token>` pré-preenchida e um bloco de texto alternativo visível com o mesmo comando e instruções para envio manual.
- **RF-05:** A página de agradecimento deve tentar redirecionar automaticamente para o WhatsApp em dispositivos móveis depois de pequeno intervalo, mantendo o fallback de copiar e colar visível durante toda a sessão.
- **RF-06:** O comando `ATIVAR <token>` recebido no canal do WhatsApp deve produzir, conforme o estado do token:
  - se inexistente: mensagem orientando a conferir o código;
  - se expirado ou com prazo vencido: mensagem orientando contato com suporte;
  - se ainda em estado pendente (pagamento não confirmado): mensagem orientando a tentar novamente em alguns instantes;
  - se em estado pago: ativação plena da conta;
  - se já consumido pelo mesmo número que está enviando: mensagem confirmando que a conta já está ativa, sem erro;
  - se já consumido por número distinto: mensagem informando que o código já foi usado em outra conta, e o sistema deve sinalizar o evento para análise de suporte.
- **RF-07:** A ativação plena de um token pago deve, em uma única operação transacional: criar ou atualizar o usuário pelo número de WhatsApp real (vindo do canal), vincular a assinatura correspondente a esse usuário, marcar o token como consumido com timestamp, e disparar invalidação do cache de direito de uso (entitlement) operado por E2.
- **RF-08:** O comando `ATIVAR <token>` deve ser idempotente quando reenviado pelo mesmo número de WhatsApp: múltiplos envios consecutivos do mesmo token pelo mesmo número devem produzir o mesmo estado final na conta e respostas amigáveis ao cliente, sem gerar erros ou cobranças adicionais.
- **RF-09:** O sistema deve executar, em cadência horária, um processo de outreach que selecione tokens pagos não consumidos há **mais de 2 horas** com número de WhatsApp digitado no checkout válido (normalizável em E.164 BR) e que ainda não tenham recebido outreach, e dispare para esse número o template pré-aprovado do WhatsApp Business com instrução para enviar o `ATIVAR <token>`. **Cada token deve receber no máximo uma única mensagem de outreach durante toda a sua vida útil**, marcada por timestamp persistente; iterações subsequentes do job devem ignorar tokens que já tenham outreach registrado, independentemente do resultado de conversão.
- **RF-10:** O sistema deve oferecer fallback de ativação por match de número, com pré-condição obrigatória: a ativação automática só pode ocorrer se o token pago vinculado ao número de origem **já tiver recebido o outreach de RF-09**. Quando uma mensagem qualquer (não restrita a `ATIVAR`) for recebida de um número de WhatsApp que, normalizado em E.164, coincida com o número digitado no checkout de um token pago **com outreach já enviado**, o sistema deve ativar a conta seguindo a mesma transação de RF-07 e registrar que a ativação ocorreu pelo caminho de fallback. Mensagens vindas de números casados com tokens pagos **sem outreach prévio** não devem disparar ativação automática — o cliente deve ser orientado a usar o comando `ATIVAR <token>`.
- **RF-11:** O sistema deve executar, em cadência diária, um processo de limpeza que marque como expirados todos os tokens cujo prazo de expiração já tenha sido atingido sem terem chegado ao estado consumido.
- **RF-12:** Para cada token marcado como expirado por RF-11 que tenha estado pago em algum momento (ou seja, cliente que pagou e nunca ativou dentro da janela), o sistema deve emitir um sinal estruturado de **subscription órfã expirada** contendo, no mínimo, identificador externo da venda, hash do token, instante de expiração e indicação de que existe assinatura ativa sem usuário vinculado. Esse sinal deve ser persistido em fila ou tabela consultável por suporte para tratamento humano (decisão de reembolso, contato proativo ou reemissão de token novo fica a cargo do suporte). A assinatura em si **não deve ser cancelada automaticamente** por este fluxo.
- **RF-13:** O sistema deve emitir as seguintes métricas mínimas: total de sessões de checkout criadas, total de tokens marcados como pagos, total de ativações consumidas com segmentação por caminho (direto pela página de agradecimento, fallback por match de número, outreach), número atual de tokens pagos não consumidos (gauge), distribuição de tempo entre marcação como pago e consumo, total de outreach enviados, total de subscriptions órfãs expiradas (RF-12), total de acessos inválidos à thank-you page segmentado por motivo (`ty_page_invalid_access_total` — RF-17), e total de pagamentos aprovados sem token (`billing_paid_without_token_total` — RF-18).
- **RF-14:** Logs estruturados emitidos por este fluxo devem mascarar campos sensíveis (número de WhatsApp digitado no checkout, número de WhatsApp real, e-mail), apresentando apenas formas mascaradas suficientes para investigação operacional.
- **RF-15:** Toda tentativa de uso de um token já consumido a partir de número distinto deve ser sinalizada por **métrica segmentada por motivo** (`token_reuse_attempt_total{reason="different_number"}`) **e por log estruturado** com hash do token, número de origem mascarado e instante. **Não é exigido canal dedicado de alerta** (Slack, e-mail, paging) no MVP — a consulta é feita por suporte via dashboard de métricas e busca em logs. Antifraude avançada (bloqueio dinâmico, quarentena de número, scoring) permanece fora do escopo do MVP.
- **RF-16:** O fluxo deve operar sob a premissa de que **somente brasileiros** (números no espaço E.164 com código de país BR) são suportados no MVP; entradas que não sejam normalizáveis nesse espaço pelo componente de identidade (E1) devem ser rejeitadas com mensagem amigável e registradas para diagnóstico.
- **RF-17:** A página de agradecimento (RF-04/RF-05), quando acessada com token inválido, expirado, inexistente, ainda pendente ou já consumido, deve apresentar **uma única mensagem genérica** no estilo "Link inválido ou expirado. Fale com nosso suporte" e oferecer CTA de contato com suporte. A página **não deve distinguir publicamente** entre estados internos do token (defesa contra enumeração e oracle). O sistema deve emitir métrica `ty_page_invalid_access_total` segmentada por motivo interno (inexistente, pendente, expirado, consumido) para diagnóstico operacional, sem expor esse motivo no HTML renderizado.
- **RF-18:** Quando o evento de pagamento aprovado (RF-03) chegar do pipeline de billing (E2) sem token associado (parâmetro `s` ausente ou vazio no payload propagado pela Kiwify), o sistema deve **aceitar o evento** (E2 segue criando a assinatura conforme suas próprias regras) e emitir um sinal estruturado de **"pagamento aprovado sem token"** contendo, no mínimo, identificador externo da venda, e-mail do comprador (mascarado em log, presente para suporte), número de WhatsApp digitado no checkout (mascarado em log) e instante de aprovação. Esse sinal deve alimentar a mesma fila/canal consultável por suporte usada em RF-12, permitindo vinculação manual posterior. Métrica obrigatória: `billing_paid_without_token_total`. **O pagamento aprovado nunca deve ser silenciosamente descartado** por falta de token, mesmo que isso implique conta órfã até intervenção humana.
- **RF-19:** A página de agradecimento (RF-04/RF-05) deve atender, no mínimo, ao nível **WCAG 2.1 AA** nos elementos críticos do caminho feliz: contraste de cor mínimo de 4.5:1 para texto normal e 3:1 para texto grande, navegação por teclado no CTA principal e no bloco de fallback de copiar e colar, texto alternativo no botão de WhatsApp, indicação visível de foco, viewport responsivo para mobile e suporte básico a leitor de tela (semântica HTML correta no botão e no bloco de instruções).

## Experiência do Usuário

### Jornada principal (caminho feliz, mobile)

1. Cliente entra na landing e clica em "Assinar".
2. É redirecionado para o checkout do provedor já com o token embutido na URL.
3. Conclui pagamento.
4. É redirecionado para a página de agradecimento do MeControla, que abre automaticamente o WhatsApp com `ATIVAR <token>` pronto para envio.
5. Aperta enviar.
6. O bot responde confirmando ativação e convida o cliente a registrar o primeiro gasto.

### Jornada de desktop

- Após o pagamento, a página de agradecimento exibe o botão de WhatsApp e, com clareza, o texto `ATIVAR <token>` e o número oficial do bot para uso em WhatsApp Web ou app de celular.
- Auto-redirect não é confiável em desktop; a página deve manter o foco no fallback de copiar e colar.

### Jornada de recuperação (esquecimento)

- Algum tempo após o pagamento sem ativação, o cliente recebe no WhatsApp uma mensagem por template oficial lembrando que basta enviar `ATIVAR <token>` para concluir.
- Caso responda qualquer coisa diretamente do número que digitou no checkout, o sistema reconhece e ativa automaticamente, respondendo com confirmação.

### Estados de erro percebidos pelo cliente

- Código inválido → mensagem curta orientando conferir o código na página de pagamento.
- Código expirado → mensagem curta orientando contato com suporte.
- Pagamento ainda processando → mensagem curta pedindo nova tentativa em 1 minuto.
- Código já usado por outra conta → mensagem curta informando o bloqueio, sem detalhes técnicos.
- Reativação pelo mesmo número → mensagem amigável confirmando que a conta já está ativa.

### Princípios de UX inegociáveis

- Caminho feliz: 1 clique para abrir WhatsApp, 1 envio para ativar.
- Página de agradecimento: própria, controlada pelo MeControla, com fallback visível sempre.
- Todas as respostas do bot são curtas, em português, e nunca expõem o token a outro usuário nem detalhes internos.
- Acessibilidade mínima da thank-you page: **WCAG 2.1 AA** nos elementos críticos (contraste, foco visível, navegação por teclado, alt text, semântica HTML correta) — formalizado em RF-19.

## Restrições Técnicas de Alto Nível

- **Identidade obrigatória pelo canal WhatsApp:** o número que opera a conta é o número que envia a mensagem pelo WhatsApp; nenhum outro canal pode autenticar o cliente neste fluxo.
- **Token opaco e não enumerável:** o token entregue na URL precisa ser opaco e seguro contra enumeração; nenhuma informação sensível deve ser embutida nele.
- **Janela de vida do token:** 7 dias corridos no MVP. Mudanças nesta janela são decisão de produto, não de implementação.
- **Estados do token estritamente limitados a:** pendente → pago → consumido; e expirado por rotina de limpeza. Não devem existir estados intermediários adicionais no MVP.
- **Ativação atômica:** a operação que cria/atualiza o usuário, vincula a assinatura, marca o token como consumido e invalida o cache de entitlement precisa ser tratada como uma unidade consistente; não pode existir estado parcialmente aplicado visível ao cliente.
- **Integração com pipeline de billing (E2):** o evento de pagamento aprovado é entregue pelo pipeline de billing; este PRD consome esse contrato e não define o comportamento do webhook do provedor.
- **Integração com módulo de identidade (E1):** a normalização do número de WhatsApp em E.164 é feita pelo componente entregue por E1; este PRD não introduz nova lógica de normalização.
- **Acoplamento operacional em runtime:** a ativação só é possível para tokens marcados como pagos, o que depende do pipeline de E2 em produção; em ambientes de pré-produção, o marcador pago pode ser simulado.
- **Conformidade com LGPD:** número de WhatsApp digitado no checkout, número real e e-mail são tratados como PII, devem ser mascarados em logs e devem suportar pedido futuro de exclusão de dados (canal de exercício de direito a ser detalhado fora deste PRD).
- **Suporte geográfico:** Brasil apenas no MVP; números fora do espaço E.164 BR são rejeitados.
- **Provedor de pagamento único no MVP:** Kiwify. Outros provedores estão fora do escopo deste PRD.
- **Template do WhatsApp Business:** o outreach automatizado depende de template oficial pré-aprovado pela Meta; a aprovação é pré-requisito operacional para habilitar o job em produção.
- **Página de agradecimento sob domínio do MeControla:** a página deve estar sob domínio controlado pelo MeControla e não pode ser a página padrão do provedor de pagamento.
- **Metas de performance mínimas em alto nível:** o endpoint de criação de sessão de checkout deve operar dentro de uma faixa que não comprometa a percepção de instantaneidade no clique do botão; metas numéricas finais ficam na techspec.
- **Acessibilidade da thank-you page:** nível mínimo WCAG 2.1 AA nos elementos críticos do caminho feliz (RF-19); níveis AAA ou conformidade integral com WCAG 2.2 ficam fora do escopo do MVP.
- **Tolerância a pagamento sem token (S-01 mitigation):** o fluxo deve operar com a premissa de que, ainda que a propagação de `?s={token}` pela Kiwify seja a expectativa, o sistema **nunca pode descartar silenciosamente** um pagamento aprovado por falta de token — o caminho de fila para suporte (RF-18) é tratamento mínimo obrigatório.

## Fora de Escopo

- Implementação do webhook de billing do provedor de pagamento, validação de assinatura, e máquina de estados de assinatura — pertencem a **E2 `billing-pipeline`** (PRD em `.specs/prd-billing-pipeline/prd.md`).
- Implementação do serviço de entitlement com cache, regras de carência e decisão de acesso — pertencem a **E2**; este PRD consome apenas o gancho de invalidação.
- Implementação do agregado `User`, repositório de usuário, normalização de número de WhatsApp e contratos de identidade — pertencem a **E1 `identity-foundation`** (PRD em `.specs/prd-identity-foundation/prd.md`); este PRD consome esses contratos.
- Hardening de reconciliação, auditoria avançada, painel de operação, antifraude estatístico, observabilidade ampliada e endurecimento operacional — pertencem a **E4 `reconciliation-hardening`** (backlog pós-MVP); **explicitamente fora do escopo deste PRD**.
- Trial gratuito alternativo (segundo CTA na landing levando direto ao WhatsApp sem checkout) — não previsto no MVP.
- Painel administrativo para suporte reverter ativação, conceder entitlement manual ou ressincronizar conta — não previsto no MVP; suporte segue por canal humano.
- Suporte multi-país no normalizador de telefone — Brasil apenas no MVP.
- Detecção avançada de fraude (padrões estatísticos, score de risco, bloqueio dinâmico) — apenas o sinal básico de "token já usado por outro número" é tratado no MVP.
- Múltiplos provedores de pagamento ativos em paralelo — apenas Kiwify no MVP.
- Customização da página de agradecimento por plano, por afiliado ou por origem de tráfego.
- Personalização de mensagens do bot por persona, idioma ou segmento.
- Métricas de negócio agregadas em dashboard pronto (BI) — neste PRD, a entrega é a emissão das métricas; dashboards são tema operacional separado.
- Mecanismos de "concordo com termos" embutidos no fluxo de ativação além do que já é coletado no checkout do provedor.
- Integração com e-mail transacional como canal de ativação — onboarding é exclusivamente via WhatsApp.
- Reembolso ou cancelamento automático de assinatura para tokens expirados sem ativação — tratado por suporte humano a partir do sinal de RF-12; automação fica para E4 se justificável.
- Segunda tentativa (ou mais) de outreach por token — MVP é cap rígido de **1 envio único por token**; reavaliação fica para revisão pós-baseline (S-11).
- Canal dedicado de alerta (Slack, e-mail, paging) para tentativas de reuso de token consumido por outro número — sinalização passiva via métrica e log apenas (RF-15); canal ativo fica para E4 se justificável.
- Bloqueio dinâmico, quarentena de número, scoring de risco ou qualquer outra forma de antifraude estatística — fora do MVP.
- Idempotência do endpoint de criação de sessão de checkout via `Idempotency-Key`, deduplicação por IP+UA ou qualquer outro coalescing — fora do MVP (rate limit de RF-02 absorve abuso; tokens PENDING órfãos são tratados por RF-11).
- Diferenciação pública de estados internos do token na thank-you page (oracle de existência/estado) — proibida por RF-17 como defesa em profundidade.
- Conformidade WCAG 2.1 AAA, WCAG 2.2 integral, auditoria de acessibilidade externa ou suporte a leitor de tela em fluxos não-críticos da thank-you page — fora do MVP.
- Reprocessamento automático ou retry programado de webhook `order_approved` sem token (RF-18 trata por fila para suporte humano; nenhuma automação de "reenvio" é parte do MVP).

## Suposições e Questões em Aberto

- **S-01 — Propagação de `?s={token}` pela Kiwify (NÃO CONFIRMADA no working tree):** o fluxo assume que a Kiwify propaga o parâmetro `s` (ou equivalente) do checkout para o webhook `order_approved` e para a URL de redirect pós-pagamento. Esta propagação ainda **não está comprovada no workspace** e é registrada como **hipótese aberta**. Caminhos alternativos a considerar fora deste PRD (techspec ou validação operacional): custom field oculto preenchido via query string, propagação por UTM, ou repasse pelo endpoint do provedor. **Risco:** se a propagação não funcionar como esperado, o webhook não consegue ligar a compra ao token e o caminho feliz inteiro do MVP precisa ser revisto.
- **S-02 — Hospedagem da página de agradecimento:** o épico aponta que a decisão entre hospedar a página de agradecimento no repositório da landing ou em domínio da API fica para a techspec. Este PRD não fixa a localização, apenas exige que esteja sob domínio controlado pelo MeControla e não seja a página padrão do provedor.
- **S-03 — Número oficial do bot no WhatsApp Business:** o link `wa.me/<numero_bot>` exige número oficial provisionado. A obtenção e configuração desse número é pré-requisito operacional e não está coberta pela entrega de software deste PRD.
- **S-04 — Template `activation_reminder` aprovado pela Meta:** o outreach automático depende de template oficial pré-aprovado. A aprovação pode levar dias úteis e é dependência externa fora do controle do time de engenharia; precisa ser iniciada em paralelo. Se a aprovação atrasar, o job de outreach existe mas permanece desabilitado em produção sem bloquear o restante do fluxo.
- **S-05 — Endpoint de criação de sessão de checkout chamado pela landing:** assume-se que a landing (`LimaTeixeiraTecnologia/mecontrola-landingpage`) integrará com o endpoint deste PRD substituindo os placeholders `CHECKOUT_URL_*` por chamada ao backend. A janela de troca depende de coordenação com Marketing e não está coberta pela entrega de software aqui.
- **S-06 — Janela operacional do outreach (2 horas) e cadência horária:** os parâmetros temporais do fallback de outreach (janela e cadência) são pontos de produto que podem ser ajustados em rodadas posteriores com base no funil real; entram no MVP com os valores indicados no épico (`> 2h`, cadência horária).
- **S-07 — Race entre `ATIVAR` e webhook `order_approved` (Pix lento):** assume-se que casos em que o cliente envia `ATIVAR` antes do webhook chegar são minoritários e tratáveis por mensagem amigável ("pagamento ainda processando, tente em 1 minuto"). Se o volume desse caso se mostrar relevante em produção, pode justificar evolução posterior — fora deste PRD.
- **S-08 — Compra duplicada acidental (cliente clica duas vezes):** o tratamento mínimo é ativar a primeira compra; a segunda gera token pago não consumido que entra no fluxo padrão (outreach único + match gated) e, eventualmente, na rotina de expiração com sinal de subscription órfã (RF-12). Tratamento administrativo (reembolso da duplicata) fica fora do escopo deste PRD.
- **S-09 — Divergência discovery vs. épico (resolvida no PRD):** a discovery de onboarding (seção 5, `tryFallbackActivation`) descreve o fallback E.164 como ativação automática a partir de qualquer mensagem do número casado, sem pré-condição de outreach. **Este PRD diverge deliberadamente** e adota a versão mais conservadora (RF-10): ativação por match só ocorre após o outreach já ter sido enviado. Motivo: reduz risco de ativar conta de terceiro cujo número foi digitado por engano no checkout. O épico não fixa essa pré-condição, então não há violação do roadmap; a divergência é refinamento de produto e deve ser refletida na techspec downstream.
- **S-10 — Métricas: baselines e metas:** os objetivos listam métricas a serem acompanhadas, mas não fixam metas numéricas (taxa de conversão alvo, SLO de latência do endpoint) por ausência de baseline em produção. As metas devem ser definidas após a primeira janela de observação real, em revisão de produto.
- **S-11 — Volume de outreach calibrado para 1 envio único por token:** decisão conservadora para preservar reputação da conta WhatsApp Business junto à Meta e evitar percepção de assédio. Caso a taxa de conversão por outreach se mostre baixa em produção e o limite de templates da Meta permita, pode justificar reavaliação posterior do cap (ex: 2 tentativas com intervalo mínimo) — explicitamente **fora do escopo do MVP**.
- **S-12 — Sinal de abuso por canal passivo (métrica + log):** o MVP não dispara push para Slack/e-mail quando token consumido é tentado em outro número (RF-15). Suporte descobre o evento via dashboard/log. Se padrão de fraude crescer e o tempo de detecção se mostrar inaceitável, canal dedicado de alerta pode ser introduzido em E4 (`reconciliation-hardening`) — fora deste PRD.
- **S-13 — Subscription órfã expirada sem cancelamento automático:** este PRD trata o caso "pagou e nunca ativou" emitindo sinal para suporte (RF-12), mas **não cancela** a assinatura no provedor. O suporte humano decide o destino (reembolso, contato, reemissão). Automação de cancelamento ou reembolso fica fora do MVP e, se justificável, pode entrar em E4.
- **S-14 — RF-18 depende de E2 propagar payload bruto com o parâmetro `s` (ou ausência dele):** o sinal "pagamento aprovado sem token" só pode ser emitido se o pipeline de billing (E2) tornar visível o resultado da extração do parâmetro `s` do payload da Kiwify para o handler de E3. **Contrato implícito:** o PRD de E2 (`.specs/prd-billing-pipeline/prd.md`) precisa expor ou repassar essa ausência; se hoje o contrato consome apenas tokens válidos, a fronteira E2↔E3 precisará de revisão na techspec de E3. Risco material rastreado: se E2 fizer `early return` silencioso quando `s` for vazio, RF-18 fica inoperante e o pagamento órfão volta a ser invisível.
- **S-15 — RF-17 e leitura humana de erro genérico na thank-you page:** a opção de mostrar mensagem única ("Link inválido ou expirado") protege contra enumeração mas piora UX para cliente legítimo que clicou em link antigo (não saberá se basta esperar pagamento ou se precisa abrir suporte). Decisão consciente: priorizar defesa contra oracle e empurrar resolução para canal humano. Se volume de tickets "meu link parou de funcionar" se mostrar alto, pode justificar reavaliação pós-baseline — explicitamente fora do escopo do MVP.
- **S-16 — Ausência de Idempotency-Key no endpoint de checkout (RF-01):** assume-se que o volume de tokens PENDING órfãos gerados por clique duplo ou retry de rede será marginal frente ao rate limit (10/min/IP) e absorvido pela rotina de limpeza diária (RF-11). Se métricas de produção mostrarem proporção alta de tokens PENDING que nunca evoluem (ex: > 30% dos criados), pode justificar introdução de Idempotency-Key — fora do MVP.
- **S-17 — WCAG 2.1 AA como teto mínimo no MVP:** aderência integral à WCAG 2.1 AA depende de validação técnica (lighthouse, axe) e revisão de design. Este PRD fixa o nível como obrigatório no MVP, mas reconhece que cobertura formal só pode ser comprovada após auditoria. Em caso de gap identificado na validação, o tratamento deve respeitar prioridade de produto (gap crítico bloqueia release; gap menor entra em rodada posterior).

## Critérios de Sucesso Mensuráveis

- O sistema emite o conjunto de métricas exigido em RF-12 e permite desenhar o funil "sessão criada → pago → consumido", segmentado por caminho de ativação (direto, fallback por número, outreach).
- A maior parte das ativações em janela operacional acontece pelo caminho direto (página de agradecimento → `wa.me` → envio de `ATIVAR`), com os caminhos de fallback servindo como rede de segurança em volume residual. A definição quantitativa de "maior parte" e "residual" será calibrada após o primeiro ciclo de produção (S-10).
- O volume de tickets de suporte do tipo "paguei e não consigo ativar" tende a níveis residuais após estabilização do fluxo.
- O gauge de tokens pagos não consumidos permanece em patamar saudável; picos sustentados indicam regressão da página de agradecimento ou do outreach e devem disparar revisão.
- Reenvios acidentais do mesmo `ATIVAR <token>` pelo mesmo número nunca produzem cobranças adicionais nem mensagens de erro técnico ao cliente.
- Tentativas de uso de um token já consumido a partir de outro número são consistentemente bloqueadas e sinalizadas para análise de suporte (RF-15).
- Tokens pagos não consumidos dentro do TTL geram sinal de subscription órfã expirada que chega à fila de suporte (RF-12), permitindo tratamento humano consistente.
- Pagamentos aprovados sem token associado (RF-18) nunca são silenciosamente descartados; chegam à mesma fila de suporte para vinculação manual, garantindo que cliente que pagou sempre tem caminho de resolução.
- Thank-you page acessada com token inválido/expirado/inexistente exibe mensagem genérica única (RF-17), preservando defesa contra enumeração; métrica segmentada permite diagnóstico operacional sem vazar estado interno publicamente.
- Caminho feliz da thank-you page atende WCAG 2.1 AA (RF-19), validável por ferramentas automatizadas no pipeline.

## Referências

- Épico: `docs/epics/epic-03-onboarding-magic-token.md`
- Discovery primária: `docs/discoveries/discovery-onboarding-flow.md`
- Discoveries relacionadas: `docs/discoveries/discovery-billing-hotmart-kiwify.md`, `docs/discoveries/discovery-identity-entitlement.md`
- Bundle decisório: `.agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/decision-brief.md` (blocos A, E, H)
- PRDs dependentes em runtime/contrato: `.specs/prd-identity-foundation/prd.md` (E1), `.specs/prd-billing-pipeline/prd.md` (E2)
- Governança: `AGENTS.md`, `CLAUDE.md`, `.claude/rules/governance.md`
- Base canônica de PRD: `docs/prompts-base/create-prd-prompt-base.md`
