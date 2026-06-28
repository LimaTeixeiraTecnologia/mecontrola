#!/usr/bin/env python3
"""Renderiza um arquivo Compose substituindo variaveis ${VAR} por valores de um .env."""
import os
import re
import sys


def resolve_env_file_paths(content, compose_dir):
    """Converte caminhos relativos em env_file para absolutos, usando o diretorio do compose."""
    def repl_env_file(match):
        prefix = match.group(1)
        raw_path = match.group(2).strip().strip('"').strip("'")
        if raw_path.startswith(("/", "~")):
            return match.group(0)
        abs_path = os.path.normpath(os.path.join(compose_dir, raw_path))
        return f"{prefix}{abs_path}"

    return re.sub(r"^(\s*env_file:\s*)(.+)$", repl_env_file, content, flags=re.MULTILINE)


def load_env(env_file):
    env = {}
    if not os.path.exists(env_file):
        return env
    with open(env_file, "r", encoding="utf-8") as f:
        for line in f:
            line = line.split("#", 1)[0].strip()
            if not line:
                continue
            if "=" not in line:
                continue
            key, value = line.split("=", 1)
            key = key.strip()
            value = value.strip().strip('"').strip("'")
            if re.match(r"^[A-Za-z_][A-Za-z0-9_]*$", key):
                env[key] = value
    return env


def render_compose(compose_file, env):
    with open(compose_file, "r", encoding="utf-8") as f:
        content = f.read()

    pattern = re.compile(r"\$\{([A-Za-z_][A-Za-z0-9_]*)(?::([-?][^}]*))?\}")

    def repl(match):
        key = match.group(1)
        modifier = match.group(2)
        value = env.get(key)
        if value is None:
            if modifier and modifier.startswith("-"):
                return modifier[1:]
            raise KeyError(f"Variavel {key} nao encontrada em .env")
        return value

    rendered = pattern.sub(repl, content)
    compose_dir = os.path.dirname(os.path.abspath(compose_file))
    return resolve_env_file_paths(rendered, compose_dir)


if __name__ == "__main__":
    compose_file = sys.argv[1] if len(sys.argv) > 1 else "deployment/compose/compose.swarm.yml"
    env_file = sys.argv[2] if len(sys.argv) > 2 else ".env"
    env = load_env(env_file)
    env.update(os.environ)
    print(render_compose(compose_file, env))
