# frozen_string_literal: true

require "minitest/autorun"
require_relative "../lib/holons"
require_relative "../lib/holons/observability"

class ObservabilityTest < Minitest::Test
  def test_parse_op_obs_drops_v2_tokens
    all = Set[:logs, :metrics, :events, :prom]
    assert_equal all, Holons::Observability.parse_op_obs("all,otel")
    assert_equal all, Holons::Observability.parse_op_obs("all,sessions")
  end

  def test_check_env_rejects_v2_tokens_and_op_sessions
    assert_raises(Holons::Observability::InvalidTokenError) do
      Holons::Observability.check_env("OP_OBS" => "logs,otel")
    end
    assert_raises(Holons::Observability::InvalidTokenError) do
      Holons::Observability.check_env("OP_OBS" => "logs,sessions")
    end
    assert_raises(Holons::Observability::InvalidTokenError) do
      Holons::Observability.check_env("OP_SESSIONS" => "metrics")
    end
  end
end
