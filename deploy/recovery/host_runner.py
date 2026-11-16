#!/usr/bin/env python3
"""Run the existing host recovery scheduler with verified R2 replication."""

import json
import os
import pathlib
import re
import runpy
import subprocess
import sys
import tempfile
import time


BASE = pathlib.Path(__file__).with_name("weev-backup-base.py")
legacy = runpy.run_path(str(BASE))
legacy_globals = legacy["backup"].__globals__
write_record = legacy["atomic_json"]
collect_backup = legacy["backup"]


def encrypt_pipeline(producer, encryptor, target, env):
    partial = target.with_suffix(target.suffix + ".partial")
    try:
        with tempfile.TemporaryFile() as errors, partial.open("xb") as output:
            producer_job = subprocess.Popen(producer, stdout=subprocess.PIPE, stderr=errors, env=env)
            try:
                with subprocess.Popen(
                    encryptor, stdin=producer_job.stdout, stdout=output, stderr=subprocess.DEVNULL
                ) as encryption_job:
                    producer_job.stdout.close()
                    try:
                        encryption_status = encryption_job.wait(timeout=1500)
                        producer_status = producer_job.wait(timeout=60)
                    except subprocess.TimeoutExpired:
                        encryption_job.kill()
                        raise
                output.flush()
                os.fsync(output.fileno())
                if producer_status or encryption_status or partial.stat().st_size < 64:
                    errors.seek(0)
                    message = errors.readline(256).decode(errors="replace").strip()
                    if re.fullmatch(r"collect: [a-zA-Z0-9 ()_-]{1,160}", message):
                        print(message, file=sys.stderr, flush=True)
                    print(
                        f"backup_pipeline_failed collector_exit={producer_status} age_exit={encryption_status}",
                        file=sys.stderr,
                        flush=True,
                    )
                    raise RuntimeError("Encrypted archive did not complete")
            finally:
                if producer_job.poll() is None:
                    producer_job.kill()
                    producer_job.wait()
        partial.chmod(0o600)
        os.replace(partial, target)
    finally:
        partial.unlink(missing_ok=True)


def offsite_environment(config):
    # Coolify stores the two scoped shared variables encrypted. Keep the
    # decrypted values between the management process and the uploader.
    command = (
        "echo json_encode(App\\Models\\SharedEnvironmentVariable::query()"
        "->whereHas('server', fn($q) => $q->where('uuid', getenv('BACKUP_HOST_UUID')))"
        "->whereIn('key', ['BACKUP_S3_ACCESS_KEY','BACKUP_S3_SECRET_KEY'])"
        "->get()->mapWithKeys(fn($v) => [$v->key => $v->value]));"
    )
    result = subprocess.run(
        ["docker", "exec", "-e", "BACKUP_HOST_UUID", "coolify", "php", "artisan", "tinker", "--execute=" + command],
        env=dict(os.environ, BACKUP_HOST_UUID=config["offsite_server_uuid"]),
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
        timeout=30,
        check=True,
    )
    keys = json.loads(result.stdout)
    lengths = {"BACKUP_S3_ACCESS_KEY": 32, "BACKUP_S3_SECRET_KEY": 64}
    if any(not re.fullmatch(r"[a-fA-F0-9]{%d}" % length, keys.get(key, "")) for key, length in lengths.items()):
        raise RuntimeError("Offsite credential is unavailable")
    return dict(
        os.environ,
        BACKUP_S3_ENDPOINT=config["offsite_endpoint"],
        BACKUP_S3_BUCKET=config["offsite_bucket"],
        **keys,
    )


def backup(config):
    env = offsite_environment(config)
    restore_recipient = config.get("offsite_restore_recipient", "")
    if not re.fullmatch(r"age1[a-z0-9]{58,70}", restore_recipient):
        raise RuntimeError("Independent public recovery recipient is unavailable")

    def encrypt_for_restore(producer, encryptor, target, source_env):
        encrypt_pipeline(producer, [*encryptor, "-r", restore_recipient], target, source_env)

    def replicate_record(path, record):
        archive = path.with_suffix(".tar.age")
        result = subprocess.run(
            [config["helper_binary"], "upload", str(archive)],
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=1400,
        )
        match = re.fullmatch(r"offsite_verified bytes=(\d+) sha256=([a-f0-9]{64})", result.stdout.strip())
        if result.returncode or not match or int(match[1]) != record["bytes"] or match[2] != record["sha256"]:
            message = result.stderr.strip()
            if re.fullmatch(r"(upload|verify): [a-zA-Z0-9 ()_-]{1,160}", message):
                print(message, file=sys.stderr, flush=True)
            print(f"backup_offsite_failed uploader_exit={result.returncode}", file=sys.stderr, flush=True)
            raise RuntimeError("Offsite verification did not complete")
        record["offsite_verified"] = {"at_epoch": time.time(), "sha256": match[2]}
        write_record(path, record)

    legacy_globals["atomic_json"] = replicate_record
    legacy_globals["encrypt_pipeline"] = encrypt_for_restore
    try:
        collect_backup(config)
    finally:
        legacy_globals["atomic_json"] = write_record
        legacy_globals["encrypt_pipeline"] = legacy["encrypt_pipeline"]


def status(config):
    records = legacy["records"](legacy["configuration"](config))
    verified = [
        r for r in records if r.get("offsite_verified", {}).get("sha256") == r.get("sha256")
    ]
    latest = verified[-1] if verified else {}
    stale = time.time() - latest.get("completed_epoch", 0) > 26 * 3600
    print(json.dumps({
        "result": "attention" if stale else "healthy",
        "issues": ["offsite_backup_stale"] if stale else [],
        "latest_id": latest.get("id"),
    }))
    return int(stale)


if __name__ == "__main__":
    legacy_globals["encrypt_pipeline"] = encrypt_pipeline
    legacy_globals["backup"] = backup
    legacy_globals["status"] = status
    try:
        sys.exit(legacy["main"]())
    except Exception:
        print("Recovery failed. Inspect the sanitized stage diagnostics.", file=sys.stderr, flush=True)
        sys.exit(1)
