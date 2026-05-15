# frozen_string_literal: true

require_relative "../support"
require "grpc"
require "holons"
require "describe_generated"
require "relay/v1/relay_services_pb"

Holons::Describe.use_static_response(Gen::DescribeGenerated.static_describe_response)

module CascadeNodeRuby
  module Internal
    module Server
      class << self
        def listen_and_serve(listen_uri, transport:, reflect: false, children: [], on_listen: nil)
          Holons::Observability.from_env(
            Holons::Observability::Config.new(slug: "observability-cascade-ruby-node")
          )
          downstream = spawn_downstream(children, transport)
          Holons::Serve.run_with_serve_options(
            normalize_listen_uri(listen_uri),
            proc { |server| register_services(server, downstream&.conn) },
            Holons::Serve::ServeOptions.new(
              reflect: reflect,
              slug: "observability-cascade-ruby-node"
            ),
            on_listen: on_listen
          )
        ensure
          downstream&.stop
        end

        def register_services(server, downstream_channel = nil)
          Holons::Relay.register_server(server, downstream_channel: downstream_channel)
        end

        def spawn_downstream(children, transport)
          specs = Array(children)
          return nil if specs.empty?

          first = specs.first
          Holons::Composite.spawn_member(
            Holons::Composite::SpawnOptions.new(
              slug: first.slug,
              binary_path: first.binary,
              transport: transport,
              downstream_chain: specs.drop(1)
            )
          )
        end

        def normalize_listen_uri(listen_uri)
          return "tcp://0.0.0.0:#{Regexp.last_match(1)}" if listen_uri =~ /\Atcp:\/\/:(\d+)\z/

          listen_uri
        end
      end
    end
  end
end
