# frozen_string_literal: true

require "minitest/autorun"
require_relative "hello"

class HelloServiceTest < Minitest::Test
  def test_greet_with_name
    assert_equal "Hello, Alice!", HelloService.greet("Alice")
  end

  def test_greet_default
    assert_equal "Hello, World!", HelloService.greet("")
  end

  def test_greet_nil
    assert_equal "Hello, World!", HelloService.greet(nil)
  end
end
