import asyncio
import threading
from typing import Protocol

from .errors import ScanError

FIREBASE_APP_NAME = "ecosnap-ai-auth"
VERIFY_TIMEOUT_SECONDS = 10
MAX_TOKEN_LENGTH = 8192


def unauthorized() -> ScanError:
    return ScanError(401, "unauthorized", "Masuk terlebih dahulu untuk menggunakan pemindaian.")


def auth_unavailable() -> ScanError:
    return ScanError(503, "auth_unavailable", "Layanan autentikasi sedang tidak tersedia. Coba lagi sebentar.", 5)


class TokenVerifier(Protocol):
    async def verify(self, token: str) -> str:
        """Return the verified Firebase UID or raise ScanError (401 unauthorized / 503 auth_unavailable)."""
        ...


def bearer_token(authorization: str | None) -> str:
    """Extract the token from `Authorization: Bearer <token>`; raise 401 when absent or malformed."""
    if not authorization:
        raise unauthorized()
    parts = authorization.split(" ")
    if len(parts) != 2 or parts[0].lower() != "bearer" or not parts[1] or len(parts[1]) > MAX_TOKEN_LENGTH:
        raise unauthorized()
    token = parts[1]
    if not token.isascii() or not token.isprintable():
        raise unauthorized()
    return token


class FirebaseTokenVerifier:
    """Firebase Admin ID-token verification.

    Credentials (GOOGLE_APPLICATION_CREDENTIALS) and the SDK are only touched on the
    first token that reaches verification, never at import, startup, health or for
    requests rejected earlier. The blocking SDK call runs on a bounded worker pool
    and is abandoned after 10 seconds. Revocation is not checked (SDK default).
    """

    def __init__(self, timeout: float = VERIFY_TIMEOUT_SECONDS):
        self.timeout = timeout
        self._lock = threading.Lock()
        self._app = None

    def _firebase_app(self):
        with self._lock:
            if self._app is None:
                import firebase_admin

                try:
                    app = firebase_admin.get_app(FIREBASE_APP_NAME)
                except ValueError:
                    app = firebase_admin.initialize_app(
                        name=FIREBASE_APP_NAME,
                        options={"httpTimeout": VERIFY_TIMEOUT_SECONDS},
                    )
                if not app.project_id:
                    raise RuntimeError("firebase project id unavailable")
                self._app = app
            return self._app

    def _verify_blocking(self, token: str) -> str:
        try:
            from firebase_admin import auth

            app = self._firebase_app()
        except Exception:
            raise auth_unavailable() from None
        try:
            claims = auth.verify_id_token(token, app=app, check_revoked=False)
        except auth.CertificateFetchError:
            raise auth_unavailable() from None
        except (auth.InvalidIdTokenError, ValueError):
            # Firebase token errors are FirebaseError subclasses, not ValueError.
            raise unauthorized() from None
        except Exception:
            raise auth_unavailable() from None
        uid = claims.get("uid") or claims.get("sub")
        if not isinstance(uid, str) or not uid:
            raise unauthorized()
        return uid

    async def verify(self, token: str) -> str:
        try:
            return await asyncio.wait_for(asyncio.to_thread(self._verify_blocking, token), timeout=self.timeout)
        except TimeoutError:
            raise auth_unavailable() from None
