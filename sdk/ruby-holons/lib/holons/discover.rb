# frozen_string_literal: true

require "pathname"

module Holons
  HolonBuild = Struct.new(:runner, :main, keyword_init: true)
  HolonArtifacts = Struct.new(:binary, :primary, keyword_init: true)
  HolonManifest = Struct.new(:kind, :build, :artifacts, keyword_init: true)
  HolonEntry = Struct.new(
    :slug, :uuid, :dir, :relative_path, :origin, :identity, :manifest,
    keyword_init: true
  )

  class << self
    def discover(root)
      discover_in_root(root, "local")
    end

    def discover_local
      discover(Dir.pwd)
    end

    def discover_all
      entries = []
      seen = {}

      [[Dir.pwd, "local"], [opbin, "$OPBIN"], [cache_dir, "cache"]].each do |root, origin|
        discover_in_root(root, origin).each do |entry|
          key = entry.uuid.to_s.strip
          key = entry.dir if key.empty?
          next if seen[key]

          seen[key] = true
          entries << entry
        end
      end

      entries
    end

    def find_by_slug(slug)
      needle = slug.to_s.strip
      return nil if needle.empty?

      match = nil
      discover_all.each do |entry|
        next unless entry.slug == needle

        if !match.nil? && match.uuid != entry.uuid
          raise "ambiguous holon \"#{needle}\""
        end

        match = entry
      end
      match
    end

    def discover_by_slug(slug)
      find_by_slug(slug)
    end

    def find_by_uuid(prefix)
      needle = prefix.to_s.strip
      return nil if needle.empty?

      match = nil
      discover_all.each do |entry|
        next unless entry.uuid.start_with?(needle)

        if !match.nil? && match.uuid != entry.uuid
          raise "ambiguous UUID prefix \"#{needle}\""
        end

        match = entry
      end
      match
    end

    private

    def discover_in_root(root, origin)
      resolved_root = File.expand_path(root.to_s.strip.empty? ? Dir.pwd : root.to_s)
      return [] unless File.directory?(resolved_root)

      entries_by_key = {}
      ordered_keys = []
      scan_dir(resolved_root, resolved_root, origin, entries_by_key, ordered_keys)

      ordered_keys
        .filter { |key| entries_by_key.key?(key) }
        .map { |key| entries_by_key[key] }
        .sort_by { |entry| [entry.relative_path, entry.uuid] }
    end

    def scan_dir(root, dir, origin, entries_by_key, ordered_keys)
      Dir.each_child(dir) do |name|
        child = File.join(dir, name)
        if File.directory?(child)
          next if should_skip_dir?(root, child, name)

          scan_dir(root, child, origin, entries_by_key, ordered_keys)
          next
        end
        next unless manifest_file?(root, child, name)

        begin
          resolved = Identity.parse_manifest(child)
        rescue StandardError
          next
        end

        holon_dir = manifest_root(child)
        entry = HolonEntry.new(
          slug: slug_for(resolved.identity),
          uuid: resolved.identity.uuid.to_s,
          dir: holon_dir,
          relative_path: relative_path(root, holon_dir),
          origin: origin,
          identity: resolved.identity,
          manifest: HolonManifest.new(
            kind: resolved.kind.to_s,
            build: HolonBuild.new(
              runner: resolved.build_runner.to_s,
              main: resolved.build_main.to_s
            ),
            artifacts: HolonArtifacts.new(
              binary: resolved.artifact_binary.to_s,
              primary: resolved.artifact_primary.to_s
            )
          )
        )

        key = entry.uuid.to_s.strip
        key = entry.dir if key.empty?
        if entries_by_key.key?(key)
          existing = entries_by_key[key]
          entries_by_key[key] = entry if path_depth(entry.relative_path) < path_depth(existing.relative_path)
          next
        end

        entries_by_key[key] = entry
        ordered_keys << key
      end
    rescue Errno::ENOENT, Errno::EACCES
      nil
    end

    def slug_for(identity)
      identity.slug
    end

    def should_skip_dir?(root, dir, name)
      return false if File.expand_path(dir) == File.expand_path(root)

      %w[.git .op node_modules vendor build].include?(name) || name.start_with?(".")
    end

    def manifest_file?(root, path, name)
      return false unless File.file?(path)
      name == Identity::PROTO_MANIFEST_FILE_NAME
    end

    def manifest_root(path)
      manifest_dir = File.expand_path(File.dirname(path))
      version_dir = File.basename(manifest_dir)
      api_dir = File.basename(File.dirname(manifest_dir))
      if version_dir.match?(/^v[0-9]+[A-Za-z0-9._-]*$/) && api_dir == "api"
        return File.expand_path(File.join(manifest_dir, "..", ".."))
      end
      manifest_dir
    end

    def relative_path(root, dir)
      rel = Pathname.new(dir).relative_path_from(Pathname.new(root)).to_s
      rel.empty? ? "." : rel.tr(File::SEPARATOR, "/")
    rescue ArgumentError
      dir.tr(File::SEPARATOR, "/")
    end

    def path_depth(relative_path)
      trimmed = relative_path.to_s.strip.gsub(%r{\A/+|/+\z}, "")
      return 0 if trimmed.empty? || trimmed == "."

      trimmed.split("/").length
    end

    def op_path
      configured = ENV.fetch("OPPATH", "").strip
      return File.expand_path(configured) unless configured.empty?

      File.expand_path("~/.op")
    end

    def opbin
      configured = ENV.fetch("OPBIN", "").strip
      return File.expand_path(configured) unless configured.empty?

      File.join(op_path, "bin")
    end

    def cache_dir
      File.join(op_path, "cache")
    end
  end

  module Discover
    class << self
      def discover(root)
        Holons.discover(root)
      end

      def discover_local
        Holons.discover_local
      end

      def discover_all
        Holons.discover_all
      end

      def find_by_slug(slug)
        Holons.find_by_slug(slug)
      end

      def discover_by_slug(slug)
        Holons.discover_by_slug(slug)
      end

      def find_by_uuid(prefix)
        Holons.find_by_uuid(prefix)
      end
    end
  end
end
