# frozen_string_literal: true

require_relative "../support"
require "holons"
require_relative "../internal/server"

module CascadeNodeRuby
  module Api
    module Cli
      VERSION = "observability-cascade-ruby-node {{ .Version }}"

      class << self
        def main(argv = ARGV, stdout: $stdout, stderr: $stderr)
          run_cli(Array(argv), stdout: stdout, stderr: stderr)
        end

        def run_cli(args, stdout: $stdout, stderr: $stderr)
          if args.empty?
            print_usage(stderr)
            return 1
          end

          case canonical_command(args[0])
          when "serve"
            run_serve(args.drop(1), stderr)
          when "version"
            stdout.puts(VERSION)
            0
          when "help"
            print_usage(stdout)
            0
          else
            stderr.puts(%(unknown command "#{args[0]}"))
            print_usage(stderr)
            1
          end
        end

        def canonical_command(raw)
          raw.to_s.strip.downcase.gsub(/[-_\s]/, "")
        end

        def print_usage(output)
          output.puts("usage: observability-cascade-ruby-node <command> [args] [flags]")
          output.puts
          output.puts("commands:")
          output.puts("  serve [--listen <uri>] [--transport <name>] [--child <slug>=<binary>]  Start the gRPC server")
          output.puts("  version                                                            Print version and exit")
          output.puts("  help                                                               Print usage")
        end

        private

        def run_serve(args, stderr)
          children, remaining = Holons::Serve.parse_child_flags(args)
          parsed = Holons::Serve.parse_options(remaining)
          Internal::Server.listen_and_serve(
            parsed.listen_uri,
            transport: parsed.transport,
            reflect: parsed.reflect,
            children: children
          )
          0
        rescue StandardError => e
          stderr.puts("serve: #{e.message}")
          1
        end
      end
    end
  end
end
