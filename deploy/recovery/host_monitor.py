#!/usr/bin/env python3
"""Host and external readiness probes with incident and recovery context."""

import contextlib
import io
import json
import os
import pathlib
import re
import runpy
import shutil
import subprocess
import time


HOST = "weevcrafts.com"
ROOT = pathlib.Path("/usr/local/lib/weevcrafts-operations")
STATE = pathlib.Path("/var/lib/weevcrafts-operations/monitor.json")
EVENTS = {"readiness": "readiness_failed", "resource": "resource_pressure", "backup": "backup_stale"}


def transition(state, name, unhealthy, required):
    item = state.setdefault(name, {"failures": 0, "alerted": False})
    item["failures"] = min(item["failures"] + 1, required) if unhealthy else 0
    if item["failures"] >= required and not item["alerted"]:
        item["alerted"] = True
        return EVENTS[name]
    if not unhealthy and item["alerted"]:
        item["alerted"] = False
        return name + "_recovered"
    return None


def probe(origin=False):
    opts = ["--resolve", HOST + ":443:127.0.0.1"] if origin else []
    result = subprocess.run(
        ["curl", "-sS", "--max-time", "8", "-o", "/dev/null", "-w", "%{http_code} %{time_total}", *opts,
         "https://" + HOST + "/health/ready"],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        timeout=10,
    )
    match = re.fullmatch(r"(\d{3}) (\d+(?:\.\d+)?)", result.stdout.strip())
    if not match:
        return (0, 0, result.returncode)
    return (int(match[1]), round(float(match[2]) * 1000), result.returncode)


def revision():
    result = subprocess.run(
        ["docker", "ps", "--filter", "label=coolify.applicationUuid=vy2hpttvmehhauvskvilgoju", "--format", "{{.Image}}"],
        capture_output=True, text=True, timeout=8,
    )
    tag = result.stdout.strip().rsplit(":", 1)[-1]
    return tag[:12] if re.fullmatch(r"[a-f0-9]{40,64}", tag) else "unknown"


def main():
    os.umask(0o077)
    STATE.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    state = json.loads(STATE.read_text()) if STATE.exists() else {}
    public, origin = probe(), probe(origin=True)
    counters = [int(x) for x in pathlib.Path("/proc/stat").read_text().splitlines()[0].split()[1:9]]
    cpu = [sum(counters), counters[3] + counters[4]]
    old_cpu = state.get("cpu", cpu)
    total, idle = cpu[0] - old_cpu[0], cpu[1] - old_cpu[1]
    busy = round(100 * (1 - idle / total), 1) if total > 0 else 0
    state["cpu"] = cpu
    ram = dict(line.split(":", 1) for line in pathlib.Path("/proc/meminfo").read_text().splitlines())
    available_mib = int(ram["MemAvailable"].split()[0]) // 1024
    disk = shutil.disk_usage("/")
    disk_free = round(100 * disk.free / disk.total, 1)
    active = ", ".join(name for name, flag in (
        ("cpu", busy > 70), ("memory", available_mib < 2048), ("disk", disk_free < 20)
    ) if flag) or "none"
    try:
        recovery = runpy.run_path(str(ROOT / "weev-backup-run.py"))
        config = json.loads(pathlib.Path("/etc/weevcrafts-operations/backup.json").read_text())
        with contextlib.redirect_stdout(io.StringIO()):
            stale = recovery["status"](config) != 0
    except (OSError, ValueError, KeyError, RuntimeError):
        stale = True
    ready = public[0] == 200 and origin[0] == 200
    checks = [
        ("readiness", not ready, 3, f"public_http={public[0]} public_ms={public[1]} public_exit={public[2]} "
         f"origin_http={origin[0]} origin_ms={origin[1]} origin_exit={origin[2]} revision={revision()}"),
        ("resource", active != "none", 4, f"thresholds={active} cpu_percent={busy}/70 "
         f"available_mib={available_mib}/2048 disk_free_percent={disk_free}/20"),
        ("backup", stale, 3, "offsite_recovery_max_age_hours=26"),
    ]
    for check, unhealthy, count, detail in checks:
        old = state.get(check, {}).copy()
        event = transition(state, check, unhealthy, count)
        if unhealthy or event:
            print(f"{event or check + '_probe_failed'} {detail}", flush=True)
        if event:
            result = subprocess.run(
                [str(ROOT / "notify-owner"), event],
                env=dict(os.environ, WEEVCRAFTS_ALERT_DETAIL=detail),
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30,
            )
            if result.returncode:
                state[check] = old or {"failures": 0, "alerted": False}
    state["updated_epoch"] = time.time()
    temporary = STATE.with_suffix(".partial")
    temporary.write_text(json.dumps(state))
    os.replace(temporary, STATE)


if __name__ == "__main__":
    main()
