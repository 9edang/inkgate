"""Generates the "routes" scenario: a config with 500 endpoints, mixed static and parameterised paths, and the list of
URLs the load is spread over. It is deterministic, so both gateways and every run see the same routes.

    python3 gen_routes.py <config.json> <urls.txt>
"""
import json
import os
import random
import sys

STATIC, ONE_PARAM, TWO_PARAMS = 250, 150, 100  # 500 in all
BACKEND = {"host": ["http://bench-backend:80"], "url_pattern": "/user", "encoding": "json"}


def routes():
    """(pattern, example url path) for every endpoint."""
    for i in range(STATIC):
        yield f"/routes/static/{i}", f"/routes/static/{i}"
    for i in range(ONE_PARAM):
        yield f"/routes/item{i}/{{id}}", f"/routes/item{i}/{1000 + i}"
    for i in range(TWO_PARAMS):
        yield f"/routes/group{i}/{{a}}/items/{{b}}", f"/routes/group{i}/{i}/items/{7 + i}"


def main(config_path, urls_path):
    base = json.load(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "config", "gateway.json")))
    cfg = {k: v for k, v in base.items() if k != "endpoints"}
    cfg["endpoints"] = [
        {"endpoint": pattern, "method": "GET", "output_encoding": "json", "backend": [BACKEND]}
        for pattern, _ in routes()
    ]
    urls = [f"http://bench-gateway:8080{path}" for _, path in routes()]
    random.Random(1).shuffle(urls)
    with open(config_path, "w") as f:
        json.dump(cfg, f, indent=2)
    with open(urls_path, "w") as f:
        f.write("\n".join(urls) + "\n")
    assert len(cfg["endpoints"]) == 500


if __name__ == "__main__":
    main(*sys.argv[1:3])
