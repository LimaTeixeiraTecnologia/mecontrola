#!/usr/bin/env python3
"""Renderiza compose.swarm.yml para um YAML compatível com docker stack deploy.

Uso:
    python3 deployment/scripts/render-stack.py <env-file> <compose-file>

Saída no stdout.

Correções aplicadas sobre o output de `docker compose config`:
- remove a propriedade `name` do root (não permitida em stack deploy);
- converte depends_on de mapa para lista de strings;
- garante cpus como string;
- garante published ports como int;
- garante secrets mode como octal numérico;
- preserva nomes explícitos de networks/volumes.
"""

import subprocess
import sys
import yaml


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
    if len(sys.argv) != 3:
        print("uso: render-stack.py <env-file> <compose-file>", file=sys.stderr)
        return 1

    env_file, compose_file = sys.argv[1], sys.argv[2]

    result = subprocess.run(
        [
            "docker", "compose",
            "--env-file", env_file,
            "-f", compose_file,
            "config",
        ],
        capture_output=True,
        text=True,
        check=False,
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
