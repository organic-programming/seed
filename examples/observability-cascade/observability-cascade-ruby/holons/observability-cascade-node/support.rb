# frozen_string_literal: true

require "pathname"

begin
  if ENV["OP_SDK_RUBY_PATH"] && !ENV["OP_SDK_RUBY_PATH"].empty?
    prebuilt_bundle = File.join(ENV["OP_SDK_RUBY_PATH"], "vendor", "bundle")
    if Dir.exist?(prebuilt_bundle)
      ENV["BUNDLE_PATH"] ||= prebuilt_bundle
      ENV["BUNDLE_DISABLE_SHARED_GEMS"] ||= "true"
    end
  end
  require "bundler/setup"
rescue LoadError
  nil
end

module CascadeNodeRuby
  ROOT = File.expand_path(__dir__)
  GENERATED_ROOT = File.expand_path("gen", ROOT)
  GEN_ROOT = File.expand_path("gen/ruby", ROOT)

  def self.find_repo_root(start)
    current = Pathname.new(start).expand_path
    loop do
      return current.to_s if current.join("sdk", "ruby-holons", "lib").directory?

      parent = current.parent
      raise "could not locate repository root" if parent == current

      current = parent
    end
  end

  SDK_LIB = File.join(find_repo_root(ROOT), "sdk", "ruby-holons", "lib")

  class << self
    def ensure_load_paths
      [ROOT, GENERATED_ROOT, GEN_ROOT, SDK_LIB].each do |path|
        $LOAD_PATH.unshift(path) unless $LOAD_PATH.include?(path)
      end
    end
  end
end

CascadeNodeRuby.ensure_load_paths
