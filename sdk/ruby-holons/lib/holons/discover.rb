# frozen_string_literal: true

require "pathname"
require "rbconfig"
require "timeout"
require "uri"
require_relative "identity"

module Holons
  DiscoveryEntry = Struct.new(
    :url,
    :info,
    :dir,
    :relative_path,
    keyword_init: true
  )

  module DiscoverySupport
    class DiscoverTimeoutError < Timeout::Error; end

    class StdioDialProxy
      attr_reader :pid, :target, :wait_thread

      def initialize(command_path, args: [], chdir: nil)
        @command_path = command_path
        @args = Array(args)
        @chdir = chdir
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

        spawn_args = [@command_path, *@args, "serve", "--listen", "stdio://"]
        @pid = Process.spawn(*spawn_args, in: stdin_read, out: stdout_write, err: stderr_write, chdir: @chdir)
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
      def with_timeout(timeout_ms, label)
        timeout_value = timeout_ms.to_i
        return yield if timeout_value <= 0

        Timeout.timeout(timeout_value.to_f / 1000.0, DiscoverTimeoutError) { yield }
      rescue DiscoverTimeoutError
        raise "#{label} timed out"
      end

      def file_url(path)
        URI::Generic.build(scheme: "file", path: File.expand_path(path).tr(File::SEPARATOR, "/")).to_s
      end

      def path_from_file_url(raw)
        parsed = URI.parse(raw.to_s.strip)
        raise "#{raw.inspect} is not a file URL" unless parsed.scheme == "file"

        path = parsed.path.to_s
        if !parsed.host.nil? && !parsed.host.empty? && parsed.host != "localhost"
          path = "//#{parsed.host}#{path}"
        end
        raise "#{raw.inspect} has no path" if path.empty?

        File.expand_path(path)
      rescue URI::InvalidURIError => e
        raise e.message
      end

      def package_arch_dir
        "#{go_os}_#{go_arch}"
      end

      def go_os
        host_os = RbConfig::CONFIG.fetch("host_os", "").downcase
        case host_os
        when /darwin/
          "darwin"
        when /linux/
          "linux"
        when /mswin|mingw|cygwin/
          "windows"
        else
          host_os.gsub(/[^a-z0-9]+/, "-").sub(/\A-+|-+\z/, "")
        end
      end

      def go_arch
        host_cpu = RbConfig::CONFIG.fetch("host_cpu", "").downcase
        case host_cpu
        when "x86_64", "amd64"
          "amd64"
        when "aarch64", "arm64"
          "arm64"
        when /i[3-6]86/
          "386"
        else
          host_cpu.gsub(/[^a-z0-9]+/, "-").sub(/\A-+|-+\z/, "")
        end
      end

      def direct_transport_expression?(expression)
        case transport_scheme(expression)
        when "tcp", "unix", "ws", "wss", "http", "https"
          true
        else
          false
        end
      end

      def host_port_expression?(expression)
        trimmed = expression.to_s.strip
        !trimmed.include?("://") && trimmed.include?(":")
      end

      def normalize_dial_target(target)
        trimmed = target.to_s.strip
        return trimmed unless trimmed.include?("://")

        parsed = Transport.parse_uri(trimmed)
        case parsed.scheme
        when "tcp"
          host = parsed.host.to_s
          host = "127.0.0.1" if host.empty? || host == "0.0.0.0" || host == "::"
          "#{host}:#{parsed.port}"
        when "unix"
          "unix://#{parsed.path}"
        else
          trimmed
        end
      rescue StandardError
        trimmed
      end

      def wait_for_ready(channel, timeout_ms)
        return if channel.nil?
        return unless channel.respond_to?(:connectivity_state) && channel.respond_to?(:watch_connectivity_state)

        deadline = timeout_ms.to_i > 0 ? monotonic_now + (timeout_ms.to_f / 1000.0) : nil
        state = channel.connectivity_state(true)

        until connectivity_ready?(state)
          raise "gRPC connection shut down before becoming ready" if connectivity_shutdown?(state)

          remaining =
            if timeout_ms.to_i <= 0
              1.0
            else
              deadline - monotonic_now
            end
          raise Timeout::Error, "timed out waiting for gRPC readiness" if timeout_ms.to_i > 0 && remaining <= 0

          changed = channel.watch_connectivity_state(state, Time.now + remaining)
          raise Timeout::Error, "timed out waiting for gRPC readiness" unless changed

          state = channel.connectivity_state(false)
        end
      rescue NoMethodError, ArgumentError
        nil
      end

      def close_channel(channel)
        channel&.close if channel&.respond_to?(:close)
      end

      def dial_ready(target, timeout_ms)
        raise Holons.grpc_load_error unless Holons.grpc_available?

        channel = GRPC::Core::Channel.new(target, {}, :this_channel_is_insecure)
        wait_for_ready(channel, timeout_ms)
        channel
      rescue StandardError
        close_channel(channel)
        raise
      end

      def probe_binary(binary_path, timeout: 5000)
        raise Holons.grpc_load_error unless Holons.grpc_available?

        load_describe_runtime!
        proxy = StdioDialProxy.new(binary_path)
        proxy.start

        channel = nil

        begin
          channel = dial_ready(proxy.target, timeout)
          response = Holons::V1::HolonMeta::Stub.new(
            "unused",
            :this_channel_is_insecure,
            channel_override: channel,
            timeout: [timeout.to_f / 1000.0, 0.25].max
          ).describe(Holons::V1::DescribeRequest.new)
          holon_info_from_describe(response)
        ensure
          close_channel(channel)
          proxy.close
          Holons.send(:stop_process, proxy.pid, proxy.wait_thread) if proxy.pid
        end
      end

      def load_describe_runtime!
        return if defined?(Holons::V1::DescribeRequest) && defined?(Holons::V1::HolonMeta::Stub)

        require "holons/v1/manifest_pb"
        require "holons/v1/describe_pb"
        require "holons/v1/describe_services_pb"
      end

      def holon_info_from_describe(response)
        raise "Describe returned no manifest" if response.nil? || response.manifest.nil?
        raise "Describe returned no identity" if response.manifest.identity.nil?

        manifest = response.manifest
        identity = manifest.identity

        HolonInfo.new(
          slug: slug_for(identity.given_name, identity.family_name),
          uuid: identity.uuid.to_s,
          identity: IdentityInfo.new(
            given_name: identity.given_name.to_s,
            family_name: identity.family_name.to_s,
            motto: identity.motto.to_s,
            aliases: Array(identity.aliases).map(&:to_s)
          ),
          lang: manifest.lang.to_s,
          runner: manifest.build&.runner.to_s,
          status: identity.status.to_s,
          kind: manifest.kind.to_s,
          transport: manifest.transport.to_s,
          entrypoint: manifest.artifacts&.binary.to_s,
          architectures: Array(manifest.platforms).map(&:to_s),
          has_dist: false,
          has_source: false
        )
      end

      def holon_info_from_manifest(resolved)
        identity = resolved&.identity
        HolonInfo.new(
          slug: identity&.slug.to_s,
          uuid: identity&.uuid.to_s,
          identity: IdentityInfo.new(
            given_name: identity&.given_name.to_s,
            family_name: identity&.family_name.to_s,
            motto: identity&.motto.to_s,
            aliases: Array(identity&.aliases).map(&:to_s)
          ),
          lang: identity&.lang.to_s,
          runner: resolved&.build_runner.to_s,
          status: identity&.status.to_s,
          kind: resolved&.kind.to_s,
          transport: "",
          entrypoint: resolved&.artifact_binary.to_s,
          architectures: [],
          has_dist: false,
          has_source: true
        )
      end

      def slug_for(given_name, family_name)
        HolonIdentity.new(given_name: given_name, family_name: family_name).slug
      end

      def transport_scheme(expression)
        URI.parse(expression.to_s.strip).scheme.to_s.downcase
      rescue URI::InvalidURIError
        ""
      end

      def monotonic_now
        Process.clock_gettime(Process::CLOCK_MONOTONIC)
      end

      private

      def connectivity_ready?(state)
        state == 2 || state.to_s.downcase == "ready"
      end

      def connectivity_shutdown?(state)
        state == 4 || state.to_s.downcase == "shutdown"
      end
    end
  end

  module Discover
    class << self
      attr_accessor :probe, :source_bridge, :executable_path_resolver

      def set_probe(callable = nil, &block)
        self.probe = callable || block
      end

      def set_source_bridge(callable = nil, &block)
        self.source_bridge = callable || block
      end

      def set_executable_path_resolver(callable = nil, &block)
        self.executable_path_resolver = callable || block
      end

      def discover(root = nil)
        Holons.discover(LOCAL, nil, root, ALL, NO_LIMIT, NO_TIMEOUT).found
      end

      def discover_local
        Holons.discover(LOCAL, nil, nil, CWD, NO_LIMIT, NO_TIMEOUT).found
      end

      def discover_all(root = nil)
        Holons.discover(LOCAL, nil, root, ALL, NO_LIMIT, NO_TIMEOUT).found
      end

      def find_by_slug(slug, root = nil)
        Holons.resolve(LOCAL, slug, root, ALL, NO_TIMEOUT).ref
      end

      def discover_by_slug(slug, root = nil)
        find_by_slug(slug, root)
      end

      def find_by_uuid(prefix, root = nil)
        Holons.resolve(LOCAL, prefix, root, ALL, NO_TIMEOUT).ref
      end
    end
  end

  class << self
    def discover(scope, expression, root, specifiers, limit, timeout)
      validate_scope!(scope)
      specifier_bits = normalize_specifiers(specifiers)
      return DiscoverResult.new(found: [], error: nil) if limit.to_i < 0

      DiscoverySupport.with_timeout(timeout, "discover") do
        expr = normalized_expression(expression)
        if !expr.nil? && DiscoverySupport.direct_transport_expression?(expr)
          return DiscoverResult.new(found: [], error: "direct URL discovery is not implemented")
        end

        if !expr.nil?
          path_result = discover_path_expression(expr, root, timeout)
          return DiscoverResult.new(found: apply_limit(path_result, limit), error: nil) unless path_result.nil?
        end

        resolved_root = resolve_discover_root(root)
        entries = discover_entries(resolved_root, specifier_bits, expr, timeout)

        found = []
        entries.each do |entry|
          next unless matches_expression?(entry, expr)

          found << HolonRef.new(url: entry.url, info: entry.info, error: nil)
          break if limit.to_i > 0 && found.length >= limit.to_i
        end

        DiscoverResult.new(found: found, error: nil)
      end
    rescue StandardError => e
      DiscoverResult.new(found: [], error: e.message)
    end

    def resolve(scope, expression, root, specifiers, timeout)
      result = discover(scope, expression, root, specifiers, 1, timeout)
      return ResolveResult.new(ref: nil, error: result.error) unless result.error.nil?

      ref = result.found.first
      if ref.nil?
        return ResolveResult.new(ref: nil, error: %(holon "#{expression}" not found))
      end
      return ResolveResult.new(ref: ref, error: ref.error) unless ref.error.to_s.empty?

      ResolveResult.new(ref: ref, error: nil)
    end

    private

    def normalized_expression(expression)
      return nil if expression.nil?

      expression.to_s.strip
    end

    def validate_scope!(scope)
      return if scope.to_i == LOCAL

      raise "scope #{scope} not supported"
    end

    def normalize_specifiers(specifiers)
      bits = specifiers.to_i
      raise format("invalid specifiers 0x%02X: valid range is 0x00-0x3F", bits) if bits.negative? || (bits & ~ALL) != 0

      bits.zero? ? ALL : bits
    end

    def resolve_discover_root(root)
      return File.expand_path(Dir.pwd) if root.nil?

      trimmed = root.to_s.strip
      raise "root cannot be empty" if trimmed.empty?

      expanded = File.expand_path(trimmed)
      raise %(root "#{trimmed}" is not a directory) unless File.directory?(expanded)

      expanded
    end

    def discover_path_expression(expression, root, timeout)
      candidate = path_expression_candidate(expression, root)
      return nil if candidate.nil?
      return [] unless File.exist?(candidate)

      if File.directory?(candidate)
        return [discover_package_at_path(candidate, File.dirname(candidate))] if package_dir?(candidate)

        manifest_path = direct_manifest_path(candidate)
        return [] if manifest_path.nil?

        return [source_entry_from_manifest(manifest_path, candidate).yield_self { |entry| HolonRef.new(url: entry.url, info: entry.info, error: nil) }]
      end

      if File.basename(candidate) == Identity::PROTO_MANIFEST_FILE_NAME
        return [source_entry_from_manifest(candidate, File.dirname(candidate)).yield_self { |entry| HolonRef.new(url: entry.url, info: entry.info, error: nil) }]
      end

      if File.basename(candidate) == ".holon.json"
        package_root = File.dirname(candidate)
        return [discover_package_at_path(package_root, File.dirname(package_root))]
      end

      info = probe_binary_info(candidate)
      [HolonRef.new(url: DiscoverySupport.file_url(candidate), info: info, error: nil)]
    rescue StandardError => e
      [HolonRef.new(url: DiscoverySupport.file_url(candidate), info: nil, error: e.message)]
    end

    def path_expression_candidate(expression, root)
      trimmed = expression.to_s.strip
      return nil if trimmed.empty?

      if trimmed.downcase.start_with?("file://")
        return DiscoverySupport.path_from_file_url(trimmed)
      end

      return nil if DiscoverySupport.direct_transport_expression?(trimmed)

      if Pathname.new(trimmed).absolute? || trimmed.start_with?(".") || trimmed.include?(File::SEPARATOR) || trimmed.include?("/") || trimmed.include?("\\") || trimmed.downcase.end_with?(".holon")
        base = root.nil? ? Dir.pwd : resolve_discover_root(root)
        return Pathname.new(trimmed).absolute? ? File.expand_path(trimmed) : File.expand_path(trimmed, base)
      end

      nil
    end

    def discover_entries(root, specifiers, expression, timeout)
      layers = [
        [SIBLINGS, "siblings", -> { discover_packages_direct(bundle_holons_root, "siblings") }],
        [CWD, "cwd", -> { discover_packages_recursive(root, "cwd") }],
        [SOURCE, "source", -> { discover_source_entries(root, expression, timeout) }],
        [BUILT, "built", -> { discover_packages_direct(File.join(root, ".op", "build"), "built") }],
        [INSTALLED, "installed", -> { discover_packages_direct(opbin, "installed") }],
        [CACHED, "cached", -> { discover_packages_recursive(cache_dir, "cached") }]
      ]

      found = []
      seen = {}

      layers.each do |flag, _name, loader|
        next if (specifiers & flag).zero?

        loader.call.each do |entry|
          key = discovery_entry_key(entry)
          next if seen[key]

          seen[key] = true
          found << entry
        end
      end

      found
    end

    def discover_packages_direct(root, origin)
      dirs = package_dirs_direct(root)
      discover_packages_from_dirs(root, origin, dirs)
    end

    def discover_packages_recursive(root, origin)
      dirs = package_dirs_recursive(root)
      discover_packages_from_dirs(root, origin, dirs)
    end

    def discover_packages_from_dirs(root, _origin, dirs)
      return [] if dirs.empty?

      entries_by_key = {}
      keys = []
      dirs.each do |dir|
        entry = load_or_probe_package_entry(root, dir)
        next if entry.nil?

        key = discovery_entry_key(entry)
        if entries_by_key.key?(key)
          entries_by_key[key] = entry if should_replace_entry?(entries_by_key[key], entry)
          next
        end

        entries_by_key[key] = entry
        keys << key
      end

      keys.map { |key| entries_by_key[key] }.compact.sort_by { |entry| [entry.relative_path.to_s, entry.info&.uuid.to_s] }
    end

    def package_dirs_direct(root)
      abs_root = safe_existing_dir(root)
      return [] if abs_root.nil?

      Dir.children(abs_root).sort.map do |name|
        path = File.join(abs_root, name)
        path if File.directory?(path) && name.downcase.end_with?(".holon")
      end.compact
    rescue SystemCallError
      []
    end

    def package_dirs_recursive(root)
      abs_root = safe_existing_dir(root)
      return [] if abs_root.nil?

      dirs = []
      walk_dir(abs_root) do |path, name|
        if name.downcase.end_with?(".holon")
          dirs << path
          :skip
        elsif should_skip_dir?(abs_root, path, name)
          :skip
        end
      end
      dirs.sort
    end

    def load_or_probe_package_entry(root, dir)
      load_package_entry(root, dir)
    rescue StandardError
      probe_package_entry(root, dir)
    end

    def load_package_entry(root, dir)
      payload = JSON.parse(File.read(File.join(dir, ".holon.json")))
      schema = payload["schema"].to_s.strip
      raise ".holon.json schema mismatch" unless schema.empty? || schema == "holon-package/v1"

      identity_payload = payload.fetch("identity", {})
      given_name = identity_payload["given_name"].to_s
      family_name = identity_payload["family_name"].to_s
      aliases = Array(identity_payload["aliases"]).map(&:to_s)
      entrypoint = payload["entrypoint"].to_s

      info = HolonInfo.new(
        slug: payload["slug"].to_s.strip.empty? ? DiscoverySupport.slug_for(given_name, family_name) : payload["slug"].to_s,
        uuid: payload["uuid"].to_s,
        identity: IdentityInfo.new(
          given_name: given_name,
          family_name: family_name,
          motto: identity_payload["motto"].to_s,
          aliases: aliases
        ),
        lang: payload["lang"].to_s,
        runner: payload["runner"].to_s,
        status: payload["status"].to_s,
        kind: payload["kind"].to_s,
        transport: payload["transport"].to_s,
        entrypoint: entrypoint,
        architectures: Array(payload["architectures"]).map(&:to_s),
        has_dist: !!payload["has_dist"],
        has_source: !!payload["has_source"]
      )

      abs_dir = File.expand_path(dir)
      DiscoveryEntry.new(
        url: DiscoverySupport.file_url(abs_dir),
        info: info,
        dir: abs_dir,
        relative_path: relative_path(root, abs_dir)
      )
    end

    def probe_package_entry(root, dir)
      override = Discover.probe
      entry =
        if override.nil?
          native_probe_package_entry(root, dir)
        else
          coerce_probe_result(override.call(File.expand_path(dir)), root, dir)
        end

      entry || native_probe_package_entry(root, dir)
    end

    def native_probe_package_entry(root, dir)
      info = probe_binary_info(package_binary_path(dir))
      abs_dir = File.expand_path(dir)
      DiscoveryEntry.new(
        url: DiscoverySupport.file_url(abs_dir),
        info: info,
        dir: abs_dir,
        relative_path: relative_path(root, abs_dir)
      )
    end

    def probe_binary_info(binary_path)
      DiscoverySupport.probe_binary(binary_path, timeout: 5000)
    end

    def package_binary_path(dir)
      candidates = []
      arch_dir = File.join(dir, "bin", DiscoverySupport.package_arch_dir)
      if File.directory?(arch_dir)
        Dir.children(arch_dir).sort.each do |name|
          path = File.join(arch_dir, name)
          candidates << path if File.file?(path)
        end
      end

      %w[bin dist].each do |subdir|
        subroot = File.join(dir, subdir)
        next unless File.directory?(subroot)

        walk_dir(subroot) do |path, _name|
          candidates << path if File.file?(path)
        end
      end

      candidate = candidates.find { |path| File.file?(path) }
      raise Errno::ENOENT, dir if candidate.nil?

      candidate
    end

    def coerce_probe_result(result, root, dir)
      return nil if result.nil?
      return normalize_entry(result, root, dir) if result.is_a?(DiscoveryEntry)

      if result.is_a?(HolonRef)
        abs_dir = File.expand_path(dir)
        return DiscoveryEntry.new(
          url: result.url.to_s.empty? ? DiscoverySupport.file_url(abs_dir) : result.url.to_s,
          info: coerce_info(result.info),
          dir: abs_dir,
          relative_path: relative_path(root, abs_dir)
        )
      end

      info = coerce_info(result)
      return nil if info.nil?

      abs_dir = File.expand_path(dir)
      DiscoveryEntry.new(
        url: DiscoverySupport.file_url(abs_dir),
        info: info,
        dir: abs_dir,
        relative_path: relative_path(root, abs_dir)
      )
    end

    def normalize_entry(entry, root, dir)
      abs_dir = File.expand_path(dir)
      DiscoveryEntry.new(
        url: entry.url.to_s.empty? ? DiscoverySupport.file_url(abs_dir) : entry.url,
        info: coerce_info(entry.info),
        dir: abs_dir,
        relative_path: relative_path(root, abs_dir)
      )
    end

    def coerce_info(value)
      return value if value.is_a?(HolonInfo)
      return nil if value.nil?

      if value.is_a?(Hash)
        identity = value[:identity] || value["identity"] || {}
        return HolonInfo.new(
          slug: value[:slug] || value["slug"],
          uuid: value[:uuid] || value["uuid"],
          identity: coerce_identity(identity),
          lang: value[:lang] || value["lang"],
          runner: value[:runner] || value["runner"],
          status: value[:status] || value["status"],
          kind: value[:kind] || value["kind"],
          transport: value[:transport] || value["transport"],
          entrypoint: value[:entrypoint] || value["entrypoint"],
          architectures: Array(value[:architectures] || value["architectures"]).map(&:to_s),
          has_dist: !!(value[:has_dist] || value["has_dist"]),
          has_source: !!(value[:has_source] || value["has_source"])
        )
      end

      nil
    end

    def coerce_identity(value)
      return value if value.is_a?(IdentityInfo)

      Hash === value ? IdentityInfo.new(
        given_name: value[:given_name] || value["given_name"],
        family_name: value[:family_name] || value["family_name"],
        motto: value[:motto] || value["motto"],
        aliases: Array(value[:aliases] || value["aliases"]).map(&:to_s)
      ) : IdentityInfo.new(given_name: "", family_name: "", motto: "", aliases: [])
    end

    def discover_source_entries(root, expression, timeout)
      bridge = Discover.source_bridge
      if bridge
        result = bridge.call(LOCAL, expression, root, SOURCE, NO_LIMIT, timeout)
        return normalize_bridge_entries(result, root)
      end

      native_source_entries(root)
    end

    def normalize_bridge_entries(result, root)
      discovered =
        case result
        when DiscoverResult
          raise result.error unless result.error.nil?

          result.found
        when HolonRef
          [result]
        when Array
          result
        else
          raise "source bridge returned unsupported result"
        end

      entries_by_key = {}
      keys = []
      discovered.each do |item|
        ref =
          if item.is_a?(HolonRef)
            item
          elsif item.is_a?(Hash)
            HolonRef.new(
              url: item[:url] || item["url"],
              info: item[:info] || item["info"],
              error: item[:error] || item["error"]
            )
          else
            raise "source bridge returned unsupported entry"
          end
        entry = entry_from_ref(ref, root)
        next if entry.nil?

        key = discovery_entry_key(entry)
        if entries_by_key.key?(key)
          entries_by_key[key] = entry if should_replace_entry?(entries_by_key[key], entry)
          next
        end

        entries_by_key[key] = entry
        keys << key
      end
      keys.map { |key| entries_by_key[key] }.compact.sort_by { |entry| [entry.relative_path.to_s, entry.info&.uuid.to_s] }
    end

    def native_source_entries(root)
      abs_root = safe_existing_dir(root)
      return [] if abs_root.nil?

      entries_by_key = {}
      keys = []
      walk_dir(abs_root) do |path, name|
        if File.directory?(path)
          next :skip if should_skip_dir?(abs_root, path, name)
          next
        end
        next unless name == Identity::PROTO_MANIFEST_FILE_NAME

        entry = source_entry_from_manifest(path, abs_root)
        key = discovery_entry_key(entry)
        if entries_by_key.key?(key)
          entries_by_key[key] = entry if should_replace_entry?(entries_by_key[key], entry)
          next
        end

        entries_by_key[key] = entry
        keys << key
      rescue StandardError
        next
      end

      keys.map { |key| entries_by_key[key] }.compact.sort_by { |entry| [entry.relative_path.to_s, entry.info&.uuid.to_s] }
    end

    def source_entry_from_manifest(manifest_path, root)
      resolved = Identity.parse_manifest(manifest_path)
      dir = manifest_root(manifest_path)
      abs_dir = File.expand_path(dir)
      DiscoveryEntry.new(
        url: DiscoverySupport.file_url(abs_dir),
        info: DiscoverySupport.holon_info_from_manifest(resolved),
        dir: abs_dir,
        relative_path: relative_path(root, abs_dir)
      )
    end

    def manifest_root(manifest_path)
      manifest_dir = File.expand_path(File.dirname(manifest_path))
      version_dir = File.basename(manifest_dir)
      api_dir = File.basename(File.dirname(manifest_dir))
      return File.expand_path(File.join(manifest_dir, "..", "..")) if version_dir.match?(/^v[0-9]+[A-Za-z0-9._-]*$/) && api_dir == "api"

      manifest_dir
    end

    def entry_from_ref(ref, root)
      info = coerce_info(ref.info)
      return nil if info.nil?

      dir =
        if ref.url.to_s.downcase.start_with?("file://")
          DiscoverySupport.path_from_file_url(ref.url)
        else
          info.entrypoint.to_s
        end
      return nil if dir.to_s.empty?

      abs_dir = File.expand_path(dir)
      DiscoveryEntry.new(
        url: ref.url,
        info: info,
        dir: abs_dir,
        relative_path: relative_path(root, abs_dir)
      )
    rescue StandardError
      nil
    end

    def discover_package_at_path(dir, root)
      entry = load_or_probe_package_entry(root, dir)
      HolonRef.new(url: entry.url, info: entry.info, error: nil)
    end

    def package_dir?(dir)
      name = File.basename(dir).downcase
      name.end_with?(".holon") || File.file?(File.join(dir, ".holon.json"))
    end

    def direct_manifest_path(dir)
      direct = File.join(dir, Identity::PROTO_MANIFEST_FILE_NAME)
      return direct if File.file?(direct)

      api_v1 = File.join(dir, "api", "v1", Identity::PROTO_MANIFEST_FILE_NAME)
      return api_v1 if File.file?(api_v1)

      nil
    end

    def discovery_entry_key(entry)
      uuid = entry.info&.uuid.to_s.strip
      uuid.empty? ? entry.dir.to_s : uuid
    end

    def should_replace_entry?(current, candidate)
      path_depth(candidate.relative_path) < path_depth(current.relative_path)
    end

    def matches_expression?(entry, expression)
      return true if expression.nil?

      needle = expression.to_s.strip
      return false if needle.empty?
      return true if entry.info&.slug.to_s == needle
      return true if entry.info&.uuid.to_s.start_with?(needle)

      aliases = Array(entry.info&.identity&.aliases).map(&:to_s)
      return true if aliases.include?(needle)

      File.basename(entry.dir.to_s).sub(/\.holon\z/i, "") == needle
    end

    def relative_path(root, dir)
      Pathname.new(File.expand_path(dir)).relative_path_from(Pathname.new(File.expand_path(root))).to_s.tr(File::SEPARATOR, "/")
    rescue ArgumentError
      File.expand_path(dir).tr(File::SEPARATOR, "/")
    end

    def path_depth(relative_path)
      trimmed = relative_path.to_s.strip.gsub(%r{\A/+|/+\z}, "")
      return 0 if trimmed.empty? || trimmed == "."

      trimmed.split("/").length
    end

    def apply_limit(items, limit)
      max = limit.to_i
      return items if max <= 0 || items.length <= max

      items.first(max)
    end

    def should_skip_dir?(root, path, name)
      return false if File.expand_path(path) == File.expand_path(root)
      return false if name.downcase.end_with?(".holon")

      return true if %w[.git .op node_modules vendor build testdata].include?(name)

      name.start_with?(".")
    end

    def safe_existing_dir(path)
      return nil if path.to_s.strip.empty?

      expanded = File.expand_path(path)
      File.directory?(expanded) ? expanded : nil
    end

    def walk_dir(root, &block)
      Dir.each_child(root) do |name|
        path = File.join(root, name)
        if File.directory?(path)
          action = block.call(path, name)
          next if action == :skip

          walk_dir(path, &block)
          next
        end
        block.call(path, name)
      end
    rescue Errno::ENOENT, Errno::EACCES, Errno::ENOTDIR
      nil
    end

    def bundle_holons_root
      resolver = Discover.executable_path_resolver
      executable =
        if resolver.nil?
          File.expand_path($PROGRAM_NAME.to_s)
        else
          resolver.call.to_s
        end
      return nil if executable.strip.empty?

      current = File.expand_path(File.dirname(executable))
      loop do
        if current.downcase.end_with?(".app")
          candidate = File.join(current, "Contents", "Resources", "Holons")
          return candidate if File.directory?(candidate)
        end

        parent = File.dirname(current)
        break if parent == current

        current = parent
      end
      nil
    end

    def oppath
      configured = ENV.fetch("OPPATH", "").to_s.strip
      return File.expand_path(configured) unless configured.empty?

      File.expand_path("~/.op")
    end

    def opbin
      configured = ENV.fetch("OPBIN", "").to_s.strip
      return File.expand_path(configured) unless configured.empty?

      File.join(oppath, "bin")
    end

    def cache_dir
      File.join(oppath, "cache")
    end
  end
end
