#!/usr/bin/env python3
from __future__ import annotations

import argparse
import re
import ssl
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

import certifi


URL_PATTERN = re.compile(r"https://www\.postgresql\.org/docs/[^\s)>\"]+")
ALLOWED_HOST = "www.postgresql.org"


def read_urls(skill_root: Path) -> list[tuple[Path, str]]:
    urls: list[tuple[Path, str]] = []
    for path in sorted(skill_root.rglob("*.md")):
        text = path.read_text(encoding="utf-8", errors="ignore")
        for match in URL_PATTERN.finditer(text):
            urls.append((path, match.group(0)))
    return urls


def fetch(url: str, cache: dict[str, str]) -> str:
    if url in cache:
        return cache[url]
    request = urllib.request.Request(url, headers={"User-Agent": "codex-postgresql-skill-validator/1.0"})
    ssl_context = ssl.create_default_context(cafile=certifi.where())
    with urllib.request.urlopen(request, timeout=20, context=ssl_context) as response:
        body = response.read().decode("utf-8", errors="ignore")
    cache[url] = body
    return body


def fragment_exists(body: str, fragment: str) -> bool:
    target = re.escape(urllib.parse.unquote(fragment))
    patterns = [
        rf'id="{target}"',
        rf"id='{target}'",
        rf'name="{target}"',
        rf"name='{target}'",
    ]
    return any(re.search(pattern, body) for pattern in patterns)


def main() -> int:
    parser = argparse.ArgumentParser(description="Valida URLs oficiais do PostgreSQL usadas pela skill.")
    parser.add_argument("skill_root", help="Diretorio raiz da skill.")
    args = parser.parse_args()

    skill_root = Path(args.skill_root).resolve()
    if not skill_root.exists():
        print(f"INPUT ERROR: caminho inexistente: {skill_root}", file=sys.stderr)
        return 1

    urls = read_urls(skill_root)
    if not urls:
        print("DOC MAP ERROR: nenhuma URL oficial do PostgreSQL foi encontrada.", file=sys.stderr)
        return 1

    cache: dict[str, str] = {}
    failures: list[str] = []
    for path, url in urls:
        parsed = urllib.parse.urlparse(url)
        if parsed.netloc != ALLOWED_HOST:
            failures.append(f"HOST ERROR: {path.relative_to(skill_root)} usa host nao permitido: {url}")
            continue
        base_url = urllib.parse.urlunparse(parsed._replace(fragment=""))
        try:
            body = fetch(base_url, cache)
        except urllib.error.URLError as exc:
            failures.append(f"FETCH ERROR: {path.relative_to(skill_root)} nao conseguiu abrir {base_url}: {exc}")
            continue
        if parsed.fragment and not fragment_exists(body, parsed.fragment):
            failures.append(f"FRAGMENT ERROR: {path.relative_to(skill_root)} referencia ancora ausente: {url}")

    if failures:
        print("\n".join(failures), file=sys.stderr)
        return 1

    print(f"SUCCESS: {len(urls)} URLs oficiais validadas.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
