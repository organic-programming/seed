# frozen_string_literal: true

require "fileutils"
require "socket"
require "thread"
require "tmpdir"

require_relative "describe"
require_relative "transport"

module Holons
  module Serve
    SIGNALS = %w[INT TERM].freeze
    DEFAULT_SIGNAL_WAIT_INTERVAL = 0.25
    ParsedFlags = Struct.new(:listen_uri, :reflect, keyword_init: true)

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

      def run_with_options(listen_uri, register, reflect = false, on_listen: nil)
        ensure_grpc_runtime!
        raise ArgumentError, "register callback is required" unless register.respond_to?(:call)

        parsed = Transport.parse_uri(listen_uri)
        server = GRPC::RpcServer.new
        register.call(server)
        auto_register_describe(server)
        reflection_enabled = maybe_register_reflection(server, reflect)

        actual_uri, cleanup, stdio_bridge = prepare_runtime(parsed, server)
        mode = reflection_mode(reflection_enabled)

        begin
          with_signal_handlers(server) do
            run_server(server) do
              stdio_bridge&.connect_to_server
              publish_listen_uri(actual_uri, on_listen) unless parsed.scheme == "stdio"
              on_listen&.call(actual_uri) if parsed.scheme == "stdio"
              warn("gRPC server listening on #{actual_uri} (#{mode})")
            end
          end
        ensure
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
