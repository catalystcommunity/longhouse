#!/usr/bin/env python3
"""Enroll Longhouse keys and store them in both Reactorcide secret stores."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile

ENVIRONMENTS = {
    "todandlorna": ("todandlorna.com", "tnl/longhouse", "019df12e-e9d7-7510-9818-b561728e79e6"),
    "catalystsquad": ("catalystsquad.com", "catalystsquad/longhouse", "019eb837-2742-7d70-a0f9-3f5171da7e2e"),
}
API_URL = "https://reactorcide.catalystsquad.com"


def run(command, *, env=None, data=None, stage):
    result = subprocess.run(command, env=env, input=data, capture_output=True)
    if result.returncode:
        # Tool errors can contain inputs. Do not copy them into operator logs.
        raise RuntimeError(f"{stage} failed (exit {result.returncode}); secret values were suppressed")
    return result.stdout


def provision(args):
    local = os.environ.copy()
    for key in ("REACTORCIDE_API_URL", "REACTORCIDE_API_TOKEN"):
        local.pop(key, None)
    local["REACTORCIDE_SECRETS_PASSWORD"] = Path(args.password_file).read_text().strip()
    remote = local.copy()
    remote.pop("REACTORCIDE_SECRETS_PASSWORD")
    remote["REACTORCIDE_API_URL"] = API_URL
    remote["REACTORCIDE_API_TOKEN"] = run(
        ["reactorcide", "secrets", "get", "reactorcide/k8s", "api_token"],
        env=local, stage="Read coordinator credential",
    ).decode().strip()
    selected = ENVIRONMENTS if args.env == "both" else {args.env: ENVIRONMENTS[args.env]}
    previous = None
    for name, (domain, path, expected_user) in selected.items():
        def secret(environment, operation, key=None, data=None):
            command = ["reactorcide", "secrets", operation, path]
            if key:
                command.append(key)
            if operation == "set":
                command.append("--stdin")
            return run(command, env=environment, data=data, stage=f"{name}: secret {operation}")

        credential = secret(local, "get", "linkkeys_api_key").strip()
        if credential != secret(remote, "get", "linkkeys_api_key").strip():
            raise RuntimeError(f"{name}: local and remote RP credentials differ")
        connection = [f"--domain={domain}"]
        address = getattr(args, name + "_tcp_addr")
        if address:
            connection.append(f"--tcp-addr={address}")
        command = [args.binary, "application-keys"]
        identity = json.loads(run(
            command + ["--action=inspect", "--account-api-key-file=/dev/stdin"] + connection,
            data=credential, stage=f"{name}: inspect existing account",
        ))
        if identity["user_id"] != expected_user or not identity["active"]:
            raise RuntimeError(f"{name}: the credential does not name the expected active account")
        print(f"{name}: confirmed existing account {identity['display_name']}", flush=True)
        found = []
        for environment in (local, remote):
            names = secret(environment, "list").decode().splitlines()
            found.append(secret(environment, "get", "application_keys") if "application_keys" in names else None)
        if all(found) and json.loads(found[0]) != json.loads(found[1]):
            raise RuntimeError(f"{name}: stored application keys differ; no keys were replaced")
        with tempfile.TemporaryDirectory(prefix="longhouse-application-keys-") as directory:
            keyfile = Path(directory) / "keys.json"
            raw = found[0] or found[1]
            if raw is None:
                run(command + ["--action=generate", f"--file={keyfile}", f"--domain={domain}",
                               f"--user-id={expected_user}", "--application-id=longhouse",
                               f"--instance-id=longhouse-{name}-1"], stage=f"{name}: generate keys")
                raw = keyfile.read_bytes()
            else:
                with keyfile.open("xb") as output:
                    os.chmod(keyfile, 0o600)
                    output.write(raw)
            keys = json.loads(raw)
            expected = {"subject_domain": domain, "subject_user_id": expected_user, "application_id": "longhouse"}
            if keys["application"] != expected or keys["instance_id"] != f"longhouse-{name}-1":
                raise RuntimeError(f"{name}: stored keys name a different application")
            seeds = {key["seed"] for key in keys["keys"]}
            if len(seeds) != 3 or (previous is not None and seeds.intersection(previous)):
                raise RuntimeError("Each deployment must have three different signing keys")
            previous = seeds
            # Save recoverable copies before enrollment. A retry reuses them.
            for environment, existing in zip((local, remote), found):
                if existing is None:
                    secret(environment, "set", "application_keys", data=raw)
                if json.loads(secret(environment, "get", "application_keys")) != keys:
                    raise RuntimeError(f"{name}: secret read-back did not match")
            print(f"{name}: application keys verified in local and production secret stores", flush=True)
            run(command + ["--action=ensure", f"--file={keyfile}",
                           "--account-api-key-file=/dev/stdin"] + connection,
                data=credential, stage=f"{name}: enroll or renew application keys")
            print(f"{name}: enrollment and public key verification passed", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--env", choices=["both", *ENVIRONMENTS], default="both")
    parser.add_argument("--binary", default="longhouse")
    parser.add_argument("--password-file", default=str(Path.home() / ".reactorcide-pass"))
    for name in ENVIRONMENTS:
        parser.add_argument(f"--{name}-tcp-addr", default="")
    args = parser.parse_args()
    try:
        provision(args)
    except (RuntimeError, OSError, ValueError, KeyError) as exc:
        # JSON decoder errors can contain secret input. Print only controlled errors.
        message = str(exc) if isinstance(exc, RuntimeError) else "Provisioning failed; secret values were suppressed"
        parser.exit(1, message + "\n")


if __name__ == "__main__":
    main()
