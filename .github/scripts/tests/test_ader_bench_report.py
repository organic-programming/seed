#!/usr/bin/env python3

from __future__ import annotations

import importlib.util
import json
import tempfile
import types
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "ader_bench_report.py"
SPEC = importlib.util.spec_from_file_location("ader_bench_report", SCRIPT)
assert SPEC and SPEC.loader
bench = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(bench)


class AderBenchReportTests(unittest.TestCase):
    def test_pass_fixture_reports_functional_pass(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            report_dir = root / "ader" / "reports" / "bench" / "100"
            write_blocks(report_dir, [("bootstrap", "PASS", "0", 3, "")])
            child = root / "ader" / "catalogues" / "grace-op" / "reports" / "child-pass"
            write_child(child, "PASS", [])
            write_bouquet(root, "local-dev", "PASS", [entry("grace-op", "op_version", "PASS", child)])

            report = generate(root, report_dir, "local-dev")
            self.assertEqual(report["outcome"]["functional_status"], "PASS")
            self.assertEqual(report["outcome"]["failed_entry_count"], 0)
            self.assertEqual(report["outcome"]["failed_step_count"], 0)
            self.assertIn("Functional status: `PASS`", (report_dir / "summary.md").read_text())
            self.assertEqual((report_dir / "failed-steps.tsv").read_text().strip(), "catalogue\tsuite\tchild_history_id\tstep_id\tstatus\tduration_seconds\treason\tlog_path")

    def test_failed_step_is_reported_with_reason_and_log(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            report_dir = root / "ader" / "reports" / "bench" / "101"
            write_blocks(report_dir, [("ader-bouquet", "FAIL", "1", 10, "ader bouquet failed")])
            child = root / "ader" / "catalogues" / "swiftui" / "reports" / "child-fail"
            write_child(
                child,
                "FAIL",
                [
                    {
                        "step_id": "integration-build-composite-swiftui",
                        "status": "FAIL",
                        "duration_seconds": 42,
                        "reason": "xcodebuild failed",
                        "log_path": "logs/integration-build-composite-swiftui.log",
                    }
                ],
            )
            write_bouquet(root, "cross-platform", "FAIL", [entry("swiftui", "swiftui-smoke", "FAIL", child, "suite failed")])

            report = generate(root, report_dir, "cross-platform")
            self.assertEqual(report["outcome"]["functional_status"], "FAIL")
            self.assertEqual(report["outcome"]["failed_entry_count"], 1)
            self.assertEqual(report["outcome"]["failed_step_count"], 1)
            failed_steps = (report_dir / "failed-steps.tsv").read_text()
            self.assertIn("integration-build-composite-swiftui", failed_steps)
            self.assertIn("xcodebuild failed", failed_steps)
            self.assertIn("logs/integration-build-composite-swiftui.log", failed_steps)

    def test_failed_step_without_reason_points_to_log_path(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            report_dir = root / "ader" / "reports" / "bench" / "103"
            write_blocks(report_dir, [("ader-bouquet", "FAIL", "1", 10, "ader bouquet failed")])
            child = root / "ader" / "catalogues" / "swiftui" / "reports" / "child-fail-no-reason"
            write_child(
                child,
                "FAIL",
                [
                    {
                        "step_id": "integration-build-composite-swiftui",
                        "status": "FAIL",
                        "duration_seconds": 42,
                        "log_path": "logs/integration-build-composite-swiftui.log",
                    }
                ],
            )
            write_bouquet(root, "cross-platform", "FAIL", [entry("swiftui", "swiftui-smoke", "FAIL", child)])

            report = generate(root, report_dir, "cross-platform")
            self.assertEqual(report["bouquet_entries"][0]["reason"], "see child_report_dir")
            self.assertEqual(report["failed_steps"][0]["reason"], "see log_path")

    def test_bootstrap_failure_reports_without_bouquet_report(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            report_dir = root / "ader" / "reports" / "bench" / "102"
            write_blocks(report_dir, [("bootstrap", "FAIL", "2", 7, "op build failed")])

            report = generate(root, report_dir, "local-dev")
            self.assertEqual(report["outcome"]["functional_status"], "FAIL")
            self.assertEqual(report["outcome"]["failed_block"], "bootstrap")
            self.assertEqual(report["outcome"]["bouquet_report_dir"], "")
            self.assertIn("No bouquet entry report was found.", (report_dir / "summary.md").read_text())

    def test_ignores_bouquet_reports_started_before_runner_acquired(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            report_dir = root / "ader" / "reports" / "bench" / "104"
            write_blocks(report_dir, [("ader-bouquet", "FAIL", "1", 10, "ader bouquet failed")])
            child = root / "ader" / "catalogues" / "grace-op" / "reports" / "old-child"
            write_child(child, "PASS", [])
            write_bouquet(
                root,
                "local-dev",
                "PASS",
                [entry("grace-op", "op_version", "PASS", child)],
                started_at="2026-05-02T23:59:00Z",
            )

            report = generate(root, report_dir, "local-dev", runner_acquired_at="2026-05-03T00:01:00Z")
            self.assertEqual(report["outcome"]["functional_status"], "FAIL")
            self.assertEqual(report["bouquet_entries"], [])
            self.assertEqual(report["outcome"]["bouquet_report_dir"], "")


def generate(root: Path, report_dir: Path, bouquet: str, runner_acquired_at: str = "2026-05-03T00:01:00Z") -> dict:
    args = types.SimpleNamespace(
        repo_root=str(root),
        report_dir=str(report_dir),
        bouquet=bouquet,
        run_id=report_dir.name,
        ref="test-ref",
        sha="abc123",
        runner_name="self-hosted-macos",
        runner_os="macOS",
        runner_arch="ARM64",
        cache_note="fixture",
        run_note="",
        github_created_at="2026-05-03T00:00:00Z",
        runner_acquired_at=runner_acquired_at,
        report_started_at="2026-05-03T00:02:00Z",
    )
    report = bench.build_report(args)
    bench.write_report(report_dir, report)
    return report


def write_blocks(report_dir: Path, rows: list[tuple[str, str, str, int, str]]) -> None:
    report_dir.mkdir(parents=True, exist_ok=True)
    lines = ["block\tstatus\texit_code\tstarted_at\tfinished_at\tseconds\treason"]
    for name, status, exit_code, seconds, reason in rows:
        lines.append(f"{name}\t{status}\t{exit_code}\t2026-05-03T00:01:00Z\t2026-05-03T00:01:{seconds:02d}Z\t{seconds}\t{reason}")
    (report_dir / "blocks.tsv").write_text("\n".join(lines) + "\n", encoding="utf-8")


def write_bouquet(root: Path, bouquet: str, status: str, entries: list[dict], started_at: str = "2026-05-03T00:01:00Z") -> None:
    report_dir = root / "ader" / "reports" / "bouquets" / f"{bouquet}-20260503"
    report_dir.mkdir(parents=True, exist_ok=True)
    manifest = {
        "bouquet": bouquet,
        "history_id": report_dir.name,
        "report_dir": str(report_dir),
        "started_at": started_at,
        "finished_at": "2026-05-03T00:02:00Z",
        "final_status": status,
    }
    (report_dir / "manifest.json").write_text(json.dumps(manifest), encoding="utf-8")
    (report_dir / "bouquet-entry-results.json").write_text(json.dumps(entries), encoding="utf-8")


def write_child(child: Path, status: str, steps: list[dict]) -> None:
    child.mkdir(parents=True, exist_ok=True)
    manifest = {
        "history_id": child.name,
        "report_dir": str(child),
        "started_at": "2026-05-03T00:01:00Z",
        "finished_at": "2026-05-03T00:02:00Z",
        "final_status": status,
    }
    (child / "manifest.json").write_text(json.dumps(manifest), encoding="utf-8")
    (child / "step-results.json").write_text(json.dumps(steps), encoding="utf-8")


def entry(catalogue: str, suite: str, status: str, child: Path, reason: str = "") -> dict:
    return {
        "catalogue": catalogue,
        "suite": suite,
        "profile": "smoke",
        "lane": "both",
        "source": "workspace",
        "archive_policy": "never",
        "final_status": status,
        "reason": reason,
        "child_history_id": child.name,
        "child_report_dir": str(child),
        "started_at": "2026-05-03T00:01:00Z",
        "finished_at": "2026-05-03T00:02:00Z",
    }


if __name__ == "__main__":
    unittest.main()
