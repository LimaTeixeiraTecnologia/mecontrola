#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


POSTGRES_USAGE_PATTERNS = [
    ("dsn", re.compile(r"postgres(?:ql)?(?:\+[a-z0-9_]+)?://", re.IGNORECASE)),
    ("jdbc-dsn", re.compile(r"jdbc:postgresql://", re.IGNORECASE)),
    ("prisma-provider", re.compile(r"provider\s*=\s*[\"']postgresql[\"']", re.IGNORECASE)),
    ("typeorm-postgres", re.compile(r"type\s*:\s*[\"']postgres[\"']", re.IGNORECASE)),
    ("sequelize-postgres", re.compile(r"dialect\s*:\s*[\"']postgres[\"']", re.IGNORECASE)),
    ("knex-postgres", re.compile(r"client\s*:\s*[\"']pg[\"']", re.IGNORECASE)),
    ("rails-postgres", re.compile(r"adapter\s*:\s*postgresql\b", re.IGNORECASE)),
    ("django-postgres", re.compile(r"django\.db\.backends\.postgresql", re.IGNORECASE)),
    ("npgsql", re.compile(r"\bNpgsql\b")),
    ("pgx-import", re.compile(r"github\.com/jackc/pgx(?:/v\d+)?(?:/pgxpool)?", re.IGNORECASE)),
    ("libpq-import", re.compile(r"github\.com/lib/pq", re.IGNORECASE)),
    ("psycopg", re.compile(r"\bpsycopg(?:2|3)?\b", re.IGNORECASE)),
    ("asyncpg", re.compile(r"\basyncpg\b", re.IGNORECASE)),
    ("pg8000", re.compile(r"\bpg8000\b", re.IGNORECASE)),
    ("postgrex", re.compile(r"\bpostgrex\b", re.IGNORECASE)),
    ("tokio-postgres", re.compile(r"\btokio-postgres\b", re.IGNORECASE)),
    ("mikroorm-postgres", re.compile(r"@mikro-orm/postgresql", re.IGNORECASE)),
]

ORM_PATTERNS = {
    "prisma": [re.compile(r"provider\s*=\s*[\"']postgresql[\"']", re.IGNORECASE)],
    "typeorm": [re.compile(r"type\s*:\s*[\"']postgres[\"']", re.IGNORECASE)],
    "sequelize": [re.compile(r"dialect\s*:\s*[\"']postgres[\"']", re.IGNORECASE)],
    "mikroorm": [re.compile(r"@mikro-orm/postgresql", re.IGNORECASE)],
    "knex": [re.compile(r"client\s*:\s*[\"']pg[\"']", re.IGNORECASE)],
    "django": [re.compile(r"django\.db\.backends\.postgresql", re.IGNORECASE)],
    "sqlalchemy": [re.compile(r"\bsqlalchemy\b", re.IGNORECASE)],
    "rails": [re.compile(r"adapter\s*:\s*postgresql\b", re.IGNORECASE)],
    "pgx": [re.compile(r"github\.com/jackc/pgx(?:/v\d+)?(?:/pgxpool)?", re.IGNORECASE)],
    "libpq": [re.compile(r"github\.com/lib/pq", re.IGNORECASE)],
    "ecto": [re.compile(r"\becto_sql\b", re.IGNORECASE), re.compile(r"\bpostgrex\b", re.IGNORECASE)],
    "diesel": [re.compile(r"\bdiesel\b", re.IGNORECASE), re.compile(r"\bpostgres\b", re.IGNORECASE)],
    "npgsql": [re.compile(r"\bNpgsql\b")],
    "asyncpg": [re.compile(r"\basyncpg\b", re.IGNORECASE)],
    "pg8000": [re.compile(r"\bpg8000\b", re.IGNORECASE)],
    "tokio-postgres": [re.compile(r"\btokio-postgres\b", re.IGNORECASE)],
}

STACK_HINTS = {
    "node": ["package.json", "pnpm-lock.yaml", "yarn.lock", "package-lock.json"],
    "python": ["pyproject.toml", "requirements.txt", "poetry.lock", "Pipfile"],
    "go": ["go.mod", "go.sum"],
    "java": ["pom.xml", "build.gradle", "build.gradle.kts"],
    "dotnet": [".csproj", ".sln"],
    "ruby": ["Gemfile", "Gemfile.lock"],
    "elixir": ["mix.exs", "mix.lock"],
    "rust": ["Cargo.toml", "Cargo.lock"],
}

STACK_EXTENSION_HINTS = {
    ".py": "python",
    ".go": "go",
    ".ts": "node",
    ".tsx": "node",
    ".js": "node",
    ".jsx": "node",
    ".prisma": "node",
    ".java": "java",
    ".kt": "java",
    ".cs": "dotnet",
    ".rb": "ruby",
    ".ex": "elixir",
    ".exs": "elixir",
    ".rs": "rust",
}

DOMAIN_PATTERNS = {
    "modelagem e migrations": [
        re.compile(r"\bmigration\b", re.IGNORECASE),
        re.compile(r"\bcreate\s+table\b", re.IGNORECASE),
        re.compile(r"\balter\s+table\b", re.IGNORECASE),
        re.compile(r"\bforeign\s+key\b", re.IGNORECASE),
        re.compile(r"\bprimary\s+key\b", re.IGNORECASE),
    ],
    "queries e indices": [
        re.compile(r"\bselect\b[\s\S]{0,200}\bfrom\b", re.IGNORECASE),
        re.compile(r"\bexplain(?:\s+analyze)?\b", re.IGNORECASE),
        re.compile(r"\bcreate\s+index\b", re.IGNORECASE),
        re.compile(r"\bdrop\s+index\b", re.IGNORECASE),
        re.compile(r"\border\s+by\b", re.IGNORECASE),
        re.compile(r"\bjoin\b", re.IGNORECASE),
    ],
    "transacoes e locking": [
        re.compile(r"\bserializable\b", re.IGNORECASE),
        re.compile(r"\bfor\s+update\b", re.IGNORECASE),
        re.compile(r"\block\s+table\b", re.IGNORECASE),
        re.compile(r"\bbegin\b[\s\S]{0,80}\bcommit\b", re.IGNORECASE),
        re.compile(r"\btransaction\b", re.IGNORECASE),
    ],
    "seguranca e controle de acesso": [
        re.compile(r"\bgrant\b", re.IGNORECASE),
        re.compile(r"\brevoke\b", re.IGNORECASE),
        re.compile(r"\bcreate\s+role\b", re.IGNORECASE),
        re.compile(r"\balter\s+role\b", re.IGNORECASE),
        re.compile(r"\brow\s+level\s+security\b", re.IGNORECASE),
    ],
    "manutencao e observabilidade": [
        re.compile(r"\bvacuum\b", re.IGNORECASE),
        re.compile(r"\banalyze\b", re.IGNORECASE),
        re.compile(r"\bpg_stat_[a-z_]+\b", re.IGNORECASE),
        re.compile(r"\bautovacuum\b", re.IGNORECASE),
    ],
    "backup, restore e replicacao": [
        re.compile(r"\bpg_basebackup\b", re.IGNORECASE),
        re.compile(r"\barchive_command\b", re.IGNORECASE),
        re.compile(r"\blogical\s+replication\b", re.IGNORECASE),
        re.compile(r"\bstreaming\s+replication\b", re.IGNORECASE),
        re.compile(r"\brestore\b", re.IGNORECASE),
        re.compile(r"\bwal\b", re.IGNORECASE),
    ],
}

DOMAIN_FILE_HINTS = (
    "migration",
    "migrations",
    "schema",
    "query",
    "queries",
    "sql",
    "prisma",
    "database",
    "db",
    "repository",
    "dao",
)

SCAN_FILE_PATTERNS = [
    "*.sql",
    "*.py",
    "*.go",
    "*.ts",
    "*.tsx",
    "*.js",
    "*.jsx",
    "*.java",
    "*.kt",
    "*.cs",
    "*.rb",
    "*.ex",
    "*.exs",
    "*.rs",
    "*.toml",
    "*.yml",
    "*.yaml",
    "*.prisma",
    "*.env",
    "*.env.*",
    "*.example",
    "*.example.*",
    "Dockerfile",
    "docker-compose*.yml",
    "docker-compose*.yaml",
]


def read_text(path: Path) -> str:
    try:
        return path.read_text(encoding="utf-8", errors="ignore")
    except OSError:
        return ""


def collect_candidate_files(root: Path) -> list[Path]:
    files: set[Path] = set()
    for pattern in SCAN_FILE_PATTERNS:
        files.update(root.rglob(pattern))
    return sorted(path for path in files if path.is_file())


def detect_stack(root: Path, files: list[Path]) -> list[str]:
    found: set[str] = set()
    names = {path.name for path in files}
    for stack, hints in STACK_HINTS.items():
        for hint in hints:
            if hint.startswith(".") and any(p.suffix == hint for p in files):
                found.add(stack)
                break
            if hint in names or any(root.rglob(hint)):
                found.add(stack)
                break
            if hint.startswith(".") is False and any(p.name.endswith(hint) for p in files):
                found.add(stack)
                break
    for path in files:
        inferred = STACK_EXTENSION_HINTS.get(path.suffix.lower())
        if inferred:
            found.add(inferred)
    return sorted(found)


def version_authority(path: Path, snippet: str) -> int:
    name = path.name.lower()
    suffix = path.suffix.lower()
    lowered = snippet.lower()
    if name == "dockerfile" or name.startswith("docker-compose"):
        return 100
    if "server_version" in lowered or "select version()" in lowered or "show server_version" in lowered:
        return 90
    if suffix in {".yml", ".yaml", ".toml", ".json", ".env", ".prisma"}:
        return 80
    if suffix in {".sql", ".py", ".go", ".ts", ".js", ".rb", ".java", ".kt", ".cs"}:
        return 30
    return 10


def detect_versions(files: list[Path], root: Path) -> tuple[str | None, list[dict[str, str]]]:
    best_version: str | None = None
    best_authority = -1
    authority_versions: dict[int, set[str]] = {}
    evidence: list[dict[str, str]] = []
    for path in files:
        text = read_text(path)
        lowered = text.lower()
        if "postgres" not in lowered:
            continue
        matches: list[tuple[str, str]] = []
        if path.name.lower() == "dockerfile" or path.name.lower().startswith("docker-compose"):
            matches.extend((match.group(1), match.group(0)) for match in re.finditer(r"postgres:(\d+)(?:\.\d+)?", text, re.IGNORECASE))
        if "server_version" in lowered or "select version()" in lowered or "show server_version" in lowered:
            matches.extend((match.group(1), match.group(0)) for match in re.finditer(r"postgresql[^\d]{0,10}(\d+)(?:\.\d+)?", text, re.IGNORECASE))
        for version, snippet in matches:
            authority = version_authority(path, snippet)
            authority_versions.setdefault(authority, set()).add(version)
            if authority > best_authority:
                best_authority = authority
                best_version = version
            evidence.append({
                "path": str(path.relative_to(root)),
                "kind": "version",
                "value": version,
                "snippet": snippet[:160],
                "authority": str(authority),
            })
    if best_authority >= 0 and len(authority_versions.get(best_authority, set())) > 1:
        versions = sorted(authority_versions[best_authority])
        raise ValueError(f"multiplas major versions conflitantes detectadas em fontes autoritativas: {', '.join(versions)}")
    return best_version, evidence


def detect_postgres_usage(files: list[Path], root: Path) -> tuple[list[dict[str, str]], list[str]]:
    evidence: list[dict[str, str]] = []
    orms: set[str] = set()
    for path in files:
        text = read_text(path)
        matched_signal = False
        for label, pattern in POSTGRES_USAGE_PATTERNS:
            match = pattern.search(text)
            if match:
                matched_signal = True
                idx = match.start()
                snippet = text[max(0, idx - 40): idx + 120].replace("\n", " ").strip()
                evidence.append({
                    "path": str(path.relative_to(root)),
                    "kind": "postgres-usage",
                    "value": label,
                    "snippet": snippet[:200],
                })
                break
    project_texts = [read_text(path) for path in files]
    for orm, patterns in ORM_PATTERNS.items():
        if all(any(pattern.search(text) for text in project_texts) for pattern in patterns):
            orms.add(orm)
    return evidence, sorted(orms)


def is_domain_candidate(path: Path, text: str) -> bool:
    lowered_path = str(path).lower()
    if path.suffix.lower() in {".sql", ".prisma"}:
        return True
    if any(hint in lowered_path for hint in DOMAIN_FILE_HINTS):
        return True
    return any(pattern.search(text) for _, pattern in POSTGRES_USAGE_PATTERNS)


def classify_surface(files: list[Path], root: Path) -> tuple[str | None, list[dict[str, str]]]:
    counts: dict[str, int] = {}
    evidence: list[dict[str, str]] = []
    for path in files:
        text = read_text(path)
        if not is_domain_candidate(path, text):
            continue
        for domain, patterns in DOMAIN_PATTERNS.items():
            for pattern in patterns:
                match = pattern.search(text)
                if match:
                    counts[domain] = counts.get(domain, 0) + 1
                    if len(evidence) < 20:
                        idx = match.start()
                        snippet = text[max(0, idx - 40): idx + 120].replace("\n", " ").strip()
                        evidence.append({
                            "path": str(path.relative_to(root)),
                            "kind": "surface",
                            "value": domain,
                            "snippet": snippet[:200],
                        })
                    break
    if not counts:
        return None, evidence
    ranked = sorted(counts.items(), key=lambda item: (-item[1], item[0]))
    if len(ranked) > 1 and ranked[0][1] == ranked[1][1]:
        return None, evidence
    return ranked[0][0], evidence


def fail(message: str) -> int:
    print(message, file=sys.stderr)
    return 1


def main() -> int:
    parser = argparse.ArgumentParser(description="Detecta contexto PostgreSQL e evidencias objetivas no projeto.")
    parser.add_argument("project_root", help="Raiz do projeto a analisar.")
    args = parser.parse_args()

    root = Path(args.project_root).resolve()
    if not root.exists() or not root.is_dir():
        return fail(f"INPUT ERROR: diretorio invalido: {args.project_root}")

    files = collect_candidate_files(root)
    try:
        version, version_evidence = detect_versions(files, root)
    except ValueError as exc:
        return fail(f"DETECTION ERROR: {exc}")
    usage_evidence, orms = detect_postgres_usage(files, root)
    if not usage_evidence and not version_evidence:
        return fail("DETECTION ERROR: nenhum sinal objetivo de PostgreSQL foi encontrado no projeto.")

    domain, surface_evidence = classify_surface(files, root)

    stacks = detect_stack(root, files)
    output = {
        "project_root": str(root),
        "postgres_detected": True,
        "version": version,
        "version_supported_by_skill": version in {"14", "15", "16", "17", "18"} if version else None,
        "stacks": stacks,
        "orms_or_drivers": orms,
        "dominant_domain": domain,
        "evidence": usage_evidence[:20] + version_evidence[:10] + surface_evidence[:20],
    }
    json.dump(output, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
