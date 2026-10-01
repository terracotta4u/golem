"""One offline round trip between this SDK and the Go server in this repo.

That current pairing is the supported one. The test does not cover older
core or SDK releases.
"""

from __future__ import annotations

import json
import os
import signal
import socket
import subprocess
import sys
from pathlib import Path

from golem import Client

ROOT = Path(__file__).resolve().parents[2]
EXT = Path(__file__).resolve().parent / "smoke_extension.py"
TOKEN = "smoke-token"


def test_go_server_and_sdk_round_trip(tmp_path: Path) -> None:
    port = _free_port()
    url = f"http://127.0.0.1:{port}"
    home = tmp_path / "home"
    golem = home / ".golem"
    golem.mkdir(parents=True)
    (golem / "conf.json").write_text(
        json.dumps(
            {
                "default_model": {"provider": "echo", "model": "test"},
                "fast_model": {"provider": "echo", "model": "test"},
                "memory": {
                    "embedding": {"provider": "echo", "model": "test"},
                    "budget_tokens": 800,
                    "min_similarity": 0.5,
                },
            }
        ),
        encoding="utf-8",
    )
    log = tmp_path / "ext.log"
    server_log = tmp_path / "server.log"
    server_out = server_log.open("w", encoding="utf-8")
    server = _popen(
        ["go", "run", "./cmd/golem", "serve", "--addr", f"127.0.0.1:{port}", "--token", TOKEN],
        cwd=ROOT,
        env=_env(home),
        stdout=server_out,
        stderr=subprocess.STDOUT,
    )
    ext: subprocess.Popen[str] | None = None
    try:
        Client(url, TOKEN).wait_ready(timeout=90)
        ext = _popen(
            [sys.executable, str(EXT), str(log)],
            cwd=ROOT,
            env=_env(home, url),
        )
        try:
            code = ext.wait(timeout=30)
        except subprocess.TimeoutExpired:
            _stop(ext)
            raise AssertionError(_details(log, server_log, "extension did not exit")) from None
        text = log.read_text(encoding="utf-8") if log.exists() else ""
        assert code == 1, _details(log, server_log, f"exit {code}")
        assert "tool:hi" in text
        assert "reply:echoed pong:hi" in text
        assert "extract" in text
        assert "missing-extract" not in text
    finally:
        if ext is not None and ext.poll() is None:
            _stop(ext)
        _stop(server)
        server_out.close()


def _free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def _env(home: Path, url: str = "") -> dict[str, str]:
    env = os.environ.copy()
    env["HOME"] = str(home)
    env["PYTHONPATH"] = str(ROOT / "sdk")
    if url:
        env["GOLEM_URL"] = url
        env["GOLEM_TOKEN"] = TOKEN
    return env


def _popen(argv: list[str], cwd: Path, env: dict[str, str], stdout=None, stderr=None) -> subprocess.Popen[str]:
    return subprocess.Popen(
        argv,
        cwd=cwd,
        env=env,
        stdout=stdout,
        stderr=stderr,
        text=True,
        start_new_session=True,
    )


def _stop(proc: subprocess.Popen[str]) -> None:
    if proc.poll() is not None:
        return
    os.killpg(proc.pid, signal.SIGTERM)
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        os.killpg(proc.pid, signal.SIGKILL)
        proc.wait(timeout=5)


def _details(log: Path, server_log: Path, why: str) -> str:
    ext = log.read_text(encoding="utf-8") if log.exists() else ""
    server = server_log.read_text(encoding="utf-8") if server_log.exists() else ""
    return f"{why}\next:\n{ext}\nserver:\n{server}"
