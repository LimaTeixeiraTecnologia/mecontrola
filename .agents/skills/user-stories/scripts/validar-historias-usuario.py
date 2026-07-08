#!/usr/bin/env python3
import argparse
import re
import sys
from pathlib import Path

SECOES_OBRIGATORIAS = [
    "## Declaração",
    "## Contexto",
    "## Regras de Negócio",
    "## Critérios de Aceite",
    "## Dados e Permissões",
    "## Dependências",
    "## Fora de Escopo",
    "## Evidências",
    "## Notas de Validação",
]

TERMOS_PROIBIDOS = [
    "TBD",
    "TODO",
    "???",
    "<persona",
    "<capacidade",
    "<benefício",
    "<titulo",
    "<título",
    "<numero",
    "<número",
    "a definir",
    "N/A",
]


def iterar_arquivos_markdown(alvo: Path):
    if alvo.is_file():
        yield alvo
        return
    for caminho in sorted(alvo.rglob("*.md")):
        if caminho.name.upper() not in {"README.MD", "CHANGELOG.MD"}:
            yield caminho


def secao(texto: str, titulo: str) -> str:
    inicio = texto.find(titulo)
    if inicio == -1:
        return ""
    inicio += len(titulo)
    proximo_titulo = re.search(r"^##\s+", texto[inicio:], flags=re.MULTILINE)
    fim = inicio + proximo_titulo.start() if proximo_titulo else len(texto)
    return texto[inicio:fim].strip()


def validar_arquivo(caminho: Path) -> list[str]:
    texto = caminho.read_text(encoding="utf-8")
    erros = []

    for titulo in SECOES_OBRIGATORIAS:
        if titulo not in texto:
            erros.append(f"{caminho}: seção obrigatória ausente: {titulo}")

    for termo in TERMOS_PROIBIDOS:
        if termo.lower() in texto.lower():
            erros.append(f"{caminho}: marcador pendente ou termo não resolvido: {termo}")

    declaracao = secao(texto, "## Declaração")
    if not re.search(r"Como\s+.+,\s+quero\s+.+,\s+para\s+.+\.", declaracao, re.IGNORECASE | re.DOTALL):
        erros.append(f"{caminho}: declaração deve seguir 'Como <persona>, quero <capacidade>, para <benefício>.'")

    criterios = secao(texto, "## Critérios de Aceite")
    quantidade_cenarios = len(re.findall(r"\bCen[aá]rio:", criterios, re.IGNORECASE))
    if quantidade_cenarios < 2:
        erros.append(f"{caminho}: critérios de aceite devem conter pelo menos dois cenários Gherkin")
    for palavra_chave in ["Dado", "Quando", "Então"]:
        if not re.search(rf"\b{palavra_chave}\b", criterios, re.IGNORECASE):
            erros.append(f"{caminho}: critérios de aceite sem '{palavra_chave}'")

    evidencias = secao(texto, "## Evidências")
    if not any(rotulo in evidencias for rotulo in ["Entrada:", "Base de código:", "Inferências:", "Não evidenciado:"]):
        erros.append(f"{caminho}: seção de evidências deve classificar Entrada, Base de código, Inferências ou Não evidenciado")

    for titulo in SECOES_OBRIGATORIAS:
        corpo = secao(texto, titulo)
        if titulo in texto and not corpo:
            erros.append(f"{caminho}: seção vazia: {titulo}")

    return erros


def main() -> int:
    parser = argparse.ArgumentParser(description="Valida arquivos Markdown de histórias de usuário geradas.")
    parser.add_argument("alvo", help="Arquivo Markdown ou diretório contendo histórias de usuário")
    args = parser.parse_args()

    alvo = Path(args.alvo)
    if not alvo.exists():
        print(f"ERRO: alvo não existe: {alvo}", file=sys.stderr)
        return 2

    arquivos = list(iterar_arquivos_markdown(alvo))
    if not arquivos:
        print(f"ERRO: nenhum arquivo Markdown encontrado em {alvo}", file=sys.stderr)
        return 2

    erros = []
    for caminho in arquivos:
        erros.extend(validar_arquivo(caminho))

    if erros:
        print("\n".join(erros), file=sys.stderr)
        return 1

    print(f"SUCESSO: {len(arquivos)} arquivo(s) de história de usuário validado(s).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
