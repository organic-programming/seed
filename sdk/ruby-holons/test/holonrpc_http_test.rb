# frozen_string_literal: true

require "json"
require "minitest/autorun"
require "webrick"
require_relative "../lib/holons"

class HolonRPCHTTPTest < Minitest::Test
  def test_invoke_posts_json_rpc_request
    with_http_rpc_server do |base_url|
      client = Holons::HolonRPCHTTPClient.new(base_url)
      out = client.invoke("echo.v1.Echo/Ping", "message" => "hello")

      assert_equal "hello", out["message"]
    end
  end

  def test_stream_posts_sse_request
    with_http_rpc_server do |base_url|
      client = Holons::HolonRPCHTTPClient.new(base_url)
      events = client.stream("build.v1.Build/Watch", "project" => "myapp")

      assert_equal 3, events.length
      assert_equal "message", events[0].event
      assert_equal "1", events[0].id
      assert_equal "building", events[0].result["status"]
      assert_equal "done", events[1].result["status"]
      assert_equal "done", events[2].event
    end
  end

  def test_stream_query_gets_sse_request
    with_http_rpc_server do |base_url|
      client = Holons::HolonRPCHTTPClient.new(base_url)
      events = client.stream_query("build.v1.Build/WatchQuery", "project" => "myapp")

      assert_equal 2, events.length
      assert_equal "watching", events[0].result["status"]
      assert_equal "done", events[1].event
    end
  end

  def test_invoke_raises_rpc_error_for_missing_method
    with_http_rpc_server do |base_url|
      client = Holons::HolonRPCHTTPClient.new(base_url)

      error = assert_raises(Holons::HolonRPCResponseError) do
        client.invoke("missing.v1.Service/Method")
      end

      assert_equal 5, error.code
      assert_match(/not found/, error.message)
    end
  end

  private

  def with_http_rpc_server
    server = WEBrick::HTTPServer.new(
      Port: 0,
      BindAddress: "127.0.0.1",
      Logger: WEBrick::Log.new(File::NULL),
      AccessLog: []
    )

    server.mount_proc("/") do |req, res|
      set_cors_headers(req, res)

      if req.request_method == "OPTIONS"
        res.status = 204
        res.body = ""
        next
      end

      method = extract_method(req.path)
      unless method
        res.status = 404
        res["Content-Type"] = "application/json"
        res.body = rpc_error_json("h0", 5, %(method "#{req.path}" not found))
        next
      end

      if req.header["accept"].to_a.any? { |value| value.to_s.downcase.include?("text/event-stream") }
        handle_sse_request(req, res, method)
      else
        handle_unary_request(req, res, method)
      end
    end

    thread = Thread.new { server.start }
    sleep 0.05 until server.status == :Running

    port = server.listeners.fetch(0).addr[1]
    yield("http://127.0.0.1:#{port}/api/v1/rpc")
  ensure
    server&.shutdown
    thread&.join
  end

  def set_cors_headers(req, res)
    res["Access-Control-Allow-Origin"] = req["Origin"].to_s.empty? ? "*" : req["Origin"]
    res["Access-Control-Allow-Methods"] = "GET, POST, OPTIONS"
    res["Access-Control-Allow-Headers"] = "Content-Type, Accept, Last-Event-ID"
    res["Access-Control-Max-Age"] = "86400"
  end

  def extract_method(path)
    prefix = "/api/v1/rpc/"
    return nil unless path.start_with?(prefix)

    method = path.delete_prefix(prefix).sub(%r{\A/+}, "").sub(%r{/+\z}, "")
    method.empty? ? nil : method
  end

  def handle_unary_request(req, res, method)
    params = parse_body(req.body)

    case method
    when "echo.v1.Echo/Ping"
      res.status = 200
      res["Content-Type"] = "application/json"
      res.body = rpc_result_json("h1", params)
    else
      res.status = 404
      res["Content-Type"] = "application/json"
      res.body = rpc_error_json("h0", 5, %(method "#{method}" not found))
    end
  end

  def handle_sse_request(req, res, method)
    res.status = 200
    res["Content-Type"] = "text/event-stream"

    case method
    when "build.v1.Build/Watch"
      params = parse_body(req.body)
      project = params["project"]
      res.body = [
        sse_event("message", "1", rpc_result_json("h1", "status" => "building", "project" => project)),
        sse_event("message", "2", rpc_result_json("h1", "status" => "done", "project" => project)),
        sse_event("done", nil, nil)
      ].join
    when "build.v1.Build/WatchQuery"
      project = req.query["project"]
      res.body = [
        sse_event("message", "1", rpc_result_json("h2", "status" => "watching", "project" => project)),
        sse_event("done", nil, nil)
      ].join
    else
      res.status = 404
      res["Content-Type"] = "application/json"
      res.body = rpc_error_json("h0", 5, %(method "#{method}" not found))
    end
  end

  def parse_body(body)
    text = body.to_s.strip
    return {} if text.empty?

    JSON.parse(text)
  end

  def rpc_result_json(id, result)
    JSON.generate(
      "jsonrpc" => "2.0",
      "id" => id,
      "result" => result
    )
  end

  def rpc_error_json(id, code, message)
    JSON.generate(
      "jsonrpc" => "2.0",
      "id" => id,
      "error" => {
        "code" => code,
        "message" => message
      }
    )
  end

  def sse_event(event, id, data)
    lines = ["event: #{event}"]
    lines << "id: #{id}" unless id.nil? || id.empty?
    if data.nil?
      lines << "data:"
    else
      lines << "data: #{data}"
    end
    lines << ""
    lines.join("\n") + "\n"
  end
end
