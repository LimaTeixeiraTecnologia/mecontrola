# Confronto Com a Base de Código

Use este procedimento quando houver área de trabalho, repositório, arquivos locais, diff, branch, módulo ou caminho mencionado.

## Investigação Mínima
1. Executar `git status --short` quando o diretório for um repositório Git.
2. Listar arquivos relevantes com `rg --files`.
3. Buscar termos de domínio da entrada com `rg -n "<termo>"`.
4. Identificar tecnologia e pontos de entrada por arquivos manifestos: `package.json`, `pom.xml`, `build.gradle`, `pyproject.toml`, `go.mod`, `Cargo.toml`, `*.csproj`, `Dockerfile`, `docker-compose.yml`.
5. Procurar modelos, rotas, controllers, handlers, services, schemas, migrations, policies e testes relacionados.
6. Ler apenas arquivos necessários para validar ou refutar escopo, regra, fluxo, permissão ou dependência.

## Evidência Aceitável
- Caminho e linha de código que implementa regra, fluxo, endpoint, schema, permissão ou teste.
- Manifesto ou configuração que comprova dependência, arcabouço, serviço externo ou sinalizador de funcionalidade.
- Teste existente que define comportamento esperado.
- Diff fornecido pelo usuário que altera comportamento relevante.

## Evidência Insuficiente
- Nome de arquivo parecido sem conteúdo lido.
- Convenção de arcabouço não confirmada no repositório.
- Suposição baseada apenas em arquitetura comum.
- Comentário antigo sem código ou teste correspondente.

## Registro na História
- `Fonte`: entrada do usuário, arquivo local, ticket, PRD ou base de código.
- `Evidências`: lista de caminhos com linhas quando possível.
- `Não evidenciado`: itens buscados e não encontrados.
- `Perguntas pendentes`: apenas quando a lacuna altera escopo, aceite ou prioridade.
