"""Consumer-visible CLI tests for the API contract gate."""
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CLI = ROOT / "scripts/check-api-contracts.py"


class ContractCLITests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        for relative in ("docs/api/openapi.yaml", "docs/api/ecosnap.postman_collection.json",
                        "docs/api/ecosnap.postman_environment.json", "internal/delivery/http/router.go",
                        "internal/delivery/http/support_routes.go"):
            target = self.root / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / relative, target)

    def tearDown(self):
        self.temp.cleanup()

    def run_cli(self):
        return subprocess.run([sys.executable, str(CLI), "--root", str(self.root)],
                              capture_output=True, text=True, check=False)

    def mutate_json(self, relative, change):
        path = self.root / relative
        value = json.loads(path.read_text(encoding="utf-8"))
        change(value)
        path.write_text(json.dumps(value), encoding="utf-8")

    def test_valid_repository_contracts(self):
        result = self.run_cli()
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_openapi_new_route_requires_postman_coverage(self):
        path = self.root / "docs/api/openapi.yaml"
        text = path.read_text(encoding="utf-8").replace(
            "paths:\n",
            "paths:\n  /api/v1/phantom:\n    get:\n      operationId: phantomRoute\n      responses:\n        '200': {description: ok}\n",
            1,
        )
        path.write_text(text, encoding="utf-8")
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Postman missing OpenAPI route", result.stderr)

    def test_duplicate_operation_id_is_rejected(self):
        path = self.root / "docs/api/openapi.yaml"
        text = path.read_text(encoding="utf-8").replace("operationId: getHealth", "operationId: getAuthIdentity", 1)
        path.write_text(text, encoding="utf-8")
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("DuplicateOperationIDError", result.stderr)

    def test_duplicate_yaml_key_is_rejected(self):
        path = self.root / "docs/api/openapi.yaml"
        with path.open("a", encoding="utf-8") as stream:
            stream.write("\nopenapi: 3.0.3\n")
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("duplicate YAML key", result.stderr)

    def test_environment_credential_must_be_empty_without_echoing_value(self):
        secret = "do-not-print-this-token"
        self.mutate_json("docs/api/ecosnap.postman_environment.json",
                         lambda value: next(entry for entry in value["values"] if entry["key"] == "user_token").update(value=secret))
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("credential variable user_token must be empty", result.stderr)
        self.assertNotIn(secret, result.stderr)

    def test_duplicate_environment_credential_cannot_hide_secret_value(self):
        secret = "duplicate-hidden-token"
        self.mutate_json(
            "docs/api/ecosnap.postman_environment.json",
            lambda value: value["values"].append({"key": "user_token", "value": secret}),
        )
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("duplicate variable key user_token", result.stderr)
        self.assertNotIn(secret, result.stderr)

    def test_invalid_postman_shape_is_rejected(self):
        self.mutate_json("docs/api/ecosnap.postman_collection.json", lambda value: value.update(item="not-an-array"))
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Postman schema validation failed", result.stderr)

    def test_invalid_internal_reference_is_rejected(self):
        path = self.root / "docs/api/openapi.yaml"
        text = path.read_text(encoding="utf-8").replace(
            "#/components/schemas/HealthStatus", "#/components/schemas/NotARealSchema", 1
        )
        path.write_text(text, encoding="utf-8")
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("OpenAPI validation failed", result.stderr)

    def test_external_reference_is_rejected_without_fetching(self):
        path = self.root / "docs/api/openapi.yaml"
        text = path.read_text(encoding="utf-8").replace(
            "#/components/schemas/HealthStatus",
            "https://example.invalid/remote-schema.json",
            1,
        )
        path.write_text(text, encoding="utf-8")
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("external OpenAPI $ref is not allowed", result.stderr)

    def test_router_drift_requires_openapi_update(self):
        path = self.root / "internal/delivery/http/router.go"
        with path.open("a", encoding="utf-8") as stream:
            stream.write('\nfunc drift(mux *http.ServeMux) { mux.HandleFunc("GET /api/v1/drift", handler.Health) }\n')
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("OpenAPI missing router route", result.stderr)

    def test_literal_go_route_is_not_absorbed_by_parameter_template(self):
        path = self.root / "internal/delivery/http/router.go"
        with path.open("a", encoding="utf-8") as stream:
            stream.write('\nfunc featured(mux *http.ServeMux) { mux.HandleFunc("GET /api/v1/partners/featured", handler.Health) }\n')
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("OpenAPI missing router route", result.stderr)

    def test_unresolved_postman_route_is_rejected(self):
        self.mutate_json("docs/api/ecosnap.postman_collection.json", lambda value: value["item"][0]["item"][0]["request"]["url"].update(raw="{{missing_base}}/health"))
        result = self.run_cli()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unresolved route", result.stderr)


if __name__ == "__main__":
    unittest.main()
