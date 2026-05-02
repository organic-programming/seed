#!/usr/bin/env python3
"""Generate a neutral ader benchmark diagnostic report."""

from __future__ import annotations

import argparse
import csv
import json
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


TSV_NA = ""


def parse_time(raw: str | None) -> datetime | None:
    value = (raw or "").strip()
    if not value:
        return None
    if value.endswith("Z"):
        value = value[:-1] + "+00:00"
    try:
        parsed = datetime.fromisoformat(value)
    except ValueError:
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed.astimezone(timezone.utc)


def iso_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def seconds_between(start: str | None, end: str | None) -> int | None:
    left = parse_time(start)
    right = parse_time(end)
    if left is None or right is None:
        return None
    return max(0, int((right - left).total_seconds()))


def human_seconds(value: int | None) -> str:
    if value is None:
        return "unknown"
    hours, rem = divmod(int(value), 3600)
    minutes, seconds = divmod(rem, 60)
    if hours:
        return f"{hours}h{minutes:02d}m{seconds:02d}s"
    if minutes:
        return f"{minutes}m{seconds:02d}s"
    return f"{seconds}s"


def read_tsv(path: Path) -> list[dict[str, str]]:
    if not path.exists():
        return []
    with path.open("r", encoding="utf-8", newline="") as fh:
        return list(csv.DictReader(fh, delimiter="\t"))


def write_tsv(path: Path, fields: list[str], rows: list[dict[str, Any]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="") as fh:
        writer = csv.DictWriter(fh, fieldnames=fields, delimiter="\t", lineterminator="\n")
        writer.writeheader()
        for row in rows:
            writer.writerow({field: clean_cell(row.get(field, TSV_NA)) for field in fields})


def clean_cell(value: Any) -> str:
    return str(value if value is not None else "").replace("\t", " ").replace("\n", " ").replace("\r", " ")


def read_json(path: Path) -> Any:
    with path.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def status_is_fail(status: str | None) -> bool:
    return (status or "").strip().upper() == "FAIL"


def status_is_pass(status: str | None) -> bool:
    return (status or "").strip().upper() == "PASS"


def normalize_block(row: dict[str, str]) -> dict[str, Any]:
    seconds_raw = (row.get("seconds") or "").strip()
    try:
        seconds: int | None = int(seconds_raw)
    except ValueError:
        seconds = None
    return {
        "block": row.get("block", ""),
        "status": row.get("status", ""),
        "exit_code": row.get("exit_code", ""),
        "started_at": row.get("started_at", ""),
        "finished_at": row.get("finished_at", ""),
        "seconds": seconds,
        "reason": row.get("reason", ""),
    }


def latest_bouquet_report(repo_root: Path, bouquet: str, not_before: datetime | None) -> Path | None:
    reports_root = repo_root / "ader" / "reports" / "bouquets"
    candidates: list[tuple[datetime, Path]] = []
    for manifest_path in reports_root.glob("*/manifest.json"):
        try:
            manifest = read_json(manifest_path)
        except (OSError, json.JSONDecodeError):
            continue
        if manifest.get("bouquet") != bouquet:
            continue
        started = parse_time(manifest.get("started_at"))
        if started is None:
            continue
        if not_before is not None and started < not_before:
            continue
        candidates.append((started, manifest_path.parent))
    if not candidates:
        return None
    candidates.sort(key=lambda item: item[0])
    return candidates[-1][1]


def resolve_existing_path(repo_root: Path, raw: str | None) -> Path | None:
    value = (raw or "").strip()
    if not value:
        return None
    path = Path(value)
    if not path.is_absolute():
        path = repo_root / path
    return path if path.exists() else None


def entry_seconds(entry: dict[str, Any]) -> int | None:
    return seconds_between(entry.get("started_at"), entry.get("finished_at"))


def collect_entries(repo_root: Path, bouquet_report: Path | None) -> list[dict[str, Any]]:
    if bouquet_report is None:
        return []
    entries_path = bouquet_report / "bouquet-entry-results.json"
    if not entries_path.exists():
        return []
    try:
        raw_entries = read_json(entries_path)
    except (OSError, json.JSONDecodeError):
        return []
    entries: list[dict[str, Any]] = []
    for entry in raw_entries:
        reason = entry.get("reason", "")
        if not reason and status_is_fail(entry.get("final_status")) and entry.get("child_report_dir"):
            reason = "see child_report_dir"
        row = {
            "catalogue": entry.get("catalogue", ""),
            "suite": entry.get("suite", ""),
            "profile": entry.get("profile", ""),
            "lane": entry.get("lane", ""),
            "source": entry.get("source", ""),
            "status": entry.get("final_status", ""),
            "seconds": entry_seconds(entry),
            "started_at": entry.get("started_at", ""),
            "finished_at": entry.get("finished_at", ""),
            "child_report_dir": entry.get("child_report_dir", ""),
            "reason": reason,
            "child_history_id": entry.get("child_history_id", ""),
        }
        entries.append(row)
    return entries


def collect_failed_steps(repo_root: Path, entries: list[dict[str, Any]]) -> list[dict[str, Any]]:
    failed: list[dict[str, Any]] = []
    for entry in entries:
        child_dir = resolve_existing_path(repo_root, entry.get("child_report_dir"))
        if child_dir is None:
            continue
        steps_path = child_dir / "step-results.json"
        if not steps_path.exists():
            continue
        try:
            steps = read_json(steps_path)
        except (OSError, json.JSONDecodeError):
            continue
        for step in steps:
            status = step.get("status", "")
            if not status_is_fail(status):
                continue
            reason = step.get("reason", "")
            if not reason and step.get("log_path"):
                reason = "see log_path"
            failed.append(
                {
                    "catalogue": entry.get("catalogue", ""),
                    "suite": entry.get("suite", ""),
                    "child_history_id": entry.get("child_history_id", ""),
                    "step_id": step.get("step_id", ""),
                    "status": status,
                    "duration_seconds": step.get("duration_seconds", ""),
                    "reason": reason,
                    "log_path": step.get("log_path", ""),
                }
            )
    return failed


def first_failure(blocks: list[dict[str, Any]], entries: list[dict[str, Any]], failed_steps: list[dict[str, Any]]) -> str:
    for block in blocks:
        if status_is_fail(block.get("status")):
            reason = block.get("reason") or f"exit code {block.get('exit_code')}"
            return f"block {block.get('block')} failed: {reason}"
    for entry in entries:
        if status_is_fail(entry.get("status")):
            reason = entry.get("reason") or "suite failed"
            return f"{entry.get('catalogue')}/{entry.get('suite')} failed: {reason}"
    if failed_steps:
        step = failed_steps[0]
        reason = step.get("reason") or "step failed"
        return f"{step.get('catalogue')}/{step.get('suite')} step {step.get('step_id')} failed: {reason}"
    return ""


def md(value: Any) -> str:
    return clean_cell(value).replace("|", "\\|")


def md_table(fields: list[str], rows: list[dict[str, Any]]) -> str:
    out = [
        "| " + " | ".join(fields) + " |",
        "| " + " | ".join("---" for _ in fields) + " |",
    ]
    for row in rows:
        out.append("| " + " | ".join(md(row.get(field, "")) for field in fields) + " |")
    return "\n".join(out)


def build_summary_markdown(report: dict[str, Any]) -> str:
    identity = report["identity"]
    wall = report["wall_clock"]
    outcome = report["outcome"]
    lines = [
        "# ader Benchmark Report",
        "",
        "## Run Identity",
        "",
        f"- Run ID: `{identity['run_id']}`",
        f"- Ref: `{identity['ref']}`",
        f"- SHA: `{identity['sha']}`",
        f"- Bouquet: `{identity['bouquet']}`",
        f"- Runner: `{identity['runner_name']}` (`{identity['runner_os']}` / `{identity['runner_arch']}`)",
        f"- Cache note: `{identity['cache_note']}`",
        f"- Run note: `{identity['run_note']}`",
        "",
        "## Wall Clock",
        "",
        md_table(
            ["metric", "value"],
            [
                {"metric": "GitHub created_at", "value": wall["github_created_at"] or "unknown"},
                {"metric": "Runner acquired_at", "value": wall["runner_acquired_at"] or "unknown"},
                {"metric": "Finished_at", "value": wall["finished_at"] or "unknown"},
                {"metric": "Runner wait", "value": human_seconds(wall["runner_wait_seconds"])},
                {"metric": "Executed wall-clock", "value": human_seconds(wall["executed_seconds"])},
            ],
        ),
        "",
        "## Outcome",
        "",
        f"- Functional status: `{outcome['functional_status']}`",
        f"- Failed block: `{outcome['failed_block'] or 'none'}`",
        f"- Failed catalogue/suite count: `{outcome['failed_entry_count']}`",
        f"- Skipped catalogue/suite count: `{outcome['skipped_entry_count']}`",
        f"- Failed internal step count: `{outcome['failed_step_count']}`",
        f"- First failure: `{outcome['first_failure'] or 'none'}`",
        f"- Bouquet report: `{outcome['bouquet_report_dir'] or 'none'}`",
        "",
        "## Blocks",
        "",
        md_table(["block", "status", "exit_code", "seconds", "reason"], report["blocks"]),
        "",
        "## Bouquet Entries",
        "",
    ]
    if report["bouquet_entries"]:
        lines.append(
            md_table(
                ["catalogue", "suite", "profile", "lane", "source", "status", "seconds", "child_report_dir", "reason"],
                report["bouquet_entries"],
            )
        )
    else:
        lines.append("No bouquet entry report was found.")
    lines.extend(["", "## Failures", ""])
    if not report["failed_steps"] and outcome["failed_entry_count"] == 0 and not outcome["failed_block"]:
        lines.append("No functional failures found.")
    else:
        failed_entries = [entry for entry in report["bouquet_entries"] if status_is_fail(entry.get("status"))]
        if failed_entries:
            lines.append("### Failed Catalogue/Suite Entries")
            lines.append("")
            lines.append(md_table(["catalogue", "suite", "status", "seconds", "reason", "child_report_dir"], failed_entries))
            lines.append("")
        if report["failed_steps"]:
            lines.append("### Failed Internal Steps")
            lines.append("")
            lines.append(
                md_table(
                    ["catalogue", "suite", "step_id", "status", "duration_seconds", "reason", "log_path"],
                    report["failed_steps"],
                )
            )
        elif failed_entries:
            lines.append("No failed internal steps were found in child step reports.")
    lines.append("")
    return "\n".join(lines)


def build_report(args: argparse.Namespace) -> dict[str, Any]:
    repo_root = Path(args.repo_root).resolve()
    report_dir = Path(args.report_dir).resolve()
    blocks_path = report_dir / "blocks.tsv"
    raw_blocks = [normalize_block(row) for row in read_tsv(blocks_path)]

    runner_acquired = parse_time(args.runner_acquired_at)
    bouquet_report = latest_bouquet_report(repo_root, args.bouquet, runner_acquired)
    bouquet_manifest: dict[str, Any] = {}
    if bouquet_report is not None:
        try:
            bouquet_manifest = read_json(bouquet_report / "manifest.json")
        except (OSError, json.JSONDecodeError):
            bouquet_manifest = {}
    entries = collect_entries(repo_root, bouquet_report)
    failed_steps = collect_failed_steps(repo_root, entries)
    report_finished_at = iso_now()
    report_seconds = seconds_between(args.report_started_at, report_finished_at)
    raw_blocks.append(
        {
            "block": "report-generation",
            "status": "PASS",
            "exit_code": "0",
            "started_at": args.report_started_at,
            "finished_at": report_finished_at,
            "seconds": report_seconds,
            "reason": "",
        }
    )
    write_tsv(blocks_path, ["block", "status", "exit_code", "started_at", "finished_at", "seconds", "reason"], raw_blocks)

    failed_block = next((block["block"] for block in raw_blocks if status_is_fail(block.get("status"))), "")
    failed_entries = [entry for entry in entries if status_is_fail(entry.get("status"))]
    skipped_entries = [entry for entry in entries if (entry.get("status") or "").strip().upper() == "SKIP"]
    bouquet_status = (bouquet_manifest.get("final_status") or "").strip().upper()
    functional_status = "PASS"
    if failed_block or failed_entries or failed_steps or (bouquet_status and bouquet_status != "PASS"):
        functional_status = "FAIL"

    report = {
        "identity": {
            "run_id": args.run_id,
            "ref": args.ref,
            "sha": args.sha,
            "bouquet": args.bouquet,
            "runner_name": args.runner_name,
            "runner_os": args.runner_os,
            "runner_arch": args.runner_arch,
            "cache_note": args.cache_note,
            "run_note": args.run_note,
        },
        "wall_clock": {
            "github_created_at": args.github_created_at,
            "runner_acquired_at": args.runner_acquired_at,
            "finished_at": report_finished_at,
            "runner_wait_seconds": seconds_between(args.github_created_at, args.runner_acquired_at),
            "executed_seconds": seconds_between(args.runner_acquired_at, report_finished_at),
        },
        "outcome": {
            "functional_status": functional_status,
            "failed_block": failed_block,
            "failed_entry_count": len(failed_entries),
            "skipped_entry_count": len(skipped_entries),
            "failed_step_count": len(failed_steps),
            "first_failure": first_failure(raw_blocks, entries, failed_steps),
            "bouquet_report_dir": str(bouquet_report) if bouquet_report else "",
        },
        "blocks": raw_blocks,
        "bouquet_entries": entries,
        "failed_steps": failed_steps,
    }
    return report


def write_report(report_dir: Path, report: dict[str, Any]) -> None:
    report_dir.mkdir(parents=True, exist_ok=True)
    write_tsv(
        report_dir / "bouquet-entries.tsv",
        ["catalogue", "suite", "profile", "lane", "source", "status", "seconds", "started_at", "finished_at", "child_report_dir", "reason"],
        report["bouquet_entries"],
    )
    write_tsv(
        report_dir / "failed-steps.tsv",
        ["catalogue", "suite", "child_history_id", "step_id", "status", "duration_seconds", "reason", "log_path"],
        report["failed_steps"],
    )
    (report_dir / "summary.json").write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    (report_dir / "summary.md").write_text(build_summary_markdown(report), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", required=True)
    parser.add_argument("--report-dir", required=True)
    parser.add_argument("--bouquet", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--ref", required=True)
    parser.add_argument("--sha", required=True)
    parser.add_argument("--runner-name", required=True)
    parser.add_argument("--runner-os", required=True)
    parser.add_argument("--runner-arch", required=True)
    parser.add_argument("--cache-note", required=True)
    parser.add_argument("--run-note", default="")
    parser.add_argument("--github-created-at", default="")
    parser.add_argument("--runner-acquired-at", required=True)
    parser.add_argument("--report-started-at", required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    report = build_report(args)
    write_report(Path(args.report_dir).resolve(), report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
