#!/usr/bin/env ruby
# frozen_string_literal: true

require "fileutils"
require "json"
require "net/http"
require "open3"
require "pathname"
require "socket"
require "tempfile"
require "tmpdir"
require "timeout"
require "uri"

def find_repo_root(start)
  current = Pathname.new(start).expand_path
  loop do
    return current.to_s if current.join("sdk", "ruby-holons", "lib").directory?
    parent = current.parent
    return nil if parent == current
    current = parent
  end
end

HERE = File.expand_path("..", __dir__)
ROOT = find_repo_root(HERE)
SDK_ROOT = File.join(ROOT, "sdk", "ruby-holons", "lib") unless ROOT.nil?
RUBY_GEN = File.join(HERE, "gen", "ruby")
GENERATED_ROOT = File.join(HERE, "gen")

begin
  if ENV["OP_SDK_RUBY_PATH"] && !ENV["OP_SDK_RUBY_PATH"].empty?
    prebuilt_bundle = File.join(ENV["OP_SDK_RUBY_PATH"], "vendor", "bundle")
    if Dir.exist?(prebuilt_bundle)
      ENV["BUNDLE_PATH"] ||= prebuilt_bundle
      ENV["BUNDLE_DISABLE_SHARED_GEMS"] ||= "true"
    end
  end
  require "bundler/setup"
rescue LoadError
  nil
end

[RUBY_GEN, GENERATED_ROOT, SDK_ROOT].each do |path|
  next if path.nil?

  $LOAD_PATH.unshift(path) unless $LOAD_PATH.include?(path)
end

require "grpc"
require "holons"
load File.join(GENERATED_ROOT, "describe_generated.rb")
require "holons/v1/describe_pb"
require "holons/v1/describe_services_pb"
require "holons/v1/observability_pb"
require "holons/v1/observability_services_pb"
require "observability_cascade/v1/service_pb"
require "observability_cascade/v1/service_services_pb"
require "relay/v1/relay_pb"
require "relay/v1/relay_services_pb"

RUN_PHASES = 4
RUN_TICKS = 3
ROLE_ORDER = %w[D C B A].freeze
TRANSPORTS = %w[tcp unix tcp unix].freeze
RUBY_SLUG = "observability-cascade-ruby-node"
GO_SLUG = "observability-cascade-go-node"

RoleSpec = Struct.new(:slug, :binary_path, keyword_init: true)
CheckResult = Struct.new(:pass, :evidence, keyword_init: true)
TickOutcome = Struct.new(:log, :event, :metric, :metric_value, keyword_init: true)
PhaseReportData = Struct.new(:name, :pass_count, :fail_count, :failures, keyword_init: true)
CascadeReportData = Struct.new(:ticks, :pass_count, :fail_count, :phases, keyword_init: true)
MultiPatternReportData = Struct.new(:patterns, :total_pass, :total_fail, keyword_init: true)

RoleRuntime = Struct.new(
  :role, :uid, :slug, :binary_path, :listen_uri, :relay_address, :client_target,
  :member_address, :member_slug, :metrics_addr, :pid, :wait_thread, :stderr_path,
  :channel,
  keyword_init: true
)

class Cascade
  attr_reader :phase, :transport, :run_root, :roles

  def initialize(phase:, transport:, run_root:, roles:)
    @phase = phase
    @transport = transport
    @run_root = run_root
    @roles = roles
  end

  def run_tick(tick, previous_metric)
    run_tick_with_sender("phase-#{phase}-tick-#{tick}", previous_metric)
  end

  def run_tick_with_sender(sender, previous_metric)
    begin
      Relay::V1::RelayService::Stub.new(
        "unused",
        :this_channel_is_insecure,
        channel_override: roles["D"].channel,
        timeout: 5
      ).tick(Relay::V1::TickRequest.new(sender: sender, note: transport))
    rescue StandardError => e
      err = CheckResult.new(pass: false, evidence: e.message)
      return TickOutcome.new(log: err, event: err, metric: err, metric_value: previous_metric)
    end

    log = wait_for(3.0) { check_log(sender) }
    event = wait_for(3.0) { check_event }
    metric_value = previous_metric
    metric = wait_for(3.0) do
      result, value = check_metric(previous_metric)
      metric_value = value if result.pass
      result
    end
    TickOutcome.new(log: log, event: event, metric: metric, metric_value: metric_value)
  end

  def run_live_tick(streams, stream_open_error, tick, previous_metric)
    run_live_tick_with_sender(streams, stream_open_error, "phase-#{phase}-tick-#{tick}", previous_metric)
  end

  def run_live_tick_with_sender(streams, stream_open_error, sender, previous_metric)
    begin
      Relay::V1::RelayService::Stub.new(
        "unused",
        :this_channel_is_insecure,
        channel_override: roles["D"].channel,
        timeout: 5
      ).tick(Relay::V1::TickRequest.new(sender: sender, note: transport))
    rescue StandardError => e
      err = CheckResult.new(pass: false, evidence: e.message)
      return TickOutcome.new(log: err, event: err, metric: err, metric_value: previous_metric)
    end

    if stream_open_error.nil? && streams
      log = wait_for(1.0, interval: 0.05) { check_live_log(streams, sender) }
      event = wait_for(1.0, interval: 0.05) { check_live_event(streams) }
    else
      evidence = "stream re-open failed: #{stream_open_error || 'streams not open'}"
      log = CheckResult.new(pass: false, evidence: evidence)
      event = CheckResult.new(pass: false, evidence: evidence)
    end

    metric_value = previous_metric
    metric = wait_for(1.0, interval: 0.05) do
      result, value = check_metric(previous_metric)
      metric_value = value if result.pass
      result
    end
    TickOutcome.new(log: log, event: event, metric: metric, metric_value: metric_value)
  end

  def check_log(sender)
    entries = read_logs(roles["A"].channel)
    entries.each do |entry|
      next unless entry.message == "tick received"
      next unless entry.fields["sender"] == sender
      next unless entry.fields["responder_uid"] == roles["D"].uid

      err = check_chain(entry.chain)
      return CheckResult.new(pass: false, evidence: "matching log has bad chain: #{err} entry=#{entry.inspect}") unless err.empty?

      return CheckResult.new(pass: true, evidence: entry.inspect)
    end
    CheckResult.new(pass: false, evidence: "no relayed D tick log for sender=#{sender} in #{entries.length} A log entries")
  end

  def check_event
    events = read_events(roles["A"].channel)
    events.each do |event|
      next unless event.type == :INSTANCE_READY && event.instance_uid == roles["D"].uid

      err = check_chain(event.chain)
      return CheckResult.new(pass: false, evidence: "matching event has bad chain: #{err} event=#{event.inspect}") unless err.empty?

      return CheckResult.new(pass: true, evidence: event.inspect)
    end
    CheckResult.new(pass: false, evidence: "no relayed D INSTANCE_READY event in #{events.length} A events")
  end

  def check_live_log(streams, sender)
    entries = streams.log_entries
    entries.each do |entry|
      next unless entry.message == "tick received"
      next unless entry.fields["sender"] == sender
      next unless entry.fields["responder_uid"] == roles["D"].uid

      err = check_chain(entry.chain)
      return CheckResult.new(pass: false, evidence: "matching live log has bad chain: #{err} entry=#{entry.inspect}") unless err.empty?

      return CheckResult.new(pass: true, evidence: entry.inspect)
    end
    CheckResult.new(pass: false, evidence: "no live log found for sender=#{sender}; buffer=#{entries.length} errors=#{streams.errors.inspect}")
  end

  def check_live_event(streams)
    events = streams.event_entries
    events.each do |event|
      next unless event.type == :INSTANCE_READY && event.instance_uid == roles["D"].uid

      err = check_chain(event.chain)
      return CheckResult.new(pass: false, evidence: "matching live event has bad chain: #{err} event=#{event.inspect}") unless err.empty?

      return CheckResult.new(pass: true, evidence: event.inspect)
    end
    CheckResult.new(pass: false, evidence: "no live INSTANCE_READY event for D; buffer=#{events.length} errors=#{streams.errors.inspect}")
  end

  def check_metric(previous)
    body = fetch_metrics(roles["D"].metrics_addr)
    value = parse_cascade_ticks(body, roles["D"].uid)
    return [CheckResult.new(pass: false, evidence: body), previous] if value.nil?
    return [CheckResult.new(pass: false, evidence: "cascade_ticks_total=#{value} did not increase beyond #{previous}\n#{body}"), value] if value <= previous

    [CheckResult.new(pass: true, evidence: "cascade_ticks_total=#{value}"), value]
  rescue StandardError => e
    [CheckResult.new(pass: false, evidence: e.message), previous]
  end

  def check_chain(chain)
    %w[D C B].each_with_index do |role, idx|
      return "chain length #{chain.length} < 3" if idx >= chain.length

      hop = chain[idx]
      want = roles[role]
      if hop.slug != want.slug || hop.instance_uid != want.uid
        return "hop #{idx} = #{hop.slug}/#{hop.instance_uid}, want #{want.slug}/#{want.uid}"
      end
    end
    ""
  end

  def stop
    ROLE_ORDER.reverse_each do |role|
      runtime = roles[role]
      runtime.channel&.close if runtime.channel&.respond_to?(:close)
      begin
        Process.kill("TERM", runtime.pid) if runtime.pid && process_alive?(runtime.pid)
      rescue Errno::ESRCH
        nil
      end
    end
    deadline = Time.now + 3
    roles.values.each do |runtime|
      next unless runtime.wait_thread

      remaining = [0.01, deadline - Time.now].max
      next unless runtime.wait_thread.join(remaining).nil?

      begin
        Process.kill("KILL", runtime.pid) if runtime.pid && process_alive?(runtime.pid)
      rescue Errno::ESRCH
        nil
      end
      runtime.wait_thread.join(2)
    end
  end
end

class LiveStreams
  def initialize(channel)
    @client = Holons::V1::HolonObservability::Stub.new("unused", :this_channel_is_insecure, channel_override: channel)
    @logs = []
    @events = []
    @errors = []
    @mutex = Mutex.new
    @stop = false
    @threads = []
  end

  def start
    @threads = [
      Thread.new { read_logs },
      Thread.new { read_events }
    ]
    @threads.each { |thread| thread.abort_on_exception = false }
  end

  def stop
    @mutex.synchronize { @stop = true }
    @threads.each { |thread| thread.join(0.2) }
  end

  def log_entries
    @mutex.synchronize { @logs.dup }
  end

  def event_entries
    @mutex.synchronize { @events.dup }
  end

  def errors
    @mutex.synchronize { @errors.dup }
  end

  private

  def stopped?
    @mutex.synchronize { @stop }
  end

  def read_logs
    @client.logs(Holons::V1::LogsRequest.new(min_level: :INFO, follow: true)).each do |entry|
      break if stopped?

      @mutex.synchronize { @logs << entry }
    end
  rescue StandardError => e
    @mutex.synchronize { @errors << "logs stream ended: #{e.message}" }
  end

  def read_events
    @client.events(Holons::V1::EventsRequest.new(follow: true)).each do |event|
      break if stopped?

      @mutex.synchronize { @events << event }
    end
  rescue StandardError => e
    @mutex.synchronize { @errors << "events stream ended: #{e.message}" }
  end
end

class ObservabilityCascadeService < ObservabilityCascade::V1::ObservabilityCascadeService::Service
  def run_default(_request, _call)
    to_cascade_report(execute_default(emit: false))
  end

  def run_live_stream(_request, _call)
    to_cascade_report(execute_live_stream(emit: false))
  end

  def run_multi_pattern(_request, _call)
    to_multi_pattern_report(execute_multi_pattern(emit: false))
  end
end

def serve_composite(args)
  Holons::Describe.use_static_response(Gen::DescribeGenerated.static_describe_response)
  options = Holons::Serve.parse_options(args)
  Holons::Serve.run_with_serve_options(
    normalize_listen_uri(options.listen_uri),
    proc { |server| server.handle(ObservabilityCascadeService.new) },
    Holons::Serve::ServeOptions.new(
      reflect: options.reflect,
      slug: "observability-cascade-ruby"
    )
  )
end

def main
  args = ARGV.dup
  if !args.empty? && canonical_command(args.first) == "serve"
    serve_composite(args[1..])
  elsif args.include?("--multi-pattern")
    execute_multi_pattern
  elsif args.include?("--live-stream")
    execute_live_stream
  else
    execute_default
  end
  0
rescue StandardError => e
  warn "\nFAIL: #{e.message}"
  1
end

def execute_default(emit: true)
  binary = find_binary(RUBY_SLUG)
  run_root = Pathname.new(Dir.mktmpdir("observability-cascade-ruby-"))
  output(emit, "=== observability-cascade-ruby ===")
  output(emit)
  total_pass = 0
  total_fail = 0
  previous = ""
  TRANSPORTS.each_with_index do |transport, idx|
    phase = idx + 1
    output(emit, previous.empty? ? "Phase #{phase}/#{RUN_PHASES}: transport=#{transport}" : "Phase #{phase}/#{RUN_PHASES}: transport=#{transport} (switching from #{previous})")
    started = monotonic_now
    begin
      specs = ROLE_ORDER.to_h { |role| [role, RoleSpec.new(slug: RUBY_SLUG, binary_path: binary)] }
      cascade = spawn_cascade(phase, transport, specs, run_root)
    rescue StandardError => e
      total_fail += RUN_TICKS
      output(emit, "  spawn FAIL: #{e.message}\n\n")
      previous = transport
      next
    end
    output(emit, "  spawned 4 nodes in #{elapsed(started)}")
    previous_metric = 0.0
    (1..RUN_TICKS).each do |tick|
      tick_start = monotonic_now
      outcome = cascade.run_tick(tick, previous_metric)
      previous_metric = outcome.metric_value if outcome.metric.pass
      overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass
      total_pass += 1 if overall
      total_fail += 1 unless overall
      output(emit, "  Tick #{tick}/#{RUN_TICKS}: log #{pass_text(outcome.log.pass)}, event #{pass_text(outcome.event.pass)}, metric #{pass_text(outcome.metric.pass)} (overall #{pass_text(overall)} in #{elapsed(tick_start)})")
      if emit
        print_failure_evidence("log", outcome.log)
        print_failure_evidence("event", outcome.event)
        print_failure_evidence("metric", outcome.metric)
      end
    end
    cascade.stop
    output(emit)
    previous = transport
  end
  output(emit, "Summary: #{total_pass + total_fail} ticks, #{total_pass} PASS, #{total_fail} FAIL")
  raise "#{total_fail} tick(s) failed" if total_fail.positive?

  CascadeReportData.new(
    ticks: total_pass + total_fail,
    pass_count: total_pass,
    fail_count: total_fail,
    phases: [PhaseReportData.new(name: "default", pass_count: total_pass, fail_count: total_fail, failures: [])]
  )
end

def execute_live_stream(emit: true)
  binary = find_binary(RUBY_SLUG)
  run_root = Pathname.new(Dir.mktmpdir("observability-cascade-ruby-live-"))
  output(emit, "=== observability-cascade-ruby --live-stream ===")
  output(emit)
  output(emit, "Setup: opening long-lived Follow:true streams on A")
  output(emit, "       (initial transport: tcp)")
  output(emit)
  total_pass = 0
  total_fail = 0
  cascade = nil
  streams = nil
  specs = ROLE_ORDER.to_h { |role| [role, RoleSpec.new(slug: RUBY_SLUG, binary_path: binary)] }
  TRANSPORTS.each_with_index do |transport, idx|
    phase = idx + 1
    if phase == 1
      output(emit, "Phase #{phase}/#{RUN_PHASES}: initial chain (#{transport})")
    else
      output(emit, "Phase #{phase}/#{RUN_PHASES}: respawn on #{transport}")
      kill_start = monotonic_now
      streams&.stop
      cascade&.stop
      output(emit, "  killed 4 nodes in #{elapsed(kill_start)}")
    end
    spawn_start = monotonic_now
    begin
      phase_cascade = spawn_cascade(phase, transport, specs, run_root)
    rescue StandardError => e
      total_fail += RUN_TICKS
      output(emit, "  spawn FAIL: #{e.message}\n\n")
      streams = nil
      next
    end
    output(emit, "  spawned 4 nodes in #{elapsed(spawn_start)}")
    output(emit, "  re-opening Follow:true streams on new A") if phase > 1
    stream_error = nil
    begin
      streams = LiveStreams.new(phase_cascade.roles["A"].channel)
      streams.start
    rescue StandardError => e
      streams = nil
      stream_error = e.message
      output(emit, "  stream re-open failed: #{e.message}")
    end
    previous_metric = 0.0
    (1..RUN_TICKS).each do |tick|
      tick_start = monotonic_now
      outcome = phase_cascade.run_live_tick(streams, stream_error, tick, previous_metric)
      previous_metric = outcome.metric_value if outcome.metric.pass
      overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass
      total_pass += 1 if overall
      total_fail += 1 unless overall
      output(emit, "  Tick #{tick}/#{RUN_TICKS}: log #{pass_text(outcome.log.pass)}, event #{pass_text(outcome.event.pass)}, metric #{pass_text(outcome.metric.pass)} (overall #{pass_text(overall)} in #{elapsed(tick_start)})")
      if emit
        print_failure_evidence("log", outcome.log)
        print_failure_evidence("event", outcome.event)
        print_failure_evidence("metric", outcome.metric)
      end
    end
    output(emit)
    cascade = phase_cascade
  end
  streams&.stop
  cascade&.stop
  output(emit, "Summary: #{total_pass} PASS / #{total_fail} FAIL across #{total_pass + total_fail} ticks")
  raise "#{total_fail} tick(s) failed" if total_fail.positive?

  CascadeReportData.new(
    ticks: total_pass + total_fail,
    pass_count: total_pass,
    fail_count: total_fail,
    phases: [PhaseReportData.new(name: "live-stream", pass_count: total_pass, fail_count: total_fail, failures: [])]
  )
end

def execute_multi_pattern(emit: true)
  ruby_binary = find_binary(RUBY_SLUG)
  go_binary = find_binary(GO_SLUG)
  patterns = [
    ["ruby-ruby-ruby-ruby", ROLE_ORDER.to_h { |role| [role, RoleSpec.new(slug: RUBY_SLUG, binary_path: ruby_binary)] }],
    ["ruby-ruby-go-ruby", {
      "A" => RoleSpec.new(slug: RUBY_SLUG, binary_path: ruby_binary),
      "B" => RoleSpec.new(slug: RUBY_SLUG, binary_path: ruby_binary),
      "C" => RoleSpec.new(slug: GO_SLUG, binary_path: go_binary),
      "D" => RoleSpec.new(slug: RUBY_SLUG, binary_path: ruby_binary)
    }],
    ["ruby-ruby-go-go", {
      "A" => RoleSpec.new(slug: RUBY_SLUG, binary_path: ruby_binary),
      "B" => RoleSpec.new(slug: RUBY_SLUG, binary_path: ruby_binary),
      "C" => RoleSpec.new(slug: GO_SLUG, binary_path: go_binary),
      "D" => RoleSpec.new(slug: GO_SLUG, binary_path: go_binary)
    }]
  ]
  run_root = Pathname.new(Dir.mktmpdir("observability-cascade-ruby-multi-"))
  output(emit, "=== observability-cascade-ruby (multi-pattern) ===")
  output(emit)
  total_pass = 0
  total_fail = 0
  patterns.each_with_index do |(name, specs), pattern_idx|
    output(emit, "Pattern #{pattern_idx + 1}/#{patterns.length}: #{name}")
    pattern_pass = 0
    TRANSPORTS.each_with_index do |transport, idx|
      phase = idx + 1
      started = monotonic_now
      begin
        cascade = spawn_cascade(phase, transport, specs, run_root)
      rescue StandardError => e
        total_fail += RUN_TICKS
        output(emit, "  Phase #{phase}/#{RUN_PHASES} (#{transport}): spawn FAIL (#{e.message})")
        next
      end
      stream_error = nil
      streams = nil
      begin
        streams = LiveStreams.new(cascade.roles["A"].channel)
        streams.start
        ready = wait_for(5.0, interval: 0.05) { cascade.check_live_event(streams) }
        stream_error = "live relay readiness: #{ready.evidence}" unless ready.pass
      rescue StandardError => e
        stream_error = e.message
      end
      previous_metric = 0.0
      results = []
      evidence = []
      (1..RUN_TICKS).each do |tick|
        sender = "#{name}-phase-#{phase}-tick-#{tick}"
        outcome = cascade.run_live_tick_with_sender(streams, stream_error, sender, previous_metric)
        previous_metric = outcome.metric_value if outcome.metric.pass
        overall = outcome.log.pass && outcome.event.pass && outcome.metric.pass
        if overall
          pattern_pass += 1
          total_pass += 1
          results << "Tick #{tick} PASS"
        else
          total_fail += 1
          results << "Tick #{tick} FAIL (#{failure_summary(outcome)})"
          evidence << "      Tick #{tick} evidence: #{compact_evidence(outcome)}"
        end
      end
      output(emit, "  Phase #{phase}/#{RUN_PHASES} (#{transport}): #{results.join(', ')} (spawned in #{elapsed(started)})")
      evidence.each { |line| puts line } if emit
      streams&.stop
      cascade.stop
    end
    output(emit, "  Subtotal: #{pattern_pass}/12 PASS")
    output(emit)
  end
  output(emit, "Summary: #{total_pass} PASS / #{total_fail} FAIL across #{total_pass + total_fail} ticks")
  raise "#{total_fail} tick(s) failed" if total_fail.positive?

  MultiPatternReportData.new(
    patterns: [
      CascadeReportData.new(
        ticks: total_pass + total_fail,
        pass_count: total_pass,
        fail_count: total_fail,
        phases: [PhaseReportData.new(name: "multi-pattern", pass_count: total_pass, fail_count: total_fail, failures: [])]
      )
    ],
    total_pass: total_pass,
    total_fail: total_fail
  )
end

def spawn_cascade(phase, transport, specs, run_root)
  roles = ROLE_ORDER.to_h { |role| [role, new_role_runtime(phase, transport, role, specs[role])] }
  roles.values.each { |runtime| FileUtils.rm_rf(run_root.join(runtime.slug, runtime.uid)) }
  cascade = Cascade.new(phase: phase, transport: transport, run_root: run_root, roles: roles)
  ROLE_ORDER.each do |role|
    runtime = roles[role]
    child = child_role(role)
    if child
      runtime.member_address = roles[child].relay_address
      runtime.member_slug = roles[child].slug
    end
    start_role(cascade, runtime)
  end
  sleep 0.15
  cascade
end

def new_role_runtime(phase, transport, role, spec)
  uid = format("relay-p%02d-%s", phase, role.downcase)
  case transport
  when "tcp"
    port = free_port
    listen_uri = "tcp://127.0.0.1:#{port}"
    client_target = "127.0.0.1:#{port}"
    relay_address = listen_uri
  when "unix"
    path = "/tmp/observability-cascade-ruby-p#{phase}-#{role.downcase}-#{Process.pid}.sock"
    File.delete(path) if File.exist?(path)
    listen_uri = "unix://#{path}"
    client_target = listen_uri
    relay_address = listen_uri
  else
    raise "unknown transport #{transport}"
  end
  RoleRuntime.new(
    role: role,
    uid: uid,
    slug: spec.slug,
    binary_path: spec.binary_path,
    listen_uri: listen_uri,
    relay_address: relay_address,
    client_target: client_target,
    member_address: "",
    member_slug: "",
    metrics_addr: "",
    stderr_path: File.join(Dir.tmpdir, "observability-cascade-ruby-#{phase}-#{role.downcase}-#{Process.pid}.stderr")
  )
end

def start_role(cascade, runtime)
  args = [runtime.binary_path, "serve", "--listen", runtime.listen_uri]
  args += ["--member", "#{runtime.member_slug}=#{runtime.member_address}"] unless runtime.member_address.to_s.empty?
  env = ENV.to_h.merge(
    "OP_OBS" => "logs,events,metrics,prom",
    "OP_RUN_DIR" => cascade.run_root.to_s,
    "OP_INSTANCE_UID" => runtime.uid,
    "OP_ORGANISM_UID" => cascade.roles["A"].uid,
    "OP_ORGANISM_SLUG" => cascade.roles["A"].slug,
    "OP_PROM_ADDR" => "127.0.0.1:0"
  )
  File.open(runtime.stderr_path, "w") do |stderr|
    runtime.pid = Process.spawn(env, *args, chdir: ROOT || HERE, out: File::NULL, err: stderr)
  end
  runtime.wait_thread = Process.detach(runtime.pid)
  meta = wait_meta(cascade.run_root, runtime.slug, runtime.uid, 10.0)
  runtime.metrics_addr = meta.fetch("metrics_addr")
  runtime.channel = dial_ready(runtime.client_target, 10.0)
rescue StandardError
  stderr = File.exist?(runtime.stderr_path) ? File.read(runtime.stderr_path) : ""
  raise "start #{runtime.role}: #{stderr}"
end

def child_role(role)
  { "A" => "B", "B" => "C", "C" => "D" }[role]
end

def wait_meta(run_root, slug, uid, timeout)
  path = run_root.join(slug, uid, "meta.json")
  deadline = monotonic_now + timeout
  last_error = nil
  while monotonic_now < deadline
    begin
      data = JSON.parse(File.read(path))
      return data if data["uid"] == uid && !data["metrics_addr"].to_s.empty?
    rescue StandardError => e
      last_error = e
    end
    sleep 0.05
  end
  raise "meta not ready for #{slug}/#{uid}: #{last_error}"
end

def dial_ready(target, timeout)
  deadline = monotonic_now + timeout
  last_error = nil
  while monotonic_now < deadline
    channel = GRPC::Core::Channel.new(target, {}, :this_channel_is_insecure)
    begin
      Holons::V1::HolonMeta::Stub.new(
        "unused",
        :this_channel_is_insecure,
        channel_override: channel,
        timeout: 0.5
      ).describe(Holons::V1::DescribeRequest.new)
      return channel
    rescue StandardError => e
      last_error = e
      channel.close if channel.respond_to?(:close)
      sleep 0.05
    end
  end
  raise "dial #{target}: #{last_error}"
end

def read_logs(channel)
  Holons::V1::HolonObservability::Stub.new(
    "unused",
    :this_channel_is_insecure,
    channel_override: channel,
    timeout: 2
  ).logs(Holons::V1::LogsRequest.new(min_level: :INFO, follow: false)).to_a
end

def read_events(channel)
  Holons::V1::HolonObservability::Stub.new(
    "unused",
    :this_channel_is_insecure,
    channel_override: channel,
    timeout: 2
  ).events(Holons::V1::EventsRequest.new(follow: false)).to_a
end

def fetch_metrics(addr)
  Net::HTTP.get(URI(addr))
end

def parse_cascade_ticks(body, uid)
  needle = %(responder_uid="#{uid}")
  body.each_line do |line|
    next unless line.start_with?("cascade_ticks_total{") && line.include?(needle)

    parts = line.split
    return Float(parts[-1]) if parts.length >= 2
  end
  nil
end

def wait_for(timeout, interval: 0.1)
  deadline = monotonic_now + timeout
  last = CheckResult.new(pass: false, evidence: "")
  loop do
    last = yield
    return last if last.pass || monotonic_now > deadline

    sleep interval
  end
end

def find_binary(slug)
  return Holons::Composite.member("ruby-node") if slug == RUBY_SLUG

  roots = []
  roots << File.join(ENV.fetch("OPBIN", File.join(Dir.home, ".op", "bin")), "#{slug}.holon", "bin")
  roots.each do |root|
    next unless Dir.exist?(root)

    match = Dir.glob(File.join(root, "**", slug)).find { |candidate| File.executable?(candidate) }
    return match if match
  end

  raise "#{slug} binary not found; run op build #{slug} --install"
end

def to_cascade_report(report)
  ObservabilityCascade::V1::CascadeReport.new(
    ticks: report.ticks,
    pass: report.pass_count,
    fail: report.fail_count,
    phases: report.phases.map do |phase|
      ObservabilityCascade::V1::PhaseResult.new(
        name: phase.name,
        pass: phase.pass_count,
        fail: phase.fail_count,
        failures: phase.failures
      )
    end
  )
end

def to_multi_pattern_report(report)
  ObservabilityCascade::V1::MultiPatternReport.new(
    patterns: report.patterns.map { |pattern| to_cascade_report(pattern) },
    total_pass: report.total_pass,
    total_fail: report.total_fail
  )
end

def normalize_listen_uri(listen_uri)
  return "tcp://0.0.0.0:#{Regexp.last_match(1)}" if listen_uri =~ %r{\Atcp://:(\d+)\z}

  listen_uri
end

def output(emit, value = "")
  puts value if emit
end

def canonical_command(raw)
  raw.strip.downcase.tr("-_ ", "")
end

def free_port
  server = TCPServer.new("127.0.0.1", 0)
  server.addr[1]
ensure
  server&.close
end

def elapsed(start)
  seconds = [0.0, monotonic_now - start].max
  return "#{(seconds * 1000).to_i}ms" if seconds < 1

  format("%.1fs", seconds)
end

def pass_text(value)
  value ? "PASS" : "FAIL"
end

def print_failure_evidence(family, result)
  puts "    #{family} evidence: #{result.evidence.empty? ? '<empty>' : result.evidence}" unless result.pass
end

def failure_summary(outcome)
  missing = []
  missing << "log family" unless outcome.log.pass
  missing << "event family" unless outcome.event.pass
  missing << "metric family" unless outcome.metric.pass
  missing.empty? ? "unknown" : missing.join(", ")
end

def compact_evidence(outcome)
  parts = []
  parts << "log=#{outcome.log.evidence}" unless outcome.log.pass
  parts << "event=#{outcome.event.evidence}" unless outcome.event.pass
  parts << "metric=#{outcome.metric.evidence}" unless outcome.metric.pass
  parts.join(" | ")
end

def process_alive?(pid)
  Process.kill(0, pid)
  true
rescue Errno::ESRCH
  false
end

def monotonic_now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

exit main
