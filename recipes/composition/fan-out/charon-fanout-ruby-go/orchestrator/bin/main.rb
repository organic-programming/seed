#!/usr/bin/env ruby
# frozen_string_literal: true

script = ENV["CHARON_RUN_SCRIPT"]
script = File.expand_path("../scripts/run.sh", __dir__) if script.nil? || script.empty?
exec "/bin/sh", script
