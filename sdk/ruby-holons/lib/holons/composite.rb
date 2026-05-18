# frozen_string_literal: true

require "fileutils"
require "json"
require "securerandom"
require "tmpdir"
require "timeout"

require_relative "discover"
require_relative "observability"

module Holons
  module Composite
    TransportCoverageSequence = %w[
      stdio stdio tcp unix tcp tcp stdio unix unix stdio
    ].freeze

    ChildSpec = Struct.new(:slug, :binary, keyword_init: true)
    SpawnOptions = Struct.new(
      :slug, :binary_path, :transport, :instance_uid, :downstream_chain,
      :extra_env, :dial_options,
      keyword_init: true
    )
    CascadeOptions = Struct.new(:transport, :members, :extra_env, keyword_init: true)
    Cascade = Struct.new(:top, keyword_init: true) do
      def stop(timeout: 3.0)
        top&.stop(timeout: timeout)
      end
    end
    CheckOutcome = Struct.new(:pass, :evidence, keyword_init: true)
    LogCheckOptions = Struct.new(
      :conn, :sender, :leaf_uid, :expected_chain, :timeout, :poll_interval,
      :live,
      keyword_init: true
    )
    EventCheckOptions = Struct.new(
      :conn, :event_name, :leaf_uid, :expected_chain, :timeout, :poll_interval,
      :live,
      keyword_init: true
    )

    class SpawnedMember
      attr_reader :slug, :uid, :listen_uri, :conn

      def initialize(slug:, uid:, listen_uri:, conn:, pid: nil, wait_thread: nil, proxy: nil, relay: nil)
        @slug = slug
        @uid = uid
        @listen_uri = listen_uri
        @conn = conn
        @pid = pid
        @wait_thread = wait_thread
        @proxy = proxy
        @relay = relay
        @stopped = false
        @mutex = Mutex.new
      end

      def relay=(relay)
        @relay = relay
      end

      def stop(timeout: 3.0)
        pid = nil
        wait_thread = nil
        proxy = nil
        conn = nil
        relay = nil
        @mutex.synchronize do
          return if @stopped

          @stopped = true
          pid = @pid
          wait_thread = @wait_thread
          proxy = @proxy
          conn = @conn
          relay = @relay
          @pid = nil
          @wait_thread = nil
          @proxy = nil
          @conn = nil
          @relay = nil
        end

        Holons::DiscoverySupport.close_channel(conn)
        relay&.stop
        proxy&.close
        stop_process(pid, wait_thread, timeout)
        nil
      end

      private

      def stop_process(pid, wait_thread, timeout)
        return if pid.nil?
        return unless process_alive?(pid)

        begin
          Process.kill("TERM", pid)
        rescue Errno::ESRCH
          return
        end
        return unless wait_thread&.join(timeout.to_f).nil?

        begin
          Process.kill("KILL", pid)
        rescue Errno::ESRCH
          nil
        end
        wait_thread&.join(2)
      end

      def process_alive?(pid)
        Process.kill(0, pid)
        true
      rescue Errno::EPERM
        true
      rescue Errno::ESRCH
        false
      end
    end

    module_function

    def member(id)
      executable = ENV.fetch("OP_HOLON_EXECUTABLE", "").strip
      executable = $PROGRAM_NAME if executable.empty?
      member_from_executable(executable, id)
    end

    def member_from_executable(executable, id)
      raise ArgumentError, "member id is required" if id.to_s.strip.empty?

      member_dir = File.join(File.dirname(File.expand_path(executable.to_s)), "holons", id.to_s)
      raise Errno::ENOENT, member_dir unless File.directory?(member_dir)

      match = Dir.children(member_dir).sort.find do |name|
        path = File.join(member_dir, name)
        File.file?(path) && (File.executable?(path) || name.end_with?(".exe"))
      end
      raise "no executable found in #{member_dir}" if match.nil?

      File.join(member_dir, match)
    end

    def with_transitive_observability(enabled)
      proc { |opts| opts[:transitive_observability] = enabled ? true : false }
    end

    def spawn_member(options)
      raise Holons.grpc_load_error unless Holons.grpc_available?

      opts = normalize_spawn_options(options)
      slug = opts.slug.to_s.strip
      binary = opts.binary_path.to_s.strip
      raise ArgumentError, "spawn member: slug is required" if slug.empty?
      raise ArgumentError, "spawn member #{slug}: binary path is required" if binary.empty?

      uid = opts.instance_uid.to_s.strip
      uid = SecureRandom.hex(12) if uid.empty?
      transport = opts.transport.to_s.strip.downcase
      transport = "stdio" if transport.empty?
      listen_uri, cleanup_path = listen_uri_for_spawn(transport, uid)
      FileUtils.rm_f(cleanup_path) unless cleanup_path.to_s.empty?

      child_specs = Array(opts.downstream_chain).map { |child| normalize_child(child) }
      serve_args = ["--transport", transport]
      child_specs.each { |child| serve_args += ["--child", "#{child.slug}=#{child.binary}"] }
      env = spawn_environment(uid, opts.extra_env || {})
      member = start_member_process(binary, slug, uid, transport, listen_uri, serve_args, env)

      dial_opts = apply_dial_options(Array(opts.dial_options))
      transitive = dial_opts.fetch(:transitive_observability, true)
      if transitive
        member.relay = start_relay_on(slug, uid, member.conn)
      end
      member
    rescue StandardError
      member&.stop
      raise
    end

    def build_cascade(options)
      opts = normalize_cascade_options(options)
      members = Array(opts.members).map { |member| normalize_child(member) }
      raise ArgumentError, "build cascade: at least one member is required" if members.empty?

      top = members.first
      spawned = spawn_member(
        SpawnOptions.new(
          slug: top.slug,
          binary_path: top.binary,
          transport: opts.transport,
          downstream_chain: members.drop(1),
          extra_env: opts.extra_env || {}
        )
      )
      Cascade.new(top: spawned)
    end

    def dial(address, *options)
      raise Holons.grpc_load_error unless Holons.grpc_available?

      target = normalize_address_for_dial(address)
      channel = dial_ready(target, 10_000, describe: true)
      dial_opts = apply_dial_options(options)
      return channel unless dial_opts.fetch(:transitive_observability, false)

      identity = resolve_relay_member_identity(channel)
      start_relay_on(identity.slug, identity.uid, channel)
      channel
    rescue StandardError
      Holons::DiscoverySupport.close_channel(channel)
      raise
    end

    def check_relayed_log(options)
      opts = normalize_log_check_options(options)
      poll_until(opts.timeout || 3.0, opts.poll_interval || 0.1) do
        entries = read_log_entries(opts.conn)
        match_relayed_log(entries, opts)
      end
    end

    def check_relayed_event(options)
      opts = normalize_event_check_options(options)
      poll_until(opts.timeout || 3.0, opts.poll_interval || 0.1) do
        events = read_event_entries(opts.conn)
        match_relayed_event(events, opts)
      end
    end

    def normalize_spawn_options(options)
      return options if options.is_a?(SpawnOptions)

      data = symbolize_hash(options)
      SpawnOptions.new(
        slug: data[:slug],
        binary_path: data[:binary_path] || data[:binary],
        transport: data[:transport],
        instance_uid: data[:instance_uid],
        downstream_chain: data[:downstream_chain] || data[:children],
        extra_env: data[:extra_env],
        dial_options: data[:dial_options]
      )
    end

    def normalize_cascade_options(options)
      return options if options.is_a?(CascadeOptions)

      data = symbolize_hash(options)
      CascadeOptions.new(
        transport: data[:transport],
        members: data[:members],
        extra_env: data[:extra_env]
      )
    end

    def normalize_log_check_options(options)
      return options if options.is_a?(LogCheckOptions)

      data = symbolize_hash(options)
      LogCheckOptions.new(
        conn: data[:conn],
        sender: data[:sender],
        leaf_uid: data[:leaf_uid],
        expected_chain: data[:expected_chain] || [],
        timeout: data[:timeout],
        poll_interval: data[:poll_interval],
        live: data[:live]
      )
    end

    def normalize_event_check_options(options)
      return options if options.is_a?(EventCheckOptions)

      data = symbolize_hash(options)
      EventCheckOptions.new(
        conn: data[:conn],
        event_name: data[:event_name],
        leaf_uid: data[:leaf_uid],
        expected_chain: data[:expected_chain] || [],
        timeout: data[:timeout],
        poll_interval: data[:poll_interval],
        live: data[:live]
      )
    end

    def normalize_child(child)
      return child if child.is_a?(ChildSpec)
      return ChildSpec.new(slug: child.slug, binary: child.binary) if child.respond_to?(:slug) && child.respond_to?(:binary)

      data = symbolize_hash(child)
      ChildSpec.new(slug: data[:slug].to_s, binary: (data[:binary] || data[:binary_path]).to_s)
    end

    def listen_uri_for_spawn(transport, uid)
      case transport
      when "stdio"
        ["stdio://", ""]
      when "tcp"
        ["tcp://127.0.0.1:0", ""]
      when "unix"
        path = File.join(Dir.tmpdir, "op-ruby-#{clean_socket_token(uid)}.sock")
        ["unix://#{path}", path]
      else
        raise ArgumentError, %(unsupported transport "#{transport}")
      end
    end

    def start_member_process(binary, slug, uid, transport, listen_uri, serve_args, env)
      if transport == "stdio"
        proxy = Holons::DiscoverySupport::StdioDialProxy.new(
          binary,
          serve_args: serve_args,
          chdir: File.dirname(File.expand_path(binary)),
          env: env
        )
        target = proxy.start
        channel = dial_ready(target, 10_000, describe: true)
        return SpawnedMember.new(
          slug: slug,
          uid: uid,
          listen_uri: "stdio://",
          conn: channel,
          pid: proxy.pid,
          wait_thread: proxy.wait_thread,
          proxy: proxy
        )
      end

      args = ["serve", "--listen", listen_uri, *serve_args]
      pid = Process.spawn(env, binary, *args, chdir: File.dirname(File.expand_path(binary)), out: File::NULL, err: $stderr)
      wait_thread = Process.detach(pid)
      meta = wait_spawn_meta(spawn_run_root(env), slug, uid, 10.0)
      channel = dial_ready(normalize_dial_target(meta.fetch("address")), 10_000, describe: true)
      SpawnedMember.new(
        slug: slug,
        uid: uid,
        listen_uri: meta.fetch("address"),
        conn: channel,
        pid: pid,
        wait_thread: wait_thread
      )
    end

    def spawn_environment(uid, extra)
      env = ENV.to_h.dup
      env["OP_INSTANCE_UID"] = uid
      env["OP_RUN_DIR"] = run_root_from_env(env)
      env["HOLONS_PARENT_PID"] = Process.pid.to_s
      families = active_observability_families
      env["OP_OBS"] = families unless families.empty?
      extra.each { |key, value| env[key.to_s] = value.to_s }
      env
    end

    def active_observability_families
      obs = Holons::Observability.current
      %i[logs metrics events prom].select { |family| obs.enabled?(family) }.join(",")
    end

    def run_root_from_env(env)
      root = env["OP_RUN_DIR"].to_s.strip
      return root unless root.empty?

      oppath = env["OPPATH"].to_s.strip
      return File.join(oppath, "run") unless oppath.empty?

      home = env["HOME"].to_s.strip
      return File.join(home, ".op", "run") unless home.empty?

      File.join(Dir.tmpdir, ".op", "run")
    end

    def spawn_run_root(env)
      run_root_from_env(env)
    end

    def wait_spawn_meta(run_root, slug, uid, timeout)
      path = File.join(run_root, slug, uid, "meta.json")
      deadline = monotonic_now + timeout.to_f
      last_error = nil
      until monotonic_now > deadline
        begin
          data = JSON.parse(File.read(path))
          return data if data["uid"] == uid && !data["address"].to_s.empty?
        rescue StandardError => e
          last_error = e
        end
        sleep 0.05
      end
      raise "meta not ready for #{slug}/#{uid}: #{last_error&.message || 'not written'}"
    end

    def dial_ready(target, timeout_ms, describe: false)
      Holons::DiscoverySupport.load_describe_runtime! if describe
      channel = Holons::DiscoverySupport.dial_ready(target, timeout_ms)
      describe_ready(channel, timeout_ms) if describe
      channel
    rescue StandardError
      Holons::DiscoverySupport.close_channel(channel)
      raise
    end

    def describe_ready(channel, timeout_ms)
      deadline = monotonic_now + (timeout_ms.to_f / 1000.0)
      last_error = nil
      until monotonic_now > deadline
        begin
          return Holons::V1::HolonMeta::Stub.new(
            "unused",
            :this_channel_is_insecure,
            channel_override: channel,
            timeout: 0.5
          ).describe(Holons::V1::DescribeRequest.new)
        rescue StandardError => e
          last_error = e
          sleep 0.05
        end
      end
      raise(last_error || "Describe did not become ready")
    end

    def normalize_address_for_dial(address)
      trimmed = address.to_s.strip
      raise ArgumentError, "dial address is required" if trimmed.empty?
      raise ArgumentError, "composite.Dial does not support stdio addresses; use SpawnMember" if trimmed.start_with?("stdio://")

      return normalize_dial_target(trimmed) if trimmed.start_with?("tcp://", "unix://")
      raise ArgumentError, %(unsupported dial address "#{address}") if trimmed.include?("://")
      raise ArgumentError, %(dial address must be tcp://host:port, unix:///path, or host:port: "#{address}") unless trimmed.include?(":")

      trimmed
    end

    def normalize_dial_target(target)
      Holons::DiscoverySupport.normalize_dial_target(target)
    end

    def start_relay_on(slug, uid, channel)
      Holons::Observability.require_grpc_observability_support!
      stub = Holons::V1::HolonObservability::Stub.new(
        "unused",
        :this_channel_is_insecure,
        channel_override: channel
      )
      Holons::Observability::MemberRelay.new(
        child_slug: slug,
        child_uid: uid,
        stub: stub
      ).start
    end

    RelayIdentity = Struct.new(:slug, :uid, keyword_init: true)

    def resolve_relay_member_identity(channel)
      Holons::Observability.require_grpc_observability_support!
      stub = Holons::V1::HolonObservability::Stub.new("unused", :this_channel_is_insecure, channel_override: channel, timeout: 1)
      identity = identity_from_events(stub) || identity_from_logs(stub)
      raise "resolve relay identity: peer did not expose a local log or event with slug and instance_uid" if identity.nil?

      identity
    end

    def identity_from_events(stub)
      stub.events(Holons::V1::EventsRequest.new(follow: false)).each do |event|
        slug = Holons::Observability.record_slug(event)
        uid = Holons::Observability.record_instance_uid(event)
        next if !event.chain.empty? || uid.empty? || slug.empty?

        return RelayIdentity.new(slug: slug, uid: uid)
      end
      nil
    rescue StandardError
      nil
    end

    def identity_from_logs(stub)
      stub.logs(Holons::V1::LogsRequest.new(follow: false)).each do |entry|
        slug = Holons::Observability.record_slug(entry)
        uid = Holons::Observability.record_instance_uid(entry)
        next if !entry.chain.empty? || uid.empty? || slug.empty?

        return RelayIdentity.new(slug: slug, uid: uid)
      end
      nil
    rescue StandardError
      nil
    end

    def read_log_entries(conn)
      if conn.nil?
        ring = Holons::Observability.current.log_ring
        raise "logs family is not enabled" if ring.nil?

        return ring.drain
      end
      Holons::Observability.require_grpc_observability_support!
      stub = Holons::V1::HolonObservability::Stub.new("unused", :this_channel_is_insecure, channel_override: conn, timeout: 2)
      stub.logs(Holons::V1::LogsRequest.new(min_severity_number: :SEVERITY_NUMBER_INFO, follow: false)).map do |entry|
        Holons::Observability.from_proto_log_record(entry)
      end
    end

    def read_event_entries(conn)
      if conn.nil?
        bus = Holons::Observability.current.event_bus
        raise "events family is not enabled" if bus.nil?

        return bus.drain
      end
      Holons::Observability.require_grpc_observability_support!
      stub = Holons::V1::HolonObservability::Stub.new("unused", :this_channel_is_insecure, channel_override: conn, timeout: 2)
      stub.events(Holons::V1::EventsRequest.new(follow: false)).map do |event|
        Holons::Observability.from_proto_log_record(event)
      end
    end

    def match_relayed_log(entries, opts)
      entries.each do |entry|
        record = entry.record
        fields = Holons::Observability.attributes_hash(record.attributes)
        next unless Holons::Observability.body_string(record) == "tick received"
        next unless fields["sender"].to_s == opts.sender.to_s
        next unless fields["responder_uid"].to_s == opts.leaf_uid.to_s

        chain_error = compare_chain(record.chain, opts.expected_chain)
        return CheckOutcome.new(pass: false, evidence: compact_evidence("matching log bad chain: #{chain_error}")) unless chain_error.empty?

        return CheckOutcome.new(pass: true, evidence: "ok")
      end
      CheckOutcome.new(pass: false, evidence: compact_evidence("no relayed tick log sender=#{opts.sender} leaf_uid=#{opts.leaf_uid} entries=#{entries.length}"))
    end

    def match_relayed_event(events, opts)
      event_name = opts.event_name.nil? ? Holons::Observability::EVENT_INSTANCE_READY : Holons::Observability.canonical_event_name(opts.event_name)
      events.each do |event|
        record = event.record
        next unless record.event_name == event_name
        next unless Holons::Observability.record_instance_uid(record) == opts.leaf_uid.to_s

        chain_error = compare_chain(record.chain, opts.expected_chain)
        return CheckOutcome.new(pass: false, evidence: compact_evidence("matching event bad chain: #{chain_error}")) unless chain_error.empty?

        return CheckOutcome.new(pass: true, evidence: "ok")
      end
      CheckOutcome.new(pass: false, evidence: compact_evidence("no relayed event leaf_uid=#{opts.leaf_uid} events=#{events.length}"))
    end

    def compare_chain(got, want)
      got = Array(got).map { |hop| normalize_hop(hop) }
      want = Array(want).map { |hop| normalize_hop(hop) }
      return "chain length #{got.length} want #{want.length}" unless got.length == want.length

      want.each_with_index do |expected, idx|
        actual = got[idx]
        if actual != expected
          return "hop #{idx}=#{actual} want #{expected}"
        end
      end
      ""
    end

    def normalize_hop(hop)
      return hop.to_s if hop.is_a?(String) || hop.is_a?(Symbol)
      return hop.slug.to_s if hop.respond_to?(:slug)

      data = symbolize_hash(hop)
      (data[:slug] || data[:name]).to_s
    end

    def poll_until(timeout, interval)
      deadline = monotonic_now + timeout.to_f
      last = CheckOutcome.new(pass: false, evidence: "")
      loop do
        last = yield
        return last if last.pass || monotonic_now > deadline

        sleep interval.to_f
      end
    rescue StandardError => e
      CheckOutcome.new(pass: false, evidence: compact_evidence(e.message))
    end

    def apply_dial_options(options)
      out = {}
      options.each do |option|
        if option.respond_to?(:call)
          option.call(out)
        elsif option.is_a?(Hash)
          out.merge!(symbolize_hash(option))
        end
      end
      out
    end

    def symbolize_hash(value)
      return {} if value.nil?
      return value.to_h.transform_keys(&:to_sym) if value.respond_to?(:to_h)

      {}
    end

    def clean_socket_token(value)
      token = value.to_s.strip.gsub(/[^a-zA-Z0-9_.-]+/, "-")
      token = token[0, 24] if token.length > 24
      token.empty? ? SecureRandom.hex(6) : token
    end

    def compact_evidence(value)
      text = value.to_s.split.join(" ")
      return "<empty>" if text.empty?
      return text if text.length <= 240

      "#{text[0, 240]}..."
    end

    def monotonic_now
      Process.clock_gettime(Process::CLOCK_MONOTONIC)
    end

    class << self
      alias_method :WithTransitiveObservability, :with_transitive_observability
      alias_method :SpawnMember, :spawn_member
      alias_method :BuildCascade, :build_cascade
      alias_method :Dial, :dial
      alias_method :CheckRelayedLog, :check_relayed_log
      alias_method :CheckRelayedEvent, :check_relayed_event
    end
  end
end
