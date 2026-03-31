# frozen_string_literal: true

require "fileutils"
require "socket"
require "timeout"
require "tmpdir"

module Holons
  ConnectHandle = Struct.new(:pid, :wait_thread, :proxy, :ephemeral, keyword_init: true)

  class << self
    def connect(scope, expression, root, specifiers, timeout)
      validate_scope!(scope)
      return ConnectResult.new(channel: nil, uid: nil, origin: nil, error: "expression is required") if expression.to_s.strip.empty?
      return ConnectResult.new(channel: nil, uid: nil, origin: nil, error: grpc_load_error_message) unless grpc_available?

      target = expression.to_s.strip
      if direct_connect_target?(target)
        channel = DiscoverySupport.dial_ready(DiscoverySupport.normalize_dial_target(target), timeout.to_i)
        return ConnectResult.new(
          channel: channel,
          uid: nil,
          origin: HolonRef.new(url: target, info: nil, error: nil),
          error: nil
        )
      end

      resolved = resolve(scope, target, root, specifiers, timeout)
      return ConnectResult.new(channel: nil, uid: nil, origin: resolved.ref, error: resolved.error) unless resolved.error.nil?
      return ConnectResult.new(channel: nil, uid: nil, origin: nil, error: %(holon "#{target}" not found)) if resolved.ref.nil?

      connect_resolved_ref(resolved.ref, timeout.to_i)
    rescue StandardError => e
      ConnectResult.new(channel: nil, uid: nil, origin: nil, error: e.message)
    end

    def disconnect(result)
      channel =
        case result
        when ConnectResult
          result.channel
        else
          result
        end
      return nil if channel.nil?

      handle = started_channels_mutex.synchronize { started_channels.delete(channel) }

      DiscoverySupport.close_channel(channel)
      handle&.proxy&.close
      stop_process(handle&.pid, handle&.wait_thread) if handle&.ephemeral
      nil
    end

    private

    def connect_resolved_ref(ref, timeout)
      if reachable_target?(ref.url)
        begin
          channel = DiscoverySupport.dial_ready(DiscoverySupport.normalize_dial_target(ref.url), timeout)
          remember_channel(channel, ConnectHandle.new(ephemeral: false))
          return ConnectResult.new(channel: channel, uid: nil, origin: ref, error: nil)
        rescue StandardError
          # Fall through to launch for local file:// targets.
          raise unless ref.url.to_s.downcase.start_with?("file://")
        end
      end

      connect_launched_ref(ref, timeout)
    rescue StandardError => e
      ConnectResult.new(channel: nil, uid: nil, origin: ref, error: e.message)
    end

    def connect_launched_ref(ref, timeout)
      command_path, args, working_directory = resolve_launch_target(ref)
      origin = HolonRef.new(url: ref.url, info: ref.info, error: ref.error)
      last_error = nil

      launch_listen_uris(ref).each do |listen_uri|
        if listen_uri == "stdio://"
          channel, handle = dial_stdio(command_path, args, working_directory, timeout)
          remember_channel(channel, handle)
          return ConnectResult.new(channel: channel, uid: nil, origin: origin, error: nil)
        end

        channel = nil
        started_uri = nil
        handle = nil
        begin
          started_uri, handle = start_advertised_holon(command_path, args, working_directory, listen_uri, timeout)
          channel = DiscoverySupport.dial_ready(DiscoverySupport.normalize_dial_target(started_uri), timeout)
          remember_channel(channel, handle)
          return ConnectResult.new(
            channel: channel,
            uid: nil,
            origin: HolonRef.new(url: started_uri, info: ref.info, error: nil),
            error: nil
          )
        rescue StandardError => e
          last_error = e
          DiscoverySupport.close_channel(channel)
          stop_process(handle&.pid, handle&.wait_thread)
        end
      end

      ConnectResult.new(channel: nil, uid: nil, origin: ref, error: (last_error || RuntimeError.new("target unreachable")).message)
    end

    def resolve_launch_target(ref)
      path = local_path_from_ref(ref)
      raise "target is not launchable" if path.nil? || path.empty?

      return [path, [], File.dirname(path)] if File.file?(path)
      raise "target is not launchable" unless File.directory?(path)

      if package_dir_path?(path)
        return resolve_package_launch_target(path, ref.info)
      end

      resolve_source_launch_target(path, ref.info)
    end

    def resolve_package_launch_target(dir, info)
      entrypoint = info&.entrypoint.to_s
      binary_name = File.basename(entrypoint)

      unless binary_name.empty?
        [
          File.join(dir, "bin", DiscoverySupport.package_arch_dir, binary_name),
          File.join(dir, "bin", binary_name),
          File.join(dir, binary_name)
        ].each do |candidate|
          return [candidate, [], dir] if File.file?(candidate)
        end
      end

      unless entrypoint.empty?
        dist_entrypoint = File.join(dir, "dist", entrypoint)
        if File.file?(dist_entrypoint)
          command_path, runner_args = launch_spec_for_runner(info&.runner.to_s, dist_entrypoint)
          return [command_path, runner_args, dir] unless command_path.nil?
        end
      end

      raise %(holon "#{info&.slug || File.basename(dir)}" package is not runnable)
    end

    def resolve_source_launch_target(dir, info)
      binary_name = File.basename(info&.entrypoint.to_s)
      raise %(holon "#{info&.slug || File.basename(dir)}" has no entrypoint) if binary_name.empty?

      [
        File.join(dir, ".op", "build", "#{info&.slug}.holon", "bin", DiscoverySupport.package_arch_dir, binary_name),
        File.join(dir, ".op", "build", "bin", binary_name),
        File.join(dir, binary_name)
      ].each do |candidate|
        return [candidate, [], dir] if File.file?(candidate)
      end

      raise %(built binary not found for holon "#{info&.slug || File.basename(dir)}")
    end

    def local_path_from_ref(ref)
      return nil unless ref.url.to_s.downcase.start_with?("file://")

      DiscoverySupport.path_from_file_url(ref.url)
    end

    def package_dir_path?(dir)
      File.basename(dir).downcase.end_with?(".holon") || File.file?(File.join(dir, ".holon.json"))
    end

    def launch_spec_for_runner(runner, entrypoint)
      case runner.to_s.strip.downcase
      when "go", "go-module"
        ["go", ["run", entrypoint]]
      when "python"
        ["python3", [entrypoint]]
      when "node", "typescript", "npm"
        ["node", [entrypoint]]
      when "ruby"
        ["ruby", [entrypoint]]
      when "dart"
        ["dart", ["run", entrypoint]]
      else
        [nil, nil]
      end
    end

    def launch_listen_uris(ref)
      uris = []
      transport = ref.info&.transport.to_s.strip.downcase

      case transport
      when "stdio"
        uris << "stdio://"
      when "unix"
        uris << default_unix_socket_uri(ref.info&.slug || "holon")
      when "tcp"
        uris << "tcp://127.0.0.1:0"
      end

      uris << "stdio://"
      uris << default_unix_socket_uri(ref.info&.slug || "holon") unless DiscoverySupport.go_os == "windows"
      uris << "tcp://127.0.0.1:0"
      uris.uniq
    end

    def dial_stdio(command_path, args, working_directory, timeout)
      proxy = DiscoverySupport::StdioDialProxy.new(command_path, args: args, chdir: working_directory)
      proxy.start

      channel = nil
      begin
        channel = DiscoverySupport.dial_ready(proxy.target, timeout)
        [channel, ConnectHandle.new(pid: proxy.pid, wait_thread: proxy.wait_thread, proxy: proxy, ephemeral: true)]
      rescue StandardError
        DiscoverySupport.close_channel(channel)
        proxy.close
        stop_process(proxy.pid, proxy.wait_thread)
        raise
      end
    end

    def start_advertised_holon(command_path, args, working_directory, listen_uri, timeout)
      stdout_read, stdout_write = IO.pipe
      stderr_read, stderr_write = IO.pipe
      stdout_read.binmode
      stdout_write.binmode
      stderr_read.binmode
      stderr_write.binmode

      pid = Process.spawn(
        command_path,
        *args,
        "serve",
        "--listen",
        listen_uri,
        chdir: working_directory,
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

      if listen_uri.start_with?("unix://")
        socket_path = listen_uri.delete_prefix("unix://")
        deadline = DiscoverySupport.monotonic_now + (timeout.to_f / 1000.0)
        until DiscoverySupport.monotonic_now > deadline
          return [listen_uri, ConnectHandle.new(pid: pid, wait_thread: wait_thread, ephemeral: true)] if File.exist?(socket_path)
          raise "holon exited before binding unix socket" unless pid_alive?(pid)

          sleep(0.02)
        end
        raise "timed out waiting for unix holon startup"
      end

      deadline = DiscoverySupport.monotonic_now + (timeout.to_f / 1000.0)
      until DiscoverySupport.monotonic_now > deadline
        uri = poll_startup_uri(queue)
        return [uri, ConnectHandle.new(pid: pid, wait_thread: wait_thread, ephemeral: true)] unless uri.nil?

        unless pid_alive?(pid)
          details = stderr_lines.join.strip
          raise "holon exited before advertising an address#{details.empty? ? "" : ": #{details}"}"
        end
        sleep(0.05)
      end

      raise "timed out waiting for holon startup"
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

    def first_uri(line)
      line.to_s.split.each do |field|
        trimmed = field.strip.gsub(/\A["'()\[\]{}.,]+|["'()\[\]{}.,]+\z/, "")
        return trimmed if trimmed.start_with?("tcp://", "unix://", "ws://", "wss://", "stdio://")
      end
      nil
    end

    def default_unix_socket_uri(slug)
      label = slug.to_s.strip.downcase.gsub(/[^a-z0-9_-]+/, "-").gsub(/\A-+|-+\z/, "")
      label = "holon" if label.empty?
      "unix://#{File.join(Dir.tmpdir, "holons-#{label[0, 24]}-#{Process.pid}-#{rand(1000)}.sock")}"
    end

    def direct_connect_target?(target)
      DiscoverySupport.direct_transport_expression?(target) || DiscoverySupport.host_port_expression?(target)
    end

    def reachable_target?(target)
      trimmed = target.to_s.strip
      return false if trimmed.empty? || trimmed.downcase.start_with?("file://")

      direct_connect_target?(trimmed)
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

    def stop_process(pid, wait_thread)
      return if pid.nil?
      return unless pid_alive?(pid)

      begin
        Process.kill("TERM", pid)
      rescue Errno::ESRCH
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

    def grpc_load_error_message
      grpc_load_error&.message.to_s.empty? ? "grpc gem is unavailable" : grpc_load_error.message
    end
  end
end
