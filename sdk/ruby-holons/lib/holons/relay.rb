# frozen_string_literal: true

require "thread"

require_relative "observability"

module Holons
  module Relay
    module_function

    def register_server(server, downstream_channel: nil)
      server.handle(service_class.new(downstream_channel: downstream_channel))
    end

    def service_class
      return @service_class if defined?(@service_class) && @service_class

      require_relay_support!
      @service_class = Class.new(::Relay::V1::RelayService::Service) do
        def initialize(downstream_channel: nil)
          @downstream_channel = downstream_channel
          @received = 0
          @mutex = Mutex.new
        end

        def tick(request, _call)
          count = @mutex.synchronize do
            @received += 1
          end
          obs = Holons::Observability.current
          slug = responder_slug(obs)
          uid = obs.cfg.instance_uid.to_s
          obs.logger("tick").info(
            "tick received",
            "sender" => request.sender,
            "note" => request.note,
            "responder_slug" => slug,
            "responder_uid" => uid
          )
          obs.counter(
            "cascade_ticks_total",
            "Ticks received by this cascade node.",
            "responder_uid" => uid
          )&.inc

          hops = []
          unless @downstream_channel.nil?
            downstream = ::Relay::V1::RelayService::Stub.new(
              "unused",
              :this_channel_is_insecure,
              channel_override: @downstream_channel,
              timeout: 5
            )
            hops.concat(downstream.tick(request).hops)
          end
          hops << ::Relay::V1::HopReceipt.new(slug: slug, uid: uid, received: count)
          ::Relay::V1::TickResponse.new(
            responder_slug: slug,
            responder_instance_uid: uid,
            hops: hops
          )
        end

        private

        def responder_slug(obs)
          configured = obs.cfg.slug.to_s.strip
          return configured unless configured.empty?

          File.basename($PROGRAM_NAME.to_s).sub(/\.exe\z/, "")
        end
      end
    end

    def require_relay_support!
      raise Holons.grpc_load_error unless Holons.grpc_available?

      Holons::Observability.ensure_generated_proto_load_path!
      require "relay/v1/relay_services_pb"
    end

    class << self
      alias_method :RegisterServer, :register_server
    end
  end
end
