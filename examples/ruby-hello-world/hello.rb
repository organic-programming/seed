# frozen_string_literal: true

require "json"
require_relative "../../sdk/ruby-holons/lib/holons"

# Pure deterministic HelloService.
module HelloService
  def self.greet(name)
    n = name.nil? || name.empty? ? "World" : name
    "Hello, #{n}!"
  end

  def self.serve(args)
    listen_uri = Holons::Serve.parse_flags(args)
    warn("ruby-hello-world listening on #{listen_uri}")
    puts(JSON.generate({ message: greet("") }))
  end
end

if __FILE__ == $PROGRAM_NAME
  args = ARGV.dup
  if args.first == "serve"
    HelloService.serve(args.drop(1))
  else
    puts HelloService.greet(args.first || "")
  end
end
