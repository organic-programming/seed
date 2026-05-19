#!/usr/bin/env python3
from __future__ import annotations

import itertools
import os
import sys
import tempfile
import time
from dataclasses import dataclass, field
from pathlib import Path

HERE = Path(__file__).resolve().parents[1]
PY_GEN = HERE / "gen" / "python"


def find_repo_root(start: Path) -> Path | None:
    for candidate in [start, *start.parents]:
        if (candidate / "sdk" / "python-holons").is_dir():
            return candidate
    return None


ROOT = find_repo_root(HERE)
SDK_ROOT = ROOT / "sdk" / "python-holons" if ROOT is not None else None

for path in (HERE, PY_GEN, SDK_ROOT):
    if path is None:
        continue
    text = str(path)
    if text not in sys.path:
        sys.path.insert(0, text)

import grpc

from gen import describe_generated
from holons import composite, describe, observability
from holons.serve import ServeOptions, parse_options, run_with_serve_options
from observability_cascade.v1 import service_pb2, service_pb2_grpc
from relay.v1 import relay_pb2, relay_pb2_grpc

RUN_TICKS = 3
PY_SLUG = "observability-cascade-python-node"
GO_SLUG = "observability-cascade-go-node"


@dataclass(frozen=True)
class LanguageMember:
    lang: str
    slug: str
    binary: str
    error: str = ""


@dataclass
class PhaseReportData:
    name: str
    pass_count: int = 0
    fail_count: int = 0
    failures: list[str] = field(default_factory=list)
    elapsed_us: int = 0


@dataclass
class CascadeReportData:
    name: str
    ticks: int = 0
    pass_count: int = 0
    fail_count: int = 0
    phases: list[PhaseReportData] = field(default_factory=list)
    elapsed_us: int = 0


@dataclass
class MultiPatternReportData:
    patterns: list[CascadeReportData] = field(default_factory=list)
    total_pass: int = 0
    total_fail: int = 0
    total_elapsed_us: int = 0


@dataclass
class TickResult:
    pass_: bool
    log: composite.CheckOutcome
    event: composite.CheckOutcome
    hops: composite.CheckOutcome

    def evidence_line(self, tick: int) -> str:
        return (
            f"tick={tick} log={evidence_text(self.log)} "
            f"event={evidence_text(self.event)} hops={evidence_text(self.hops)}"
        )


class ObservabilityCascadeService(service_pb2_grpc.ObservabilityCascadeServiceServicer):
    def RunDefault(self, request, context):
        del request, context
        return to_cascade_report(run_report("default", own_language_members(), live=False, emit=False))

    def RunLiveStream(self, request, context):
        del request, context
        return to_cascade_report(run_report("live-stream", own_language_members(), live=True, emit=False))

    def RunMultiPattern(self, request, context):
        del request, context
        return to_multi_pattern_report(run_multi_pattern_report(emit=False))


def main() -> int:
    args = sys.argv[1:]
    if args and canonical_command(args[0]) == "serve":
        serve_composite(args[1:])
        return 0

    if "--multi-pattern" in args:
        report = run_multi_pattern_report(emit=True)
        return 1 if report.total_fail > 0 else 0

    live = "--live-stream" in args
    report = run_report(
        "live-stream" if live else "default",
        own_language_members(),
        live=live,
        emit=True,
    )
    return 1 if report.fail_count > 0 else 0


def serve_composite(args: list[str]) -> None:
    describe.use_static_response(describe_generated.static_describe_response())
    options = parse_options(args)

    def register(server) -> None:
        service_pb2_grpc.add_ObservabilityCascadeServiceServicer_to_server(
            ObservabilityCascadeService(),
            server,
        )

    run_with_serve_options(
        options.listen_uri,
        register,
        ServeOptions(reflect=options.reflect, slug="observability-cascade-python"),
    )


def run_multi_pattern_report(emit: bool) -> MultiPatternReportData:
    total_start = time.time()
    patterns = python_patterns()
    out = MultiPatternReportData()
    output(emit, "=== observability-cascade-python --multi-pattern ===")
    output(emit)
    for index, (name, members) in enumerate(patterns, start=1):
        output(emit, f"Pattern {index}/{len(patterns)}: {name}")
        report = run_report(name, members, live=True, emit=emit)
        out.patterns.append(report)
        out.total_pass += report.pass_count
        out.total_fail += report.fail_count
        status = "PASS" if report.fail_count == 0 else "FAIL"
        output(emit, f"Pattern {name}: {report.pass_count}/{report.ticks} {status} (elapsed={elapsed_text(report.elapsed_us)})")
        output(emit)
    out.total_elapsed_us = elapsed_us(total_start)
    output(emit, f"Summary: {out.total_pass} PASS / {out.total_fail} FAIL across {out.total_pass + out.total_fail} ticks (total elapsed={elapsed_text(out.total_elapsed_us)})")
    return out


def run_report(name: str, members: list[LanguageMember], live: bool, emit: bool) -> CascadeReportData:
    ensure_cascade_observability()
    report_start = time.time()
    report = CascadeReportData(name=name)
    poll = 0.05 if live else 0.1
    timeout = 3.0
    output(emit, f"=== observability-cascade-python {mode_suffix(name)}===")
    output(emit)

    for phase_index, transport in enumerate(composite.TransportCoverageSequence):
        phase_start = time.time()
        previous_transport = composite.TransportCoverageSequence[phase_index - 1] if phase_index else transport
        phase = PhaseReportData(name=f"{phase_index + 1:02d}-{previous_transport}→{transport}")
        output(emit, f"Phase {phase_index + 1}/{len(composite.TransportCoverageSequence)}: {phase.name}")

        missing = [member for member in members if not member.binary]
        if missing:
            evidence = "; ".join(member.error or f"{member.slug} binary unavailable" for member in missing)
            phase.fail_count += RUN_TICKS
            for tick in range(1, RUN_TICKS + 1):
                phase.failures.append(f"tick={tick} log=spawn event=spawn hops={compact_evidence(evidence)}")
            phase.elapsed_us = elapsed_us(phase_start)
            add_phase(report, phase)
            print_phase_summary(emit, phase)
            continue

        cascade = None
        previous: dict[str, int] = {}
        try:
            cascade = composite.BuildCascade(
                composite.CascadeOptions(
                    transport=transport,
                    members=tuple(composite.ChildSpec(member.slug, member.binary) for member in members),
                    extra_env={
                        "OP_OBS": "logs,events,metrics,prom",
                        "OP_PROM_ADDR": "127.0.0.1:0",
                        "OP_RUN_DIR": tempfile.mkdtemp(prefix="observability-cascade-python-"),
                    },
                )
            )
        except Exception as exc:
            phase.fail_count += RUN_TICKS
            for tick in range(1, RUN_TICKS + 1):
                phase.failures.append(f"tick={tick} log=spawn event=spawn hops={compact_evidence(str(exc))}")
            phase.elapsed_us = elapsed_us(phase_start)
            add_phase(report, phase)
            print_phase_summary(emit, phase)
            continue

        try:
            for tick in range(1, RUN_TICKS + 1):
                sender = f"{name}-phase-{phase_index + 1:02d}-tick-{tick}"
                result = run_tick(cascade, sender, transport, members, previous, timeout, poll, live)
                if result.pass_:
                    phase.pass_count += 1
                else:
                    phase.fail_count += 1
                    phase.failures.append(result.evidence_line(tick))
                output(emit, f"  Tick {tick}/{RUN_TICKS}: {pass_text(result.pass_)}")
                if emit and not result.pass_:
                    print(f"    {result.evidence_line(tick)}", file=sys.stderr)
        finally:
            cascade.stop()

        phase.elapsed_us = elapsed_us(phase_start)
        add_phase(report, phase)
        print_phase_summary(emit, phase)

    report.elapsed_us = elapsed_us(report_start)
    output(emit, f"\nSummary: {report.ticks} ticks, {report.pass_count} PASS, {report.fail_count} FAIL (total elapsed={elapsed_text(report.elapsed_us)})")
    return report


def run_tick(
    cascade: composite.Cascade,
    sender: str,
    note: str,
    members: list[LanguageMember],
    previous: dict[str, int],
    timeout: float,
    poll: float,
    live: bool,
) -> TickResult:
    try:
        response = relay_pb2_grpc.RelayServiceStub(cascade.top.conn).Tick(
            relay_pb2.TickRequest(sender=sender, note=note),
            timeout=5.0,
        )
    except Exception as exc:
        out = composite.CheckOutcome(evidence=compact_evidence(str(exc)))
        return TickResult(pass_=False, log=out, event=out, hops=out)

    hops = check_hops(response.hops, members, previous)
    if not hops.pass_:
        skipped = composite.CheckOutcome(evidence="skipped")
        return TickResult(pass_=False, log=skipped, event=skipped, hops=hops)

    expected = tuple(hop.slug for hop in response.hops)
    leaf_uid = response.hops[0].uid
    log = composite.CheckRelayedLog(
        composite.LogCheckOptions(
            sender=sender,
            leaf_uid=leaf_uid,
            expected_chain=expected,
            timeout=timeout,
            poll_interval=poll,
            live=live,
        )
    )
    event = composite.CheckRelayedEvent(
        composite.EventCheckOptions(
            event_name=observability.EVENT_INSTANCE_READY,
            leaf_uid=leaf_uid,
            expected_chain=expected,
            timeout=timeout,
            poll_interval=poll,
            live=live,
        )
    )
    return TickResult(pass_=hops.pass_ and log.pass_ and event.pass_, log=log, event=event, hops=hops)


def check_hops(hops, members: list[LanguageMember], previous: dict[str, int]) -> composite.CheckOutcome:
    if len(hops) != len(members):
        return composite.CheckOutcome(evidence=f"hops length {len(hops)} want {len(members)}")
    for index, hop in enumerate(hops):
        want = members[len(members) - 1 - index]
        if hop.slug != want.slug:
            return composite.CheckOutcome(evidence=f"hop {index} slug={hop.slug} want {want.slug}")
        if not hop.uid:
            return composite.CheckOutcome(evidence=f"hop {index} uid empty")
        if hop.received <= previous.get(hop.uid, 0):
            return composite.CheckOutcome(evidence=f"hop {index} received={hop.received} previous={previous.get(hop.uid, 0)}")
        previous[hop.uid] = hop.received
    return composite.CheckOutcome(pass_=True)


def own_language_members() -> list[LanguageMember]:
    python = language_member("python")
    return [python, python, python]


def python_patterns() -> list[tuple[str, list[LanguageMember]]]:
    members = {"python": language_member("python"), "go": language_member("go")}
    out: list[tuple[str, list[LanguageMember]]] = []
    for parts in itertools.product(("python", "go"), repeat=3):
        name = "-".join(parts)
        out.append((name, [members[part] for part in parts]))
    return out


def language_member(lang: str) -> LanguageMember:
    slug = PY_SLUG if lang == "python" else GO_SLUG
    member_id = "python-node" if lang == "python" else "go-node"
    try:
        return LanguageMember(lang=lang, slug=slug, binary=composite.member(member_id))
    except Exception as exc:
        return LanguageMember(lang=lang, slug=slug, binary="", error=str(exc))


def ensure_cascade_observability() -> None:
    current = observability.current()
    if current.enabled(observability.Family.LOGS) and current.enabled(observability.Family.EVENTS):
        return
    os.environ["OP_OBS"] = "logs,events,metrics,prom"
    observability.from_env(observability.Config(slug="observability-cascade-python"))


def add_phase(report: CascadeReportData, phase: PhaseReportData) -> None:
    report.phases.append(phase)
    report.pass_count += phase.pass_count
    report.fail_count += phase.fail_count
    report.ticks += phase.pass_count + phase.fail_count


def to_cascade_report(report: CascadeReportData):
    message = service_pb2.CascadeReport(
        name=report.name,
        ticks=report.ticks,
        fail=report.fail_count,
        phases=[
            service_pb2.PhaseResult(
                name=phase.name,
                fail=phase.fail_count,
                failures=phase.failures,
                elapsed_us=phase.elapsed_us,
            )
            for phase in report.phases
        ],
        elapsed_us=report.elapsed_us,
    )
    setattr(message, "pass", report.pass_count)
    for index, phase in enumerate(report.phases):
        setattr(message.phases[index], "pass", phase.pass_count)
    return message


def to_multi_pattern_report(report: MultiPatternReportData):
    return service_pb2.MultiPatternReport(
        patterns=[to_cascade_report(pattern) for pattern in report.patterns],
        total_pass=report.total_pass,
        total_fail=report.total_fail,
        total_elapsed_us=report.total_elapsed_us,
    )


def output(emit: bool, value: object = "") -> None:
    if emit:
        print(value)


def print_phase_summary(emit: bool, phase: PhaseReportData) -> None:
    status = "PASS" if phase.fail_count == 0 else "FAIL"
    output(emit, f"Phase {phase.name}: {phase.pass_count}/{phase.pass_count + phase.fail_count} {status} (elapsed={elapsed_text(phase.elapsed_us)})")


def evidence_text(out: composite.CheckOutcome) -> str:
    return "ok" if out.pass_ else compact_evidence(out.evidence)


def compact_evidence(value: str) -> str:
    value = " ".join(str(value).split())
    if not value:
        return "<empty>"
    if len(value) <= 240:
        return value
    return value[:240] + "..."


def pass_text(value: bool) -> str:
    return "PASS" if value else "FAIL"


def elapsed_us(start: float) -> int:
    return max(1, int((time.time() - start) * 1_000_000))


def elapsed_text(value: int) -> str:
    if value < 1_000_000:
        return f"{value // 1000}ms"
    seconds = value / 1_000_000
    if seconds < 60:
        return f"{seconds:.2f}s"
    return f"{seconds / 60:.1f}m"


def mode_suffix(name: str) -> str:
    return "" if name == "default" else f"--{name} "


def canonical_command(raw: str) -> str:
    return raw.strip().lower().replace("-", "").replace("_", "").replace(" ", "")


if __name__ == "__main__":
    raise SystemExit(main())
