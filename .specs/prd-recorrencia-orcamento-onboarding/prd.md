# Documento de Requisitos do Produto (PRD) — Recorrência do Orçamento por Linguagem Natural no Onboarding

<!-- spec-version: 2 -->

## Visão Geral

Durante o onboarding financeiro pelo WhatsApp, o MeControla pergunta ao usuário se o orçamento recém-montado deve se repetir nos próximos meses. Hoje o step de recorrência entende apenas "sim/não" e, quando positivo, replica sempre por 12 meses fixos — o usuário não consegue escolher por quantos meses quer repetir, não recebe confirmação explícita da decisão no momento em que ela acontece, e respostas fora do padrão (quantidade específica, número inválido ou texto ambíguo) não são tratadas.

Esta funcionalidade permite que o agente compreenda, por linguagem natural, três decisões de recorrência — sem recorrência, recorrência padrão de 12 meses, ou recorrência por uma quantidade específica de 1 a 12 meses (informada numericamente ou por extenso) — aplique exatamente o período resolvido, confirme sempre a decisão ao usuário e trate com clareza valores inválidos e respostas ambíguas, tudo aderente ao Tom de Voz oficial. O valor: o usuário fecha o onboarding com o planejamento replicado exatamente pelo período que decidiu, sem ambiguidade e sem desfechos silenciosos errados.

O escopo é deliberadamente estreito e de baixo risco: a capacidade de replicar o orçamento por 1 a 12 meses já existe ponta a ponta no módulo de domínio de orçamentos; o gap está isolado no step de recorrência do onboarding. Esta é uma mudança aditiva, com zero regressão exigida nos comportamentos hoje suportados (positivo-padrão e negativo).

Fonte de origem: `docs/us/2026-07-15-us-recorrencia-do-orcamento.md` (US única aprovada) e o documento de produto `US_Recorrencia_do_Orcamento_MeControla.md`.

## Objetivos

- **Compreensão em linguagem natural**: o agente interpreta corretamente as três decisões de recorrência (negativa, positiva-padrão de 12 meses, quantidade específica de 1 a 12) sem exigir comandos ou frases fixas.
- **Acurácia mensurável**: gate de validação por casos golden com LLM real atingindo ≥ 0,90 de aderência nas categorias que cobrem os três cenários mais valor inválido e resposta ambígua, com 0 falso-sucesso (nenhum caso que aplica recorrência errada seja pontuado como sucesso).
- **Confirmação universal**: 100% das decisões aplicadas (sem recorrência, 12 meses, ou N meses) são confirmadas ao usuário — imediatamente no momento da decisão e refletidas no resumo final com o período real.
- **Zero recorrência indevida**: 0 recorrência aplicada quando a intenção for negativa, quando a quantidade estiver fora de 1–12, ou quando a resposta for ininteligível.
- **Prioridade correta**: em 100% dos casos com quantidade válida informada, o período específico prevalece sobre o padrão de 12 meses.
- **Zero regressão**: os fluxos hoje suportados (positivo-padrão de 12 meses e negativo) preservam comportamento; posição do step, durabilidade suspend/resume e o contrato de domínio de recorrência 1–12 permanecem inalterados.
- **Zero interrupção em andamento**: onboardings suspensos no step de recorrência no momento da liberação retomam com a nova lógica sem drenagem, migração ou reinício.
- **Observabilidade de produto**: 100% das saídas do step são contabilizadas em um contador de outcome com cardinalidade controlada, permitindo acompanhar a distribuição das respostas de recorrência.

## Histórias de Usuário

- História primária (aprovada): `docs/us/2026-07-15-us-recorrencia-do-orcamento.md` — "Como novo assinante do MeControla que está concluindo o onboarding financeiro pelo WhatsApp, quero responder em linguagem natural se e por quantos meses (entre 1 e 12) meu orçamento deve se repetir, para deixar meu planejamento replicado exatamente pelo período que eu decidir, com confirmação clara da decisão aplicada."
- Persona primária: novo assinante em onboarding pelo WhatsApp, respondendo à pergunta de recorrência após a ativação do orçamento.
- Fluxos cobertos: recusa da recorrência; aceite sem quantidade (12 meses); quantidade específica numérica (ex.: "coloca só pra 3 meses"); quantidade por extenso em linguagem natural (ex.: "manter por oito meses"); quantidade fora do intervalo (ex.: "24 meses"); resposta ambígua (ex.: "talvez"); reperguntas sucessivas até intenção válida.

## Funcionalidades Core

- **Interpretação de intenção e quantidade por linguagem natural**: o agente classifica a resposta do usuário em intenção (negativa, positiva) e, quando houver, extrai a quantidade de meses, convertendo números por extenso para valor numérico. Importa porque elimina a exigência de comandos fixos e captura a real intenção do usuário. Em alto nível, reaproveita o mecanismo de saída estruturada (Structured Output) já usado nos demais steps do onboarding.
- **Resolução determinística da decisão de recorrência**: dada a intenção e a quantidade extraídas, uma decisão pura resolve o período final (sem recorrência / 12 meses / N meses) aplicando a regra de prioridade. Importa porque concentra a regra de negócio num ponto puro, testável e sem efeitos colaterais.
- **Aplicação e confirmação**: o agente aplica a recorrência pelo período resolvido e confirma a decisão ao usuário imediatamente e no resumo final. Importa porque garante transparência e evita desfechos silenciosos.
- **Tratamento de inválido e ambíguo com repergunta**: quantidades fora de 1–12 e respostas ininteligíveis não aplicam recorrência; o agente repergunta no Tom de Voz até obter intenção válida. Importa porque protege o usuário de recorrência errada ou negada por engano.
- **Observabilidade de outcome**: cada saída do step é registrada num contador com rótulo de outcome fechado. Importa porque dá visibilidade de produto sobre como os usuários respondem.

## Requisitos Funcionais

- RF-01: O agente DEVE interpretar a resposta do usuário no step de recorrência por linguagem natural, sem exigir comandos ou frases fixas, resolvendo uma entre três decisões: sem recorrência, recorrência de 12 meses, ou recorrência por quantidade específica de 1 a 12 meses.
- RF-02: Quando a intenção for negativa (ex.: "não", "num quero", "só esse mês", "deixa só esse mês", "ñ", "n"), o agente NÃO DEVE criar recorrência e DEVE manter o orçamento apenas na competência atual.
- RF-03: Quando a intenção for positiva sem quantidade informada (ex.: "sim", "quero", "pode repetir", "pode ser", "repete"), o agente DEVE aplicar recorrência por 12 meses.
- RF-04: Quando o usuário informar uma quantidade inteira entre 1 e 12, numérica (ex.: "6 meses", "só 3", "coloca por 6") ou por extenso (ex.: "seis meses"), inclusive embutida em frase de linguagem natural (ex.: "sim, mas coloca só pra 3 meses", "manter por oito meses"), o agente DEVE aplicar recorrência por exatamente essa quantidade de meses.
- RF-05: O agente DEVE converter quantidades escritas por extenso (um…doze) para o valor numérico correspondente antes de aplicar a decisão.
- RF-06: A prioridade de interpretação DEVE ser: (1) quantidade específica válida entre 1 e 12 identificada → aplicar a quantidade informada; (2) intenção positiva sem quantidade → aplicar 12 meses; (3) intenção negativa → não aplicar. Uma quantidade válida informada SEMPRE prevalece sobre o padrão de 12 meses.
- RF-07: Quando a resposta contiver uma quantidade fora do intervalo 1–12 (ex.: "0 meses", "13 meses", "24 meses"), o agente NÃO DEVE aplicar recorrência e DEVE reperguntar informando, no Tom de Voz oficial, que é possível repetir por um período entre 1 e 12 meses, solicitando uma quantidade válida.
- RF-08: Quando a resposta não for claramente positiva, negativa nem uma quantidade utilizável (ex.: "talvez", "sei lá", emoji isolado, texto sem intenção reconhecível), o agente DEVE reperguntar a questão de recorrência no Tom de Voz oficial, NÃO DEVE assumir intenção negativa nem positiva e NÃO DEVE aplicar recorrência.
- RF-09: Para valores inválidos (RF-07) e respostas ambíguas (RF-08), o agente DEVE reperguntar até obter intenção válida, sem introduzir limite máximo de tentativas; o abandono do onboarding permanece coberto pelo mecanismo existente de expiração de execuções suspensas obsoletas.
- RF-10: O agente DEVE confirmar a decisão aplicada ao usuário imediatamente no momento da decisão (sem recorrência, 12 meses, ou N meses), de forma verificável, encadeada no prompt do step seguinte.
- RF-11: O resumo final do onboarding DEVE refletir o período real aplicado (N meses específicos ou "sem recorrência"), substituindo o texto fixo atual de 12 meses.
- RF-12: O agente DEVE aplicar a decisão de recorrência por meio do contrato existente de criação de recorrência do módulo de orçamentos, passando o número de meses resolvido; o contrato de domínio (validação e materialização de 1 a 12 meses) NÃO DEVE ser alterado por esta funcionalidade.
- RF-13: O estado do onboarding DEVE passar a representar a quantidade de meses da recorrência (além do indicador atual de sim/não), representando a ausência de recorrência de forma explícita.
- RF-14: A pergunta de recorrência apresentada ao usuário DEVE sinalizar, no Tom de Voz oficial, que ele pode escolher uma quantidade específica de 1 a 12 meses, além de aceitar (12 meses) ou recusar, tornando a capacidade descobrível.
- RF-15: Todas as mensagens deste step (pergunta, reperguntas de inválido/ambíguo e confirmações) DEVEM seguir o Tom de Voz oficial do MeControla, verificável pelos scorers oficiais de aderência (asterisco simples em vez de negrito duplo, presença de emoji oficial e avaliação de tom).
- RF-16: Cada saída do step de recorrência DEVE ser registrada num contador de outcome (`agents_onboarding_recurrence_total`) com rótulo `outcome` fechado (ex.: sem recorrência, padrão de 12 meses, quantidade específica, repergunta por inválido, repergunta por ambíguo), com cardinalidade controlada — proibido usar a quantidade de meses ou identificador de usuário como rótulo de métrica.
- RF-17: A funcionalidade DEVE ser validada por (a) casos golden executados com LLM real cobrindo os três cenários mais valor inválido e resposta ambígua, com gate ≥ 0,90 de aderência e 0 falso-sucesso; e (b) testes unitários determinísticos da resolução pura da decisão (negativa/12/N/inválido/ambíguo) e do comportamento do step.
- RF-18: Os fluxos hoje suportados — positivo-padrão de 12 meses e negativo — DEVEM preservar o comportamento observável; a posição do step na sequência do onboarding e a semântica de execução durável (suspend/resume) DEVEM ser mantidas.
- RF-19: A funcionalidade DEVE ser liberada diretamente, sem feature flag em runtime; o gate de qualidade que autoriza a liberação é o conjunto de validações do RF-17 (casos golden com LLM real ≥ 0,90 e 0 falso-sucesso, mais testes unitários determinísticos) executado antes do merge. Nenhum caminho de código condicional de flag DEVE ser introduzido.
- RF-20: Onboardings já suspensos no step de recorrência no momento da liberação DEVEM retomar de forma transparente com a nova lógica, sem drenagem, migração ou reinício forçado. A adição do campo de quantidade de meses ao estado DEVE ser retrocompatível: em snapshots anteriores, a ausência do campo resolve para o valor padrão (decisão de recorrência ainda a tomar), preservando o indicador booleano existente. Zero interrupção para usuários em andamento.

## Experiência do Usuário

- Persona: novo assinante em onboarding pelo WhatsApp, imediatamente após a ativação do orçamento.
- Fluxo principal: o agente pergunta sobre repetir o orçamento, sinalizando que o usuário pode aceitar (12 meses), recusar, ou escolher de 1 a 12 meses. O usuário responde livremente. O agente confirma a decisão e segue para o step de cartões.
- Variações: quantidade específica (ex.: "3 meses") aplica exatamente o período e confirma "3 meses"; recusa confirma que não repetirá; aceite simples confirma 12 meses.
- Erros/bloqueios: quantidade fora de 1–12 ou resposta ambígua geram repergunta clara, no Tom de Voz, sem aplicar recorrência, até o usuário informar uma resposta válida.
- Consistência de tom: todas as mensagens seguem o Tom de Voz oficial (leve, direto, motivacional, com emoji oficial e ênfase por asterisco simples), coerente com os demais steps do onboarding.

## Restrições Técnicas de Alto Nível

- **Contrato de domínio imutável**: a validação e materialização de recorrência por 1 a 12 meses no módulo de orçamentos são reaproveitadas como estão; esta funcionalidade não altera comandos, usecases ou validações desse módulo (defesa em profundidade preservada).
- **Fronteira do agente (governança R-AGENT-WF-001)**: o comportamento entra como extensão do step do workflow de onboarding; o step permanece fino e delega a aplicação ao binding/usecase existente, sem SQL, sem regra de negócio duplicada e sem branching de domínio no adapter. A regra de negócio de resolução do período vive numa decisão pura.
- **Estados como tipos fechados (DMMF state-as-type)**: a intenção/decisão de recorrência e o outcome de métrica DEVEM ser representados por tipos fechados enumerados, nunca por string livre em assinatura pública.
- **LLM apenas nas call-sites sancionadas**: a interpretação de linguagem natural ocorre via saída estruturada do agente (provider único OpenRouter), coerente com os demais steps; nenhum LLM no kernel de workflow nem na decisão pura.
- **Cardinalidade de métricas controlada**: rótulos de métrica restritos a enums fechados; proibido `user_id`, `correlation_key` ou a quantidade de meses como rótulo.
- **Zero comentários em Go de produção** e adaptadores finos, conforme as regras vigentes do repositório.
- **Tom de Voz como contrato verificável**: não existe documento único em prosa de Tom de Voz; a fonte de verdade verificável são os scorers oficiais de aderência, usados como gate de conformidade.

## Fora de Escopo

- Alterar, listar ou excluir a recorrência após a conclusão do onboarding (fluxos conversacionais do dia a dia permanecem inalterados).
- Recorrência por período superior a 12 meses, período parcial, ou recorrência por categoria — mantém-se a replicação do orçamento inteiro dentro do limite 1–12.
- Qualquer alteração no contrato de domínio do módulo de orçamentos (comando, usecase, validação de meses).
- Canais fora do onboarding pelo WhatsApp e qualquer redesenho da ordem dos steps do onboarding.
- Migração das mensagens do step para um catálogo centralizado — as mensagens permanecem determinísticas no próprio workflow, consistentes com o padrão atual do onboarding.

## Suposições e Questões em Aberto

Não há questões em aberto. As decisões de produto que poderiam divergir foram resolvidas explicitamente com o solicitante:

- Confirmação da decisão: imediata (encadeada no prompt do step seguinte) e refletida no resumo final com o período real. (Confirmado)
- Resposta ambígua/ininteligível: reperguntar, sem assumir intenção negativa nem positiva. (Confirmado)
- Limite de reperguntas para inválido/ambíguo: sem limite até intenção válida; abandono coberto pelo mecanismo de expiração existente. (Confirmado)
- Estado do onboarding passa a guardar a quantidade de meses e o resumo reflete o período real. (Confirmado, no escopo)
- Gate de sucesso: casos golden com LLM real ≥ 0,90 e 0 falso-sucesso, mais testes unitários determinísticos e contador de outcome. (Confirmado)
- Observabilidade: contador `agents_onboarding_recurrence_total` com rótulo `outcome` fechado e cardinalidade controlada. (Confirmado)
- Descoberta da opção de N meses: a pergunta sinaliza a possibilidade de escolher de 1 a 12 meses. (Confirmado)
- Rollout: liberação direta, sem feature flag em runtime; gate de qualidade do RF-17 antes do merge. (Confirmado)
- Onboardings em andamento no deploy: retomada transparente com a nova lógica, sem drenagem/migração; adição de campo de estado retrocompatível. (Confirmado)

Suposição declarada (não bloqueante): a interpretação por linguagem natural será feita via saída estruturada do agente, coerente com os demais steps do onboarding que já convertem números por extenso; a definição do schema e do formato exato das mensagens é responsabilidade da Especificação Técnica.
