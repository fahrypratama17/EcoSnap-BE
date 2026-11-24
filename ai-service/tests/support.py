from app.auth import auth_unavailable, unauthorized


def auth_headers(uid="user-a"):
    return {"Authorization": f"Bearer fake-token-for-{uid}"}


class FakeVerifier:
    """Deterministic stand-in for Firebase: `fake-token-for-<uid>` verifies as <uid>.

    Tokens listed in `rejected` fail with 401; `unavailable=True` simulates the
    certificate/config outage (503). Never touches credentials or the network.
    """

    prefix = "fake-token-for-"

    def __init__(self, rejected=(), unavailable=False):
        self.rejected = set(rejected)
        self.unavailable = unavailable
        self.calls = []

    async def verify(self, token):
        self.calls.append(token)
        if self.unavailable:
            raise auth_unavailable()
        if token in self.rejected or not token.startswith(self.prefix):
            raise unauthorized()
        return token[len(self.prefix):]
