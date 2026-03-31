# frozen_string_literal: true

module Holons
  LOCAL = 0
  PROXY = 1
  DELEGATED = 2

  SIBLINGS = 0x01
  CWD = 0x02
  SOURCE = 0x04
  BUILT = 0x08
  INSTALLED = 0x10
  CACHED = 0x20
  ALL = 0x3F

  NO_LIMIT = 0
  NO_TIMEOUT = 0

  IdentityInfo = Struct.new(
    :given_name,
    :family_name,
    :motto,
    :aliases,
    keyword_init: true
  )

  HolonInfo = Struct.new(
    :slug,
    :uuid,
    :identity,
    :lang,
    :runner,
    :status,
    :kind,
    :transport,
    :entrypoint,
    :architectures,
    :has_dist,
    :has_source,
    keyword_init: true
  )

  HolonRef = Struct.new(:url, :info, :error, keyword_init: true)
  DiscoverResult = Struct.new(:found, :error, keyword_init: true)
  ResolveResult = Struct.new(:ref, :error, keyword_init: true)
  ConnectResult = Struct.new(:channel, :uid, :origin, :error, keyword_init: true)

  begin
    require "grpc"
    @grpc_load_error = nil
  rescue LoadError => e
    @grpc_load_error = e
  end

  class << self
    def grpc_available?
      @grpc_load_error.nil?
    end

    def grpc_load_error
      @grpc_load_error
    end
  end
end
