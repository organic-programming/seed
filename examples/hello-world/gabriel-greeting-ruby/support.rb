# frozen_string_literal: true

require "rbconfig"

begin
  if ENV["OP_SDK_RUBY_PATH"] && !ENV["OP_SDK_RUBY_PATH"].empty?
    prebuilt_bundle = File.join(ENV["OP_SDK_RUBY_PATH"], "vendor", "bundle")
    if Dir.exist?(prebuilt_bundle)
      ENV["BUNDLE_PATH"] ||= prebuilt_bundle
      ENV["BUNDLE_DISABLE_SHARED_GEMS"] ||= "true"
      gem_root = File.join(prebuilt_bundle, "ruby", RbConfig::CONFIG["ruby_version"])
      if Dir.exist?(gem_root)
        ENV["GEM_HOME"] ||= gem_root
        ENV["GEM_PATH"] = [gem_root, ENV["GEM_PATH"]].compact.reject(&:empty?).join(File::PATH_SEPARATOR)
        Gem.clear_paths if defined?(Gem)
      end
    end
  end
  require "bundler/setup"
rescue LoadError
  nil
rescue StandardError => e
  raise unless defined?(Bundler::GemNotFound) && e.is_a?(Bundler::GemNotFound)
end

module GabrielGreetingRuby
  ROOT = File.expand_path(__dir__)
  SDK_LIB = File.expand_path("../../../sdk/ruby-holons/lib", ROOT)
  GENERATED_ROOT = File.expand_path("gen", ROOT)
  GEN_ROOT = File.expand_path("gen/ruby/greeting", ROOT)

  class << self
    def ensure_load_paths
      [ROOT, GENERATED_ROOT, GEN_ROOT, SDK_LIB].each do |path|
        $LOAD_PATH.unshift(path) unless $LOAD_PATH.include?(path)
      end
    end
  end
end

GabrielGreetingRuby.ensure_load_paths
