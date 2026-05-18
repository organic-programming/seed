# frozen_string_literal: true

require_relative "../support"
require "grpc"
require "holons"
require "describe_generated"
require "v1/greeting_services_pb"
require_relative "../api/public"
require_relative "greetings"

Holons::Describe.use_static_response(Gen::DescribeGenerated.static_describe_response)

module GabrielGreetingRuby
  module Internal
    class GreetingService < Greeting::V1::GreetingService::Service
      def list_languages(request, _call)
        Api::Public.list_languages(request)
      end

      def say_hello(request, _call)
        start_time = Process.clock_gettime(Process::CLOCK_MONOTONIC, :nanosecond)
        response = Api::Public.say_hello(request)
        name = request.name.to_s.strip
        name = Internal.lookup(response.lang_code).default_name if name.empty?
        transport = Holons.current_transport.to_s
        transport = "unknown" if transport.empty?
        duration_ns = Process.clock_gettime(Process::CLOCK_MONOTONIC, :nanosecond) - start_time
        message = "Greeted #{name} in #{response.language} (#{response.lang_code})"
        obs = Holons::Observability.current
        obs.logger("greeting").info(
          message,
          {
            "lang_code" => response.lang_code,
            "language" => response.language,
            "name" => name,
            "greeting" => response.greeting,
            "transport" => transport,
            "duration_ns" => duration_ns
          }
        )
        obs.counter(
          "greeting_emitted_total",
          "Greetings emitted, partitioned by language and transport.",
          {
            "lang_code" => response.lang_code,
            "language" => response.language,
            "transport" => transport
          }
        )&.inc
        response
      end
    end

    module Server
      class << self
        def listen_and_serve(listen_uri, reflect: false, on_listen: nil)
          Holons::Serve.run_with_serve_options(
            normalize_listen_uri(listen_uri),
            proc { |server| register_services(server, include_meta: false) },
            Holons::Serve::ServeOptions.new(
              reflect: reflect,
              slug: "gabriel-greeting-ruby"
            ),
            on_listen: on_listen
          )
        end

        def register_services(server, include_meta: true)
          Holons::Describe.register(server) if include_meta
          server.handle(GreetingService.new)
        end

        def normalize_listen_uri(listen_uri)
          return "tcp://0.0.0.0:#{Regexp.last_match(1)}" if listen_uri =~ /\Atcp:\/\/:(\d+)\z/

          listen_uri
        end
      end
    end
  end
end
