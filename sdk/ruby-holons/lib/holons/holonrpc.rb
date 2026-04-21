# frozen_string_literal: true

require "base64"
require "digest/sha1"
require "json"
require "net/http"
require "openssl"
require "securerandom"
require "socket"
require "uri"

module Holons
  class HolonRPCResponseError < StandardError
    attr_reader :code, :data

    def initialize(code, message, data = nil)
      super("rpc error #{code}: #{message}")
      @code = code
      @data = data
    end
  end

  HolonRPCSSEEvent = Struct.new(:event, :id, :result, :error, keyword_init: true)

  class HolonRPCHTTPClient
    def initialize(base_url, open_timeout: 10, read_timeout: 10, ssl_verify: false)
      @base_url = normalize_base_url(base_url)
      @open_timeout = open_timeout
      @read_timeout = read_timeout
      @ssl_verify = ssl_verify
    end

    def invoke(method, params = {})
      response = perform_request(
        Net::HTTP::Post,
        method_uri(method),
        accept: "application/json",
        body: encode_params(params)
      )

      decode_http_rpc_response(response)
    end

    def stream(method, params = {})
      response = perform_request(
        Net::HTTP::Post,
        method_uri(method),
        accept: "text/event-stream",
        body: encode_params(params)
      )

      read_sse_events(response)
    end

    def stream_query(method, params = {})
      uri = method_uri(method)
      values = []
      params.to_h.each do |key, value|
        Array(value).each do |entry|
          values << [key.to_s, entry.to_s]
        end
      end
      uri.query = URI.encode_www_form(values) unless values.empty?

      response = perform_request(
        Net::HTTP::Get,
        uri,
        accept: "text/event-stream"
      )

      read_sse_events(response)
    end

    private

    def normalize_base_url(base_url)
      trimmed = base_url.to_s.strip.sub(%r{/*\z}, "")
      raise ArgumentError, "base_url is required" if trimmed.empty?

      trimmed
    end

    def method_uri(method)
      trimmed = method.to_s.strip.gsub(%r{\A/+}, "")
      raise ArgumentError, "method is required" if trimmed.empty?

      URI.parse("#{@base_url}/#{trimmed}")
    end

    def encode_params(params)
      JSON.generate(params.is_a?(Hash) ? params : {})
    end

    def perform_request(request_class, uri, accept:, body: nil)
      http = Net::HTTP.new(uri.host, uri.port)
      http.open_timeout = @open_timeout if http.respond_to?(:open_timeout=)
      http.read_timeout = @read_timeout if http.respond_to?(:read_timeout=)

      if uri.scheme == "https"
        http.use_ssl = true
        http.verify_mode = @ssl_verify ? OpenSSL::SSL::VERIFY_PEER : OpenSSL::SSL::VERIFY_NONE
      end

      request = request_class.new(uri)
      request["Accept"] = accept
      if body
        request["Content-Type"] = "application/json"
        request.body = body
      end

      http.request(request)
    rescue SocketError, SystemCallError, IOError, OpenSSL::SSL::SSLError, Timeout::Error => e
      raise RuntimeError, "holon-rpc http request failed: #{e.message}"
    end

    def decode_http_rpc_response(response)
      parsed = parse_json(response.body.to_s)

      if parsed.is_a?(Hash) && parsed["error"].is_a?(Hash)
        error = parsed["error"]
        code = error["code"].is_a?(Numeric) ? error["code"].to_i : -32603
        message = error["message"].to_s.strip
        message = "internal error" if message.empty?
        raise HolonRPCResponseError.new(code, message, error["data"])
      end

      if parsed.is_a?(Hash) && parsed.key?("result")
        return parsed["result"].is_a?(Hash) ? parsed["result"] : {}
      end

      raise RuntimeError, "holon-rpc http status #{response.code}" if response.code.to_i >= 400

      parsed.is_a?(Hash) ? parsed : {}
    end

    def read_sse_events(response)
      decode_http_rpc_response(response) if response.code.to_i >= 400

      events = []
      current_event = nil
      current_id = nil
      current_data = []

      flush = lambda do
        return if current_event.nil? && current_id.nil? && current_data.empty?

        payload = current_data.join("\n")
        event = HolonRPCSSEEvent.new(
          event: current_event.to_s,
          id: current_id.to_s,
          result: {},
          error: nil
        )

        if %w[message error].include?(event.event)
          parsed = parse_json(payload)
          if parsed.is_a?(Hash) && parsed["error"].is_a?(Hash)
            error = parsed["error"]
            event.error = HolonRPCResponseError.new(
              error["code"].is_a?(Numeric) ? error["code"].to_i : -32603,
              error["message"].to_s.strip.empty? ? "internal error" : error["message"].to_s,
              error["data"]
            )
          elsif parsed.is_a?(Hash) && parsed.key?("result")
            event.result = parsed["result"].is_a?(Hash) ? parsed["result"] : {}
          end
        end

        events << event
      end

      response.body.to_s.each_line do |raw_line|
        line = raw_line.chomp
        if line.empty?
          flush.call
          current_event = nil
          current_id = nil
          current_data = []
          next
        end

        case
        when line.start_with?("event:")
          current_event = line.delete_prefix("event:").strip
        when line.start_with?("id:")
          current_id = line.delete_prefix("id:").strip
        when line.start_with?("data:")
          current_data << line.delete_prefix("data:").strip
        end
      end

      flush.call
      events
    end

    def parse_json(text)
      JSON.parse(text)
    rescue JSON::ParserError
      nil
    end
  end

  class HolonRPCClient
    PendingCall = Struct.new(:mutex, :cv, :done, :result, :error)

    def initialize(
      heartbeat_interval_ms: 15_000,
      heartbeat_timeout_ms: 5_000,
      reconnect_min_delay_ms: 500,
      reconnect_max_delay_ms: 30_000,
      reconnect_factor: 2.0,
      reconnect_jitter: 0.1,
      connect_timeout_ms: 10_000,
      request_timeout_ms: 10_000
    )
      @heartbeat_interval_ms = heartbeat_interval_ms
      @heartbeat_timeout_ms = heartbeat_timeout_ms
      @reconnect_min_delay_ms = reconnect_min_delay_ms
      @reconnect_max_delay_ms = reconnect_max_delay_ms
      @reconnect_factor = reconnect_factor
      @reconnect_jitter = reconnect_jitter
      @connect_timeout_ms = connect_timeout_ms
      @request_timeout_ms = request_timeout_ms

      @state_mutex = Mutex.new
      @connected_cv = ConditionVariable.new
      @send_mutex = Mutex.new
      @pending_mutex = Mutex.new
      @handlers_mutex = Mutex.new

      @pending = {}
      @handlers = {}

      @endpoint = nil
      @socket = nil
      @connected = false
      @running = false
      @closed = true
      @next_id = 0
      @reconnect_attempt = 0
      @last_error = nil
      @io_thread = nil
      @heartbeat_thread = nil
    end

    def connect(url)
      raise ArgumentError, "url is required" if url.nil? || url.empty?

      close
      @endpoint = url
      @running = true
      @closed = false
      @reconnect_attempt = 0

      @state_mutex.synchronize do
        @connected = false
        @last_error = nil
      end

      @io_thread = Thread.new { io_loop }
      @heartbeat_thread = Thread.new { heartbeat_loop }

      return if wait_connected(@connect_timeout_ms / 1000.0)

      reason = @state_mutex.synchronize { @last_error&.message || "holon-rpc connect timeout" }
      close
      raise RuntimeError, reason
    end

    def register(method, &handler)
      raise ArgumentError, "method is required" if method.nil? || method.empty?
      raise ArgumentError, "handler block is required" unless block_given?

      @handlers_mutex.synchronize do
        @handlers[method] = handler
      end
    end

    def invoke(method, params = {}, timeout_ms: nil)
      raise ArgumentError, "method is required" if method.nil? || method.empty?

      wait_connected!(@connect_timeout_ms / 1000.0)

      request_id = nil
      call = PendingCall.new(Mutex.new, ConditionVariable.new, false, nil, nil)

      @pending_mutex.synchronize do
        @next_id += 1
        request_id = "c#{@next_id}"
        @pending[request_id] = call
      end

      payload = {
        "jsonrpc" => "2.0",
        "id" => request_id,
        "method" => method,
        "params" => params.is_a?(Hash) ? params : {}
      }

      begin
        send_json(payload)
      rescue StandardError
        remove_pending(request_id)
        raise
      end

      timeout = (timeout_ms || @request_timeout_ms) / 1000.0
      done = false
      call.mutex.synchronize do
        done = call.cv.wait(call.mutex, timeout) if !call.done
        done = true if call.done
      end

      unless done
        remove_pending(request_id)
        raise RuntimeError, "invoke timeout"
      end

      raise call.error if call.error

      call.result || {}
    end

    def close
      already_closed = @state_mutex.synchronize do
        was_closed = @closed && !@running
        @closed = true
        @running = false
        @connected = false
        @last_error = RuntimeError.new("holon-rpc client closed")
        @connected_cv.broadcast
        was_closed
      end
      return if already_closed

      force_disconnect

      begin
        @io_thread&.join
      rescue StandardError
        nil
      end
      begin
        @heartbeat_thread&.join
      rescue StandardError
        nil
      end
      @io_thread = nil
      @heartbeat_thread = nil

      close_socket
      fail_all_pending(RuntimeError.new("holon-rpc client closed"))
    end

    private

    def io_loop
      while @running
        if socket.nil?
          begin
            connected = open_socket
            unless connected
              sleep(compute_backoff_delay_seconds(@reconnect_attempt))
              @reconnect_attempt += 1
              next
            end
          rescue StandardError => e
            @state_mutex.synchronize do
              @connected = false
              @last_error = e
              @connected_cv.broadcast
            end
            @running = false
            @closed = true
            fail_all_pending(e)
            return
          end

          @reconnect_attempt = 0
          @state_mutex.synchronize do
            @connected = true
            @last_error = nil
            @connected_cv.broadcast
          end
        end

        text = read_text_frame
        unless text
          mark_disconnected(RuntimeError.new("holon-rpc connection closed"))
          next
        end

        handle_incoming(text)
      end
    end

    def heartbeat_loop
      while @running
        sleep_interruptible(@heartbeat_interval_ms / 1000.0)
        break unless @running
        next unless connected?

        begin
          invoke("rpc.heartbeat", {}, timeout_ms: @heartbeat_timeout_ms)
        rescue StandardError
          force_disconnect
        end
      end
    end

    def sleep_interruptible(seconds)
      slept = 0.0
      while @running && slept < seconds
        step = [0.1, seconds - slept].min
        sleep(step)
        slept += step
      end
    end

    def wait_connected(timeout_seconds)
      deadline = Process.clock_gettime(Process::CLOCK_MONOTONIC) + timeout_seconds
      @state_mutex.synchronize do
        until @connected || !@running
          remaining = deadline - Process.clock_gettime(Process::CLOCK_MONOTONIC)
          return false if remaining <= 0

          @connected_cv.wait(@state_mutex, remaining)
        end
        @connected
      end
    end

    def wait_connected!(timeout_seconds)
      return if wait_connected(timeout_seconds)

      reason = @state_mutex.synchronize { @last_error&.message || "not connected" }
      raise RuntimeError, reason
    end

    def connected?
      @state_mutex.synchronize { @connected }
    end

    def socket
      @state_mutex.synchronize { @socket }
    end

    def socket=(value)
      @state_mutex.synchronize { @socket = value }
    end

    def close_socket
      @state_mutex.synchronize do
        if @socket
          begin
            @socket.close
          rescue StandardError
            nil
          end
        end
        @socket = nil
      end
    end

    def force_disconnect
      sock = socket
      return unless sock

      begin
        sock.close
      rescue StandardError
        nil
      end
    end

    def mark_disconnected(error)
      close_socket
      @state_mutex.synchronize do
        @connected = false
        @last_error = error
        @connected_cv.broadcast
      end
      fail_all_pending(error)
    end

    def open_socket
      uri = URI.parse(@endpoint)
      unless %w[ws wss].include?(uri.scheme)
        raise RuntimeError, "holon-rpc endpoint must use ws:// or wss://"
      end

      host = uri.host || "127.0.0.1"
      port = uri.port || (uri.scheme == "wss" ? 443 : 80)
      path = uri.path.nil? || uri.path.empty? ? "/rpc" : uri.path
      path = "#{path}?#{uri.query}" if uri.query

      tcp = TCPSocket.new(host, port)
      ws = tcp
      if uri.scheme == "wss"
        ctx = OpenSSL::SSL::SSLContext.new
        ctx.verify_mode = OpenSSL::SSL::VERIFY_NONE
        ssl = OpenSSL::SSL::SSLSocket.new(tcp, ctx)
        ssl.sync_close = true
        ssl.hostname = host if ssl.respond_to?(:hostname=)
        ssl.connect
        ws = ssl
      end

      key = Base64.strict_encode64(SecureRandom.random_bytes(16))
      request = +"GET #{path} HTTP/1.1\r\n"
      request << "Host: #{host}:#{port}\r\n"
      request << "Upgrade: websocket\r\n"
      request << "Connection: Upgrade\r\n"
      request << "Sec-WebSocket-Key: #{key}\r\n"
      request << "Sec-WebSocket-Version: 13\r\n"
      request << "Sec-WebSocket-Protocol: holon-rpc\r\n\r\n"
      ws.write(request)
      ws.flush

      headers = +""
      while !headers.include?("\r\n\r\n")
        chunk = ws.readpartial(1)
        headers << chunk
        raise RuntimeError, "websocket handshake too large" if headers.bytesize > 16_384
      end

      lower = headers.downcase
      unless lower.include?(" 101 ") && lower.include?("sec-websocket-protocol: holon-rpc")
        ws.close
        raise RuntimeError, "server did not negotiate holon-rpc websocket protocol"
      end

      self.socket = ws
      true
    rescue SystemCallError, OpenSSL::SSL::SSLError
      false
    end

    def compute_backoff_delay_seconds(attempt)
      base = [
        @reconnect_min_delay_ms * (@reconnect_factor**attempt),
        @reconnect_max_delay_ms
      ].min
      jitter = base * @reconnect_jitter * rand
      (base + jitter) / 1000.0
    end

    def send_json(payload)
      text = JSON.generate(payload)
      send_frame(0x1, text)
    end

    def send_frame(opcode, payload)
      sock = socket
      raise RuntimeError, "websocket is not connected" if sock.nil?

      payload = payload.to_s.dup.force_encoding(Encoding::BINARY)
      bytes = payload.bytes
      len = bytes.length

      frame = +""
      frame << (0x80 | opcode).chr
      if len < 126
        frame << (0x80 | len).chr
      elsif len <= 0xFFFF
        frame << (0x80 | 126).chr
        frame << [len].pack("n")
      else
        frame << (0x80 | 127).chr
        frame << [len].pack("Q>")
      end

      mask = SecureRandom.random_bytes(4).bytes
      frame << mask.pack("C*")
      masked = bytes.each_with_index.map { |b, i| b ^ mask[i % 4] }
      frame << masked.pack("C*")

      @send_mutex.synchronize do
        sock.write(frame)
        sock.flush if sock.respond_to?(:flush)
      end
      true
    end

    def read_text_frame
      sock = socket
      return nil if sock.nil?

      fragmented = +""

      loop do
        header = read_exact(sock, 2)
        return nil if header.nil?

        b0 = header.getbyte(0)
        b1 = header.getbyte(1)
        fin = (b0 & 0x80) != 0
        opcode = b0 & 0x0F
        masked = (b1 & 0x80) != 0
        len = b1 & 0x7F

        if len == 126
          ext = read_exact(sock, 2)
          return nil if ext.nil?

          len = ext.unpack1("n")
        elsif len == 127
          ext = read_exact(sock, 8)
          return nil if ext.nil?

          len = ext.unpack1("Q>")
        end

        mask_key = nil
        if masked
          mask_raw = read_exact(sock, 4)
          return nil if mask_raw.nil?

          mask_key = mask_raw.bytes
        end

        payload = len.zero? ? "".b : read_exact(sock, len)
        return nil if payload.nil?
        payload = payload.dup.force_encoding(Encoding::BINARY)

        if masked
          bytes = payload.bytes.each_with_index.map { |b, i| b ^ mask_key[i % 4] }
          payload = bytes.pack("C*")
        end

        case opcode
        when 0x8 # close
          return nil
        when 0x9 # ping
          send_frame(0xA, payload)
          next
        when 0xA # pong
          next
        when 0x1, 0x0 # text or continuation
          fragmented << payload
          return fragmented.force_encoding(Encoding::UTF_8) if fin
        else
          next
        end
      end
    rescue EOFError, IOError, SystemCallError, OpenSSL::SSL::SSLError
      nil
    end

    def read_exact(sock, size)
      data = +""
      while data.bytesize < size
        chunk = sock.readpartial(size - data.bytesize)
        return nil if chunk.nil? || chunk.empty?

        data << chunk
      end
      data
    rescue EOFError, IOError, SystemCallError, OpenSSL::SSL::SSLError
      nil
    end

    def handle_incoming(text)
      msg = JSON.parse(text)
      return unless msg.is_a?(Hash)

      if msg.key?("method")
        handle_request(msg)
      elsif msg.key?("result") || msg.key?("error")
        handle_response(msg)
      end
    rescue JSON::ParserError
      nil
    end

    def handle_request(msg)
      request_id = msg["id"]
      method = msg["method"]
      jsonrpc = msg["jsonrpc"]

      if jsonrpc != "2.0" || !method.is_a?(String) || method.empty?
        send_error(request_id, -32600, "invalid request") unless request_id.nil?
        return
      end

      if method == "rpc.heartbeat"
        send_result(request_id, {}) unless request_id.nil?
        return
      end

      unless request_id.nil?
        sid = request_id.is_a?(String) ? request_id : request_id.to_s
        unless sid.start_with?("s")
          send_error(request_id, -32600, "server request id must start with 's'")
          return
        end
      end

      handler = @handlers_mutex.synchronize { @handlers[method] }
      unless handler
        send_error(request_id, -32601, %(method "#{method}" not found)) unless request_id.nil?
        return
      end

      params = msg["params"].is_a?(Hash) ? msg["params"] : {}

      begin
        result = handler.call(params)
        send_result(request_id, result.is_a?(Hash) ? result : {}) unless request_id.nil?
      rescue HolonRPCResponseError => e
        send_error(request_id, e.code, e.message, e.data) unless request_id.nil?
      rescue StandardError => e
        send_error(request_id, 13, e.message) unless request_id.nil?
      end
    end

    def handle_response(msg)
      return unless msg.key?("id")

      request_id = msg["id"].is_a?(String) ? msg["id"] : msg["id"].to_s
      call = nil
      @pending_mutex.synchronize do
        call = @pending.delete(request_id)
      end
      return unless call

      call.mutex.synchronize do
        if msg["error"].is_a?(Hash)
          err = msg["error"]
          code = err["code"].is_a?(Numeric) ? err["code"].to_i : -32603
          message = err["message"].is_a?(String) ? err["message"] : "internal error"
          call.error = HolonRPCResponseError.new(code, message, err["data"])
        else
          call.result = msg["result"].is_a?(Hash) ? msg["result"] : {}
        end
        call.done = true
        call.cv.broadcast
      end
    end

    def send_result(request_id, result)
      send_json(
        "jsonrpc" => "2.0",
        "id" => request_id,
        "result" => result.is_a?(Hash) ? result : {}
      )
    end

    def send_error(request_id, code, message, data = nil)
      payload = {
        "jsonrpc" => "2.0",
        "id" => request_id,
        "error" => {
          "code" => code,
          "message" => message
        }
      }
      payload["error"]["data"] = data unless data.nil?
      send_json(payload)
    end

    def fail_all_pending(error)
      snapshot = nil
      @pending_mutex.synchronize do
        snapshot = @pending.values
        @pending.clear
      end

      snapshot.each do |call|
        call.mutex.synchronize do
          call.error = error
          call.done = true
          call.cv.broadcast
        end
      end
    end

    def remove_pending(request_id)
      @pending_mutex.synchronize do
        @pending.delete(request_id)
      end
    end
  end
end
