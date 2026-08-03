#!/usr/bin/env python3
"""Valida um STANDARD.md de telemetria e um otel-collector.yaml gerados pela skill.

Uso:
    python3 validate-standard.py --standard <STANDARD.md> --collector <otel-collector.yaml>

Sucesso: imprime "SUCCESS" no stdout e sai com codigo 0.
Falha: lista as lacunas no stderr e sai com codigo 1.
Sem dependencias externas (checagem textual; nao requer PyYAML).
"""
import argparse
import os
import re
import sys

# Os quatro Golden Signals que DEVEM estar mapeados no documento.
GOLDEN_SIGNALS = ["Latência", "Tráfego", "Erros", "Saturação"]

# Métrica canônica de semconv esperada para os sinais de servidor HTTP.
CANONICAL_METRIC = "http.server.request.duration"

# Buckets estáveis de latência (nao podem ser inventados).
STABLE_BUCKETS = "0.005"  # ancora do array de buckets estaveis

# Seções obrigatórias do STANDARD.md.
REQUIRED_SECTIONS = [
    "Contexto do serviço",
    "Golden Signals",
    "Instrumentação",
    "Coleta",
    "Sampling",
    "Alertas",
    "Referências",
]

# Atributos de alta cardinalidade proibidos em métricas.
FORBIDDEN_CARDINALITY = ["user_id", "request_id", "session_id"]


def validate_standard(path, errors):
    if not os.path.isfile(path):
        errors.append(f"STANDARD ERROR: arquivo nao encontrado: {path}")
        return
    text = open(path, encoding="utf-8").read()

    # Placeholders nao preenchidos.
    leftovers = re.findall(r"\{\{[^}]+\}\}", text)
    if leftovers:
        sample = ", ".join(sorted(set(leftovers))[:5])
        errors.append(
            f"STANDARD ERROR: placeholders nao preenchidos ({len(set(leftovers))}): {sample} ..."
        )

    # Seções obrigatórias.
    for section in REQUIRED_SECTIONS:
        if section not in text:
            errors.append(f"STANDARD ERROR: seção obrigatória ausente: '{section}'.")

    # Os quatro Golden Signals.
    for signal in GOLDEN_SIGNALS:
        if signal not in text:
            errors.append(f"STANDARD ERROR: Golden Signal ausente no mapeamento: '{signal}'.")

    # Métrica canônica e service.name.
    if CANONICAL_METRIC not in text:
        errors.append(
            f"STANDARD ERROR: métrica canônica de semconv ausente: '{CANONICAL_METRIC}'."
        )
    if "service.name" not in text:
        errors.append("STANDARD ERROR: 'service.name' (atributo obrigatório) ausente.")

    # Buckets estáveis de latência.
    if STABLE_BUCKETS not in text:
        errors.append(
            "STANDARD ERROR: buckets estáveis de latência ausentes (esperado iniciar em 0.005)."
        )

    # Latência não pode ser via média simples.
    if re.search(r"m[ée]dia simples", text, re.IGNORECASE) and "histogram_quantile" not in text:
        errors.append(
            "STANDARD ERROR: latência deve usar histogram_quantile, não média simples."
        )
    if "histogram_quantile" not in text:
        errors.append("STANDARD ERROR: query de latência com histogram_quantile ausente.")

    # Cardinalidade proibida não deve aparecer como atributo de métrica recomendado.
    # Aceita a citação apenas quando há contexto de proibição próximo (antes ou depois).
    for attr in FORBIDDEN_CARDINALITY:
        for m in re.finditer(re.escape(attr), text):
            ctx = text[max(0, m.start() - 80): m.end() + 80].lower()
            if not any(k in ctx for k in ["proib", "não", "nao", "evit", "nunca"]):
                errors.append(
                    f"STANDARD ERROR: atributo de alta cardinalidade '{attr}' citado sem contexto de proibição."
                )
                break

    # Referências oficiais.
    if "opentelemetry.io" not in text or "sre.google" not in text:
        errors.append(
            "STANDARD ERROR: referências oficiais ausentes (esperado sre.google e opentelemetry.io)."
        )


def validate_collector(path, errors):
    if not os.path.isfile(path):
        errors.append(f"COLLECTOR ERROR: arquivo nao encontrado: {path}")
        return
    text = open(path, encoding="utf-8").read()

    required = {
        "receiver otlp": "otlp",
        "porta gRPC 4317": "4317",
        "porta HTTP 4318": "4318",
        "processor batch": "batch",
        "service.pipelines": "pipelines",
    }
    for label, needle in required.items():
        if needle not in text:
            errors.append(f"COLLECTOR ERROR: {label} ausente (esperado '{needle}').")

    # Pipelines dos três sinais.
    for sig in ["traces", "metrics", "logs"]:
        if sig not in text:
            errors.append(f"COLLECTOR ERROR: pipeline '{sig}' ausente.")

    # Se houver tail_sampling, deve estar no pipeline de traces, não em metrics.
    if "tail_sampling" in text:
        metrics_block = re.search(r"metrics:\s*(.*?)(?:logs:|$)", text, re.DOTALL)
        if metrics_block and "tail_sampling" in metrics_block.group(1):
            errors.append(
                "COLLECTOR ERROR: tail_sampling não deve ser aplicado ao pipeline de metrics."
            )


def main():
    parser = argparse.ArgumentParser(description="Valida STANDARD.md e otel-collector.yaml.")
    parser.add_argument("--standard", required=True, help="Caminho do STANDARD.md")
    parser.add_argument("--collector", required=True, help="Caminho do otel-collector.yaml")
    args = parser.parse_args()

    errors = []
    validate_standard(args.standard, errors)
    validate_collector(args.collector, errors)

    if errors:
        print("\n".join(errors), file=sys.stderr)
        sys.exit(1)
    print("SUCCESS: padrão de telemetria e collector válidos.")
    sys.exit(0)


if __name__ == "__main__":
    main()
