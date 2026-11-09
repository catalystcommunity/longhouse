#!/usr/bin/env python3
"""Load the enrolled application keys into the deployment's Kubernetes Secret."""
import base64
import json
import os
import subprocess
import sys


def main():
    raw = os.environ.get("LONGHOUSE_APPLICATION_KEYS", "")
    name = os.environ.get("LONGHOUSE_APPLICATION_KEYS_SECRET", "")
    if not raw and not name:
        return 0
    if not raw or not name:
        raise ValueError("application keys and Secret name must both be set")
    keys = json.loads(raw)
    application = keys["application"]
    if not all(application.get(k) for k in ("subject_domain", "subject_user_id", "application_id")) or not keys["instance_id"]:
        raise ValueError("application identity is incomplete")
    if application["application_id"] != "longhouse" or len(keys["keys"]) != 3:
        raise ValueError("Longhouse needs three application signing keys")
    seeds, ids = set(), set()
    for key in keys["keys"]:
        seed = base64.b64decode(key["seed"], validate=True)
        if len(seed) != 32 or not key["key_id"] or seed in seeds or key["key_id"] in ids:
            raise ValueError("application keys are invalid or repeated")
        seeds.add(seed)
        ids.add(key["key_id"])
    # Bind the secret to the deployment overlay before making a cluster change.
    domains = {"longhouse": "todandlorna.com", "longhouse-catalystsquad": "catalystsquad.com"}
    namespace = os.environ["K8S_NAMESPACE"]
    if namespace in domains and application["subject_domain"] != domains[namespace]:
        raise ValueError("application keys belong to a different deployment")
    manifest = {
        "apiVersion": "v1", "kind": "Secret", "type": "Opaque",
        "metadata": {"name": name, "namespace": namespace},
        "data": {"keys.json": base64.b64encode(raw.encode()).decode()},
    }
    result = subprocess.run(
        ["kubectl", "apply", "--server-side", "--field-manager=longhouse-deploy", "-f", "-"],
        input=json.dumps(manifest).encode(), capture_output=True,
    )
    if result.returncode:
        raise RuntimeError("application key Secret update failed")
    print("Application key Secret is ready")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, KeyError, TypeError, OSError, RuntimeError):
        # kubectl errors and JSON parse errors can contain the secret payload.
        print("Application key setup failed; secret values were suppressed", file=sys.stderr)
        sys.exit(1)
