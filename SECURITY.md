# Security Policy

## Disclosure Channel

Para reportar vulnerabilidades de segurança no MeControla, envie um e-mail para:

**security@limateixeiratecnologia.com.br**

**Não abra issues públicas no GitHub para relatar vulnerabilidades de segurança.**

Utilize o campo de assunto: `[SECURITY] <titulo breve da vulnerabilidade>`

## SLA de Resposta

| Etapa | Prazo |
|-------|-------|
| Confirmação de recebimento | 48 horas |
| Triagem e avaliação inicial | 7 dias corridos |
| Notificação sobre plano de correção | 14 dias corridos |
| Publicação de CVE (quando aplicável) | Após correção implantada |

## Escopo

Estão **dentro** do escopo desta política:

- Backend API (`github.com/LimaTeixeiraTecnologia/mecontrola`)
- Pipelines de CI/CD (`.github/workflows/`)
- Imagem Docker publicada em `ghcr.io/limateixeiratecnologia/mecontrola`
- Dependências diretas listadas em `go.mod`
- Infraestrutura Fly.io do projeto (app `mecontrola`, região `gru`)

Estão **fora** do escopo:

- Serviços de terceiros (Fly.io, GitHub, Grafana Cloud)
- Repositórios não listados acima
- Ataques de engenharia social
- Ataques de negação de serviço (DoS/DDoS)

## Safe Harbor

Pesquisadores de segurança que reportarem vulnerabilidades de boa-fé, seguindo esta política, **não serão responsabilizados** por violações legais decorrentes da pesquisa. Comprometemo-nos a:

- Não tomar ações legais contra pesquisadores que agirem de acordo com esta política
- Reconhecer publicamente a contribuição (com permissão do pesquisador)
- Trabalhar colaborativamente para resolver o problema reportado

Esta política segue os princípios do [safe harbor](https://cheatsheetseries.owasp.org/cheatsheets/Vulnerability_Disclosure_Cheat_Sheet.html) da OWASP.

---

*Esta política está em conformidade com ADR-013 (SECURITY.md + gitsign commits).*
