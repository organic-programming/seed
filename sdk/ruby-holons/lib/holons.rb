# frozen_string_literal: true

lib_root = __dir__
gen_root = File.expand_path("gen", lib_root)
$LOAD_PATH.unshift(gen_root) unless $LOAD_PATH.include?(gen_root)

require "json"

require_relative "holons/transport"
require_relative "holons/observability"
require_relative "holons/serve"
require_relative "holons/identity"
require_relative "holons/discovery_types"
require_relative "holons/discover"
require_relative "holons/connect"
require_relative "holons/describe"
require_relative "holons/holonrpc"

module Holons
end
