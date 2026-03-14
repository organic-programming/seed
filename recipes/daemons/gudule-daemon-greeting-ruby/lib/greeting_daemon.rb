# frozen_string_literal: true

require "grpc"

$LOAD_PATH.unshift(File.expand_path("../../../../sdk/ruby-holons/lib", __dir__))

require "holons"
require_relative "greeting_services_pb"
require_relative "greetings"

module Gudule
  module GreetingDaemon
    VERSION = "v0.4.5"
    BINARY_NAME = "gudule-daemon-greeting-ruby"

    class GreetingService < Greeting::V1::GreetingService::Service
      def list_languages(_request, _call)
        Greeting::V1::ListLanguagesResponse.new(
          languages: GreetingCatalog::GREETINGS.map do |entry|
            Greeting::V1::Language.new(
              code: entry.fetch("code"),
              name: entry.fetch("name"),
              native: entry.fetch("native")
            )
          end
        )
      end

      def say_hello(request, _call)
        greeting = GreetingCatalog.lookup(request.lang_code)
        name = request.name.to_s.strip
        name = "World" if name.empty?

        Greeting::V1::SayHelloResponse.new(
          greeting: format(greeting.fetch("template"), name),
          language: greeting.fetch("name"),
          lang_code: greeting.fetch("code")
        )
      end
    end

    class CLI
      def self.run(argv, stdout: $stdout, stderr: $stderr)
        new(stdout: stdout, stderr: stderr).run(argv)
      end

      def initialize(stdout:, stderr:)
        @stdout = stdout
        @stderr = stderr
      end

      def run(argv)
        case argv.first
        when "serve"
          listen_uri = Holons::Serve.parse_flags(argv.drop(1))
          Holons::Serve.run_with_options(listen_uri, method(:register_services), true)
          0
        when "version"
          @stdout.puts("#{BINARY_NAME} #{VERSION}")
          0
        else
          usage
          1
        end
      end

      private

      def register_services(server)
        server.handle(GreetingService.new)
      end

      def usage
        @stderr.puts("usage: #{BINARY_NAME} <serve|version> [flags]")
      end
    end
  end
end
