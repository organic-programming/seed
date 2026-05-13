#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import pathlib
import re
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import time
from dataclasses import dataclass, field
from urllib.request import urlopen

ROOT = pathlib.Path(__file__).resolve().parents[4]
HERE = pathlib.Path(__file__).resolve().parents[1]
PY_NODE = HERE.parent / "cascade-node-python"
SDK_ROOT = ROOT / "sdk" / "python-holons"

for path in (PY_NODE / "gen" / "python", SDK_ROOT):
    text = str(path)
    if text not in sys.path:
        sys.path.insert(0, text)

import grpc
from holons.v1 import describe_pb2, describe_pb2_grpc, observability_pb2, observability_pb2_grpc
from relay.v1 import relay_pb2, relay_pb2_grpc

RUN_PHASES = 4
RUN_TICKS = 3
ROLE_ORDER = ["D", "C", "B", "A"]
TRANSPORTS = ["tcp", "unix", "tcp", "unix"]
PY_SLUG = "cascade-node-python"
GO_SLUG = "cascade-node-go"


@dataclass(frozen=True)
class RoleSpec:
    slug: str
    binary_path: str


@dataclass
class RoleRuntime:
    role: str
    uid: str
    slug: str
    binary_path: str
    listen_uri: str
    relay_address: str
    client_target: str
    member_address: str = ""
    member_slug: str = ""
    metrics_addr: str = ""
    process: subprocess.Popen | None = None
    channel: grpc.Channel | None = None


@dataclass
class CheckResult:
    pass_: bool = False
    evidence: str = ""


@dataclass
class TickOutcome:
    log: CheckResult
    event: CheckResult
    metric: CheckResult
    metric_value: float = 0.0


@dataclass
class Cascade:
    phase: int
    transport: str
    run_root: pathlib.Path
    roles: dict[str, RoleRuntime]

    def run_tick(self, tick: int, previous_metric: float) -> TickOutcome:
        sender = f"phase-{self.phase}-tick-{tick}"
        return self.run_tick_with_sender(sender, previous_metric)

    def run_tick_with_sender(self, sender: str, previous_metric: float) -> TickOutcome:
        try:
            relay_pb2_grpc.RelayServiceStub(self.roles["D"].channel).Tick(
                relay_pb2.TickRequest(sender=sender, note=self.transport),
                timeout=5,
            )
        except Exception as exc:
            err = CheckResult(False, str(exc))
            return TickOutcome(err, err, err, previous_metric)

        log = wait_for(3.0, lambda: self.check_log(sender))
        event = wait_for(3.0, self.check_event)
        metric_value = previous_metric

        def metric_check() -> CheckResult:
            nonlocal metric_value
            result, value = self.check_metric(previous_metric)
            if result.pass_:
                metric_value = value
            return result

        metric = wait_for(3.0, metric_check)
        return TickOutcome(log, event, metric, metric_value)

    def run_live_tick(
        self,
        streams: LiveStreams | None,
        stream_open_error: str | None,
        tick: int,
        previous_metric: float,
    ) -> TickOutcome:
        sender = f"phase-{self.phase}-tick-{tick}"
        return self.run_live_tick_with_sender(streams, stream_open_error, sender, previous_metric)

    def run_live_tick_with_sender(
        self,
        streams: LiveStreams | None,
        stream_open_error: str | None,
        sender: str,
        previous_metric: float,
    ) -> TickOutcome:
        try:
            relay_pb2_grpc.RelayServiceStub(self.roles["D"].channel).Tick(
                relay_pb2.TickRequest(sender=sender, note=self.transport),
                timeout=5,
            )
        except Exception as exc:
            err = CheckResult(False, str(exc))
            return TickOutcome(err, err, err, previous_metric)

        if stream_open_error is None and streams is not None:
            log = wait_for(1.0, lambda: self.check_live_log(streams, sender), interval=0.05)
            event = wait_for(1.0, lambda: self.check_live_event(streams), interval=0.05)
        else:
            evidence = f"stream re-open failed: {stream_open_error or 'streams not open'}"
            log = CheckResult(False, evidence)
            event = CheckResult(False, evidence)

        metric_value = previous_metric

        def metric_check() -> CheckResult:
            nonlocal metric_value
            result, value = self.check_metric(previous_metric)
            if result.pass_:
                metric_value = value
            return result

        metric = wait_for(1.0, metric_check, interval=0.05)
        return TickOutcome(log, event, metric, metric_value)

    def check_log(self, sender: str) -> CheckResult:
        entries = read_logs(self.roles["A"].channel)
        for entry in entries:
            if entry.message != "tick received":
                continue
            if entry.fields.get("sender") != sender:
                continue
            if entry.fields.get("responder_uid") != self.roles["D"].uid:
                continue
            err = self.check_chain(entry.chain)
            if err:
                return CheckResult(False, f"matching log has bad chain: {err} entry={entry}")
            return CheckResult(True, str(entry))
        return CheckResult(False, f"no relayed D tick log for sender={sender} in {len(entries)} A log entries")

    def check_event(self) -> CheckResult:
        events = read_events(self.roles["A"].channel)
        for event in events:
            if event.type != observability_pb2.INSTANCE_READY or event.instance_uid != self.roles["D"].uid:
                continue
            err = self.check_chain(event.chain)
            if err:
                return CheckResult(False, f"matching event has bad chain: {err} event={event}")
            return CheckResult(True, str(event))
        return CheckResult(False, f"no relayed D INSTANCE_READY event in {len(events)} A events")

    def check_live_log(self, streams: "LiveStreams", sender: str) -> CheckResult:
        entries = streams.log_entries()
        for entry in entries:
            if entry.message != "tick received":
                continue
            if entry.fields.get("sender") != sender:
                continue
            if entry.fields.get("responder_uid") != self.roles["D"].uid:
                continue
            err = self.check_chain(entry.chain)
            if err:
                return CheckResult(False, f"matching live log has bad chain: {err} entry={entry}")
            return CheckResult(True, str(entry))
        return CheckResult(False, f"no live log found for sender={sender}; buffer={len(entries)} errors={streams.errors()}")

    def check_live_event(self, streams: "LiveStreams") -> CheckResult:
        events = streams.event_entries()
        for event in events:
            if event.type != observability_pb2.INSTANCE_READY or event.instance_uid != self.roles["D"].uid:
                continue
            err = self.check_chain(event.chain)
            if err:
                return CheckResult(False, f"matching live event has bad chain: {err} event={event}")
            return CheckResult(True, str(event))
        return CheckResult(False, f"no live INSTANCE_READY event for D; buffer={len(events)} errors={streams.errors()}")

    def check_metric(self, previous: float) -> tuple[CheckResult, float]:
        try:
            body = fetch_metrics(self.roles["D"].metrics_addr)
        except Exception as exc:
            return CheckResult(False, str(exc)), previous
        value = parse_cascade_ticks(body, self.roles["D"].uid)
        if value is None:
            return CheckResult(False, body), previous
        if value <= previous:
            return CheckResult(False, f"cascade_ticks_total={value} did not increase beyond {previous}\n{body}"), value
        return CheckResult(True, f"cascade_ticks_total={value}"), value

    def check_chain(self, chain) -> str:
        for idx, role in enumerate(["D", "C", "B"]):
            if idx >= len(chain):
                return f"chain length {len(chain)} < 3"
            hop = chain[idx]
            want = self.roles[role]
            if hop.slug != want.slug or hop.instance_uid != want.uid:
                return f"hop {idx} = {hop.slug}/{hop.instance_uid}, want {want.slug}/{want.uid}"
        return ""

    def stop(self) -> None:
        for role in reversed(ROLE_ORDER):
            runtime = self.roles.get(role)
            if runtime is None:
                continue
            if runtime.channel is not None:
                runtime.channel.close()
            if runtime.process is not None and runtime.process.poll() is None:
                runtime.process.send_signal(signal.SIGTERM)
        deadline = time.time() + 3
        for runtime in self.roles.values():
            proc = runtime.process
            if proc is None:
                continue
            remaining = max(0.01, deadline - time.time())
            try:
                proc.wait(timeout=remaining)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=2)


class LiveStreams:
    def __init__(self, channel: grpc.Channel) -> None:
        self._client = observability_pb2_grpc.HolonObservabilityStub(channel)
        self._logs = []
        self._events = []
        self._errors = []
        self._lock = threading.Lock()
        self._calls = []
        self._threads = []

    def start(self) -> None:
        log_call = self._client.Logs(observability_pb2.LogsRequest(min_level=observability_pb2.INFO, follow=True))
        event_call = self._client.Events(observability_pb2.EventsRequest(follow=True))
        self._calls = [log_call, event_call]
        self._threads = [
            threading.Thread(target=self._read_logs, args=(log_call,), daemon=True),
            threading.Thread(target=self._read_events, args=(event_call,), daemon=True),
        ]
        for thread in self._threads:
            thread.start()

    def stop(self) -> None:
        for call in self._calls:
            call.cancel()
        for thread in self._threads:
            thread.join(timeout=2)

    def log_entries(self):
        with self._lock:
            return list(self._logs)

    def event_entries(self):
        with self._lock:
            return list(self._events)

    def errors(self):
        with self._lock:
            return list(self._errors)

    def _read_logs(self, call) -> None:
        try:
            for entry in call:
                with self._lock:
                    self._logs.append(entry)
        except Exception as exc:
            with self._lock:
                self._errors.append(f"logs stream ended: {exc}")

    def _read_events(self, call) -> None:
        try:
            for event in call:
                with self._lock:
                    self._events.append(event)
        except Exception as exc:
            with self._lock:
                self._errors.append(f"events stream ended: {exc}")


def main() -> int:
    args = sys.argv[1:]
    try:
        if "--multi-pattern" in args:
            run_multi_pattern()
        elif "--live-stream" in args:
            run_live_stream()
        else:
            run_default()
    except Exception as exc:
        print(f"\nFAIL: {exc}", file=sys.stderr)
        return 1
    return 0


def run_default() -> None:
    binary = find_binary(PY_SLUG)
    run_root = pathlib.Path(tempfile.mkdtemp(prefix="relay-cascade-python-"))
    print("=== relay-cascade-python ===")
    print()
    total_pass = 0
    total_fail = 0
    previous = ""
    for phase, transport in enumerate(TRANSPORTS, start=1):
        if not previous:
            print(f"Phase {phase}/{RUN_PHASES}: transport={transport}")
        else:
            print(f"Phase {phase}/{RUN_PHASES}: transport={transport} (switching from {previous})")
        started = time.time()
        try:
            cascade = spawn_cascade(phase, transport, {"A": RoleSpec(PY_SLUG, binary), "B": RoleSpec(PY_SLUG, binary), "C": RoleSpec(PY_SLUG, binary), "D": RoleSpec(PY_SLUG, binary)}, run_root)
        except Exception as exc:
            total_fail += RUN_TICKS
            print(f"  spawn FAIL: {exc}\n")
            previous = transport
            continue
        print(f"  spawned 4 nodes in {elapsed(started)}")
        previous_metric = 0.0
        for tick in range(1, RUN_TICKS + 1):
            tick_start = time.time()
            outcome = cascade.run_tick(tick, previous_metric)
            if outcome.metric.pass_:
                previous_metric = outcome.metric_value
            overall = outcome.log.pass_ and outcome.event.pass_ and outcome.metric.pass_
            total_pass += 1 if overall else 0
            total_fail += 0 if overall else 1
            print(f"  Tick {tick}/{RUN_TICKS}: log {pass_text(outcome.log.pass_)}, event {pass_text(outcome.event.pass_)}, metric {pass_text(outcome.metric.pass_)} (overall {pass_text(overall)} in {elapsed(tick_start)})")
            if not overall:
                print_failure_evidence("log", outcome.log)
                print_failure_evidence("event", outcome.event)
                print_failure_evidence("metric", outcome.metric)
        cascade.stop()
        print()
        previous = transport
    print(f"Summary: {total_pass + total_fail} ticks, {total_pass} PASS, {total_fail} FAIL")
    if total_fail:
        raise RuntimeError(f"{total_fail} tick(s) failed")


def run_live_stream() -> None:
    binary = find_binary(PY_SLUG)
    run_root = pathlib.Path(tempfile.mkdtemp(prefix="relay-cascade-python-live-"))
    print("=== relay-cascade-python --live-stream ===")
    print()
    print("Setup: opening long-lived Follow:true streams on A")
    print("       (initial transport: tcp)")
    print()
    total_pass = 0
    total_fail = 0
    cascade = None
    streams = None
    specs = {"A": RoleSpec(PY_SLUG, binary), "B": RoleSpec(PY_SLUG, binary), "C": RoleSpec(PY_SLUG, binary), "D": RoleSpec(PY_SLUG, binary)}
    for phase, transport in enumerate(TRANSPORTS, start=1):
        if phase == 1:
            print(f"Phase {phase}/{RUN_PHASES}: initial chain ({transport})")
        else:
            print(f"Phase {phase}/{RUN_PHASES}: respawn on {transport}")
            kill_start = time.time()
            if streams is not None:
                streams.stop()
            if cascade is not None:
                cascade.stop()
            print(f"  killed 4 nodes in {elapsed(kill_start)}")
        spawn_start = time.time()
        try:
            phase_cascade = spawn_cascade(phase, transport, specs, run_root)
        except Exception as exc:
            total_fail += RUN_TICKS
            print(f"  spawn FAIL: {exc}\n")
            streams = None
            continue
        print(f"  spawned 4 nodes in {elapsed(spawn_start)}")
        if phase > 1:
            print("  re-opening Follow:true streams on new A")
        stream_error = None
        try:
            streams = LiveStreams(phase_cascade.roles["A"].channel)
            streams.start()
        except Exception as exc:
            streams = None
            stream_error = str(exc)
            print(f"  stream re-open failed: {exc}")
        previous_metric = 0.0
        for tick in range(1, RUN_TICKS + 1):
            tick_start = time.time()
            outcome = phase_cascade.run_live_tick(streams, stream_error, tick, previous_metric)
            if outcome.metric.pass_:
                previous_metric = outcome.metric_value
            overall = outcome.log.pass_ and outcome.event.pass_ and outcome.metric.pass_
            total_pass += 1 if overall else 0
            total_fail += 0 if overall else 1
            print(f"  Tick {tick}/{RUN_TICKS}: log {pass_text(outcome.log.pass_)}, event {pass_text(outcome.event.pass_)}, metric {pass_text(outcome.metric.pass_)} (overall {pass_text(overall)} in {elapsed(tick_start)})")
            if not overall:
                print_failure_evidence("log", outcome.log)
                print_failure_evidence("event", outcome.event)
                print_failure_evidence("metric", outcome.metric)
        print()
        cascade = phase_cascade
    if streams is not None:
        streams.stop()
    if cascade is not None:
        cascade.stop()
    print(f"Summary: {total_pass} PASS / {total_fail} FAIL across {total_pass + total_fail} ticks")
    if total_fail:
        raise RuntimeError(f"{total_fail} tick(s) failed")


def run_multi_pattern() -> None:
    py_binary = find_binary(PY_SLUG)
    go_binary = find_binary(GO_SLUG)
    patterns = [
        ("python-python-python-python", {"A": RoleSpec(PY_SLUG, py_binary), "B": RoleSpec(PY_SLUG, py_binary), "C": RoleSpec(PY_SLUG, py_binary), "D": RoleSpec(PY_SLUG, py_binary)}),
        ("python-python-go-python", {"A": RoleSpec(PY_SLUG, py_binary), "B": RoleSpec(PY_SLUG, py_binary), "C": RoleSpec(GO_SLUG, go_binary), "D": RoleSpec(PY_SLUG, py_binary)}),
        ("python-python-go-go", {"A": RoleSpec(PY_SLUG, py_binary), "B": RoleSpec(PY_SLUG, py_binary), "C": RoleSpec(GO_SLUG, go_binary), "D": RoleSpec(GO_SLUG, go_binary)}),
    ]
    run_root = pathlib.Path(tempfile.mkdtemp(prefix="relay-cascade-python-multi-"))
    print("=== relay-cascade-python (multi-pattern) ===")
    print()
    total_pass = 0
    total_fail = 0
    for pattern_idx, (name, specs) in enumerate(patterns, start=1):
        print(f"Pattern {pattern_idx}/{len(patterns)}: {name}")
        pattern_pass = 0
        for phase, transport in enumerate(TRANSPORTS, start=1):
            started = time.time()
            try:
                cascade = spawn_cascade(phase, transport, specs, run_root)
            except Exception as exc:
                total_fail += RUN_TICKS
                print(f"  Phase {phase}/{RUN_PHASES} ({transport}): spawn FAIL ({exc})")
                continue
            stream_error = None
            streams = None
            try:
                streams = LiveStreams(cascade.roles["A"].channel)
                streams.start()
                ready = wait_for(5.0, lambda: cascade.check_live_event(streams), interval=0.05)
                if not ready.pass_:
                    stream_error = f"live relay readiness: {ready.evidence}"
            except Exception as exc:
                stream_error = str(exc)
            previous_metric = 0.0
            results = []
            evidence = []
            for tick in range(1, RUN_TICKS + 1):
                sender = f"{name}-phase-{phase}-tick-{tick}"
                outcome = cascade.run_live_tick_with_sender(streams, stream_error, sender, previous_metric)
                if outcome.metric.pass_:
                    previous_metric = outcome.metric_value
                overall = outcome.log.pass_ and outcome.event.pass_ and outcome.metric.pass_
                if overall:
                    pattern_pass += 1
                    total_pass += 1
                    results.append(f"Tick {tick} PASS")
                else:
                    total_fail += 1
                    results.append(f"Tick {tick} FAIL ({failure_summary(outcome)})")
                    evidence.append(f"      Tick {tick} evidence: {compact_evidence(outcome)}")
            print(f"  Phase {phase}/{RUN_PHASES} ({transport}): {', '.join(results)} (spawned in {elapsed(started)})")
            for line in evidence:
                print(line)
            if streams is not None:
                streams.stop()
            cascade.stop()
        print(f"  Subtotal: {pattern_pass}/12 PASS")
        print()
    print(f"Summary: {total_pass} PASS / {total_fail} FAIL across {total_pass + total_fail} ticks")
    if total_fail:
        raise RuntimeError(f"{total_fail} tick(s) failed")


def spawn_cascade(phase: int, transport: str, specs: dict[str, RoleSpec], run_root: pathlib.Path) -> Cascade:
    roles = {role: new_role_runtime(phase, transport, role, specs[role]) for role in ROLE_ORDER}
    for runtime in roles.values():
        shutil.rmtree(run_root / runtime.slug / runtime.uid, ignore_errors=True)
    cascade = Cascade(phase, transport, run_root, roles)
    for role in ROLE_ORDER:
        runtime = roles[role]
        child = child_role(role)
        if child:
            runtime.member_address = roles[child].relay_address
            runtime.member_slug = roles[child].slug
        start_role(cascade, runtime)
    time.sleep(0.15)
    return cascade


def new_role_runtime(phase: int, transport: str, role: str, spec: RoleSpec) -> RoleRuntime:
    uid = f"relay-p{phase:02d}-{role.lower()}"
    if transport == "tcp":
        port = free_port()
        listen_uri = f"tcp://127.0.0.1:{port}"
        client_target = f"127.0.0.1:{port}"
        relay_address = listen_uri
    elif transport == "unix":
        path = f"/tmp/relay-cascade-python-p{phase}-{role.lower()}-{os.getpid()}.sock"
        try:
            os.remove(path)
        except FileNotFoundError:
            pass
        listen_uri = f"unix://{path}"
        client_target = listen_uri
        relay_address = listen_uri
    else:
        raise RuntimeError(f"unknown transport {transport}")
    return RoleRuntime(role, uid, spec.slug, spec.binary_path, listen_uri, relay_address, client_target)


def start_role(cascade: Cascade, runtime: RoleRuntime) -> None:
    args = [runtime.binary_path, "serve", "--listen", runtime.listen_uri]
    if runtime.member_address:
        args += ["--member", f"{runtime.member_slug}={runtime.member_address}"]
    env = os.environ.copy()
    env.update({
        "OP_OBS": "logs,events,metrics,prom",
        "OP_RUN_DIR": str(cascade.run_root),
        "OP_INSTANCE_UID": runtime.uid,
        "OP_ORGANISM_UID": cascade.roles["A"].uid,
        "OP_ORGANISM_SLUG": cascade.roles["A"].slug,
        "OP_PROM_ADDR": "127.0.0.1:0",
    })
    proc = subprocess.Popen(args, cwd=ROOT, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True)
    runtime.process = proc
    try:
        meta = wait_meta(cascade.run_root, runtime.slug, runtime.uid, 10.0)
        runtime.metrics_addr = meta["metrics_addr"]
        runtime.channel = dial_ready(runtime.client_target, 10.0)
    except Exception:
        stderr = proc.stderr.read() if proc.stderr else ""
        raise RuntimeError(f"start {runtime.role}: {stderr}") from None


def child_role(role: str) -> str:
    return {"A": "B", "B": "C", "C": "D"}.get(role, "")


def wait_meta(run_root: pathlib.Path, slug: str, uid: str, timeout: float) -> dict:
    path = run_root / slug / uid / "meta.json"
    deadline = time.time() + timeout
    last_error = None
    while time.time() < deadline:
        try:
            data = json.loads(path.read_text())
            if data.get("uid") == uid and data.get("metrics_addr"):
                return data
        except Exception as exc:
            last_error = exc
        time.sleep(0.05)
    raise RuntimeError(f"meta not ready for {slug}/{uid}: {last_error}")


def dial_ready(target: str, timeout: float) -> grpc.Channel:
    deadline = time.time() + timeout
    last_error = None
    while time.time() < deadline:
        channel = grpc.insecure_channel(target)
        try:
            describe_pb2_grpc.HolonMetaStub(channel).Describe(describe_pb2.DescribeRequest(), timeout=0.5)
            return channel
        except Exception as exc:
            last_error = exc
            channel.close()
            time.sleep(0.05)
    raise RuntimeError(f"dial {target}: {last_error}")


def read_logs(channel: grpc.Channel):
    call = observability_pb2_grpc.HolonObservabilityStub(channel).Logs(
        observability_pb2.LogsRequest(min_level=observability_pb2.INFO, follow=False),
        timeout=2,
    )
    return list(call)


def read_events(channel: grpc.Channel):
    call = observability_pb2_grpc.HolonObservabilityStub(channel).Events(
        observability_pb2.EventsRequest(follow=False),
        timeout=2,
    )
    return list(call)


def fetch_metrics(addr: str) -> str:
    with urlopen(addr, timeout=2) as response:
        return response.read().decode("utf-8")


def parse_cascade_ticks(body: str, uid: str) -> float | None:
    needle = f'responder_uid="{uid}"'
    for line in body.splitlines():
        if not line.startswith("cascade_ticks_total{") or needle not in line:
            continue
        parts = line.split()
        if len(parts) >= 2:
            return float(parts[-1])
    return None


def wait_for(timeout: float, fn, interval: float = 0.1) -> CheckResult:
    deadline = time.time() + timeout
    last = CheckResult(False, "")
    while True:
        last = fn()
        if last.pass_ or time.time() > deadline:
            return last
        time.sleep(interval)


def find_binary(slug: str) -> str:
    env_name = "CASCADE_NODE_" + slug.removeprefix("cascade-node-").upper().replace("-", "_") + "_BIN"
    override = os.environ.get(env_name, "").strip()
    if override:
        return override
    try:
        path = subprocess.check_output(["op", "--bin", slug], cwd=ROOT, text=True, stderr=subprocess.DEVNULL).strip()
        if path:
            return path
    except Exception:
        pass
    root = pathlib.Path.home() / ".op" / "bin" / f"{slug}.holon" / "bin"
    for path in root.rglob(slug):
        if os.access(path, os.X_OK):
            return str(path)
    raise RuntimeError(f"{slug} binary not found; run op build {slug} --install")


def free_port() -> int:
    sock = socket.socket()
    sock.bind(("127.0.0.1", 0))
    port = sock.getsockname()[1]
    sock.close()
    return int(port)


def elapsed(start: float) -> str:
    seconds = max(0.0, time.time() - start)
    if seconds < 1:
        return f"{int(seconds * 1000)}ms"
    return f"{seconds:.1f}s"


def pass_text(value: bool) -> str:
    return "PASS" if value else "FAIL"


def print_failure_evidence(family: str, result: CheckResult) -> None:
    if not result.pass_:
        print(f"    {family} evidence: {result.evidence or '<empty>'}")


def failure_summary(outcome: TickOutcome) -> str:
    missing = []
    if not outcome.log.pass_:
        missing.append("log family")
    if not outcome.event.pass_:
        missing.append("event family")
    if not outcome.metric.pass_:
        missing.append("metric family")
    return ", ".join(missing) if missing else "unknown"


def compact_evidence(outcome: TickOutcome) -> str:
    parts = []
    if not outcome.log.pass_:
        parts.append("log=" + outcome.log.evidence)
    if not outcome.event.pass_:
        parts.append("event=" + outcome.event.evidence)
    if not outcome.metric.pass_:
        parts.append("metric=" + outcome.metric.evidence)
    return " | ".join(parts)


if __name__ == "__main__":
    raise SystemExit(main())
