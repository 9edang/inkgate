"""Reads results/raw/*.json (oha) and *.stats (docker stats) and writes results/report.md, summary.json and rps.svg."""
import json
import re
import statistics
from pathlib import Path

ROOT = Path(__file__).parent / "results"
RAW = ROOT / "raw"
GATEWAYS = ["inkgate", "krakend"]
LABEL = {"inkgate": "inkgate", "krakend": "KrakenD CE"}
SCENARIO_DOC = {
    "passthrough": "one backend, no-op encoding (the body is streamed, never parsed)",
    "json": "one backend, JSON is decoded and encoded again",
    "merge": "two backends called in parallel, both JSON bodies merged into one",
    "plugin-modifier": "one backend, a request modifier plugin adds a header and a response modifier plugin adds a field",
    "plugin-server": "one backend, a server plugin wraps the HTTP handler and adds a response header",
    "plugin-client": "one backend, a client plugin replaces the backend client and calls the API itself",
}
PLUGIN_BASELINE = "json"  # the same request without a plugin
UNITS = {"B": 1, "KB": 1e3, "MB": 1e6, "GB": 1e9, "KIB": 1024, "MIB": 1024**2, "GIB": 1024**3}


def mem_bytes(text):
    m = re.match(r"([\d.]+)\s*([A-Za-z]+)", text.strip())
    return float(m.group(1)) * UNITS[m.group(2).upper()] if m else 0.0


def read_stats(path):
    cpu, mem = [], []
    for line in path.read_text().splitlines():
        try:
            c, m = line.split("|")
            cpu.append(float(c.strip().rstrip("%")))
            mem.append(mem_bytes(m.split("/")[0]))
        except ValueError:
            continue
    return (statistics.mean(cpu) if cpu else 0.0, max(mem) if mem else 0.0)


def read_run(gw, scenario, rnd):
    data = json.loads((RAW / f"{gw}-{scenario}-r{rnd}.json").read_text())
    s, p = data["summary"], data["latencyPercentiles"]
    ok = data["statusCodeDistribution"].get("200", 0)
    # requests still in flight when the timer ends are aborted by the load generator: not a failure
    errors = {k: v for k, v in data.get("errorDistribution", {}).items() if "deadline" not in k}
    total = sum(data["statusCodeDistribution"].values()) + sum(errors.values())
    cpu, mem = read_stats(RAW / f"{gw}-{scenario}-r{rnd}.stats")
    return {
        "rps": s["requestsPerSec"],
        "avg_ms": s["average"] * 1000,
        "p50_ms": p["p50"] * 1000,
        "p90_ms": p["p90"] * 1000,
        "p99_ms": p["p99"] * 1000,
        "success": 100.0 * ok / total if total else 0.0,
        "cpu_pct": cpu,
        "mem_mb": mem / 1e6,
        "round": rnd,
    }


def median_run(runs):
    return sorted(runs, key=lambda r: r["rps"])[len(runs) // 2]


def main():
    meta = json.loads((ROOT / "meta.json").read_text())
    summary = {}
    for scenario in meta["scenarios"]:
        for gw in GATEWAYS:
            runs = [read_run(gw, scenario, r) for r in range(1, meta["run"]["rounds"] + 1)]
            best = median_run(runs)
            best["rps_runs"] = [round(r["rps"]) for r in runs]
            summary.setdefault(scenario, {})[gw] = best
    (ROOT / "summary.json").write_text(json.dumps(summary, indent=2))
    (ROOT / "report.md").write_text(markdown(meta, summary))
    (ROOT / "rps.svg").write_text(svg(meta, summary))
    update_readme(meta, summary)


def update_readme(meta, summary):
    """Rewrites the block between the benchmark markers in the root README."""
    readme = ROOT.parent.parent / "README.md"
    text = readme.read_text()
    start, end = "<!-- benchmark:start -->", "<!-- benchmark:end -->"
    if start not in text or end not in text:
        return
    lim, run = meta["limits"], meta["run"]
    rows = [
        "| Scenario | inkgate req/s | KrakenD CE req/s | inkgate p99 | KrakenD CE p99 | inkgate memory | KrakenD CE memory |",
        "|---|--:|--:|--:|--:|--:|--:|",
    ]
    for scenario, per in summary.items():
        a, b = per["inkgate"], per["krakend"]
        rows.append(
            f"| {scenario} | **{a['rps']:,.0f}** ({a['rps'] / b['rps']:.2f}x) | {b['rps']:,.0f} | {a['p99_ms']:.1f} ms "
            f"| {b['p99_ms']:.1f} ms | {a['mem_mb']:.0f} MB | {b['mem_mb']:.0f} MB |"
        )
    block = "\n".join([
        start,
        f"inkgate against KrakenD CE `{meta['images']['krakend'].split(':')[-1]}`, in containers with the same config and the "
        f"same limits ({lim['gateway_cpus']} CPUs and {lim['gateway_memory']} memory each, {run['connections']} connections, "
        f"{run['duration']} per run, median of {run['rounds']} rounds), on {meta['host']['cpu']}.",
        "",
        *rows,
        "",
        "![requests per second](benchmark/results/rps.svg)",
        "",
        "Setup, scenarios and how to rerun it: [benchmark/](benchmark/README.md). Full report: "
        "[benchmark/results/report.md](benchmark/results/report.md).",
        end,
    ])
    head, rest = text.split(start, 1)
    tail = rest.split(end, 1)[1]
    readme.write_text(head + block + tail)


def build_note(meta):
    """What the inkgate image holds: git revision, Go and glibc. Older results do not have it."""
    b = meta.get("inkgate_build")
    if not b:
        return ""
    return f", revision `{b['revision']}`, Go {b['go']}, glibc {b['glibc']}"


def markdown(meta, summary):
    h, lim, run, img = meta["host"], meta["limits"], meta["run"], meta["images"]
    out = [
        "# inkgate vs KrakenD CE",
        "",
        f"Run on {meta['date']}. The median of {run['rounds']} rounds is shown, picked by requests per second.",
        "",
        "## Setup",
        "",
        "| | |",
        "|---|---|",
        f"| Host | {h['cpu']}, {h['threads']} threads, {h['memory_gb']} GB RAM, {h['kernel']} |",
        f"| inkgate | `{img['inkgate']}` (built from this repository){build_note(meta)} |",
        f"| KrakenD CE | `{img['krakend']}` |",
        f"| Backend | `{img['backend']}`, fixed JSON answers, {lim['backend_cpus']} CPUs |",
        f"| Load generator | `{img['load']}`, {run['connections']} keep-alive connections, {lim['load_cpus']} CPUs |",
        f"| Each gateway | {lim['gateway_cpus']} CPUs, {lim['gateway_memory']} memory, one at a time |",
        f"| Per run | {run['warmup']} warm-up, then {run['duration']} measured |",
        "| Config | the same file for both: [config/gateway.json](../config/gateway.json) |",
        "",
        "## Results",
    ]
    for scenario, per in summary.items():
        a, b = per["inkgate"], per["krakend"]
        out += [
            "",
            f"### {scenario}: {SCENARIO_DOC.get(scenario, '')}",
            "",
            "| Gateway | Requests/s | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |",
            "|---|--:|--:|--:|--:|--:|--:|--:|--:|",
        ]
        for gw in GATEWAYS:
            r = per[gw]
            out.append(
                f"| {LABEL[gw]} | {r['rps']:,.0f} | {r['avg_ms']:.2f} ms | {r['p50_ms']:.2f} ms | {r['p90_ms']:.2f} ms "
                f"| {r['p99_ms']:.2f} ms | {r['success']:.2f}% | {r['cpu_pct']:.0f}% | {r['mem_mb']:.0f} MB |"
            )
        ratio = a["rps"] / b["rps"] if b["rps"] else 0
        out += ["", f"inkgate serves **{ratio:.2f}x** the requests per second of KrakenD CE here. "
                    f"Runs (req/s): inkgate {a['rps_runs']}, KrakenD {b['rps_runs']}."]
    plugins = [s for s in summary if s.startswith("plugin-") and PLUGIN_BASELINE in summary]
    if plugins:
        out += [
            "",
            "## Plugin cost",
            "",
            f"Requests per second of each plugin scenario as a share of the same gateway on `{PLUGIN_BASELINE}` (one backend, "
            "no plugin). The plugin source is the same for both gateways; it is built once with each toolchain.",
            "",
            "| Scenario | inkgate | KrakenD CE |",
            "|---|--:|--:|",
        ]
        for scenario in plugins:
            cells = [f"{100 * summary[scenario][g]['rps'] / summary[PLUGIN_BASELINE][g]['rps']:.0f}%" for g in GATEWAYS]
            out.append(f"| {scenario} | {cells[0]} | {cells[1]} |")
    out += [
        "",
        "![requests per second](rps.svg)",
        "",
        "## Reproduce",
        "",
        "```sh",
        "cd benchmark",
        "./run.sh                       # builds the inkgate image, runs everything, rewrites results/",
        "DURATION=60s ROUNDS=5 ./run.sh  # longer, more rounds",
        "```",
        "",
        "Numbers depend on the machine; compare the two gateways within one run, not across machines.",
        "",
    ]
    return "\n".join(out)


def svg(meta, summary):
    colors = {"inkgate": "#2563eb", "krakend": "#9ca3af"}
    scen = list(summary)
    top = max(summary[s][g]["rps"] for s in scen for g in GATEWAYS) or 1
    w, bar_h, gap, left, right = 720, 22, 14, 120, 90
    h = 60 + len(scen) * (2 * bar_h + gap + 14)
    out = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" font-family="sans-serif" font-size="13">',
           f'<rect width="{w}" height="{h}" fill="#ffffff"/>',
           '<text x="16" y="24" font-size="15" font-weight="bold" fill="#111827">Requests per second (higher is better)</text>']
    for i, g in enumerate(GATEWAYS):
        x = 16 + i * 120
        out.append(f'<rect x="{x}" y="34" width="12" height="12" fill="{colors[g]}"/><text x="{x + 18}" y="45" fill="#111827">{LABEL[g]}</text>')
    y = 66
    for s in scen:
        out.append(f'<text x="16" y="{y + bar_h}" fill="#111827">{s}</text>')
        for g in GATEWAYS:
            v = summary[s][g]["rps"]
            bw = (w - left - right) * v / top
            out.append(f'<rect x="{left}" y="{y}" width="{bw:.1f}" height="{bar_h}" fill="{colors[g]}"/>'
                       f'<text x="{left + bw + 6:.1f}" y="{y + 16}" fill="#111827">{v:,.0f}</text>')
            y += bar_h + 4
        y += gap
    out.append("</svg>")
    return "\n".join(out)


if __name__ == "__main__":
    main()
