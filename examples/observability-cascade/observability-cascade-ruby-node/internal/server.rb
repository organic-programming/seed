# frozen_string_literal: true

require_relative "../support"
require "grpc"
require "holons"
require "describe_generated"
require "relay/v1/relay_services_pb"
require_relative "../api/public"

Holons::Describe.use_static_response(Gen::DescribeGenerated.static_describe_response)

module CascadeNodeRuby
  module Internal
    class RelayService < Relay::V1::RelayService::Service
      def tick(request, _call)
        Api::Public.tick(request)
      end
    end

    module Server
      class << self
        def listen_and_serve(listen_uri, reflect: false, members: [], on_listen: nil)
          Holons::Serve.run_with_serve_options(
            normalize_listen_uri(listen_uri),
            proc { |server| register_services(server, include_meta: false) },
            Holons::Serve::ServeOptions.new(
              reflect: reflect,
              member_endpoints: members,
              slug: "observability-cascade-ruby-node"
            ),
            on_listen: on_listen
          )
        end

        def register_services(server, include_meta: true)
          Holons::Describe.register(server) if include_meta
          server.handle(RelayService.new)
        end

        def normalize_listen_uri(listen_uri)
          return "tcp://0.0.0.0:#{Regexp.last_match(1)}" if listen_uri =~ /\Atcp:\/\/:(\d+)\z/

          listen_uri
        end
      end
    end
  end
end

