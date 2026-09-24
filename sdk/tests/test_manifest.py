import pytest

from golem.manifest import Decl, Entrypoint, parse

PROVIDER = """
[project]
name = "golem-echo"
version = "0.1.0"

[tool.golem.provider]
id = "echo"
entrypoint = "golem_echo:Echo"
"""

CHANNEL = """
[project]
name = "golem-cli"

[tool.golem.channel]
id = "cli"
entrypoint = "golem_cli:CLI"
"""

BOTH = """
[project]
name = "golem-echo"

[tool.golem]
tools = "golem_echo:tools"

[tool.golem.provider]
id = "echo"
entrypoint = "pkg.mod:Echo"

[tool.golem.channel]
id = "cli"
entrypoint = "golem_cli:CLI"
"""


def test_parse_provider() -> None:
    manifest = parse(PROVIDER)
    assert manifest.name == "golem-echo"
    assert manifest.provider == Decl("echo", Entrypoint("golem_echo", "Echo"))
    assert manifest.channel is None
    assert str(manifest.provider.entrypoint) == "golem_echo:Echo"


def test_parse_channel() -> None:
    manifest = parse(CHANNEL)
    assert manifest.name == "golem-cli"
    assert manifest.provider is None
    assert manifest.channel == Decl("cli", Entrypoint("golem_cli", "CLI"))


def test_parse_provider_and_channel_ignores_tools() -> None:
    manifest = parse(BOTH)
    assert manifest.provider == Decl("echo", Entrypoint("pkg.mod", "Echo"))
    assert manifest.channel == Decl("cli", Entrypoint("golem_cli", "CLI"))


@pytest.mark.parametrize(
    ("text", "match"),
    [
        ("[project]\nversion = \"0.1.0\"\n", "missing name"),
        ("[project]\nname = \"  \"\n", "missing name"),
        ("[project]\nname = \"echo\"\n", "provider or channel"),
        (
            "[project]\nname = \"echo\"\n[tool.golem]\ntools = \"pkg:tools\"\n",
            "provider or channel",
        ),
        (
            "[project]\nname = \"echo\"\n[tool.golem.provider]\nentrypoint = \"pkg:Echo\"\n",
            "provider id is required",
        ),
        (
            "[project]\nname = \"echo\"\n[tool.golem.provider]\nid = \" \"\nentrypoint = \"pkg:Echo\"\n",
            "provider id is required",
        ),
        (
            "[project]\nname = \"echo\"\n[tool.golem.provider]\nid = \"echo\"\n",
            "provider entrypoint is required",
        ),
        (
            "[project]\nname = \"echo\"\n[tool.golem.channel]\nentrypoint = \"pkg:CLI\"\n",
            "channel id is required",
        ),
        (
            "[project]\nname = \"echo\"\n[tool.golem.channel]\nid = \"cli\"\n",
            "channel entrypoint is required",
        ),
        (
            "[project]\nname = \"echo\"\n[tool.golem.provider]\nid = \"echo\"\nentrypoint = \"Echo\"\n",
            "invalid entrypoint",
        ),
        (
            "[project]\nname = \"echo\"\n[tool.golem.provider]\nid = \"echo\"\nentrypoint = \":Echo\"\n",
            "invalid entrypoint",
        ),
        (
            "[project]\nname = \"echo\"\n[tool.golem.provider]\nid = \"echo\"\nentrypoint = \"pkg:\"\n",
            "invalid entrypoint",
        ),
        (
            "[project]\nname = \"echo\"\n[tool.golem.provider]\nid = \"echo\"\nentrypoint = \"pkg:Echo:extra\"\n",
            "invalid entrypoint",
        ),
        ("not toml", "invalid pyproject.toml"),
    ],
)
def test_parse_errors(text: str, match: str) -> None:
    with pytest.raises(ValueError, match=match):
        parse(text)
