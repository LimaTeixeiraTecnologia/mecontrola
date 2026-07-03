#!/usr/bin/env python3
"""Renderiza compose.swarm.yml para um YAML compatível com docker stack deploy.

Uso:
    python3 deployment/scripts/render-stack.py <compose-file> [options]

Opções:
    --env-file <arquivo>         Arquivo .env com configuração não-secreta.
    --secrets-env-file <arquivo> Arquivo .env descriptografado com secrets
                                  (usado apenas para interpolação; não entra
                                  no container além do declarado no compose).

Saída no stdout.

Correções aplicadas sobre o output de `docker compose config`:
- remove a propriedade `name` do root (não permitida em stack deploy);
- converte depends_on de mapa para lista de strings;
- garante cpus como string;
- garante published ports como int;
- garante secrets mode como octal numérico;
- preserva nomes explícitos de networks/volumes.
"""

import argparse
import os
import subprocess
import sys
import yaml


def parse_env_file(path: str) -> dict[str, str]:
    values: dict[str, str] = {}
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.split("#", 1)[0].strip()
            if not line:
                continue
            if "=" not in line:
                continue
            key, _, value = line.partition("=")
            key = key.strip()
            value = value.strip().strip("\"'\"")
            if key:
                values[key] = value
    return values


def normalize_stack(doc: dict) -> dict:
    doc.pop("name", None)

    for svc in doc.get("services", {}).values():
        deps = svc.get("depends_on")
        if isinstance(deps, dict):
            svc["depends_on"] = list(deps.keys())

        deploy = svc.get("deploy", {})
        resources = deploy.get("resources", {})
        for section in ("limits", "reservations"):
            res = resources.get(section)
            if res and "cpus" in res:
                res["cpus"] = str(res["cpus"])

        for secret in svc.get("secrets", []):
            if isinstance(secret, dict) and "mode" in secret:
                secret["mode"] = int(str(secret["mode"]), 8)

        for port in svc.get("ports", []):
            if isinstance(port, dict) and "published" in port:
                port["published"] = int(port["published"])

    return doc


def main() -> int:
    parser = argparse.ArgumentParser(description="Renderiza compose para docker stack deploy")
    parser.add_argument("compose_file", help="Caminho para o arquivo compose")
    parser.add_argument("--env-file", help="Arquivo .env com configuração não-secreta")
    parser.add_argument("--secrets-env-file", help="Arquivo .env descriptografado com secrets")
    args = parser.parse_args()

    env = os.environ.copy()
    compose_args = ["docker", "compose", "-f", args.compose_file]

    if args.env_file:
        compose_args.extend(["--env-file", args.env_file])
        env.update(parse_env_file(args.env_file))

    if args.secrets_env_file:
        env.update(parse_env_file(args.secrets_env_file))

    compose_args.append("config")

    result = subprocess.run(
        compose_args,
        capture_output=True,
        text=True,
        check=False,
        env=env,
    )
    if result.returncode != 0:
        print(result.stderr, file=sys.stderr)
        return result.returncode

    rendered = yaml.safe_load(result.stdout)
    normalized = normalize_stack(rendered)
    yaml.safe_dump(normalized, sys.stdout, default_flow_style=False, sort_keys=False)
    return 0


if __name__ == "__main__":
    sys.exit(main())
