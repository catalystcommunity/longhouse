"""Runnerlib lifecycle jobs for Longhouse CI and release workflows."""

from __future__ import annotations

import os
import re
import shlex
import subprocess
from pathlib import Path
from typing import Callable, Dict, List, Optional

from src.logging import log_stdout
from src.plugins import Plugin, PluginContext, PluginPhase


CONVENTIONAL_COMMIT_PATTERN = re.compile(
    r"^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)"
    r"(\(.+\))?!?: .+"
)


def _run(
    command: List[str],
    *,
    cwd: Path,
    capture_output: bool = False,
    env: Optional[Dict[str, str]] = None,
) -> subprocess.CompletedProcess:
    log_stdout(f"Running: {shlex.join(command)}")
    return subprocess.run(
        command,
        cwd=cwd,
        check=True,
        text=True,
        capture_output=capture_output,
        env=env,
    )


def _script(code_dir: Path, name: str) -> None:
    env = os.environ.copy()
    env["REACTORCIDE_REPOROOT"] = str(code_dir)
    _run(
        ["bash", str(code_dir / ".reactorcide" / "jobs" / "scripts" / name)],
        cwd=code_dir,
        env=env,
    )


def _git_ref_exists(code_dir: Path, ref: str) -> bool:
    result = subprocess.run(
        ["git", "rev-parse", "--verify", "--quiet", f"{ref}^{{commit}}"],
        cwd=code_dir,
        check=False,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    return result.returncode == 0


def _merge_base(code_dir: Path, ref: str) -> Optional[str]:
    if not _git_ref_exists(code_dir, ref):
        return None
    result = _run(
        ["git", "merge-base", "HEAD", ref],
        cwd=code_dir,
        capture_output=True,
    )
    value = result.stdout.strip()
    return value or None


def _find_diff_base(code_dir: Path) -> Optional[str]:
    explicit_base = os.environ.get("REACTORCIDE_DIFF_BASE", "").strip()
    if explicit_base:
        if not _git_ref_exists(code_dir, explicit_base):
            raise RuntimeError(f"REACTORCIDE_DIFF_BASE is not a commit: {explicit_base}")
        return explicit_base

    base_branch = (
        os.environ.get("REACTORCIDE_BASE_REF")
        or os.environ.get("REACTORCIDE_PR_BASE_REF")
        or "main"
    )
    for candidate in (f"upstream/{base_branch}", f"origin/{base_branch}", base_branch):
        base = _merge_base(code_dir, candidate)
        if base:
            return base

    if _git_ref_exists(code_dir, "HEAD^"):
        result = _run(["git", "rev-parse", "HEAD^"], cwd=code_dir, capture_output=True)
        return result.stdout.strip()

    return None


def _commit_records(code_dir: Path, diff_base: Optional[str]) -> List[tuple[str, str]]:
    revision = f"{diff_base}..HEAD" if diff_base else "HEAD"
    result = _run(
        ["git", "log", revision, "--pretty=format:%H%x00%s"],
        cwd=code_dir,
        capture_output=True,
    )
    records: List[tuple[str, str]] = []
    for line in result.stdout.splitlines():
        commit_hash, separator, subject = line.partition("\0")
        if separator:
            records.append((commit_hash, subject))
    return records


def conventional_commits(code_dir: Path) -> None:
    log_stdout("Validating conventional commits")
    failed: List[str] = []
    for commit_hash, subject in _commit_records(code_dir, _find_diff_base(code_dir)):
        if CONVENTIONAL_COMMIT_PATTERN.fullmatch(subject):
            log_stdout(f"OK: {subject}")
        else:
            log_stdout(f"FAIL: {subject} ({commit_hash})")
            failed.append(subject)

    if failed:
        raise RuntimeError(
            "Commit messages must match 'type(scope)?: description'. "
            "Valid types: feat, fix, docs, style, refactor, perf, test, "
            "build, ci, chore, and revert."
        )

    log_stdout("All commits follow the conventional commit format.")


def build_test(code_dir: Path) -> None:
    env = os.environ.copy()
    home = Path(env.get("HOME", "/home/runner"))
    env["GOPATH"] = str(home / "go")
    env["GOMODCACHE"] = str(home / "go" / "pkg" / "mod")
    env["GOCACHE"] = str(home / ".cache" / "go-build")
    _run(["go", "build", "./..."], cwd=code_dir / "api", env=env)
    _run(["go", "test", "./..."], cwd=code_dir / "api", env=env)


LONGHOUSE_JOBS: Dict[str, Callable[[Path], None]] = {
    "conventional-commits": conventional_commits,
    "build-test": build_test,
    "api-build-test": lambda code_dir: _script(code_dir, "api-build-test.sh"),
    "web-build-test": lambda code_dir: _script(code_dir, "web-build-test.sh"),
    "api-test-postgres": lambda code_dir: _script(code_dir, "api-test-postgres.sh"),
    "webapp-test": lambda code_dir: _script(code_dir, "webapp-test.sh"),
    "api-build-and-deploy": lambda code_dir: _script(code_dir, "api-build-and-deploy.sh"),
    "web-build-and-deploy": lambda code_dir: _script(code_dir, "web-build-and-deploy.sh"),
    "deploy": lambda code_dir: _script(code_dir, "deploy.sh"),
    "release": lambda code_dir: _script(code_dir, "release.sh"),
}


class LonghouseJobsPlugin(Plugin):
    """Run one selected Longhouse job after runnerlib prepares source."""

    def __init__(self):
        super().__init__(name="longhouse_jobs", priority=50)

    def supported_phases(self):
        return [PluginPhase.POST_SOURCE_PREP]

    def execute(self, context: PluginContext) -> None:
        if context.phase != PluginPhase.POST_SOURCE_PREP:
            return

        job_name = os.environ.get("REACTORCIDE_LONGHOUSE_JOB", "").strip()
        if not job_name:
            return

        job = LONGHOUSE_JOBS.get(job_name)
        if job is None:
            names = ", ".join(sorted(LONGHOUSE_JOBS))
            raise RuntimeError(
                f"Unknown REACTORCIDE_LONGHOUSE_JOB '{job_name}'. Valid jobs: {names}"
            )

        code_dir = Path(context.config.code_dir)
        if not code_dir.is_dir():
            raise RuntimeError(f"Code directory does not exist: {code_dir}")

        log_stdout(f"Starting Longhouse runnerlib lifecycle job: {job_name}")
        job(code_dir)
        log_stdout(f"Completed Longhouse runnerlib lifecycle job: {job_name}")
