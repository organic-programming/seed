# frozen_string_literal: true

require "json"
require "minitest/autorun"
require "stringio"
require_relative "../support"
require_relative "cli"

class GreetingCliTest < Minitest::Test
  def test_run_cli_version
    stdout = StringIO.new
    stderr = StringIO.new

    code = GabrielGreetingRuby::Api::Cli.run_cli(["version"], stdout: stdout, stderr: stderr)

    assert_equal 0, code
    assert_equal GabrielGreetingRuby::Api::Cli::VERSION, stdout.string.strip
    assert_equal "", stderr.string
  end

  def test_run_cli_help
    stdout = StringIO.new
    stderr = StringIO.new

    code = GabrielGreetingRuby::Api::Cli.run_cli(["help"], stdout: stdout, stderr: stderr)

    assert_equal 0, code
    assert_includes stdout.string, "usage: gabriel-greeting-ruby"
    assert_includes stdout.string, "listLanguages"
    assert_equal "", stderr.string
  end

  def test_run_cli_list_languages_json
    stdout = StringIO.new
    stderr = StringIO.new

    code = GabrielGreetingRuby::Api::Cli.run_cli(
      ["listLanguages", "--format", "json"],
      stdout: stdout,
      stderr: stderr
    )

    assert_equal 0, code
    payload = JSON.parse(stdout.string)
    assert_equal 56, payload.fetch("languages").length
    assert_equal "en", payload.fetch("languages").first.fetch("code")
    assert_equal "", stderr.string
  end

  def test_run_cli_say_hello_text
    stdout = StringIO.new
    stderr = StringIO.new

    code = GabrielGreetingRuby::Api::Cli.run_cli(
      ["sayHello", "Alice", "fr"],
      stdout: stdout,
      stderr: stderr
    )

    assert_equal 0, code
    assert_equal "Bonjour Alice", stdout.string.strip
    assert_equal "", stderr.string
  end

  def test_run_cli_say_hello_defaults_to_english_json
    stdout = StringIO.new
    stderr = StringIO.new

    code = GabrielGreetingRuby::Api::Cli.run_cli(["sayHello", "--json"], stdout: stdout, stderr: stderr)

    assert_equal 0, code
    payload = JSON.parse(stdout.string)
    assert_equal "Hello Mary", payload.fetch("greeting")
    assert_equal "English", payload.fetch("language")
    assert_equal "en", payload.fetch("langCode")
    assert_equal "", stderr.string
  end
end
