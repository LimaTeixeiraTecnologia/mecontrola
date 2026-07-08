---
name: user-stories
description: Cria histórias de usuário prontas para backlog a partir de entrada de produto, tickets, PRDs ou evidências da base de código, usando persona, necessidade, benefício, critérios de aceite, checagem de lacunas e perguntas de múltipla escolha. Confronta repositórios disponíveis antes de fechar escopo e rejeita suposições sem suporte. Use quando converter pedidos de funcionalidade em histórias robustas. Não use para criar itens externos, publicar em Jira ou Azure DevOps, escrever código de implementação ou produzir apenas épicos.
---

# Histórias de Usuário

<critical>Produzir histórias de usuário sem lacunas materiais, sem falso positivo de evidência, sem ressalvas, sem marcadores pendentes e sem pressupostos não suportados.</critical>
<critical>Confrontar a base de código quando houver repositório, arquivos, branch, diff ou caminho local disponível.</critical>
<critical>Fazer perguntas de múltipla escolha sempre que persona, objetivo, escopo, regra de negócio, dependência, critério de aceite ou evidência técnica ficarem ambíguos.</critical>
<critical>Não finalizar histórias enquanto `scripts/validar-historias-usuario.py` reprovar a saída.</critical>

## Entrada
- Pedido bruto do usuário: funcionalidade, problema, ticket, PRD, conversa, bug, diff, arquivos locais ou link.
- Contexto opcional: produto, persona, restrições, prazo, destino do backlog, padrão de escrita, granularidade desejada.

## Saída
- Histórias de usuário em Markdown seguindo `assets/modelo-historia-usuario.md`.
- Perguntas de múltipla escolha quando houver incerteza material.
- Evidências da base de código quando um repositório ou arquivo local estiver disponível.
- Lista explícita de itens fora de escopo e dependências verificadas.

## Procedimentos

1. Normalizar a entrada.
   Extrair objetivo de negócio, resultado esperado, personas candidatas, fluxo do usuário, restrições e sinais técnicos do pedido.
   Separar fatos fornecidos, inferências razoáveis e lacunas. Não tratar inferência como fato.
   Se a entrada estiver vazia ou apenas indicar "criar histórias de usuário" sem contexto de produto, perguntar em múltipla escolha pelo tipo de iniciativa, persona primária, resultado esperado e destino da entrega.

2. Carregar regras de qualidade.
   Ler `references/regras-atlassian-historias-usuario.md` para aplicar a estrutura de história, os 3 Cs e os critérios de divisão.
   Ler `references/criterios-qualidade-historia.md` para identificar lacunas materiais e falso positivo de evidência.
   Ler `assets/modelo-historia-usuario.md` antes de redigir qualquer história final.

3. Confrontar com a base de código quando aplicável.
   Detectar se existe base de código no contexto atual por presença de `git`, arquivos locais, caminhos mencionados, diff, tecnologia ou área de trabalho.
   Se houver base de código, ler `references/confronto-base-codigo.md` e executar a investigação mínima antes de redigir conclusões.
   Usar evidências concretas com caminho e linha quando suportarem regras, fluxos, endpoints, modelos, permissões ou dependências.
   Marcar como `Não evidenciado na base de código` apenas quando a busca foi executada e não encontrou suporte.
   Nunca afirmar que a base de código suporta uma história sem evidência textual, estrutural ou comportamental verificável.

4. Formular perguntas de múltipla escolha.
   Ler `references/perguntas-esclarecimento.md` quando qualquer campo obrigatório tiver mais de uma interpretação plausível.
   Fazer no máximo 4 perguntas por rodada, cada uma com 2 a 4 opções mutuamente exclusivas.
   Incluir uma opção recomendada apenas quando houver evidência clara; explicar o impacto de cada opção.
   Repetir rodadas até remover lacunas materiais. Prosseguir sem pergunta apenas quando a decisão puder ser derivada de fatos fornecidos ou evidência da base de código.

5. Projetar o lote de histórias.
   Definir uma história por persona, etapa de fluxo, regra de negócio ou fatia de valor independente.
   Dividir histórias grandes quando não puderem caber em um sprint, quando misturarem personas ou quando exigirem critérios de aceite independentes.
   Rejeitar histórias técnicas puras se não houver benefício de usuário ou negócio; converter para história de habilitação somente com resultado observável.
   Preservar termos de negócio do usuário e nomes reais encontrados na base de código.

6. Redigir cada história de usuário.
   Preencher integralmente `assets/modelo-historia-usuario.md`.
   Escrever a declaração no formato `Como <persona>, quero <capacidade>, para <benefício>`.
   Incluir contexto, regras de negócio, critérios de aceite em Gherkin, dados e permissões quando relevantes.
   Incluir pelo menos um cenário feliz, um alternativo e um de erro, salvo quando a história for comprovadamente somente informacional; nesse caso explicar a ausência no campo de notas de validação.
   Incluir dependências, fora de escopo, riscos e evidências sem usar marcadores pendentes.

7. Validar e corrigir.
   Salvar a resposta proposta em arquivo temporário quando a entrega for local, ou validar o bloco final antes de responder quando a entrega for apenas textual.
   Executar `python3 scripts/validar-historias-usuario.py <arquivo-ou-diretorio>` para saídas locais.
   Se o script falhar, corrigir os pontos indicados no `stderr` e executar novamente.
   Se a entrega for no chat, aplicar manualmente os mesmos critérios de `references/criterios-qualidade-historia.md` antes de responder.

8. Entregar resultado.
   Apresentar as histórias de usuário em ordem de valor ou sequência do fluxo.
   Separar decisões assumidas de decisões confirmadas por entrada ou base de código.
   Se restar lacuna material, não entregar como final; apresentar perguntas de múltipla escolha e aguardar resposta.
   Não publicar em Jira, Azure DevOps, GitHub Issues ou outra ferramenta externa neste skill.

## Estados Finais
- `done`: histórias de usuário completas, validadas e sem lacunas materiais conhecidas.
- `needs_input`: perguntas de múltipla escolha pendentes para resolver ambiguidade material.
- `blocked`: base de código, arquivo ou evidência prometida pelo usuário está inacessível.
- `failed`: validação falhou após correção ou houve erro inesperado de I/O.

## Tratamento de Erros
- Se a base de código estiver indisponível apesar de citada, perguntar se deve continuar apenas com a entrada textual ou aguardar o acesso correto.
- Se a fonte externa ou link não puder ser lido, declarar a limitação e basear a saída apenas no material acessível.
- Se o usuário pedir "0 lacunas" mas recusar perguntas necessárias, encerrar com `needs_input` em vez de inventar informação.
- Se `scripts/validar-historias-usuario.py` reprovar falso positivo de evidência, substituir a afirmação por evidência real ou mover o item para lacuna/pergunta.
