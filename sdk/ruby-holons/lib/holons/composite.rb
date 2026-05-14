# frozen_string_literal: true

module Holons
  module Composite
    module_function

    def member(id)
      executable = ENV.fetch("OP_HOLON_EXECUTABLE", "").strip
      executable = $PROGRAM_NAME if executable.empty?
      member_from_executable(executable, id)
    end

    def member_from_executable(executable, id)
      raise ArgumentError, "member id is required" if id.to_s.strip.empty?

      member_dir = File.join(File.dirname(File.expand_path(executable.to_s)), "holons", id.to_s)
      raise Errno::ENOENT, member_dir unless File.directory?(member_dir)

      match = Dir.children(member_dir).sort.find do |name|
        path = File.join(member_dir, name)
        File.file?(path) && (File.executable?(path) || name.end_with?(".exe"))
      end
      raise "no executable found in #{member_dir}" if match.nil?

      File.join(member_dir, match)
    end
  end
end
