"""Check the rendered release configuration without contacting Kubernetes."""
import json
from pathlib import Path
import subprocess
import unittest
import yaml

ROOT = Path(__file__).resolve().parents[1]


class ActAsDeploymentTests(unittest.TestCase):
    def test_each_release_uses_its_own_keys(self):
        for environment, namespace, job, secret_path in (
            ("todandlorna", "longhouse", "deploy-tnl-example.yaml", "tnl/longhouse"),
            ("catalystsquad", "longhouse-catalystsquad", "deploy-catalystsquad.yaml", "catalystsquad/longhouse"),
        ):
            with self.subTest(environment=environment):
                raw = subprocess.check_output([
                    "helm", "template", "longhouse", str(ROOT / "helm_chart"), "--namespace", namespace,
                    "-f", str(ROOT / "deploy" / f"values-{environment}.yaml"),
                ], text=True)
                resources = list(yaml.safe_load_all(raw))
                config = next(r for r in resources if r and r["kind"] == "ConfigMap" and r["metadata"]["name"] == "longhouse")
                path = config["data"]["LONGHOUSE_ACT_AS_KEYS_FILE"]
                self.assertNotIn("LONGHOUSE_ACT_AS_KEYS_FILE", config["metadata"])
                deployment = next(r for r in resources if r and r["kind"] == "Deployment" and r["metadata"]["name"] == "longhouse-api")
                pod = deployment["spec"]["template"]
                self.assertIn("checksum/act-as-keys", pod["metadata"]["annotations"])
                container = next(c for c in pod["spec"]["containers"] if c["name"] == "api")
                mount = next(m for m in container["volumeMounts"] if m["name"] == "act-as-keys")
                self.assertTrue(mount["readOnly"])
                self.assertEqual(path, mount["mountPath"] + "/keys.json")
                volume = next(v for v in pod["spec"]["volumes"] if v["name"] == "act-as-keys")
                self.assertEqual(volume["secret"]["secretName"], "longhouse-application-keys")
                job_env = yaml.safe_load((ROOT / ".reactorcide/jobs" / job).read_text())["environment"]
                self.assertEqual(job_env["LONGHOUSE_APPLICATION_KEYS"], "${secret:" + secret_path + ":application_keys}")
                self.assertEqual(job_env["LONGHOUSE_APPLICATION_KEYS_SECRET"], volume["secret"]["secretName"])
                self.assertEqual(job_env["K8S_NAMESPACE"], namespace)


if __name__ == "__main__":
    unittest.main()
