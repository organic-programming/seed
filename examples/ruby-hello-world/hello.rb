# frozen_string_literal: true

module HelloService
  class << self
    def greet(name)
      value = name.to_s.strip
      value = "World" if value.empty?
      "Hello, #{value}!"
    end

    def register(server)
      load_runtime!
      server.handle(Server.new)
    end

    def serve(args)
      load_runtime!
      listen_uri = Holons::Serve.parse_flags(args)
      Holons::Serve.run_with_options(listen_uri, method(:register), true)
    end

    private

    def load_runtime!
      return if @runtime_loaded

      require_relative "../../sdk/ruby-holons/lib/holons"
      require_relative "hello_pb"
      require_relative "hello_services_pb"

      unless const_defined?(:Server, false)
        const_set(:Server, Class.new(::Hello::V1::HelloService::Service) do
          def greet(request, _call)
            ::Hello::V1::GreetResponse.new(message: ::HelloService.greet(request.name))
          end
        end)
      end

      @runtime_loaded = true
    end
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
