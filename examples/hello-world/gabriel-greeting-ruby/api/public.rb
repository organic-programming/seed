# frozen_string_literal: true

require_relative "../support"
require "v1/greeting_pb"
require_relative "../internal/greetings"

module GabrielGreetingRuby
  module Api
    module Public
      class << self
        def list_languages(_request)
          Greeting::V1::ListLanguagesResponse.new(
            languages: Internal::GREETINGS.map do |greeting|
              Greeting::V1::Language.new(
                code: greeting.lang_code,
                name: greeting.lang_english,
                native: greeting.lang_native
              )
            end
          )
        end

        def say_hello(request)
          greeting = Internal.lookup(request.lang_code)
          subject = request.name.to_s.strip
          subject = greeting.default_name if subject.empty?

          Greeting::V1::SayHelloResponse.new(
            greeting: format(greeting.template, subject),
            language: greeting.lang_english,
            lang_code: greeting.lang_code
          )
        end
      end
    end
  end
end
