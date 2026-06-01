#!/usr/bin/env python3
"""Valida o bundle de brainstorming decisório production-ready.

Uso:
    python3 validate-bundle.py <caminho-do-bundle>

Exit 0: SUCCESS
Exit 1: erros estruturais ou de conteúdo
Exit 2: uso incorreto
"""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

REQUIRED_FILES = {
    "bundle.json",
    "decision-brief.md",
    "transcript.md",
    "assumptions.md",
    "option-scorecard.md",
}

REQUIRED_SECTIONS = [
    "## Problema",
    "## Objetivo",
    "## Escopo Inicial",
    "## Restrições",
    "## Hipóteses",
    "## Alternativas Avaliadas",
    "## Trade-offs",
    "## Riscos",
    "## Custos",
    "## Impactos Operacionais",
    "## Segurança",
    "## Observabilidade",
    "## Escalabilidade",
    "## Alternativa Recomendada",
    "## Justificativa",
    "## Decisões Pendentes",
    "## Próximo Passo Recomendado",
]

MANDATORY_MARKERS = {
    "## Escopo Inicial": ["Inclui:", "Exclui:"],
    "## Riscos": ["Risco:", "Impacto:", "Mitigação:"],
    "## Custos": ["Estimativa relativa:", "Drivers de custo:"],
}

SCORE_COLUMNS = [
    "Complexidade",
    "Tempo de entrega",
    "Custo",
    "Escalabilidade",
    "Segurança",
    "Confiabilidade",
    "Observabilidade",
    "Manutenibilidade",
    "Risco operacional",
]

PLACEHOLDER_EXACT = {
    "TBD",
    "A DEFINIR",
    "A CONFIRMAR",
    "PENDENTE",
    "NÃO DEFINIDO",
    "NAO DEFINIDO",
    "N/A",
    "?",
    "-",
    "...",
}

BULLET_STRIP = re.compile(r"^\s*(?:[-*+]|\d+\.)?\s*")
BRACKET_ONLY = re.compile(r"^\s*(?:[-*+]|\d+\.)?\s*\[(?![ xX]?\s*\]).+\]\s*$")
ALT_HEADING = re.compile(r"(?m)^### Alternativa \d+ - (.+)$")
VALID_STATUS = {"draft", "done", "needs_input", "blocked", "failed"}


def split_sections(text: str) -> dict[str, str]:
    sections: dict[str, str] = {}
    current: str | None = None
    buffer: list[str] = []
    for line in text.splitlines():
        if line.startswith("## "):
            if current is not None:
                sections[current] = "\n".join(buffer).strip()
            current = line.strip()
            buffer = []
        else:
            buffer.append(line)
    if current is not None:
        sections[current] = "\n".join(buffer).strip()
    return sections


def detect_placeholders(body: str) -> list[str]:
    issues: list[str] = []
    for line in body.splitlines():
        if not line.strip():
            continue
        stripped = BULLET_STRIP.sub("", line).strip()
        if stripped.upper() in PLACEHOLDER_EXACT:
            issues.append(f"placeholder proibido: '{line.strip()}'")
        elif BRACKET_ONLY.match(line):
            issues.append(f"bracket-only não preenchido: '{line.strip()}'")
    return issues


def normalize_name(value: str) -> str:
    cleaned = re.sub(r"^Alternativa\s+\d+\s+-\s+", "", value.strip(), flags=re.IGNORECASE)
    return re.sub(r"\s+", " ", cleaned).strip().casefold()


def is_meaningful(value: object) -> bool:
    return isinstance(value, str) and bool(value.strip()) and not detect_placeholders(value)


def validate_bundle_json(path: Path) -> list[str]:
    if not path.is_file():
        return ["bundle.json: arquivo ausente"]

    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        return [f"bundle.json: JSON inválido - {exc}"]

    errors: list[str] = []
    required_top = {
        "version",
        "slug",
        "title",
        "created_at",
        "language",
        "status",
        "decision_brief",
        "transcript",
        "assumptions",
        "option_scorecard",
        "alternatives",
        "recommended_alternative",
        "decisions",
        "readiness",
    }
    missing = required_top - data.keys()
    if missing:
        return [f"bundle.json: campos ausentes - {sorted(missing)}"]

    if data["version"] != 1:
        errors.append(f"bundle.json: versão não suportada - {data['version']}")
    if data["language"] != "pt-BR":
        errors.append("bundle.json: language deve ser 'pt-BR'")
    if data["status"] not in VALID_STATUS:
        errors.append(f"bundle.json: status inválido - '{data['status']}'")
    if not re.match(r"^[a-z0-9]+(?:-[a-z0-9]+)*$", str(data.get("slug", ""))):
        errors.append(f"bundle.json: slug inválido - '{data.get('slug')}'")
    if not str(data.get("title", "")).strip():
        errors.append("bundle.json: title vazio")

    expected_files = {
        "decision_brief": "decision-brief.md",
        "transcript": "transcript.md",
        "assumptions": "assumptions.md",
        "option_scorecard": "option-scorecard.md",
    }
    for key, filename in expected_files.items():
        if data.get(key, {}).get("file") != filename:
            errors.append(f"bundle.json: {key}.file deve ser '{filename}'")

    if not str(data.get("decision_brief", {}).get("title", "")).strip():
        errors.append("bundle.json: decision_brief.title vazio")

    alternatives = data.get("alternatives", [])
    if not isinstance(alternatives, list):
        errors.append("bundle.json: alternatives deve ser lista")
    elif not 3 <= len(alternatives) <= 5:
        errors.append("bundle.json: alternatives deve conter entre 3 e 5 itens")
    else:
        normalized_alternatives = [normalize_name(str(item)) for item in alternatives]
        if any(not item for item in normalized_alternatives):
            errors.append("bundle.json: alternatives contém item vazio")
        if len(set(normalized_alternatives)) != len(normalized_alternatives):
            errors.append("bundle.json: alternatives contém itens duplicados")

    recommended = str(data.get("recommended_alternative", "")).strip()
    if not recommended:
        errors.append("bundle.json: recommended_alternative vazio")
    elif isinstance(alternatives, list) and 3 <= len(alternatives) <= 5:
        if normalize_name(recommended) not in {normalize_name(str(item)) for item in alternatives}:
            errors.append("bundle.json: recommended_alternative não consta em alternatives")

    decisions = data.get("decisions", [])
    if not isinstance(decisions, list) or not decisions:
        errors.append("bundle.json: decisions vazio ou inválido")
    elif any(not is_meaningful(item) for item in decisions):
        errors.append("bundle.json: decisions contém item vazio ou placeholder")

    readiness = data.get("readiness", {})
    if "status" not in readiness or "blockers" not in readiness:
        errors.append("bundle.json: readiness deve conter 'status' e 'blockers'")
    elif readiness.get("status") not in VALID_STATUS:
        errors.append(f"bundle.json: readiness.status inválido - '{readiness.get('status')}'")
    if not isinstance(readiness.get("blockers", []), list):
        errors.append("bundle.json: readiness.blockers deve ser lista")
    elif any(not isinstance(item, str) for item in readiness.get("blockers", [])):
        errors.append("bundle.json: readiness.blockers deve conter apenas strings")

    if data.get("status") == "done" and readiness.get("status") != "done":
        errors.append("bundle.json: status done exige readiness.status done")

    return errors


def validate_decision_brief(path: Path) -> list[str]:
    if not path.is_file():
        return ["decision-brief.md: arquivo ausente"]

    text = path.read_text(encoding="utf-8")
    if not text.strip():
        return ["decision-brief.md: arquivo vazio"]

    sections = split_sections(text)
    errors: list[str] = []

    for required in REQUIRED_SECTIONS:
        if required not in sections:
            errors.append(f"decision-brief.md: seção ausente '{required}'")
            continue

        body = sections[required]
        if not body:
            errors.append(f"decision-brief.md: seção vazia '{required}'")
            continue
        for issue in detect_placeholders(body):
            errors.append(f"decision-brief.md -> {required}: {issue}")

        for marker in MANDATORY_MARKERS.get(required, []):
            if marker not in body:
                errors.append(f"decision-brief.md -> {required}: marcador obrigatório ausente '{marker}'")

    alternatives_body = sections.get("## Alternativas Avaliadas", "")
    alternative_count = len(ALT_HEADING.findall(alternatives_body))
    if alternative_count < 3:
        errors.append("decision-brief.md -> ## Alternativas Avaliadas: menos de 3 alternativas")
    if alternative_count > 5:
        errors.append("decision-brief.md -> ## Alternativas Avaliadas: mais de 5 alternativas")

    return errors


def parse_markdown_rows(text: str) -> list[list[str]]:
    rows: list[list[str]] = []
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped.startswith("|") or not stripped.endswith("|"):
            continue
        cells = [cell.strip() for cell in stripped.strip("|").split("|")]
        if cells and all(re.fullmatch(r":?-{3,}:?", cell) for cell in cells):
            continue
        rows.append(cells)
    return rows


def validate_scorecard(path: Path) -> list[str]:
    if not path.is_file():
        return ["option-scorecard.md: arquivo ausente"]

    text = path.read_text(encoding="utf-8")
    if not text.strip():
        return ["option-scorecard.md: arquivo vazio"]

    errors: list[str] = []
    rows = parse_markdown_rows(text)
    if not rows:
        return ["option-scorecard.md: tabela de scorecard ausente"]

    header = rows[0]
    for column in ["Alternativa", *SCORE_COLUMNS]:
        if column not in header:
            errors.append(f"option-scorecard.md: coluna obrigatória ausente '{column}'")

    if errors:
        return errors

    index = {name: header.index(name) for name in ["Alternativa", *SCORE_COLUMNS]}
    data_rows = [row for row in rows[1:] if row and row[0].startswith("Alternativa ")]
    if len(data_rows) < 3:
        errors.append("option-scorecard.md: menos de 3 alternativas pontuadas")
    if len(data_rows) > 5:
        errors.append("option-scorecard.md: mais de 5 alternativas pontuadas")

    for row in data_rows:
        if len(row) < len(header):
            errors.append(f"option-scorecard.md: linha incompleta para '{row[0]}'")
            continue
        score_total = 0
        for column in SCORE_COLUMNS:
            value = row[index[column]]
            if not re.fullmatch(r"[1-5]", value):
                errors.append(
                    f"option-scorecard.md: nota inválida em '{row[0]}' coluna '{column}' - '{value}'"
                )
            else:
                score_total += int(value)
        if "Total" in header:
            total_value = row[header.index("Total")]
            if not re.fullmatch(r"\d+", total_value):
                errors.append(f"option-scorecard.md: total inválido em '{row[0]}' - '{total_value}'")
            elif int(total_value) != score_total:
                errors.append(
                    f"option-scorecard.md: total divergente em '{row[0]}' - esperado {score_total}, obtido {total_value}"
                )

    for issue in detect_placeholders(text):
        errors.append(f"option-scorecard.md: {issue}")

    return errors


def extract_brief_alternatives(path: Path) -> list[str]:
    if not path.is_file():
        return []
    text = path.read_text(encoding="utf-8")
    sections = split_sections(text)
    return [match.strip() for match in ALT_HEADING.findall(sections.get("## Alternativas Avaliadas", ""))]


def extract_scorecard_alternatives(path: Path) -> list[str]:
    if not path.is_file():
        return []
    rows = parse_markdown_rows(path.read_text(encoding="utf-8"))
    if not rows:
        return []
    return [row[0] for row in rows[1:] if row and row[0].startswith("Alternativa ")]


def validate_cross_file_consistency(bundle_dir: Path) -> list[str]:
    bundle_path = bundle_dir / "bundle.json"
    if not bundle_path.is_file():
        return []

    try:
        data = json.loads(bundle_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return []

    errors: list[str] = []
    bundle_alternatives = {
        normalize_name(str(item)) for item in data.get("alternatives", []) if str(item).strip()
    }
    brief_alternatives = {
        normalize_name(item) for item in extract_brief_alternatives(bundle_dir / "decision-brief.md")
    }
    scorecard_alternatives = {
        normalize_name(item) for item in extract_scorecard_alternatives(bundle_dir / "option-scorecard.md")
    }

    if bundle_alternatives and brief_alternatives and bundle_alternatives != brief_alternatives:
        errors.append("consistência: alternatives do bundle.json divergem das alternativas do decision-brief.md")
    if bundle_alternatives and scorecard_alternatives and bundle_alternatives != scorecard_alternatives:
        errors.append("consistência: alternatives do bundle.json divergem das alternativas do option-scorecard.md")

    recommended = normalize_name(str(data.get("recommended_alternative", "")))
    if recommended and brief_alternatives and recommended not in brief_alternatives:
        errors.append("consistência: recommended_alternative não consta no decision-brief.md")
    if recommended and scorecard_alternatives and recommended not in scorecard_alternatives:
        errors.append("consistência: recommended_alternative não consta no option-scorecard.md")

    return errors


def validate_transcript(path: Path) -> list[str]:
    if not path.is_file():
        return ["transcript.md: arquivo ausente"]
    text = path.read_text(encoding="utf-8")
    required = [
        "## Contexto Inicial",
        "## Rodada 1 - Entendimento do Problema",
        "## Rodada 2 - Escopo e Restrições",
        "## Rodada 3 - Alternativas",
        "## Rodada 4 - Trade-offs",
        "## Rodada 5 - Seleção de Direção",
        "## Decisões Registradas",
    ]
    errors = [f"transcript.md: seção obrigatória ausente '{item}'" for item in required if item not in text]
    if errors:
        return errors

    sections = split_sections(text)
    for item in required:
        body = sections.get(item, "")
        if not body:
            errors.append(f"transcript.md: seção vazia '{item}'")
        for issue in detect_placeholders(body):
            errors.append(f"transcript.md -> {item}: {issue}")
    return errors


def validate_assumptions(path: Path) -> list[str]:
    if not path.is_file():
        return ["assumptions.md: arquivo ausente"]
    text = path.read_text(encoding="utf-8")
    if not text.strip():
        return ["assumptions.md: arquivo vazio"]
    required = [
        "## Hipóteses Confirmadas",
        "## Hipóteses Não Validadas",
        "## Restrições Confirmadas",
    ]
    errors = [f"assumptions.md: seção obrigatória ausente '{item}'" for item in required if item not in text]
    if errors:
        return errors

    sections = split_sections(text)
    for item in required:
        body = sections.get(item, "")
        if not body:
            errors.append(f"assumptions.md: seção vazia '{item}'")
        for issue in detect_placeholders(body):
            errors.append(f"assumptions.md -> {item}: {issue}")
    return errors


def main() -> int:
    if len(sys.argv) != 2:
        print("USO: validate-bundle.py <caminho-do-bundle>", file=sys.stderr)
        return 2

    bundle_dir = Path(sys.argv[1]).resolve()
    if not bundle_dir.is_dir():
        print(f"DIRETÓRIO INVÁLIDO: {bundle_dir}", file=sys.stderr)
        return 1

    errors: list[str] = []
    present = {path.name for path in bundle_dir.iterdir() if path.is_file()}
    for missing in sorted(REQUIRED_FILES - present):
        errors.append(f"{missing}: arquivo ausente")

    errors += validate_bundle_json(bundle_dir / "bundle.json")
    errors += validate_decision_brief(bundle_dir / "decision-brief.md")
    errors += validate_scorecard(bundle_dir / "option-scorecard.md")
    errors += validate_transcript(bundle_dir / "transcript.md")
    errors += validate_assumptions(bundle_dir / "assumptions.md")
    errors += validate_cross_file_consistency(bundle_dir)

    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1

    print("SUCCESS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
