# frozen_string_literal: true
# Generated from greeting/v1/greeting.proto.

require "google/protobuf"

Google::Protobuf::DescriptorPool.generated_pool.build do
  add_file("greeting/v1/greeting.proto", syntax: :proto3) do
    add_message "greeting.v1.ListLanguagesRequest" do
    end
    add_message "greeting.v1.ListLanguagesResponse" do
      repeated :languages, :message, 1, "greeting.v1.Language"
    end
    add_message "greeting.v1.Language" do
      optional :code, :string, 1
      optional :name, :string, 2
      optional :native, :string, 3
    end
    add_message "greeting.v1.SayHelloRequest" do
      optional :name, :string, 1
      optional :lang_code, :string, 2
    end
    add_message "greeting.v1.SayHelloResponse" do
      optional :greeting, :string, 1
      optional :language, :string, 2
      optional :lang_code, :string, 3
    end
  end
end

module Greeting
  module V1
    ListLanguagesRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("greeting.v1.ListLanguagesRequest").msgclass
    ListLanguagesResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("greeting.v1.ListLanguagesResponse").msgclass
    Language = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("greeting.v1.Language").msgclass
    SayHelloRequest = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("greeting.v1.SayHelloRequest").msgclass
    SayHelloResponse = ::Google::Protobuf::DescriptorPool.generated_pool.lookup("greeting.v1.SayHelloResponse").msgclass
  end
end
