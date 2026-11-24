import atexit
import os
import tempfile
from pathlib import Path

# app.main builds a module-level app (Settings.from_env -> load_dotenv) at import
# time. Redirect the env path to an empty temp location and pin GROQ_* in the
# process environment BEFORE app.main is imported, so the real BE root .env is
# never opened by the suite.
import app.settings as _settings

_sandbox = tempfile.TemporaryDirectory(prefix="ecosnap-ai-tests-")
atexit.register(_sandbox.cleanup)
_settings.ENV_PATH = Path(_sandbox.name) / ".env"

os.environ["GROQ_API_KEY"] = ""
os.environ["GROQ_MODEL"] = "qwen/qwen3.8-27b"
os.environ["GROQ_BASE_URL"] = "https://groq-mock.invalid/openai/v1"

import pytest  # noqa: E402


@pytest.fixture(autouse=True)
def disable_live_credentials(monkeypatch):
    monkeypatch.setenv("GROQ_API_KEY", "")
    monkeypatch.setenv("GROQ_BASE_URL", "https://groq-mock.invalid/openai/v1")
