"""SDK process used by the Go server smoke test.

The channel posts one turn, waits until memory extraction has fallen back
to chat, then raises so the process exits 1.
"""

from __future__ import annotations

import sys
import threading
import time
from pathlib import Path

from golem import Channel, Client, Extension, FunctionCall, Message, Provider, ToolCall
from golem.provider import ToolDef

LOG = Path(sys.argv[1])


def note(line: str) -> None:
    with LOG.open("a", encoding="utf-8") as fh:
        fh.write(line + "\n")


class Echo(Provider):
    """Chat-only provider. Structured output is intentionally not implemented."""

    def chat(
        self,
        model: str,
        messages: list[Message],
        tools: list[ToolDef] | None = None,
    ) -> Message:
        del model, tools
        blob = "\n".join(m.content for m in messages)
        if "Extract lasting facts" in blob:
            note("extract")
            return Message(role="assistant", content='{"memories":[]}')
        if any(m.role == "tool" for m in messages):
            note("chat")
            return Message(role="assistant", content="echoed " + messages[-1].content)
        note("chat")
        return Message(
            role="assistant",
            tool_calls=[
                ToolCall(
                    id="c1",
                    function=FunctionCall(name="echo", arguments='{"text":"hi"}'),
                )
            ],
        )


def echo(text: str) -> str:
    """Echo text."""
    note("tool:" + text)
    return "pong:" + text


class Once(Channel):
    id = "cli"

    def run(self, client: Client, stop: threading.Event) -> None:
        del stop
        reply = client.send("conv-smoke", self.id, "hello")
        note("reply:" + reply)
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            if "extract" in LOG.read_text(encoding="utf-8"):
                time.sleep(0.5)
                break
            time.sleep(0.05)
        else:
            note("missing-extract")
        raise RuntimeError("channel failed")


def main() -> None:
    Extension(
        "smoke",
        provider=("echo", Echo()),
        channel=Once(),
        tools=[echo],
    ).run()


if __name__ == "__main__":
    main()
