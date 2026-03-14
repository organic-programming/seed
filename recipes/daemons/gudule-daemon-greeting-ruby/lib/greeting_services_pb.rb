# frozen_string_literal: true
# Generated from greeting/v1/greeting.proto.

require "grpc"
require_relative "greeting_pb"

module Greeting
  module V1
    module GreetingService
      class Service
        include ::GRPC::GenericService

        self.marshal_class_method = :encode
        self.unmarshal_class_method = :decode
        self.service_name = "greeting.v1.GreetingService"

        rpc :ListLanguages, ::Greeting::V1::ListLanguagesRequest, ::Greeting::V1::ListLanguagesResponse
        rpc :SayHello, ::Greeting::V1::SayHelloRequest, ::Greeting::V1::SayHelloResponse
      end

      Stub = Service.rpc_stub_class
    end
  end
end
