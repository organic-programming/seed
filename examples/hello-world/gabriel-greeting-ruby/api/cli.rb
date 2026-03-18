# frozen_string_literal: true

require "json"
require_relative "../support"
require "holons"
require "v1/greeting_pb"
require_relative "public"
require_relative "../internal/server"

module GabrielGreetingRuby
  module Api
    module Cli
      VERSION = "gabriel-greeting-ruby {{ .Version }}"
      CommandOptions = Struct.new(:format, :lang, keyword_init: true)

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
          when "listlanguages"
            run_list_languages(args.drop(1), stdout, stderr)
          when "sayhello"
            run_say_hello(args.drop(1), stdout, stderr)
          else
            stderr.puts(%(unknown command "#{args[0]}"))
            print_usage(stderr)
            1
          end
        end

        def run_list_languages(args, stdout, stderr)
          options, positional = parse_command_options(args)
          if positional.any?
            stderr.puts("listLanguages: accepts no positional arguments")
            return 1
          end

          response = Public.list_languages(Greeting::V1::ListLanguagesRequest.new)
          write_response(stdout, response, options.format)
          0
        rescue StandardError => e
          stderr.puts("listLanguages: #{e.message}")
          1
        end

        def run_say_hello(args, stdout, stderr)
          options, positional = parse_command_options(args)
          if positional.length > 2
            stderr.puts("sayHello: accepts at most <name> [lang_code]")
            return 1
          end

          request = Greeting::V1::SayHelloRequest.new(lang_code: "en")
          request.name = positional[0] if positional[0]

          if positional[1]
            if !options.lang.to_s.empty?
              stderr.puts("sayHello: use either a positional lang_code or --lang, not both")
              return 1
            end
            request.lang_code = positional[1]
          elsif !options.lang.to_s.empty?
            request.lang_code = options.lang
          end

          response = Public.say_hello(request)
          write_response(stdout, response, options.format)
          0
        rescue StandardError => e
          stderr.puts("sayHello: #{e.message}")
          1
        end

        def parse_command_options(args)
          options = CommandOptions.new(format: "text", lang: "")
          positional = []
          index = 0

          while index < args.length
            arg = args[index]
            case arg
            when "--json"
              options.format = "json"
            when "--format"
              index += 1
              raise ArgumentError, "--format requires a value" if index >= args.length

              options.format = parse_output_format(args[index])
            when /\A--format=(.+)\z/
              options.format = parse_output_format(Regexp.last_match(1))
            when "--lang"
              index += 1
              raise ArgumentError, "--lang requires a value" if index >= args.length

              options.lang = args[index].to_s.strip
            when /\A--lang=(.+)\z/
              options.lang = Regexp.last_match(1).to_s.strip
            when /\A--/
              raise ArgumentError, %(unknown flag "#{arg}")
            else
              positional << arg
            end
            index += 1
          end

          [options, positional]
        end

        def parse_output_format(raw)
          normalized = raw.to_s.strip.downcase
          return "text" if normalized.empty? || normalized == "text" || normalized == "txt"
          return "json" if normalized == "json"

          raise ArgumentError, %(unsupported format "#{raw}")
        end

        def write_response(stdout, message, output_format)
          case output_format
          when "json"
            stdout.puts(JSON.pretty_generate(JSON.parse(message.to_json)))
          when "text"
            write_text(stdout, message)
          else
            raise ArgumentError, %(unsupported format "#{output_format}")
          end
        end

        def write_text(stdout, message)
          case message
          when Greeting::V1::SayHelloResponse
            stdout.puts(message.greeting)
          when Greeting::V1::ListLanguagesResponse
            message.languages.each do |language|
              stdout.puts([language.code, language.name, language.native].join("\t"))
            end
          else
            raise ArgumentError, "unsupported text output for #{message.class}"
          end
        end

        def canonical_command(raw)
          raw.to_s.strip.downcase.gsub(/[-_\s]/, "")
        end

        def print_usage(output)
          output.puts("usage: gabriel-greeting-ruby <command> [args] [flags]")
          output.puts
          output.puts("commands:")
          output.puts("  serve [--listen <uri>]                    Start the gRPC server")
          output.puts("  version                                   Print version and exit")
          output.puts("  help                                      Print usage")
          output.puts("  listLanguages [--format text|json]        List supported languages")
          output.puts("  sayHello [name] [lang_code] [--format text|json] [--lang <code>]")
          output.puts
          output.puts("examples:")
          output.puts("  gabriel-greeting-ruby serve --listen stdio://")
          output.puts("  gabriel-greeting-ruby listLanguages --format json")
          output.puts("  gabriel-greeting-ruby sayHello Alice fr")
          output.puts("  gabriel-greeting-ruby sayHello Alice --lang fr --format json")
        end

        private

        def run_serve(args, stderr)
          listen_uri = Holons::Serve.parse_flags(args)
          Internal::Server.listen_and_serve(listen_uri)
          0
        rescue StandardError => e
          stderr.puts("serve: #{e.message}")
          1
        end
      end
    end
  end
end
