# frozen_string_literal: true

require_relative "../support"
require "holons"
require "relay/v1/relay_pb"

module CascadeNodeRuby
  module Api
    module Public
      class << self
        def tick(request)
          obs = Holons::Observability.current
          slug = responder_slug(obs)
          uid = obs.cfg.instance_uid
          obs.logger("tick").info(
            "tick received",
            "sender" => request.sender,
            "note" => request.note,
            "responder_slug" => slug,
            "responder_uid" => uid
          )
          counter = obs.counter(
            "cascade_ticks_total",
            "Ticks received by this cascade node.",
            "responder_uid" => uid
          )
          counter&.inc
          Relay::V1::TickResponse.new(
            responder_slug: slug,
            responder_instance_uid: uid
          )
        end

        def responder_slug(obs)
          configured = obs.cfg.slug.to_s.strip
          return configured unless configured.empty?

          File.basename($PROGRAM_NAME.to_s).empty? ? "observability-cascade-node-ruby" : File.basename($PROGRAM_NAME.to_s)
        end
      end
    end
  end
end

