# Registro de Decisão Arquitetural (ADR-003)

## Metadados

- **Título:** Formatação de valores e mapeamento slug→nome declarativos no prompt (núcleo-puro único), não presenter Go
- **Data:** 2026-07-07
- **Status:** Aceita
- **Decisores:** Autor da techspec, dono do módulo `internal/agents`
- **Relacionados:** PRD spec-version 3 (RF-18..RF-22, RF-36, RF-19, D-02), techspec desta pasta, `.agents/skills/mastra/`, DMMF (núcleo-puro / casca-IO)

## Contexto

RF-36 pede que a conversão centavos→reais seja uma "função pura reutilizável (helper de
presenter/mapper)". Contudo, na arquitetura mastra do `mecontrola`, as tools de leitura retornam
**dados tipados** (centavos, `rootSlug`) e é o **LLM** que compõe a resposta em linguagem natural para
o WhatsApp. Não há camada Go entre o retorno da tool e o usuário. Inserir um presenter Go exigiria que
as tools devolvessem strings formatadas — violando o princípio de tool fina (retorna dado, não
apresentação) e D-03 (mudar tools de leitura).

## Decisão

Especificar a formatação de valores (centavos→reais, 2 casas, milhar `.`, decimal `,`) e o mapa
`rootSlug`→nome de exibição como **uma regra única e canônica** dentro da const de instruções do
agente. O LLM aplica essa regra de forma determinística em C1–C7. A "pureza" e a reutilização de
RF-36 são satisfeitas por **um só ponto de verdade declarativo** (DRY), não por código Go — coerente
com DMMF (a regra de transformação é pura e centralizada) e com mastra (apresentação fora da tool).

## Alternativas Consideradas

- **Helper Go de formatação chamado pelas tools**: forçaria tools de leitura a formatar strings.
  Rejeitada: quebra tool-fina, viola D-03, e a saída formatada atrapalharia o LLM (que precisa dos
  números para agregações como "quanto ainda tenho disponível").
- **Presenter Go pós-agente**: não existe hook de pós-processamento determinístico da resposta do
  agente; a resposta final é o texto do LLM. Rejeitada: exigiria novo ponto de arquitetura fora de D-03.
- **Formatação implícita (sem regra explícita)**: deixaria o LLM inventar formato. Rejeitada: quebra
  consistência entre C2/C3/C7 e a exigência WhatsApp de `*negrito simples*`.

## Consequências

### Benefícios Esperados

- Consistência garantida por ponto único de verdade; aderência a mastra e D-03.
- Zero código de apresentação; tools permanecem finas e reutilizáveis.

### Trade-offs e Custos

- A formatação passa a depender da fidelidade do LLM à regra — risco de variação de casas/separador.

### Riscos e Mitigações

- **Risco:** LLM formatar `R$ 1234.5` em vez de `R$ 1.234,50`. **Mitigação:** exemplos canônicos no
  prompt (verbatim dos diálogos C1–C7 do PRD) e verificação de formato nos cenários real-LLM/cadeia.

## Plano de Implementação

1. Incluir na const de instruções o bloco de formatação (com exemplo `123450`→`R$ 1.234,50`) e o mapa
   slug→nome.
2. Referenciar essa regra única em todos os cenários de resposta (C1–C7).

## Monitoramento e Validação

- Cenários de cadeia real-LLM conferem que valores exibidos batem com o retorno das tools (sem
  alucinação) e seguem o formato pt-BR.

## Impacto em Documentação e Operação

- Nenhum. Regra vive na instrução do agente.

## Revisão Futura

- Se a variação de formatação do LLM se mostrar material em produção, reconsiderar um pós-processador
  determinístico de saída no runtime (fora do escopo desta feature, exigiria mudança de plataforma).
