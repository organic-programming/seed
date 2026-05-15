#!/usr/bin/env ruby
# frozen_string_literal: true

require "pathname"

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
require "observability_cascade/v1/service_pb"
require "observability_cascade/v1/service_services_pb"
require "relay/v1/relay_pb"
require "relay/v1/relay_services_pb"

Holons::Describe.use_static_response(Gen::DescribeGenerated.static_describe_response)

RUN_TICKS = 3
RUBY_SLUG = "observability-cascade-ruby-node"
GO_SLUG = "observability-cascade-go-node"

LanguageMember = Struct.new(:lang, :slug, :binary, keyword_init: true)
TickResult = Struct.new(:pass, :log, :event, :hops, keyword_init: true)
NamedPattern = Struct.new(:name, :members, keyword_init: true)

class ObservabilityCascadeService < ObservabilityCascade::V1::ObservabilityCascadeService::Service
  def run_default(_request, _call)
    run_report("default", own_language_members, false, false)
  end

  def run_live_stream(_request, _call)
    run_report("live-stream", own_language_members, true, false)
  end

  def run_multi_pattern(_request, _call)
    run_multi_pattern_report(false)
  end
end

def serve_composite(args)
  options = Holons::Serve.parse_options(args)
  Holons::Serve.run_with_serve_options(
    normalize_listen_uri(options.listen_uri),
    proc { |server| server.handle(ObservabilityCascadeService.new) },
    Holons::Serve::ServeOptions.new(reflect: options.reflect, slug: "observability-cascade-ruby")
  )
end

def main
  args = ARGV.dup
  if !args.empty? && canonical_command(args.first) == "serve"
    serve_composite(args[1..])
    return 0
  end

  failed =
    if args.include?("--multi-pattern")
      run_multi_pattern_report(true).total_fail
    else
      report = run_report(args.include?("--live-stream") ? "live-stream" : "default", own_language_members, args.include?("--live-stream"), true)
      report.fail
    end
  failed.positive? ? 1 : 0
rescue StandardError => e
  warn "FAIL: #{e.message}"
  1
end

def run_multi_pattern_report(emit)
  total_start = monotonic_now
  patterns = ruby_patterns
  out = ObservabilityCascade::V1::MultiPatternReport.new
  if emit
    puts "=== observability-cascade-ruby --multi-pattern ==="
    puts
  end
  patterns.each_with_index do |pattern, idx|
    puts "Pattern #{idx + 1}/#{patterns.length}: #{pattern.name}" if emit
    report = run_report(pattern.name, pattern.members, true, emit)
    out.patterns << report
    out.total_pass += report.pass
    out.total_fail += report.fail
    if emit
      status = report.fail.positive? ? "FAIL" : "PASS"
      puts "Pattern #{pattern.name}: #{report.pass}/#{report.ticks} #{status} (elapsed=#{elapsed_text(report.elapsed_us)})"
      puts
    end
  end
  out.total_elapsed_us = elapsed_us(total_start)
  if emit
    puts "Summary: #{out.total_pass} PASS / #{out.total_fail} FAIL across #{out.total_pass + out.total_fail} ticks (total elapsed=#{elapsed_text(out.total_elapsed_us)})"
  end
  out
end

def run_report(name, members, live, emit)
  ensure_cascade_observability
  report_start = monotonic_now
  report = ObservabilityCascade::V1::CascadeReport.new(name: name)
  poll = live ? 0.05 : 0.1
  timeout = live ? 1.0 : 3.0
  if emit
    puts "=== observability-cascade-ruby #{mode_suffix(name)}==="
    puts
  end

  Holons::Composite::TransportCoverageSequence.each_with_index do |transport, idx|
    phase_start = monotonic_now
    from = idx.zero? ? transport : Holons::Composite::TransportCoverageSequence[idx - 1]
    phase = ObservabilityCascade::V1::PhaseResult.new(name: format("%02d-%s→%s", idx + 1, from, transport))
    puts "Phase #{idx + 1}/#{Holons::Composite::TransportCoverageSequence.length}: #{phase.name}" if emit
    cascade = nil
    begin
      cascade = Holons::Composite.build_cascade(
        Holons::Composite::CascadeOptions.new(
          transport: transport,
          members: child_specs(members),
          extra_env: {
            "OP_OBS" => "logs,events,metrics,prom",
            "OP_PROM_ADDR" => "127.0.0.1:0"
          }
        )
      )
    rescue StandardError => e
      phase.fail += RUN_TICKS
      RUN_TICKS.times do |tick|
        phase.failures << "tick=#{tick + 1} log=spawn event=spawn hops=#{compact_evidence(e.message)}"
      end
      finish_phase(report, phase, phase_start, emit)
      next
    end

    previous = {}
    (1..RUN_TICKS).each do |tick|
      sender = "#{name}-phase-#{format('%02d', idx + 1)}-tick-#{tick}"
      result = run_tick(cascade, sender, transport, members, previous, timeout, poll, live)
      if result.pass
        phase.pass += 1
      else
        phase.fail += 1
        phase.failures << evidence_line(result, tick)
      end
      if emit
        puts "  Tick #{tick}/#{RUN_TICKS}: #{pass_text(result.pass)}"
        warn "    #{evidence_line(result, tick)}" unless result.pass
      end
    end
    cascade.stop
    finish_phase(report, phase, phase_start, emit)
  end
  report.elapsed_us = elapsed_us(report_start)
  puts "\nSummary: #{report.ticks} ticks, #{report.pass} PASS, #{report.fail} FAIL (total elapsed=#{elapsed_text(report.elapsed_us)})" if emit
  report
end

def run_tick(cascade, sender, note, members, previous, timeout, poll, live)
  resp = Relay::V1::RelayService::Stub.new(
    "unused",
    :this_channel_is_insecure,
    channel_override: cascade.top.conn,
    timeout: 5
  ).tick(Relay::V1::TickRequest.new(sender: sender, note: note))
  hops = check_hops(resp.hops, members, previous)
  return TickResult.new(pass: false, log: skipped("skipped"), event: skipped("skipped"), hops: hops) unless hops.pass

  expected_chain = resp.hops.map { |hop| { slug: hop.slug, instance_uid: hop.uid } }
  leaf_uid = resp.hops.first.uid
  log = Holons::Composite.check_relayed_log(
    Holons::Composite::LogCheckOptions.new(
      sender: sender,
      leaf_uid: leaf_uid,
      expected_chain: expected_chain,
      timeout: timeout,
      poll_interval: poll,
      live: live
    )
  )
  event = Holons::Composite.check_relayed_event(
    Holons::Composite::EventCheckOptions.new(
      event_type: Holons::Observability::EVENT_TYPES[:instance_ready],
      leaf_uid: leaf_uid,
      expected_chain: expected_chain,
      timeout: timeout,
      poll_interval: poll,
      live: live
    )
  )
  TickResult.new(pass: hops.pass && log.pass && event.pass, log: log, event: event, hops: hops)
rescue StandardError => e
  failure = skipped(compact_evidence(e.message))
  TickResult.new(pass: false, log: failure, event: failure, hops: failure)
end

def check_hops(hops, members, previous)
  return skipped("hops length #{hops.length} want #{members.length}") unless hops.length == members.length

  hops.each_with_index do |hop, idx|
    want = members[members.length - 1 - idx]
    return skipped("hop #{idx} slug=#{hop.slug} want #{want.slug}") unless hop.slug == want.slug
    return skipped("hop #{idx} uid empty") if hop.uid.to_s.empty?
    return skipped("hop #{idx} received=#{hop.received} previous=#{previous[hop.uid]}") if hop.received <= previous.fetch(hop.uid, 0)

    previous[hop.uid] = hop.received
  end
  Holons::Composite::CheckOutcome.new(pass: true, evidence: "ok")
end

def skipped(evidence)
  Holons::Composite::CheckOutcome.new(pass: false, evidence: evidence)
end

def own_language_members
  binary = Holons::Composite.member("ruby-node")
  [
    LanguageMember.new(lang: "ruby", slug: RUBY_SLUG, binary: binary),
    LanguageMember.new(lang: "ruby", slug: RUBY_SLUG, binary: binary),
    LanguageMember.new(lang: "ruby", slug: RUBY_SLUG, binary: binary)
  ]
end

def ruby_patterns
  ruby_binary = Holons::Composite.member("ruby-node")
  go_binary = Holons::Composite.member("go-node")
  bins = {
    "ruby" => LanguageMember.new(lang: "ruby", slug: RUBY_SLUG, binary: ruby_binary),
    "go" => LanguageMember.new(lang: "go", slug: GO_SLUG, binary: go_binary)
  }
  %w[
    ruby-ruby-ruby ruby-ruby-go ruby-go-ruby ruby-go-go
    go-ruby-ruby go-ruby-go go-go-ruby go-go-go
  ].map do |name|
    NamedPattern.new(name: name, members: name.split("-").map { |part| bins.fetch(part) })
  end
end

def child_specs(members)
  members.map { |member| Holons::Composite::ChildSpec.new(slug: member.slug, binary: member.binary) }
end

def finish_phase(report, phase, phase_start, emit)
  phase.elapsed_us = elapsed_us(phase_start)
  report.phases << phase
  report.pass += phase.pass
  report.fail += phase.fail
  report.ticks += phase.pass + phase.fail
  print_phase_summary(phase) if emit
end

def ensure_cascade_observability
  obs = Holons::Observability.current
  return if obs.enabled?(:logs) && obs.enabled?(:events)

  ENV["OP_OBS"] = "logs,events,metrics,prom"
  Holons::Observability.from_env(Holons::Observability::Config.new(slug: "observability-cascade-ruby"))
end

def evidence_line(result, tick)
  "tick=#{tick} log=#{evidence_text(result.log)} event=#{evidence_text(result.event)} hops=#{evidence_text(result.hops)}"
end

def evidence_text(result)
  result.pass ? "ok" : compact_evidence(result.evidence)
end

def compact_evidence(value)
  text = value.to_s.split.join(" ")
  return "<empty>" if text.empty?
  return text if text.length <= 240

  "#{text[0, 240]}..."
end

def pass_text(pass)
  pass ? "PASS" : "FAIL"
end

def print_phase_summary(phase)
  status = phase.fail.positive? ? "FAIL" : "PASS"
  puts "Phase #{phase.name}: #{phase.pass}/#{phase.pass + phase.fail} #{status} (elapsed=#{elapsed_text(phase.elapsed_us)})"
end

def elapsed_text(us)
  seconds = us.to_f / 1_000_000.0
  return "#{(seconds * 1000).to_i}ms" if seconds < 1.0
  return format("%.2fs", seconds) if seconds < 60.0

  format("%.1fm", seconds / 60.0)
end

def elapsed_us(start)
  ((monotonic_now - start) * 1_000_000).to_i
end

def mode_suffix(name)
  name == "default" ? "" : "--#{name} "
end

def normalize_listen_uri(listen_uri)
  return "tcp://0.0.0.0:#{Regexp.last_match(1)}" if listen_uri =~ %r{\Atcp://:(\d+)\z}

  listen_uri
end

def canonical_command(raw)
  raw.to_s.strip.downcase.tr("-_ ", "")
end

def monotonic_now
  Process.clock_gettime(Process::CLOCK_MONOTONIC)
end

exit main
