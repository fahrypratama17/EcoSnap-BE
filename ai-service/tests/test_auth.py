import asyncio
import io
import json
import sys
import types
from pathlib import Path
from uuid import uuid4

import pytest
from fastapi.testclient import TestClient
from PIL import Image

from app.auth import FirebaseTokenVerifier
from app.main import create_app
from app.schema import ScanResponse
from support import FakeVerifier, auth_headers

BOTTLE = json.loads((Path(__file__).resolve().parent / "fixtures" / "waste_scan_success.json").read_text(encoding="utf-8"))


@pytest.fixture
def photo():
    stream = io.BytesIO()
    Image.new("RGB", (80, 40), "green").save(stream, "PNG")
    return stream.getvalue()


class CountingAnalyzer:
    def __init__(self):
        self.calls = 0

    async def analyze(self, jpeg):
        self.calls += 1
        return ScanResponse.model_validate(BOTTLE)


def post(client, photo, scan_id=None, headers=None):
    return client.post("/api/scan", files={"image": ("a.png", photo, "image/png")},
                       headers={"X-Scan-Id": scan_id or str(uuid4()), **(headers or {})})


def assert_unauthorized(response):
    assert response.status_code == 401
    assert response.json()["error"]["code"] == "unauthorized"
    assert response.headers["www-authenticate"] == "Bearer"
    assert response.headers["cache-control"] == "no-store"


def test_health_docs_and_openapi_are_public():
    verifier = FakeVerifier()
    with TestClient(create_app(token_verifier=verifier)) as client:
        assert client.get("/health").json() == {"status": "ok"}
        assert client.get("/docs").status_code == 200
        assert client.get("/openapi.json").status_code == 200
    assert verifier.calls == []


@pytest.mark.parametrize("authorization", [
    None, "", "Bearer", "Bearer ", "Basic abc", "Token fake-token-for-a", "Bearer two parts",
    "Bearer  fake-token-for-a", "Bearer " + "x" * 9000, "fake-token-for-a",
])
def test_missing_or_malformed_authorization_is_rejected_before_any_processing(authorization):
    verifier, analyzer = FakeVerifier(), CountingAnalyzer()
    headers = {} if authorization is None else {"Authorization": authorization}
    with TestClient(create_app(analyzer=analyzer, token_verifier=verifier)) as client:
        # An oversized declared body or non-multipart upload would otherwise yield 413/415.
        too_large = client.post("/api/scan", content=b"x", headers={**headers, "content-length": str(50_000_000)})
        not_multipart = client.post("/api/scan", content=b"{}", headers={**headers, "content-type": "application/json"})
    assert_unauthorized(too_large)
    assert_unauthorized(not_multipart)
    assert analyzer.calls == 0
    assert verifier.calls == []


@pytest.mark.parametrize("reason", ["invalid-signature", "expired", "foreign-project-audience", "revoked"])
def test_tokens_rejected_by_verifier_get_401_without_analysis(photo, reason):
    token = f"firebase-{reason}"
    analyzer = CountingAnalyzer()
    with TestClient(create_app(analyzer=analyzer, token_verifier=FakeVerifier(rejected={token}))) as client:
        response = post(client, photo, headers={"Authorization": f"Bearer {token}"})
        too_large = client.post("/api/scan", content=b"x", headers={"Authorization": f"Bearer {token}", "content-length": str(50_000_000)})
    assert_unauthorized(response)
    assert_unauthorized(too_large)
    assert token not in response.text
    assert analyzer.calls == 0


def test_valid_user_receives_raw_scan_response(photo):
    analyzer = CountingAnalyzer()
    with TestClient(create_app(analyzer=analyzer, token_verifier=FakeVerifier())) as client:
        response = post(client, photo, headers=auth_headers("user-a"))
    assert response.status_code == 200
    assert response.json() == BOTTLE
    assert analyzer.calls == 1


def test_provider_outage_returns_503_without_analysis(photo):
    analyzer = CountingAnalyzer()
    with TestClient(create_app(analyzer=analyzer, token_verifier=FakeVerifier(unavailable=True))) as client:
        response = post(client, photo, headers=auth_headers())
    assert response.status_code == 503
    assert response.json()["error"]["code"] == "auth_unavailable"
    assert "www-authenticate" not in response.headers
    assert analyzer.calls == 0


def test_same_scan_id_is_isolated_per_user(photo):
    analyzer = CountingAnalyzer()
    other = io.BytesIO()
    Image.new("RGB", (60, 60), "red").save(other, "PNG")
    scan_id = str(uuid4())
    with TestClient(create_app(analyzer=analyzer, token_verifier=FakeVerifier())) as client:
        first = post(client, photo, scan_id, auth_headers("user-a"))
        second = post(client, other.getvalue(), scan_id, auth_headers("user-b"))
        same_image_other_user = post(client, photo, scan_id, auth_headers("user-c"))
        conflict = post(client, other.getvalue(), scan_id, auth_headers("user-a"))
    assert (first.status_code, second.status_code, same_image_other_user.status_code) == (200, 200, 200)
    assert analyzer.calls == 3  # no cross-user cache reuse
    assert conflict.status_code == 409  # still enforced within the same UID


def test_same_user_cache_is_reused_across_client_addresses(photo):
    analyzer = CountingAnalyzer()
    app = create_app(analyzer=analyzer, token_verifier=FakeVerifier())
    scan_id = str(uuid4())
    with TestClient(app, client=("10.0.0.1", 50000)) as first_client:
        second_client = TestClient(app, client=("10.0.0.2", 50001))
        assert post(first_client, photo, scan_id, auth_headers("user-a")).status_code == 200
        assert post(second_client, photo, scan_id, auth_headers("user-a")).status_code == 200
    assert analyzer.calls == 1


def test_rate_limit_is_per_user_not_per_proxy_address(photo):
    analyzer = CountingAnalyzer()
    with TestClient(create_app(analyzer=analyzer, token_verifier=FakeVerifier(), requests_per_minute=1), client=("127.0.0.1", 1)) as client:
        assert post(client, photo, headers=auth_headers("user-a")).status_code == 200
        assert post(client, photo, headers=auth_headers("user-b")).status_code == 200
        limited = post(client, photo, headers=auth_headers("user-a"))
    assert limited.status_code == 429 and limited.headers["retry-after"] == "60"
    assert analyzer.calls == 2


# Real FirebaseTokenVerifier against a stand-in firebase_admin module: no SDK, credentials or network.

class FakeFirebase:
    def __init__(self, monkeypatch):
        self.initialized = []
        self.outcome = {"uid": "firebase-user"}
        self.project_id = "demo-project"
        self.verify_calls = []
        fb = types.ModuleType("firebase_admin")
        auth = types.ModuleType("firebase_admin.auth")

        from firebase_admin.auth import CertificateFetchError, ExpiredIdTokenError, InvalidIdTokenError

        auth.InvalidIdTokenError, auth.ExpiredIdTokenError, auth.CertificateFetchError = InvalidIdTokenError, ExpiredIdTokenError, CertificateFetchError
        self.auth = auth
        apps = {}

        def get_app(name):
            if name not in apps:
                raise ValueError("no app")
            return apps[name]

        def initialize_app(credential=None, options=None, name="[DEFAULT]"):
            if self.project_id == "raise":
                raise RuntimeError("credentials unreadable /secret/path.json")
            self.initialized.append((name, options))
            apps[name] = type("App", (), {"project_id": property(lambda _: self.project_id)})()
            return apps[name]

        def verify_id_token(token, app=None, check_revoked=False):
            self.verify_calls.append((token, app, check_revoked))
            if isinstance(self.outcome, BaseException):
                raise self.outcome
            if callable(self.outcome):
                return self.outcome()
            return self.outcome

        fb.get_app, fb.initialize_app, fb.auth = get_app, initialize_app, auth
        auth.verify_id_token = verify_id_token
        monkeypatch.setitem(sys.modules, "firebase_admin", fb)
        monkeypatch.setitem(sys.modules, "firebase_admin.auth", auth)


@pytest.fixture
def firebase(monkeypatch):
    return FakeFirebase(monkeypatch)


def real_client(analyzer, timeout=10):
    return TestClient(create_app(analyzer=analyzer, token_verifier=FirebaseTokenVerifier(timeout=timeout)))


def test_real_verifier_is_lazy_and_initializes_once_with_timeout(firebase, photo):
    analyzer = CountingAnalyzer()
    with real_client(analyzer) as client:
        assert client.get("/health").status_code == 200
        assert_unauthorized(post(client, photo))
        assert firebase.initialized == []
        assert post(client, photo, headers={"Authorization": "Bearer id-token-1"}).status_code == 200
        assert post(client, photo, headers={"Authorization": "Bearer id-token-2"}).status_code == 200
    assert len(firebase.initialized) == 1
    assert firebase.initialized[0][1] == {"httpTimeout": 10}
    assert [call[2] for call in firebase.verify_calls] == [False, False]


@pytest.mark.parametrize("error", ["invalid", "expired", "value"])
def test_real_verifier_maps_invalid_tokens_to_401(firebase, photo, error):
    firebase.outcome = {"invalid": firebase.auth.InvalidIdTokenError("private detail"),
                        "expired": firebase.auth.ExpiredIdTokenError("private detail", cause=ValueError("expired")),
                        "value": ValueError("private detail")}[error]
    analyzer = CountingAnalyzer()
    with real_client(analyzer) as client:
        response = post(client, photo, headers={"Authorization": "Bearer secret-token-value"})
    assert_unauthorized(response)
    assert "private detail" not in response.text and "secret-token-value" not in response.text
    assert analyzer.calls == 0


def test_real_verifier_maps_certificate_and_unknown_failures_to_503(firebase, photo):
    analyzer = CountingAnalyzer()
    with real_client(analyzer) as client:
        for outcome in (firebase.auth.CertificateFetchError("network detail", cause=OSError("offline")), RuntimeError("boom")):
            firebase.outcome = outcome
            response = post(client, photo, headers={"Authorization": "Bearer secret-token-value"})
            assert response.status_code == 503
            assert response.json()["error"]["code"] == "auth_unavailable"
            assert "detail" not in response.text and "secret-token-value" not in response.text
    assert analyzer.calls == 0


@pytest.mark.parametrize("project_id", ["raise", None])
def test_real_verifier_config_failure_is_503_and_recovers(firebase, photo, project_id):
    firebase.project_id = project_id
    analyzer = CountingAnalyzer()
    with real_client(analyzer) as client:
        response = post(client, photo, headers={"Authorization": "Bearer id-token"})
        assert response.status_code == 503
        assert response.json()["error"]["code"] == "auth_unavailable"
        assert "secret" not in response.text
        firebase.project_id = "demo-project"
        # A failed first initialization is retried instead of being cached forever.
        assert post(client, photo, headers={"Authorization": "Bearer id-token"}).status_code == 200
    assert analyzer.calls == 1


def test_real_verifier_timeout_is_503(firebase, photo):
    import time

    firebase.outcome = lambda: time.sleep(0.5) or {"uid": "late"}
    analyzer = CountingAnalyzer()
    with real_client(analyzer, timeout=0.05) as client:
        response = post(client, photo, headers={"Authorization": "Bearer id-token"})
    assert response.status_code == 503
    assert response.json()["error"]["code"] == "auth_unavailable"
    assert analyzer.calls == 0


def test_real_verifier_without_uid_claim_is_rejected(firebase, photo):
    firebase.outcome = {}
    with real_client(CountingAnalyzer()) as client:
        assert_unauthorized(post(client, photo, headers={"Authorization": "Bearer id-token"}))


def test_cancellation_is_not_swallowed(firebase):
    import time

    firebase.outcome = lambda: time.sleep(0.2) or {"uid": "u"}

    async def scenario():
        task = asyncio.create_task(FirebaseTokenVerifier().verify("id-token"))
        await asyncio.sleep(0.02)
        task.cancel()
        with pytest.raises(asyncio.CancelledError):
            await task

    asyncio.run(scenario())
