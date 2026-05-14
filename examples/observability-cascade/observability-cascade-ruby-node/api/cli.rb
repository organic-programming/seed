# frozen_string_literal: true

require "json"
require_relative "../support"
require "holons"
require "relay/v1/relay_pb"
require_relative "public"
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
          when "tick"
            run_tick(args.drop(1), stdout, stderr)
          else
            stderr.puts(%(unknown command "#{args[0]}"))
            print_usage(stderr)
            1
          end
        end

        def run_tick(args, stdout, stderr)
          request = Relay::V1::TickRequest.new
          positional = []
          index = 0
          while index < args.length
            arg = args[index]
            case arg
            when "--sender"
              index += 1
              raise ArgumentError, "--sender requires a value" if index >= args.length

              request.sender = args[index].to_s
            when /\A--sender=(.+)\z/
              request.sender = Regexp.last_match(1).to_s
            when "--note"
              index += 1
              raise ArgumentError, "--note requires a value" if index >= args.length

              request.note = args[index].to_s
            when /\A--note=(.+)\z/
              request.note = Regexp.last_match(1).to_s
            when /\A--/
              raise ArgumentError, %(unknown flag "#{arg}")
            else
              positional << arg
            end
            index += 1
          end
          request.sender = positional[0].to_s if request.sender.empty? && positional[0]
          request.note = positional[1].to_s if request.note.empty? && positional[1]
          response = Public.tick(request)
          stdout.puts(JSON.generate(JSON.parse(response.to_json)))
          0
        rescue StandardError => e
          stderr.puts("tick: #{e.message}")
          1
        end

        def canonical_command(raw)
          raw.to_s.strip.downcase.gsub(/[-_\s]/, "")
        end

        def print_usage(output)
          output.puts("usage: observability-cascade-ruby-node <command> [args] [flags]")
          output.puts
          output.puts("commands:")
          output.puts("  serve [--listen <uri>] [--member <slug>=<address>]  Start the gRPC server")
          output.puts("  tick [sender] [note]                                Emit one local tick")
          output.puts("  version                                             Print version and exit")
          output.puts("  help                                                Print usage")
        end

        private

        def run_serve(args, stderr)
          members = parse_members(args)
          parsed = Holons::Serve.parse_options(args)
          Internal::Server.listen_and_serve(parsed.listen_uri, reflect: parsed.reflect, members: members)
          0
        rescue StandardError => e
          stderr.puts("serve: #{e.message}")
          1
        end

        def parse_members(args)
          members = []
          index = 0
          while index < args.length
            arg = args[index]
            if arg == "--member"
              index += 1
              raise ArgumentError, "--member requires <slug>=<address>" if index >= args.length

              members << parse_member(args[index])
            elsif arg =~ /\A--member=(.+)\z/
              members << parse_member(Regexp.last_match(1))
            end
            index += 1
          end
          members
        end

        def parse_member(raw)
          slug, separator, address = raw.to_s.partition("=")
          raise ArgumentError, "--member requires <slug>=<address>" if separator.empty?

          slug = slug.strip
          address = address.strip
          raise ArgumentError, "--member requires non-empty slug and address" if slug.empty? || address.empty?

          Holons::Serve::MemberRef.new(slug: slug, address: address)
        end
      end
    end
  end
end

