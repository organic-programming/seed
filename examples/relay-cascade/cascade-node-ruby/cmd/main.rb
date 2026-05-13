#!/usr/bin/env ruby
# frozen_string_literal: true

require_relative "../api/cli"

exit CascadeNodeRuby::Api::Cli.main(ARGV)

