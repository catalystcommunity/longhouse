import base64
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).with_name("application-keys.py")
spec = importlib.util.spec_from_file_location("application_keys", SCRIPT)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def sample_keys(domain="todandlorna.com"):
    return {"application": {"subject_domain": domain, "subject_user_id": "account", "application_id": "longhouse"},
            "instance_id": "test-instance", "keys": [
                {"key_id": str(i), "seed": base64.b64encode(bytes([i]) * 32).decode()} for i in range(1, 4)]}


class ApplicationKeyDeploymentTests(unittest.TestCase):
    def environment(self, keys):
        return {"LONGHOUSE_APPLICATION_KEYS": json.dumps(keys),
                "LONGHOUSE_APPLICATION_KEYS_SECRET": "longhouse-application-keys", "K8S_NAMESPACE": "longhouse"}

    def test_manifest_sends_values_only_through_stdin(self):
        keys = sample_keys()
        with patch.dict(os.environ, self.environment(keys), clear=True), patch.object(module.subprocess, "run") as run:
            run.return_value.returncode = 0
            module.main()
        args, kwargs = run.call_args
        manifest = json.loads(kwargs["input"])
        self.assertEqual(manifest["metadata"], {"name": "longhouse-application-keys", "namespace": "longhouse"})
        self.assertEqual(json.loads(base64.b64decode(manifest["data"]["keys.json"])), keys)
        self.assertNotIn(keys["keys"][0]["seed"], " ".join(args[0]))
        self.assertIn("--server-side", args[0])

    def test_wrong_domain_and_repeated_keys_do_not_touch_cluster(self):
        bad_domain = sample_keys("catalystsquad.com")
        repeated = sample_keys()
        repeated["keys"][1] = repeated["keys"][0]
        for keys in (bad_domain, repeated):
            with self.subTest(keys=keys["application"]), patch.dict(os.environ, self.environment(keys), clear=True), patch.object(module.subprocess, "run") as run:
                with self.assertRaises(ValueError):
                    module.main()
                run.assert_not_called()

    def test_missing_required_secret_is_an_error(self):
        with patch.dict(os.environ, {"LONGHOUSE_APPLICATION_KEYS_SECRET": "required"}, clear=True):
            with self.assertRaises(ValueError):
                module.main()

    def test_kubectl_failure_does_not_expose_payload(self):
        with tempfile.TemporaryDirectory() as directory:
            fake = Path(directory) / "kubectl"
            fake.write_text("#!/bin/sh\ncat >&2\nexit 1\n")
            fake.chmod(0o700)
            env = {**os.environ, **self.environment(sample_keys()), "PATH": directory + os.pathsep + os.environ["PATH"]}
            result = subprocess.run([os.sys.executable, str(SCRIPT)], env=env, capture_output=True)
            self.assertEqual(result.returncode, 1)
            output = (result.stdout + result.stderr).decode()
            self.assertNotIn(sample_keys()["keys"][0]["seed"], output)
            self.assertNotIn("keys.json", output)
            self.assertIn("secret values were suppressed", output)


if __name__ == "__main__":
    unittest.main()
