import os
import re

import pytest
from fastapi.testclient import TestClient

from app import settings as settings_module
from app.main import create_app
from app.schema import ScanResponse
from app.settings import Settings

GROQ_VARS = ("GROQ_API_KEY", "GROQ_MODEL", "GROQ_BASE_URL")
FAKE_KEY = "fake-key-from-temp-env-file"


@pytest.fixture
def env_file(tmp_path, monkeypatch):
    """Point Settings at a throwaway env file; the real BE .env is never read."""
    path = tmp_path / ".env"
    monkeypatch.setattr(settings_module, "ENV_PATH", path)
    for name in GROQ_VARS:
        monkeypatch.delenv(name, raising=False)
    yield path
    for name in GROQ_VARS:  # load_dotenv writes os.environ directly
        os.environ.pop(name, None)


def test_settings_loads_env_file_with_bom(env_file):
    env_file.write_bytes(f"GROQ_API_KEY={FAKE_KEY}\nGROQ_MODEL=test-model\n".encode("utf-8-sig"))
    loaded = Settings.from_env()
    assert loaded.api_key == FAKE_KEY
    assert loaded.model == "test-model"
    assert FAKE_KEY not in repr(loaded)


def test_existing_environment_wins_over_env_file(env_file, monkeypatch):
    env_file.write_text(f"GROQ_API_KEY={FAKE_KEY}\nGROQ_MODEL=file-model\n", encoding="utf-8")
    monkeypatch.setenv("GROQ_API_KEY", "")
    monkeypatch.setenv("GROQ_MODEL", "process-model")
    loaded = Settings.from_env()
    assert loaded.api_key == ""
    assert loaded.model == "process-model"


def test_malformed_env_file_does_not_leak_contents(env_file, capfd, caplog):
    env_file.write_text(f"GROQ_API_KEY={FAKE_KEY}\n=== broken {FAKE_KEY}\n'unterminated {FAKE_KEY}\n", encoding="utf-8")
    Settings.from_env()
    captured = capfd.readouterr()
    assert FAKE_KEY not in captured.out + captured.err + caplog.text


def test_env_file_is_not_searched_in_working_directory(env_file, tmp_path, monkeypatch):
    other = tmp_path / "cwd"
    other.mkdir()
    (other / ".env").write_text(f"GROQ_API_KEY={FAKE_KEY}\n", encoding="utf-8")
    monkeypatch.chdir(other)
    assert Settings.from_env().api_key == ""


@pytest.fixture
def spec():
    with TestClient(create_app()) as client:
        response = client.get("/openapi.json")
    assert response.status_code == 200
    return response.json()


def test_openapi_documents_actual_routes_upload_and_header(spec):
    assert set(spec["paths"]) == {"/health", "/api/scan"}
    assert set(spec["paths"]["/health"]) == {"get"}
    post = spec["paths"]["/api/scan"]["post"]
    assert set(spec["paths"]["/api/scan"]) == {"post"}
    form = post["requestBody"]["content"]["multipart/form-data"]["schema"]
    assert form["properties"]["image"] == {
        "type": "string",
        "format": "binary",
        "description": form["properties"]["image"]["description"],
    }
    assert form["required"] == ["image"]
    assert form["additionalProperties"] is False
    header = next(p for p in post["parameters"] if p["name"] == "X-Scan-Id")
    assert header["in"] == "header" and header["required"] is False
    assert header["schema"] == {"type": "string", "format": "uuid"}
    assert {"200", "400", "408", "409", "413", "415", "429", "500", "502", "503", "504"} <= set(post["responses"])


def test_openapi_response_refs_resolve(spec):
    text = __import__("json").dumps(spec)
    for ref in set(re.findall(r'"\$ref": "#/([^"]+)"', text)):
        node = spec
        for part in ref.split("/"):
            node = node[part]


def strip_description(schema):
    return {key: value for key, value in schema.items() if key != "description"}


def rewrite_refs(value):
    if isinstance(value, dict):
        return {k: ("#/components/schemas/" + v.rsplit("/", 1)[1] if k == "$ref" else rewrite_refs(v)) for k, v in value.items()}
    if isinstance(value, list):
        return [rewrite_refs(item) for item in value]
    return value


def test_openapi_schemas_match_scan_response(spec):
    generated = ScanResponse.model_json_schema()
    schemas = {"ScanResponse": {k: v for k, v in generated.items() if k != "$defs"}, **generated.get("$defs", {})}
    documented = spec["components"]["schemas"]
    assert {"ScanResponse", "Identification", "Upcycling", "Idea", "IndustrialRecycling", "DepositRecommendation"} == set(schemas)
    for name, schema in schemas.items():
        assert strip_description(documented[name]) == strip_description(rewrite_refs(schema)), name


