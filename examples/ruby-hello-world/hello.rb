# frozen_string_literal: true

# Pure deterministic HelloService.
module HelloService
  def self.greet(name)
    n = name.nil? || name.empty? ? "World" : name
    "Hello, #{n}!"
  end
end

if __FILE__ == $PROGRAM_NAME
  puts HelloService.greet(ARGV.first || "")
end
