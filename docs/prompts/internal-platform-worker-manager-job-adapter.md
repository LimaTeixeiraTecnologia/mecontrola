# Prompt original

Criar `worker manager` e `job_adapter` em `internal/platform` para reutilizacao entre modulos, com foco em robustez, eficiencia e nivel inegociavel de production-ready. Usar os exemplos fornecidos apenas como base conceitual, sem usar `uber/fx` em hipotese nenhuma. Carregar obrigatoriamente a skill `go-implementation`, seus exemplos e tambem usar os exemplos deste proprio prompt como referencia obrigatoria de desenho, sempre adaptados ao contexto real do repositorio. Todo fluxo deve respeitar `handler -> usecase -> repositories e/ou client http`.

Mandatorio e inegociavel ter `0 comentarios` no codigo produzido.

# Prompt enriquecido

```text
Quero que voce implemente uma capacidade compartilhada em `internal/platform` para orquestracao de workers e jobs reutilizavel entre modulos do monolito, com foco inegociavel em robustez, eficiencia, previsibilidade operacional e readiness de producao.

Tambem e obrigatorio, mandatorio e inegociavel:
1. usar a skill `go-implementation` e seus exemplos como base normativa de implementacao;
2. usar os exemplos deste proprio prompt como referencia obrigatoria de desenho, bootstrap e organizacao, adaptando-os ao estado real do repositorio quando houver divergencia;
3. entregar codigo com `0 comentarios` no resultado final, sem comentarios de linha, bloco, doc comments ou observacoes inline adicionadas pela implementacao.

Antes de qualquer alteracao, carregue obrigatoriamente:
1. `AGENTS.md`
2. `.github/skills/agent-governance/SKILL.md`
3. `.github/skills/go-implementation/SKILL.md`
4. Exemplos e referencias da skill Go diretamente relevantes para esta tarefa:
   - `.github/skills/go-implementation/references/architecture.md`
   - `.github/skills/go-implementation/references/interfaces.md`
   - `.github/skills/go-implementation/references/concurrency.md`
   - `.github/skills/go-implementation/references/observability.md`
   - `.github/skills/go-implementation/references/graceful-lifecycle.md`
   - `.github/skills/go-implementation/references/resilience.md`
   - `.github/skills/go-implementation/references/configuration.md`
   - `.github/skills/go-implementation/references/testing.md`
   - `.github/skills/go-implementation/references/examples-infrastructure.md`
   - `.github/skills/go-implementation/references/examples-domain-flow.md`
   - `.github/skills/go-implementation/references/examples-testing.md`
5. `go.mod` para respeitar a versao declarada do Go no repositorio.

Contexto e restricoes obrigatorias do repositorio:
- O projeto e um monolito modular em Go.
- Capacidades tecnicas reutilizaveis entre modulos devem viver em `internal/platform`.
- `internal/platform` nao pode importar `internal/<modulo>/...`.
- O fluxo de negocio deve ser sempre `handler -> usecase -> repositories e/ou client http`.
- Nao pular camadas. Nao acessar repository diretamente a partir de handler, scheduler, consumer ou worker concreto.
- Se algum job disparar regra de negocio, o gatilho do job deve delegar para um use case da camada `application`.
- Se houver HTTP outbound em fluxos concretos de modulo, usar `internal/platform/httpclient`; nao usar `&http.Client{}` diretamente fora de teste.
- E proibido usar `uber/fx`, `fx.Lifecycle`, `dig` ou qualquer abordagem equivalente de DI/framework runtime. Use composicao explicita com construtores, interfaces estritamente necessarias e bootstrap claro.
- Nao criar pacote global de clock em `internal/platform`; tempo deve entrar por dependencia local quando necessario.
- Nao implementar nada em modulo de negocio dentro de `internal/platform`; ali devem existir apenas abstracoes e coordenacao tecnica compartilhada.

Objetivo:
Criar uma base compartilhada em `internal/platform` para:
- `worker manager`: coordenar ciclo de vida de workers de longa duracao e jobs agendados;
- `job adapter`: adaptar uma funcao de execucao para a interface padrao de job;
- permitir reutilizacao real entre modulos, sem acoplamento com regras de negocio;
- oferecer startup e shutdown graciosos, observabilidade, controle de concorrencia e semantica segura de cancelamento.

Use os exemplos fornecidos apenas como inspiracao de comportamento e preocupacoes operacionais, nao como blueprint literal. O desenho final deve seguir a arquitetura e as regras deste repositorio, e precisa remover qualquer dependencia de `uber/fx`.

Diretrizes de desenho obrigatorias:
1. Coloque a capacidade compartilhada em `internal/platform/<nome-adequado>/...`, com naming coerente ao repositorio.
2. Modele interfaces pequenas e orientadas ao consumidor, por exemplo para `Worker` e `Job`, apenas se houver necessidade real.
3. O manager deve coordenar:
   - registro e inicializacao de workers;
   - registro e inicializacao de jobs cron;
   - cancelamento cooperativo via `context.Context`;
   - timeout de shutdown;
   - espera coordenada de encerramento;
   - protecao contra goroutine leak;
   - propagacao explicita de erro, sem swallow silencioso;
   - logging estruturado com `log/slog`;
   - pontos de extensao para metricas/tracing quando fizer sentido.
4. O manager nao pode conhecer repositories, casos de uso concretos, handlers HTTP nem detalhes de modulos.
5. O `job adapter` deve ser generico, pequeno e reutilizavel, sem acoplamento com configs de modulo. Adaptacoes concretas por modulo devem ficar no proprio modulo, fora de `internal/platform`.
6. Se usar cron, definir explicitamente estrategia para:
   - timezone;
   - politica de overlap (`skip if still running`, serializacao ou alternativa equivalente);
   - comportamento em shutdown enquanto job estiver em andamento.
7. A API do componente deve ser simples de integrar no bootstrap da aplicacao sem framework de DI.
8. Toda decisao de concorrencia deve ser justificada pela robustez operacional e validada contra data race, vazamento de goroutine e shutdown incompleto.
9. Os exemplos da skill `go-implementation` e os exemplos deste prompt devem ser seguidos obrigatoriamente como referencia de desenho, sempre com adaptacao ao contexto real do repositorio quando houver conflito objetivo.
10. O codigo final deve ter `0 comentarios`; nao adicionar comentarios de qualquer tipo.
11. Evite overengineering. Entregue o menor desenho que seja realmente production-ready.

Regras arquiteturais inegociaveis:
1. `internal/platform` fornece infraestrutura compartilhada; modulos plugam implementacoes concretas.
2. Fluxos concretos devem continuar obedecendo:
   - handler/consumer/job-trigger do modulo -> usecase -> repositories e/ou client http
3. O scheduler/worker manager pode orquestrar execucao, mas nao executar regra de negocio diretamente.
4. Jobs concretos de modulo devem invocar use cases de aplicacao; nao podem falar direto com banco, provider HTTP ou detalhes de persistencia.
5. Se precisar de client HTTP concreto em um modulo, ele deve ficar em `internal/<modulo>/infrastructure/http/client`.

Requisitos funcionais:
1. Definir contrato compartilhado para workers de longa duracao.
2. Definir contrato compartilhado para jobs agendados.
3. Implementar um manager compartilhado que suba e pare ambos com seguranca.
4. Implementar um adapter reutilizavel para transformar `name + schedule + func(ctx)` em `Job`.
5. Permitir que modulos registrem jobs/workers concretos sem acoplamento com plataforma.

Requisitos nao funcionais obrigatorios:
1. Production-ready de verdade: sem leak de goroutine, sem data race obvia, sem shutdown abrupto, sem dependencia escondida.
2. Observabilidade: logging estruturado e pontos claros para metricas/tracing.
3. Resiliencia: cancelamento, timeout, politicas explicitas de concorrencia e tratamento de erro.
4. Testabilidade: testes unitarios e, se necessario, testes adicionais focados no lifecycle/concurrency.
5. Clareza operacional: erro e comportamento de startup/shutdown devem ser diagnosticaveis.

Proibicoes explicitas:
- Nao usar `uber/fx`.
- Nao copiar literalmente os exemplos.
- Nao colocar regra de negocio em `internal/platform`.
- Nao acessar repository diretamente a partir do manager.
- Nao depender de import ciclico ou de tipos concretos de modulo dentro da plataforma.
- Nao adicionar abstrações sem necessidade real.
- Nao esconder erro com fallback silencioso.
- Nao ignorar os exemplos da skill `go-implementation` nem os exemplos deste prompt.
- Nao deixar comentarios no codigo final sob nenhuma forma.

Criterios de aceitacao:
1. Existe uma implementacao compartilhada em `internal/platform` para manager e adapter reutilizavel entre modulos.
2. A solucao nao importa `go.uber.org/fx` nem bibliotecas equivalentes.
3. O desenho preserva as fronteiras `handler -> usecase -> repositories e/ou client http`.
4. `internal/platform` continua desacoplado de qualquer modulo de negocio.
5. Startup e shutdown sao graciosos, com cancelamento e timeout explicitos.
6. Ha protecao contra execucao concorrente indevida de jobs, ou a politica esta explicitamente definida.
7. Logging e tratamento de erro seguem as convencoes do repositorio.
8. O codigo final entregue possui `0 comentarios`.
9. A implementacao segue obrigatoriamente a skill `go-implementation`, seus exemplos e os exemplos deste prompt, com adaptacao ao contexto real quando necessario.
10. A implementacao e acompanhada de testes relevantes para lifecycle, cancelamento, concorrencia e adapter.
11. A resposta final explica:
   - desenho escolhido;
   - arquivos alterados;
   - como a arquitetura foi preservada;
   - quais trade-offs foram assumidos.

Saida esperada:
1. Analise curta do desenho antes de codar, apontando onde cada responsabilidade ficara.
2. Implementacao completa.
3. Testes cobrindo os pontos criticos.
4. Resumo final objetivo, em PT-BR, com foco em arquitetura, robustez operacional e aderencia as regras.

Exemplos de referencia conceitual a preservar, sem copiar:
- Um manager que coordena workers de longa duracao e jobs cron com startup/shutdown ordenados.
- Um adapter enxuto que transforma uma funcao em implementacao de `Job`.
- Um bootstrap sem framework magico, com dependencias explicitas.

Importante:
- Se voce perceber que alguma parte do exemplo conflita com `AGENTS.md` ou com a skill `go-implementation`, prevalecem `AGENTS.md` e a restricao mais segura.
- Se houver mais de uma abordagem valida, escolha a que tiver menos indirecao, menos acoplamento e menor custo de teste, mantendo nivel production-ready.
```

# Melhorias aplicadas

- Tornou explicita a carga obrigatoria de `AGENTS.md`, `agent-governance`, `go-implementation` e dos exemplos/referencias Go realmente relevantes.
- Converteu o objetivo em escopo implementavel, com fronteiras arquiteturais e proibicoes claras.
- Transformou "production-ready/proof" em criterios mensuraveis: lifecycle, cancelamento, observabilidade, resiliencia, testes e desacoplamento.
- Amarrou o desenho ao contrato do repositorio: `internal/platform` compartilhado e fluxo `handler -> usecase -> repositories e/ou client http`.
- Removeu ambiguidade do exemplo original deixando explicito que `uber/fx` e proibido e serve apenas como inspiracao conceitual.
- Tornou explicito que o uso da skill `go-implementation`, de seus exemplos e dos exemplos do proprio prompt e obrigatorio e inegociavel.
- Adicionou a exigencia objetiva de `0 comentarios` no codigo final.
