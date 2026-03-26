# frozen_string_literal: true

require "socket"
require "thread"
require "time"
require "pathname"
require "fileutils"

module Holons
  begin
    require "grpc"
    @grpc_load_error = nil
  rescue LoadError => e
    @grpc_load_error = e
  end

  ConnectOptions = Struct.new(
    :timeout,
    :transport,
    :start,
    :port_file,
    keyword_init: true
  )

  ConnectHandle = Struct.new(:pid, :wait_thread, :proxy, :ephemeral, keyword_init: true)

  class StdioDialProxy
    attr_reader :pid, :target, :wait_thread

    def initialize(binary_path)
      @binary_path = binary_path
      @mutex = Mutex.new
      @pending_stdout = []
      @closed = false
      @pid = nil
      @wait_thread = nil
      @target = nil
      @listener = nil
      @socket = nil
      @child_stdin = nil
      @child_stdout = nil
      @child_stderr = nil
      @accept_thread = nil
      @stdout_thread = nil
      @stderr_thread = nil
      @stdin_thread = nil
    end

    def start
      return target unless target.nil?

      stdin_read, stdin_write = IO.pipe
      stdout_read, stdout_write = IO.pipe
      stderr_read, stderr_write = IO.pipe

      [stdin_read, stdin_write, stdout_read, stdout_write, stderr_read, stderr_write].each(&:binmode)

      @pid = Process.spawn(
        @binary_path,
        "serve",
        "--listen",
        "stdio://",
        in: stdin_read,
        out: stdout_write,
        err: stderr_write
      )
      @wait_thread = Process.detach(@pid)

      stdin_read.close
      stdout_write.close
      stderr_write.close

      @child_stdin = stdin_write
      @child_stdout = stdout_read
      @child_stderr = stderr_read

      @listener = TCPServer.new("127.0.0.1", 0)
      @target = "127.0.0.1:#{@listener.local_address.ip_port}"

      @accept_thread = Thread.new { accept_loop }
      @stdout_thread = Thread.new(@child_stdout) { |io| stdout_loop(io) }
      @stderr_thread = Thread.new(@child_stderr) { |io| drain_stderr(io) }

      target
    rescue StandardError
      close
      raise
    end

    def close
      listener = nil
      socket = nil
      child_stdin = nil
      child_stdout = nil
      child_stderr = nil

      @mutex.synchronize do
        return if @closed

        @closed = true
        listener = @listener
        socket = @socket
        child_stdin = @child_stdin
        child_stdout = @child_stdout
        child_stderr = @child_stderr
        @listener = nil
        @socket = nil
        @child_stdin = nil
        @child_stdout = nil
        @child_stderr = nil
      end

      [listener, socket, child_stdin, child_stdout, child_stderr].each do |io|
        next if io.nil?

        begin
          io.close unless io.closed?
        rescue IOError, SystemCallError
          nil
        end
      end

      [@accept_thread, @stdout_thread, @stderr_thread, @stdin_thread].each do |thread|
        next if thread.nil?

        thread.join(0.2)
      rescue StandardError
        nil
      end
    end

    private

    def closed?
      @mutex.synchronize { @closed }
    end

    def accept_loop
      loop do
        break if closed?

        socket = @listener.accept
        socket.binmode

        replace_socket(socket)
        flush_pending_stdout(socket)

        @stdin_thread = Thread.new { socket_to_stdin_loop(socket) }
      end
    rescue IOError, Errno::EBADF, Errno::EINVAL, SystemCallError
      nil
    end

    def replace_socket(socket)
      previous = nil

      @mutex.synchronize do
        previous = @socket
        @socket = socket
      end

      begin
        previous.close unless previous.nil? || previous.closed?
      rescue IOError, SystemCallError
        nil
      end
    end

    def flush_pending_stdout(socket)
      pending = nil

      @mutex.synchronize do
        pending = @pending_stdout
        @pending_stdout = []
      end

      pending.each do |chunk|
        socket.write(chunk)
      rescue IOError, SystemCallError
        break
      end
    end

    def stdout_loop(io)
      return if io.nil?

      loop do
        chunk = io.readpartial(16 * 1024)
        socket = @mutex.synchronize { @socket }
        if socket.nil? || socket.closed?
          @mutex.synchronize { @pending_stdout << chunk.dup }
          next
        end

        socket.write(chunk)
      end
    rescue EOFError, IOError, Errno::EPIPE, SystemCallError
      nil
    end

    def socket_to_stdin_loop(socket)
      loop do
        chunk = socket.readpartial(16 * 1024)
        child_stdin = @mutex.synchronize { @child_stdin }
        break if child_stdin.nil? || child_stdin.closed?

        child_stdin.write(chunk)
        child_stdin.flush
      end
    rescue EOFError, IOError, Errno::EPIPE, SystemCallError
      nil
    ensure
      @mutex.synchronize do
        @socket = nil if @socket.equal?(socket)
      end
      begin
        socket.close unless socket.closed?
      rescue IOError, SystemCallError
        nil
      end
    end

    def drain_stderr(io)
      return if io.nil?

      loop do
        io.readpartial(16 * 1024)
      end
    rescue EOFError, IOError, SystemCallError
      nil
    end
  end

  class << self
    DEFAULT_CONNECT_TIMEOUT = 5
    DEFAULT_CONNECT_TRANSPORT = "stdio"

    def connect(target, opts = nil)
      ensure_grpc!

      trimmed = target.to_s.strip
      raise ArgumentError, "target is required" if trimmed.empty?

      options = normalize_connect_options(opts)
      ephemeral = opts.nil? || options.transport == "stdio"

      return dial_ready(normalize_dial_target(trimmed), options.timeout) if direct_target?(trimmed)

      entry = discover_by_slug(trimmed)
      raise "holon \"#{trimmed}\" not found" if entry.nil?

      port_file = options.port_file.empty? ? default_port_file_path(entry.slug) : options.port_file
      reusable_channel = usable_port_file(port_file, options.timeout)
      return reusable_channel unless reusable_channel.nil?

      raise "holon \"#{trimmed}\" is not running" unless options.start

      binary_path = resolve_binary_path(entry)

      if options.transport == "stdio"
        channel, handle = dial_stdio(binary_path, options.timeout)
        remember_channel(channel, handle)
        return channel
      end

      advertised_uri, handle =
        if options.transport == "unix"
          start_unix_holon(binary_path, entry.slug, port_file, options.timeout)
        else
          start_tcp_holon(binary_path, options.timeout)
        end
      channel = nil

      begin
        channel = dial_ready(normalize_dial_target(advertised_uri), options.timeout)
        write_port_file(port_file, advertised_uri) unless ephemeral
      rescue StandardError
        close_channel(channel)
        stop_process(handle.pid, handle.wait_thread)
        raise
      end

      handle.ephemeral = ephemeral
      remember_channel(channel, handle)
      channel
    end

    def disconnect(channel)
      return if channel.nil?

      handle = started_channels_mutex.synchronize { started_channels.delete(channel) }

      close_error = nil
      cleanup_error = nil

      begin
        close_channel(channel)
      rescue StandardError => e
        close_error = e
      end

      begin
        handle&.proxy&.close
        stop_process(handle.pid, handle.wait_thread) if handle&.ephemeral
      rescue StandardError => e
        cleanup_error = e
      end

      raise close_error unless close_error.nil?
      raise cleanup_error unless cleanup_error.nil?

      nil
    end

    def grpc_available?
      @grpc_load_error.nil?
    end

    private

    def normalize_connect_options(opts)
      options =
        case opts
        when nil
          ConnectOptions.new
        when ConnectOptions
          opts
        when Hash
          ConnectOptions.new(**symbolize_keys(opts))
        else
          raise ArgumentError, "opts must be a Hash or Holons::ConnectOptions"
        end

      timeout = options.timeout.to_f
      timeout = DEFAULT_CONNECT_TIMEOUT if timeout <= 0

      transport = options.transport.to_s.strip.downcase
      transport = DEFAULT_CONNECT_TRANSPORT if transport.empty?
      raise ArgumentError, "unsupported transport #{options.transport.inspect}" unless %w[stdio tcp unix].include?(transport)

      ConnectOptions.new(
        timeout: timeout,
        transport: transport,
        start: options.start.nil? ? true : !!options.start,
        port_file: options.port_file.to_s.strip
      )
    end

    def dial_ready(target, timeout)
      channel = GRPC::Core::Channel.new(target, {}, :this_channel_is_insecure)
      wait_for_ready(channel, timeout)
      channel
    rescue StandardError
      close_channel(channel)
      raise
    end

    def wait_for_ready(channel, timeout)
      return if channel.nil?
      return unless channel.respond_to?(:connectivity_state) && channel.respond_to?(:watch_connectivity_state)

      deadline = monotonic_now + timeout
      state = channel.connectivity_state(true)

      until connectivity_ready?(state)
        raise "gRPC connection shut down before becoming ready" if connectivity_shutdown?(state)

        remaining = deadline - monotonic_now
        raise Timeout::Error, "timed out waiting for gRPC readiness" if remaining <= 0

        changed = channel.watch_connectivity_state(state, Time.now + remaining)
        raise Timeout::Error, "timed out waiting for gRPC readiness" unless changed

        state = channel.connectivity_state(false)
      end
    rescue NoMethodError, ArgumentError
      nil
    end

    def connectivity_ready?(state)
      state == 2 || state.to_s.downcase == "ready"
    end

    def connectivity_shutdown?(state)
      state == 4 || state.to_s.downcase == "shutdown"
    end

    def dial_stdio(binary_path, timeout)
      proxy = StdioDialProxy.new(binary_path)
      proxy.start

      channel = nil

      begin
        channel = dial_ready(proxy.target, timeout)
      rescue StandardError
        close_channel(channel)
        proxy.close
        stop_process(proxy.pid, proxy.wait_thread)
        raise
      end

      [channel, ConnectHandle.new(pid: proxy.pid, wait_thread: proxy.wait_thread, proxy: proxy, ephemeral: true)]
    end

    def usable_port_file(path, timeout)
      data = File.read(path)
      target = data.to_s.strip
      if target.empty?
        File.delete(path)
        return nil
      end

      dial_ready(normalize_dial_target(target), port_check_timeout(timeout))
    rescue Errno::ENOENT, Errno::EACCES
      nil
    rescue StandardError
      begin
        File.delete(path)
      rescue Errno::ENOENT, Errno::EACCES
        nil
      end
      nil
    end

    def port_check_timeout(timeout)
      [[timeout.to_f / 4.0, 0.25].max, 1.0].min
    end

    def start_tcp_holon(binary_path, timeout)
      pid = nil
      wait_thread = nil
      stdout_read = nil
      stderr_read = nil
      readers = nil

      stdout_read, stdout_write = IO.pipe
      stderr_read, stderr_write = IO.pipe

      stdout_read.binmode
      stdout_write.binmode
      stderr_read.binmode
      stderr_write.binmode

      pid = Process.spawn(
        binary_path,
        "serve",
        "--listen",
        "tcp://127.0.0.1:0",
        in: File::NULL,
        out: stdout_write,
        err: stderr_write
      )
      wait_thread = Process.detach(pid)

      stdout_write.close
      stderr_write.close

      queue = Queue.new
      stderr_lines = []

      readers = [
        Thread.new { read_startup_stream(stdout_read, queue) },
        Thread.new { read_startup_stream(stderr_read, queue, stderr_lines) }
      ]

      uri = nil
      deadline = monotonic_now + timeout

      until monotonic_now > deadline
        uri = poll_startup_uri(queue)
        break unless uri.nil?

        unless pid_alive?(pid)
          details = stderr_lines.join.strip
          raise "holon exited before advertising an address#{details.empty? ? "" : ": #{details}"}"
        end

        sleep(0.05)
      end

      raise "timed out waiting for holon startup" if uri.nil?

      [uri, ConnectHandle.new(pid: pid, wait_thread: wait_thread, ephemeral: false)]
    rescue StandardError
      stop_process(pid, wait_thread)
      raise
    ensure
      [stdout_read, stderr_read].compact.each do |io|
        begin
          io.close unless io.closed?
        rescue IOError, SystemCallError
          nil
        end
      end
      readers&.each do |thread|
        thread.join(0.1)
      rescue StandardError
        nil
      end
    end

    def start_unix_holon(binary_path, slug, port_file, timeout)
      socket_uri = default_unix_socket_uri(slug, port_file)
      socket_path = socket_uri.delete_prefix("unix://")
      stderr_read, stderr_write = IO.pipe
      stderr_read.binmode
      stderr_write.binmode

      pid = Process.spawn(
        binary_path,
        "serve",
        "--listen",
        socket_uri,
        in: File::NULL,
        out: File::NULL,
        err: stderr_write
      )
      wait_thread = Process.detach(pid)
      stderr_write.close

      stderr_lines = []
      reader = Thread.new { read_startup_stream(stderr_read, Queue.new, stderr_lines) }
      deadline = monotonic_now + timeout

      until monotonic_now > deadline
        return [socket_uri, ConnectHandle.new(pid: pid, wait_thread: wait_thread, ephemeral: false)] if File.exist?(socket_path)

        unless pid_alive?(pid)
          details = stderr_lines.join.strip
          raise "holon exited before binding unix socket#{details.empty? ? "" : ": #{details}"}"
        end

        sleep(0.02)
      end

      raise "timed out waiting for unix holon startup#{stderr_lines.empty? ? "" : ": #{stderr_lines.join.strip}"}"
    rescue StandardError
      stop_process(pid, wait_thread)
      raise
    ensure
      begin
        stderr_read.close unless stderr_read.nil? || stderr_read.closed?
      rescue IOError, SystemCallError
        nil
      end
      reader&.join(0.1)
    end

    def read_startup_stream(io, queue, collector = nil)
      io.each_line do |line|
        collector << line if collector
        queue << line
      end
    rescue IOError, SystemCallError
      nil
    end

    def poll_startup_uri(queue)
      loop do
        line = queue.pop(true)
        uri = first_uri(line)
        return uri unless uri.nil?
      end
    rescue ThreadError
      nil
    end

    def resolve_binary_path(entry)
      manifest = entry.manifest
      raise "holon \"#{entry.slug}\" has no manifest" if manifest.nil?

      binary_name = manifest.artifacts.binary.to_s.strip
      raise "holon \"#{entry.slug}\" has no artifacts.binary" if binary_name.empty?

      if Pathname.new(binary_name).absolute?
        return binary_name if File.file?(binary_name)

        raise "built binary not found for holon \"#{entry.slug}\""
      end

      candidate = File.join(entry.dir, ".op", "build", "bin", File.basename(binary_name))
      return candidate if File.file?(candidate)

      looked_up = lookup_executable(File.basename(binary_name))
      return looked_up unless looked_up.nil?

      raise "built binary not found for holon \"#{entry.slug}\""
    end

    def lookup_executable(name)
      ENV.fetch("PATH", "").split(File::PATH_SEPARATOR).each do |dir|
        candidate = File.join(dir, name)
        return candidate if File.file?(candidate) && File.executable?(candidate)
      end
      nil
    end

    def default_port_file_path(slug)
      File.join(Dir.pwd, ".op", "run", "#{slug}.port")
    end

    def default_unix_socket_uri(slug, port_file)
      label = socket_label(slug)
      hash = fnv1a64(port_file.to_s)
      format("unix:///tmp/holons-%<label>s-%<hash>012x.sock", label: label, hash: hash & 0xffffffffffff)
    end

    def socket_label(slug)
      label = +""
      last_dash = false

      slug.to_s.strip.downcase.each_char do |char|
        if char.match?(/[a-z0-9]/)
          label << char
          last_dash = false
        elsif (char == "-" || char == "_") && !label.empty? && !last_dash
          label << "-"
          last_dash = true
        end

        break if label.length >= 24
      end

      label = label.gsub(/\A-+|-+\z/, "")
      label.empty? ? "socket" : label
    end

    def fnv1a64(text)
      hash = 0xcbf29ce484222325
      text.to_s.b.each_byte do |byte|
        hash ^= byte
        hash = (hash * 0x100000001b3) & 0xffffffffffffffff
      end
      hash
    end

    def write_port_file(path, uri)
      FileUtils.mkdir_p(File.dirname(path))
      File.write(path, "#{uri.strip}\n")
    end

    def direct_target?(target)
      target.include?("://") || target.include?(":")
    end

    def normalize_dial_target(target)
      trimmed = target.to_s.strip
      return trimmed unless trimmed.include?("://")

      parsed = Transport.parse_uri(trimmed)
      case parsed.scheme
      when "tcp"
        host = parsed.host.to_s
        host = "127.0.0.1" if host.empty? || host == "0.0.0.0" || host == "::"
        return "#{host}:#{parsed.port}" unless parsed.port.nil?
      when "unix"
        return "unix://#{parsed.path}"
      end

      trimmed
    rescue StandardError
      trimmed
    end

    def first_uri(line)
      line.to_s.split.each do |field|
        trimmed = field.strip.gsub(/\A["'()\[\]{}.,]+|["'()\[\]{}.,]+\z/, "")
        return trimmed if trimmed.start_with?("tcp://", "unix://", "ws://", "wss://", "stdio://")
      end
      nil
    end

    def remember_channel(channel, handle)
      started_channels_mutex.synchronize { started_channels[channel] = handle }
    end

    def started_channels
      @started_channels ||= {}
    end

    def started_channels_mutex
      @started_channels_mutex ||= Mutex.new
    end

    def close_channel(channel)
      channel&.close if channel&.respond_to?(:close)
    end

    def stop_process(pid, wait_thread)
      return if pid.nil?

      unless pid_alive?(pid)
        wait_thread&.join(0.1)
        return
      end

      begin
        Process.kill("TERM", pid)
      rescue Errno::ESRCH
        wait_thread&.join(0.1)
        return
      end

      return unless wait_thread&.join(2).nil?

      begin
        Process.kill("KILL", pid)
      rescue Errno::ESRCH
        nil
      end
      wait_thread&.join(2)
    end

    def pid_alive?(pid)
      Process.kill(0, pid)
      true
    rescue Errno::EPERM
      true
    rescue Errno::ESRCH
      false
    end

    def monotonic_now
      Process.clock_gettime(Process::CLOCK_MONOTONIC)
    end

    def symbolize_keys(hash)
      hash.each_with_object({}) do |(key, value), memo|
        memo[key.to_sym] = value
      end
    end

    def ensure_grpc!
      return if grpc_available?

      raise @grpc_load_error
    end
  end
end
