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


def remove_empty_secrets(doc: dict, secrets_env: dict[str, str]) -> dict:
    empty_keys = {name for name, value in secrets_env.items() if value == ""}
    if not empty_keys:
        return doc

    stack = doc.get("secrets", {})
    removed_stack_secrets: set[str] = set()
    for secret_name in list(stack.keys()):
        for empty_key in empty_keys:
            if secret_name.endswith(f"_{empty_key}"):
                del stack[secret_name]
                removed_stack_secrets.add(secret_name)
                break

    if not removed_stack_secrets:
        return doc

    for svc in doc.get("services", {}).values():
        svc_secrets = svc.get("secrets", [])
        svc["secrets"] = [
            s for s in svc_secrets
            if not (
                (isinstance(s, dict) and s.get("source") in removed_stack_secrets)
                or (isinstance(s, str) and s in removed_stack_secrets)
            )
        ]
        if not svc["secrets"]:
            del svc["secrets"]

    return doc


def pin_images_to_digest(doc: dict, image_name: str, image_digest: str) -> dict:
    if not image_digest:
        return doc
    target_ref = f"{image_name}@{image_digest}"
    for svc in doc.get("services", {}).values():
        img = svc.get("image", "")
        if img.startswith(f"{image_name}:"):
            svc["image"] = target_ref
    return doc


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
                v = str(port["published"])
                port["published"] = v if ":" in v else int(v)

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
        for key, value in parse_env_file(args.env_file).items():
            env.setdefault(key, value)

    secrets_env: dict[str, str] = {}
    if args.secrets_env_file:
        secrets_env = parse_env_file(args.secrets_env_file)
        for key, value in secrets_env.items():
            env.setdefault(key, value)

    if env.get("ENVIRONMENT") == "production":
        pg_image = env.get("POSTGRES_IMAGE", "")
        if "mecontrola-postgres:" not in pg_image:
            print(
                f"ERRO: POSTGRES_IMAGE='{pg_image}' nao e a imagem custom com pgBackRest.\n"
                "      Producao exige mecontrola-postgres:<tag>. "
                "Configure POSTGRES_IMAGE no env de producao.",
                file=sys.stderr,
            )
            return 1

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
    rendered = remove_empty_secrets(rendered, secrets_env)
    rendered = pin_images_to_digest(
        rendered,
        env.get("IMAGE_NAME", "ghcr.io/limateixeiratecnologia/mecontrola"),
        env.get("IMAGE_DIGEST", ""),
    )
    normalized = normalize_stack(rendered)
    yaml.safe_dump(normalized, sys.stdout, default_flow_style=False, sort_keys=False)
    return 0


if __name__ == "__main__":
    sys.exit(main())
