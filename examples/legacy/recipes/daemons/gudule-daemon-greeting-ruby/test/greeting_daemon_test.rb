# frozen_string_literal: true

require "minitest/autorun"

$LOAD_PATH.unshift(File.expand_path("../lib", __dir__))

require "greeting_daemon"

class GreetingDaemonTest < Minitest::Test
  def setup
    @service = Gudule::GreetingDaemon::GreetingService.new
  end

  def test_list_languages_returns_full_catalog
    response = @service.list_languages(Greeting::V1::ListLanguagesRequest.new, nil)

    assert_equal 56, response.languages.length
    assert_equal "en", response.languages.first.code
    assert_equal "zu", response.languages.last.code
  end

  def test_say_hello_uses_requested_language
    response = @service.say_hello(
      Greeting::V1::SayHelloRequest.new(name: "Bob", lang_code: "fr"),
      nil
    )

    assert_equal "Bonjour, Bob", response.greeting
    assert_equal "French", response.language
    assert_equal "fr", response.lang_code
  end

  def test_say_hello_falls_back_to_english_and_world
    response = @service.say_hello(
      Greeting::V1::SayHelloRequest.new(name: "   ", lang_code: "unknown"),
      nil
    )

    assert_equal "Hello, World", response.greeting
    assert_equal "English", response.language
    assert_equal "en", response.lang_code
  end
end
