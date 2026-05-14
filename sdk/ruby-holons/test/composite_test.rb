# frozen_string_literal: true

require "fileutils"
require "minitest/autorun"
require "tmpdir"
require_relative "../lib/holons/composite"

class CompositeTest < Minitest::Test
  def test_member_resolves_executable_relative_to_launcher
    Dir.mktmpdir("ruby-composite-") do |dir|
      launcher = File.join(dir, "bin", "darwin_arm64", "parent")
      member_dir = File.join(File.dirname(launcher), "holons", "ruby-node")
      member = File.join(member_dir, "observability-cascade-ruby-node")
      FileUtils.mkdir_p(member_dir)
      File.write(launcher, "#!/bin/sh\n")
      File.write(member, "#!/bin/sh\n")
      File.chmod(0o755, launcher)
      File.chmod(0o755, member)

      assert_equal member, Holons::Composite.member_from_executable(launcher, "ruby-node")
    end
  end

  def test_member_errors_when_missing
    Dir.mktmpdir("ruby-composite-") do |dir|
      launcher = File.join(dir, "bin", "darwin_arm64", "parent")
      FileUtils.mkdir_p(File.dirname(launcher))
      File.write(launcher, "#!/bin/sh\n")

      assert_raises(Errno::ENOENT) do
        Holons::Composite.member_from_executable(launcher, "ruby-node")
      end
    end
  end
end
