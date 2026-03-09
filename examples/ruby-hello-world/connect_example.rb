# frozen_string_literal: true

require "json"
require "tmpdir"
require "fileutils"
require_relative "../../sdk/ruby-holons/lib/holons"

def sdk_echo_server
  path = File.expand_path("../../sdk/ruby-holons/bin/echo-server", __dir__)
  raise "echo-server not found at #{path}" unless File.file?(path)

  path
end

def write_echo_holon(root, binary_path)
  holon_dir = File.join(root, "holons", "echo-server")
  FileUtils.mkdir_p(holon_dir)
  File.write(
    File.join(holon_dir, "holon.yaml"),
    <<~YAML
      uuid: "echo-server-connect-example"
      given_name: "Echo"
      family_name: "Server"
      motto: "Reply precisely."
      composer: "connect-example"
      kind: service
      build:
        runner: ruby
        main: bin/echo-server
      artifacts:
        binary: "#{binary_path}"
    YAML
  )
end

def invoke_ping(channel, payload)
  stub = GRPC::ClientStub.new(
    "unused",
    :this_channel_is_insecure,
    channel_override: channel,
    timeout: 5
  )

  stub.request_response(
    "/echo.v1.Echo/Ping",
    payload,
    ->(value) { value },
    ->(response) { response },
    deadline: Time.now + 5
  )
end

abort("grpc gem is unavailable in this Ruby environment") unless Holons.grpc_available?

Dir.mktmpdir("ruby-holons-connect-") do |root|
  write_echo_holon(root, sdk_echo_server)

  Dir.chdir(root) do
    channel = Holons.connect("echo-server")
    begin
      puts invoke_ping(channel, JSON.generate({ message: "hello-from-ruby" }))
    ensure
      Holons.disconnect(channel)
    end
  end
end
