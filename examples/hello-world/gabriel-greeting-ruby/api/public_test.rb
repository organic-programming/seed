# frozen_string_literal: true

require "minitest/autorun"
require_relative "../support"
require "v1/greeting_pb"
require_relative "public"

class GreetingPublicTest < Minitest::Test
  def test_list_languages_includes_english
    response = GabrielGreetingRuby::Api::Public.list_languages(
      Greeting::V1::ListLanguagesRequest.new
    )

    assert_equal 56, response.languages.length
    english = response.languages.find { |language| language.code == "en" }
    refute_nil english
    assert_equal "English", english.name
    assert_equal "English", english.native
  end

  def test_say_hello_uses_requested_language
    response = GabrielGreetingRuby::Api::Public.say_hello(
      Greeting::V1::SayHelloRequest.new(name: "Bob", lang_code: "fr")
    )

    assert_equal "Bonjour Bob", response.greeting
    assert_equal "French", response.language
    assert_equal "fr", response.lang_code
  end

  def test_say_hello_uses_localized_default_name
    response = GabrielGreetingRuby::Api::Public.say_hello(
      Greeting::V1::SayHelloRequest.new(lang_code: "ja")
    )

    assert_equal "こんにちは、マリアさん", response.greeting
    assert_equal "Japanese", response.language
    assert_equal "ja", response.lang_code
  end

  def test_say_hello_falls_back_to_english
    response = GabrielGreetingRuby::Api::Public.say_hello(
      Greeting::V1::SayHelloRequest.new(lang_code: "unknown")
    )

    assert_equal "Hello Mary", response.greeting
    assert_equal "English", response.language
    assert_equal "en", response.lang_code
  end
end
