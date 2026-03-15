# frozen_string_literal: true
# Generated from hello/v1/hello.proto.

require "google/protobuf"

Google::Protobuf::DescriptorPool.generated_pool.build do
  add_file("hello/v1/hello.proto", syntax: :proto3) do
    add_message "hello.v1.GreetRequest" do
      optional :name, :string, 1
    end

    add_message "hello.v1.GreetResponse" do
      optional :message, :string, 1
    end
  end
end

module Hello
  module V1
    GreetRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("hello.v1.GreetRequest").msgclass
    GreetResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("hello.v1.GreetResponse").msgclass
  end
end
