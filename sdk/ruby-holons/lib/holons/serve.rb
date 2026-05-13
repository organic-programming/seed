# frozen_string_literal: true

require "fileutils"
require "socket"
require "thread"
require "tmpdir"

require_relative "describe"
require_relative "observability"
require_relative "transport"

module Holons
  module Serve
    SIGNALS = %w[INT TERM].freeze
    DEFAULT_SIGNAL_WAIT_INTERVAL = 0.25
    ParsedFlags = Struct.new(:listen_uri, :reflect, keyword_init: true)
    MemberRef = Struct.new(:slug, :address, :uid, keyword_init: true)
    ServeOptions = Struct.new(:reflect, :member_endpoints, :slug, keyword_init: true) do
      def initialize(**opts)
        super
        self.reflect = false if reflect.nil?
        self.member_endpoints ||= []
        self.slug ||= ""
      end
    end

    class << self
      # Parse --listen or --port from args.
      def parse_flags(args)
        parse_options(args).listen_uri
      end

      def parse_options(args)
        listen_uri = Transport::DEFAULT_URI
        reflect = false

        args.each_with_index do |arg, i|
          listen_uri = args[i + 1] if arg == "--listen" && i + 1 < args.length
          listen_uri = "tcp://:#{args[i + 1]}" if arg == "--port" && i + 1 < args.length
          reflect = true if arg == "--reflect"
        end

        ParsedFlags.new(listen_uri: listen_uri, reflect: reflect)
      end

      def run(listen_uri, register)
        run_with_options(listen_uri, register, false)
      end

      def run_with_options(listen_uri, register, reflect = false, on_listen: nil, env: ENV)
        run_with_serve_options(
          listen_uri,
          register,
          ServeOptions.new(reflect: reflect),
          on_listen: on_listen,
          env: env
        )
      end

      def run_with_serve_options(listen_uri, register, options = ServeOptions.new, on_listen: nil, env: ENV)
        ensure_grpc_runtime!
        raise ArgumentError, "register callback is required" unless register.respond_to?(:call)

        options ||= ServeOptions.new
        parsed = Transport.parse_uri(listen_uri)
        server = GRPC::RpcServer.new
        register.call(server)
        auto_register_describe(server)
        observability = auto_register_observability(server, env, options)
        reflection_enabled = maybe_register_reflection(server, options.reflect)

        actual_uri, cleanup, stdio_bridge = prepare_runtime(parsed, server)
        mode = reflection_mode(reflection_enabled)
        prom_server = start_prom_server(observability)
        metrics_addr = prom_server&.start.to_s
        start_observability_runtime(observability, actual_uri, parsed.scheme, metrics_addr)
        started_relays = []

        begin
          with_signal_handlers(server) do
            run_server(server) do
              stdio_bridge&.connect_to_server
              publish_listen_uri(actual_uri, on_listen) unless parsed.scheme == "stdio"
              on_listen&.call(actual_uri) if parsed.scheme == "stdio"
              warn("gRPC server listening on #{actual_uri} (#{mode})")
              started_relays = start_member_relays(observability, options.member_endpoints)
            end
          end
        ensure
          started_relays.each do |relay, channel|
            relay.stop
            Holons::DiscoverySupport.close_channel(channel) if defined?(Holons::DiscoverySupport)
          end
          prom_server&.close
          stdio_bridge&.close
          cleanup&.call
        end
      end

      private

      def ensure_grpc_runtime!
        require "grpc"
      rescue LoadError => e
        raise e
      end

      def auto_register_describe(server)
        Describe.register(server)
      rescue Describe::ErrNoIncodeDescription
        raise
      rescue StandardError => e
        warn("HolonMeta registration failed: #{e.message}")
        raise
      end

      def auto_register_observability(server, env, options = ServeOptions.new)
        Observability.check_env(env)
        raw = (env["OP_OBS"] || "").strip
        return nil if raw.empty?

        inst = Observability.from_env(Observability::Config.new(slug: options.slug.to_s), env: env)
        return nil if inst.families.empty?

        Observability.register_grpc_service(server, inst)
        inst
      end

      def start_observability_runtime(inst, actual_uri, transport, metrics_addr = "")
        return if inst.nil?

        run_dir = inst.cfg.run_dir.to_s
        Observability.enable_disk_writers(run_dir) unless run_dir.empty?
        inst.emit(Observability::EVENT_TYPES[:instance_ready], "listener" => actual_uri) if inst.enabled?(:events)
        return if run_dir.empty?

        Observability.write_meta_json(
          run_dir,
          Observability::MetaJson.new(
            slug: inst.cfg.slug,
            uid: inst.cfg.instance_uid,
            pid: Process.pid,
            started_at: Time.now,
            transport: transport,
            address: actual_uri,
            metrics_addr: metrics_addr,
            log_path: inst.enabled?(:logs) ? File.join(run_dir, "stdout.log") : "",
            organism_uid: inst.cfg.organism_uid,
            organism_slug: inst.cfg.organism_slug
          )
        )
      rescue StandardError
        nil
      end

      def start_prom_server(inst)
        return nil if inst.nil? || !inst.enabled?(:prom)

        server = Observability::PromServer.new(inst.cfg.prom_addr.to_s.empty? ? ":0" : inst.cfg.prom_addr)
        server.start
        server
      rescue StandardError => e
        warn("warning: prom HTTP bind failed: #{e.message}")
        nil
      end

      def start_member_relays(inst, members)
        return [] if inst.nil? || (!inst.enabled?(:logs) && !inst.enabled?(:events))

        Observability.require_grpc_observability_support!
        Array(members).each_with_object([]) do |raw, started|
          member = normalize_member_ref(raw)
          if member.slug.to_s.empty? || member.address.to_s.empty?
            warn(%(warning: observability relay skipped incomplete member ref: slug="#{member.slug}" uid="#{member.uid}" address="#{member.address}"))
            next
          end

          channel = nil
          begin
            target = normalize_relay_dial_target(member.address)
            channel = Holons::DiscoverySupport.dial_ready(target, 5000)
            member = resolve_relay_member_identity(channel, member)
            if member.uid.to_s.empty?
              warn("warning: observability relay uid unresolved for #{member.slug} at #{member.address}; chain hops will have empty uid")
            end
            stub = Holons::V1::HolonObservability::Stub.new(
              "unused",
              :this_channel_is_insecure,
              channel_override: channel
            )
            relay = Observability::MemberRelay.new(
              child_slug: member.slug,
              child_uid: member.uid,
              stub: stub,
              observability: inst
            )
            relay.start
            started << [relay, channel]
            channel = nil
          rescue StandardError => e
            Holons::DiscoverySupport.close_channel(channel) unless channel.nil?
            warn("warning: observability relay start #{member.slug}/#{member.uid}: #{e.message}")
          end
        end
      end

      def normalize_member_ref(raw)
        if raw.respond_to?(:slug) && raw.respond_to?(:address)
          return MemberRef.new(
            slug: raw.slug.to_s.strip,
            address: raw.address.to_s.strip,
            uid: raw.respond_to?(:uid) ? raw.uid.to_s.strip : ""
          )
        end

        MemberRef.new(slug: "", address: "", uid: "")
      end

      def resolve_relay_member_identity(channel, member)
        return member unless member.uid.to_s.empty?

        stub = Holons::V1::HolonObservability::Stub.new(
          "unused",
          :this_channel_is_insecure,
          channel_override: channel,
          timeout: 2
        )
        begin
          request = Holons::V1::EventsRequest.new(types: [:INSTANCE_READY], follow: false)
          stub.events(request).each do |event|
            next if event.instance_uid.to_s.empty? || !event.chain.empty?

            return MemberRef.new(
              slug: event.slug.to_s.empty? ? member.slug : event.slug.to_s,
              uid: event.instance_uid.to_s,
              address: member.address
            )
          end
        rescue StandardError
          nil
        end

        begin
          snapshot = stub.metrics(Holons::V1::MetricsRequest.new)
          return member if snapshot.instance_uid.to_s.empty?

          MemberRef.new(
            slug: snapshot.slug.to_s.empty? ? member.slug : snapshot.slug.to_s,
            uid: snapshot.instance_uid.to_s,
            address: member.address
          )
        rescue StandardError
          member
        end
      end

      def normalize_relay_dial_target(address)
        Holons::DiscoverySupport.normalize_dial_target(address)
      rescue StandardError
        address.to_s
      end


      def prepare_runtime(parsed, server)
        case parsed.scheme
        when "tcp"
          actual_uri = bind_tcp(server, parsed)
          [actual_uri, nil, nil]
        when "unix"
          cleanup, actual_uri = bind_unix(server, parsed.path)
          [actual_uri, cleanup, nil]
        when "stdio"
          bridge = StdioServeBridge.new(server: server)
          socket_path = bridge.start
          bound = server.add_http2_port("unix:#{socket_path}", :this_port_is_insecure)
          raise "failed to bind stdio bridge on #{socket_path}" if bound.to_i <= 0

          ["stdio://", nil, bridge]
        else
          raise ArgumentError, "unsupported transport URI: #{parsed.raw}"
        end
      end

      def bind_tcp(server, parsed)
        bind_host = parsed.host.to_s.strip
        bind_host = "0.0.0.0" if bind_host.empty?
        bind_port = parsed.port || 9090

        actual_port = server.add_http2_port("#{bind_host}:#{bind_port}", :this_port_is_insecure)
        raise "failed to bind tcp://#{bind_host}:#{bind_port}" if actual_port.to_i <= 0

        "tcp://#{normalize_host(bind_host)}:#{actual_port}"
      end

      def bind_unix(server, path)
        raise ArgumentError, "unix socket path is required" if path.to_s.strip.empty?

        FileUtils.mkdir_p(File.dirname(path))
        File.delete(path) if File.exist?(path)

        bound = server.add_http2_port("unix:#{path}", :this_port_is_insecure)
        raise "failed to bind unix://#{path}" if bound.to_i <= 0

        [proc { File.delete(path) if File.exist?(path) }, "unix://#{path}"]
      end

      def normalize_host(host)
        case host
        when "", "0.0.0.0", "::", "[::]"
          "127.0.0.1"
        else
          host
        end
      end

      def reflection_mode(reflect)
        return "reflection OFF" unless reflect

        "reflection ON"
      end

      def maybe_register_reflection(server, reflect)
        return false unless reflect

        require_relative "../grpc_reflection"
        server.handle(GrpcReflection::Server)
        server.handle(GrpcReflection::ServerAlpha)
        true
      end

      def publish_listen_uri(actual_uri, on_listen)
        if on_listen
          on_listen.call(actual_uri)
          return
        end

        puts(actual_uri)
        $stdout.flush if $stdout.respond_to?(:flush)
      end

      def run_server(server)
        server_error = nil
        server_thread = Thread.new do
          begin
            server.run_till_terminated
          rescue StandardError => e
            server_error = e
          end
        end

        begin
          raise "gRPC server failed to start" unless server.wait_till_running(5)
          yield if block_given?
          server_thread.join
        ensure
          stop_server(server)
          server_thread.join(0.2) if server_thread&.alive?
        end

        raise server_error unless server_error.nil?
      end

      def with_signal_handlers(server)
        previous = {}
        SIGNALS.each do |signal_name|
          previous[signal_name] = trap(signal_name) { stop_server(server) }
        rescue ArgumentError
          nil
        end
        yield
      ensure
        previous.each do |signal_name, handler|
          trap(signal_name, handler)
        rescue ArgumentError
          nil
        end
      end

      def stop_server(server)
        return unless server.respond_to?(:running?)
        return unless server.running?

        server.stop
      rescue StandardError
        nil
      end
    end

    class StdioServeBridge
      def initialize(server:, stdin: $stdin, stdout: $stdout, stderr: $stderr)
        @server = server
        @stdin = stdin.dup
        @stdout = stdout.dup
        @stderr = stderr
        @socket_dir = nil
        @socket_path = nil
        @socket = nil
        @stdin_thread = nil
        @stdout_thread = nil
        @closed = false
        @lock = Mutex.new

        @stdin.binmode if @stdin.respond_to?(:binmode)
        @stdout.binmode if @stdout.respond_to?(:binmode)
      end

      def start
        @socket_dir = Dir.mktmpdir("holons-stdio-serve-")
        @socket_path = File.join(@socket_dir, "bridge.sock")
      end

      def connect_to_server(timeout: 5.0)
        redirect_stdout_to_stderr

        deadline = monotonic_now + timeout
        begin
          @socket = UNIXSocket.new(@socket_path)
        rescue Errno::ENOENT, Errno::ECONNREFUSED
          raise "failed to connect to stdio bridge #{@socket_path}" if monotonic_now >= deadline

          sleep 0.05
          retry
        end

        @socket.binmode
        @stdin_thread = Thread.new { stdin_to_socket_loop }
        @stdout_thread = Thread.new { socket_to_stdout_loop }
      end

      def close
        socket = nil
        stdin = nil
        stdout = nil
        socket_path = nil
        socket_dir = nil

        @lock.synchronize do
          return if @closed

          @closed = true
          socket = @socket
          stdin = @stdin
          stdout = @stdout
          socket_path = @socket_path
          socket_dir = @socket_dir
          @socket = nil
          @stdin = nil
          @stdout = nil
          @socket_path = nil
          @socket_dir = nil
        end

        [socket, stdin, stdout].compact.each do |io|
          close_io(io)
        end

        [@stdin_thread, @stdout_thread].compact.each do |thread|
          thread.join(0.2)
        rescue StandardError
          nil
        end

        File.delete(socket_path) if socket_path && File.exist?(socket_path)
        Dir.rmdir(socket_dir) if socket_dir && Dir.exist?(socket_dir)
      rescue SystemCallError
        nil
      end

      private

      def redirect_stdout_to_stderr
        return if @stdout_redirected

        $stdout.flush if $stdout.respond_to?(:flush)
        @stderr.flush if @stderr.respond_to?(:flush)
        STDOUT.reopen(@stderr)
        STDOUT.sync = true
        @stdout_redirected = true
      rescue StandardError
        nil
      end

      def stdin_to_socket_loop
        loop do
          chunk = @stdin.readpartial(16 * 1024)
          @socket.write(chunk)
          @socket.flush if @socket.respond_to?(:flush)
        end
      rescue EOFError, IOError, SystemCallError
        nil
      ensure
        begin
          @socket.close_write unless @socket.nil? || @socket.closed?
        rescue IOError, SystemCallError
          nil
        end
        stop_server_once
      end

      def socket_to_stdout_loop
        loop do
          chunk = @socket.readpartial(16 * 1024)
          @stdout.write(chunk)
          @stdout.flush if @stdout.respond_to?(:flush)
        end
      rescue EOFError, IOError, SystemCallError
        nil
      ensure
        stop_server_once
      end

      def stop_server_once
        @lock.synchronize do
          return if @server_stopped

          @server_stopped = true
          @server.stop
        end
      rescue StandardError
        nil
      end

      def close_io(io)
        return if io.closed?

        io.close
      rescue IOError, SystemCallError
        nil
      end

      def monotonic_now
        Process.clock_gettime(Process::CLOCK_MONOTONIC)
      end
    end
  end
end
