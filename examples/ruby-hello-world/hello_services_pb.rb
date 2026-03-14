# frozen_string_literal: true
# Generated from hello/v1/hello.proto.

require "grpc"
require_relative "hello_pb"

module Hello
  module V1
    module HelloService
      class Service
        include ::GRPC::GenericService

        self.marshal_class_method = :encode
        self.unmarshal_class_method = :decode
        self.service_name = "hello.v1.HelloService"

        rpc :Greet, ::Hello::V1::GreetRequest, ::Hello::V1::GreetResponse
      end

      Stub = Service.rpc_stub_class
    end
  end
end
