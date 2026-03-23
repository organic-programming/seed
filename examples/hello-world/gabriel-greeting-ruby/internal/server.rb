# frozen_string_literal: true

require_relative "../support"
require "grpc"
require "holons"
require "describe_generated"
require "v1/greeting_services_pb"
require_relative "../api/public"

Holons::Describe.use_static_response(Gen::DescribeGenerated.static_describe_response)

module GabrielGreetingRuby
  module Internal
    class GreetingService < Greeting::V1::GreetingService::Service
      def list_languages(request, _call)
        Api::Public.list_languages(request)
      end

      def say_hello(request, _call)
        Api::Public.say_hello(request)
      end
    end

    module Server
      class << self
        def listen_and_serve(listen_uri, reflect: false, on_listen: nil)
          Holons::Serve.run_with_options(
            normalize_listen_uri(listen_uri),
            method(:register_services),
            reflect,
            on_listen: on_listen
          )
        end

        def register_services(server)
          server.handle(GreetingService.new)
          Holons::Describe.register(server)
        end

        def normalize_listen_uri(listen_uri)
          return "tcp://0.0.0.0:#{Regexp.last_match(1)}" if listen_uri =~ /\Atcp:\/\/:(\d+)\z/

          listen_uri
        end
      end
    end
  end
end
