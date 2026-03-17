# frozen_string_literal: true

begin
  require "bundler/setup"
rescue LoadError
  nil
end

module GabrielGreetingRuby
  ROOT = File.expand_path(__dir__)
  SDK_LIB = File.expand_path("../../../sdk/ruby-holons/lib", ROOT)
  GEN_ROOT = File.expand_path("gen/ruby/greeting", ROOT)
  SHARED_PROTO_ROOT = File.expand_path("../../_protos", ROOT)
  HOLON_PROTO_PATH = File.expand_path("api/v1/holon.proto", ROOT)

  class << self
    def ensure_load_paths
      [ROOT, GEN_ROOT, SDK_LIB].each do |path|
        $LOAD_PATH.unshift(path) unless $LOAD_PATH.include?(path)
      end
    end
  end
end

GabrielGreetingRuby.ensure_load_paths
