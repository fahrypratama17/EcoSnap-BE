#!/usr/bin/env python3
"""Validate EcoSnap's OpenAPI, Postman collection/environment, and router parity."""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from urllib.parse import urlsplit

import yaml
from jsonschema import Draft4Validator
from openapi_spec_validator import validate

METHODS = {"get", "put", "post", "delete", "options", "head", "patch", "trace"}
ROUTE_RE = re.compile(r'\.Handle(?:Func)?\(\s*"([A-Z]+) ([^"?]+)"')
PARAM_RE = re.compile(r"\{[^{}]+\}")
SECRET_KEY_RE = re.compile(r"(?:token|secret|password|credential|api[_-]?key)", re.I)


class ContractError(Exception):
    pass


class UniqueSafeLoader(yaml.SafeLoader):
    pass


def _unique_mapping(loader, node, deep=False):
    result = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=deep)
        if key in result:
            raise ContractError(f"duplicate YAML key: {key}")
        result[key] = loader.construct_object(value_node, deep=deep)
    return result


UniqueSafeLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, _unique_mapping)


def fail(message: str) -> None:
    raise ContractError(message)


def load_yaml(path: Path):
    try:
        with path.open(encoding="utf-8") as stream:
            return yaml.load(stream, Loader=UniqueSafeLoader)
    except ContractError as exc:
        fail(f"{path}: {exc}")
    except (OSError, UnicodeError, yaml.YAMLError) as exc:
        fail(f"{path}: invalid YAML ({type(exc).__name__})")


def load_json(path: Path):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        fail(f"{path}: invalid JSON ({type(exc).__name__})")


def route_key(method: str, path: str) -> tuple[str, str]:
    path = re.sub(r"\{\{[^{}]+\}\}", "{}", path)
    path = PARAM_RE.sub("{}", path)
    return method.upper(), path


def concrete_matches_template(route: tuple[str, str], template: tuple[str, str]) -> bool:
    method, path = route
    template_method, template_path = template
    if method != template_method:
        return False
    parts, template_parts = path.split("/"), template_path.split("/")
    return len(parts) == len(template_parts) and all(
        expected == actual or (expected == "{}" and bool(actual) and actual != "{}")
        for actual, expected in zip(parts, template_parts)
    )


def map_postman_routes(routes: set[tuple[str, str]], spec_routes: set[tuple[str, str]]) -> set[tuple[str, str]]:
    mapped = set()
    for route in routes:
        if route in spec_routes:
            mapped.add(route)
            continue
        matches = [template for template in spec_routes if concrete_matches_template(route, template)]
        if matches:
            specificity = max(sum(part != "{}" for part in template[1].split("/")) for template in matches)
            best = [template for template in matches
                    if sum(part != "{}" for part in template[1].split("/")) == specificity]
            if len(best) != 1:
                fail(f"docs/api/ecosnap.postman_collection.json: ambiguous route {route[0]} {route[1]}")
            mapped.add(best[0])
        else:
            mapped.add(route)
    return mapped

def collect_routes(root: Path) -> set[tuple[str, str]]:
    found = set()
    files = [root / "internal/delivery/http/router.go"]
    files.extend(sorted((root / "internal/delivery/http").glob("*_routes.go")))
    for path in files:
        if not path.is_file():
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except (OSError, UnicodeError):
            fail(f"{path}: cannot read router source")
        found.update(route_key(method, route) for method, route in ROUTE_RE.findall(text))
    if not found:
        fail("internal/delivery/http/router.go: no literal Handle/HandleFunc routes found")
    return found


def openapi_routes(spec: dict) -> tuple[set[tuple[str, str]], list[str]]:
    routes, operation_ids = set(), []
    for path, path_item in (spec.get("paths") or {}).items():
        if not isinstance(path_item, dict):
            continue
        for method, operation in path_item.items():
            if method.lower() not in METHODS or not isinstance(operation, dict):
                continue
            routes.add(route_key(method, path))
            operation_id = operation.get("operationId")
            if isinstance(operation_id, str):
                operation_ids.append(operation_id)
    duplicates = sorted(value for value in set(operation_ids) if operation_ids.count(value) > 1)
    if duplicates:
        fail(f"docs/api/openapi.yaml: duplicate operationId: {', '.join(duplicates)}")
    return routes, operation_ids


def walk_requests(items):
    for item in items or []:
        if not isinstance(item, dict):
            continue
        request = item.get("request")
        if isinstance(request, dict):
            yield item, request
        yield from walk_requests(item.get("item"))


def postman_routes(collection: dict) -> set[tuple[str, str]]:
    routes = set()
    for item, request in walk_requests(collection.get("item")):
        method = request.get("method")
        url = request.get("url")
        raw = url.get("raw", "") if isinstance(url, dict) else url if isinstance(url, str) else ""
        if not isinstance(method, str) or not isinstance(raw, str):
            continue
        # Resolve the collection's conventional environment base URL before parsing its path.
        raw = raw.replace("{{base_url}}", "http://contract.invalid")
        path = urlsplit(raw).path
        path = re.sub(r":([A-Za-z_][A-Za-z0-9_]*)", r"{\1}", path)
        if path and not path.startswith("{{"):
            routes.add(route_key(method, path))
        elif path.startswith("{{"):
            fail(f"docs/api/ecosnap.postman_collection.json: request {item.get('name', '<unnamed>')} has unresolved route")
    return routes


def check_environment(environment: dict, collection: dict) -> None:
    env_values = environment.get("values", [])
    if not isinstance(env_values, list):
        fail("docs/api/ecosnap.postman_environment.json: values must be an array")
    entries = {}
    for entry in env_values:
        if not isinstance(entry, dict):
            continue
        key, value = entry.get("key"), entry.get("value")
        if isinstance(key, str) and key in entries:
            fail(f"docs/api/ecosnap.postman_environment.json: duplicate variable key {key}")
        if isinstance(key, str):
            entries[key] = value
        if isinstance(key, str) and SECRET_KEY_RE.search(key) and value not in (None, ""):
            fail(f"docs/api/ecosnap.postman_environment.json: credential variable {key} must be empty")
    base = entries.get("base_url")
    if not isinstance(base, str) or not base.strip():
        fail("docs/api/ecosnap.postman_environment.json: base_url must be a nonempty string")

    def check_vars(variables, scope):
        if not isinstance(variables, list):
            return
        for entry in variables:
            if not isinstance(entry, dict):
                continue
            key, value = entry.get("key", ""), entry.get("value", "")
            if isinstance(key, str) and SECRET_KEY_RE.search(key) and value not in (None, ""):
                fail(f"docs/api/ecosnap.postman_collection.json: {scope} credential variable {key} must be empty")
            if scope == "collection" and isinstance(key, str) and key in entries:
                fail(f"docs/api/ecosnap.postman_collection.json: collection variable {key} shadows environment")

    check_vars(collection.get("variable"), "collection")

    def nested(items):
        for item in items or []:
            if not isinstance(item, dict):
                continue
            check_vars(item.get("variable"), "item")
            nested(item.get("item"))

    nested(collection.get("item"))


def reject_external_refs(value, path: str) -> None:
    if isinstance(value, dict):
        ref = value.get("$ref")
        if isinstance(ref, str) and not ref.startswith("#/"):
            fail(f"{path}: external OpenAPI $ref is not allowed")
        for child in value.values():
            reject_external_refs(child, path)
    elif isinstance(value, list):
        for child in value:
            reject_external_refs(child, path)



def validate_contracts(root: Path) -> None:
    api = root / "docs/api"
    openapi_path = api / "openapi.yaml"
    collection_path = api / "ecosnap.postman_collection.json"
    environment_path = api / "ecosnap.postman_environment.json"
    spec = load_yaml(openapi_path)
    reject_external_refs(spec, str(openapi_path))
    try:
        validate(spec)
    except Exception as exc:
        fail(f"{openapi_path}: OpenAPI validation failed ({type(exc).__name__})")
    collection = load_json(collection_path)
    # Vendored official schema: https://schema.postman.com/json/collection/v2.1.0/collection.json
    schema_path = Path(__file__).with_name("postman-collection-v2.1.schema.json")
    schema = load_json(schema_path)
    errors = sorted(Draft4Validator(schema).iter_errors(collection), key=lambda e: list(map(str, e.path)))
    if errors:
        fail(f"{collection_path}: Postman schema validation failed at {list(errors[0].absolute_path)}")
    environment = load_json(environment_path)
    check_environment(environment, collection)
    spec_routes, _ = openapi_routes(spec)
    pm_routes = map_postman_routes(postman_routes(collection), spec_routes)
    router_routes = collect_routes(root)
    for expected, candidates, label in (
        (spec_routes, pm_routes, "Postman missing OpenAPI route(s)"),
        (pm_routes, spec_routes, "Postman has route(s) absent from OpenAPI"),
        (router_routes, spec_routes, "OpenAPI missing router route(s)"),
        (spec_routes, router_routes, "OpenAPI has route(s) absent from router"),
    ):
        missing = sorted(expected - candidates)
        if missing:
            fail(f"{label}: {', '.join(f'{m} {p}' for m, p in missing)}")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parent.parent, help="repository root")
    args = parser.parse_args()
    try:
        validate_contracts(args.root.resolve())
    except ContractError as exc:
        print(f"API contract validation failed: {exc}", file=sys.stderr)
        return 1
    print("API contracts valid: OpenAPI, Postman, environment, and router routes match.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
