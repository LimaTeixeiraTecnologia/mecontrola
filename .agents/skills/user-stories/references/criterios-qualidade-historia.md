# Critérios de Qualidade da História

Uma história de usuário só pode ser finalizada quando todos os critérios passarem.

## Critérios Obrigatórios
- A declaração segue `Como <persona>, quero <capacidade>, para <benefício>`.
- Persona, capacidade e benefício são específicos e não vazios.
- Benefício é mensurável ou observável.
- Cada critério de aceite tem condição verificável.
- Pelo menos um critério cobre fluxo feliz.
- Pelo menos um critério cobre fluxo alternativo ou variação válida.
- Pelo menos um critério cobre erro, bloqueio ou permissão negada quando aplicável.
- Dependências são listadas ou marcadas como inexistentes com justificativa.
- Fora de escopo está explícito.
- Evidências distinguem entrada, base de código e inferência.
- Nenhuma afirmação técnica usa a base de código como suporte sem caminho/linha ou explicação da busca.
- Não existem marcadores pendentes como `TBD`, `TODO`, `N/A`, `a definir`, `???` ou campos vazios.

## Lacuna Material
Tratar como lacuna material qualquer ausência que altere:
- Persona primária.
- Valor esperado.
- Regra de negócio.
- Permissão.
- Dados obrigatórios.
- Integração ou fonte de verdade.
- Critério de aceite.
- Limite de escopo.
- Dependência bloqueante.

## Falso Positivo
Tratar como falso positivo quando:
- A história diz que algo existe na base de código sem evidência concreta.
- A aceitação afirma comportamento que não foi informado nem encontrado.
- A dependência é apresentada como disponível sem verificação.
- A ausência de achados vira prova de inexistência sem busca descrita.
