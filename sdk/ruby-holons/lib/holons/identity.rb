# frozen_string_literal: true

module Holons
  # Parsed identity from a holon.proto manifest file.
  HolonIdentity = Struct.new(
    :uuid, :given_name, :family_name, :motto, :composer,
    :clade, :status, :born, :lang, :parents, :reproduction, :generated_by,
    :proto_status, :aliases,
    keyword_init: true
  ) do
    def slug
      given = given_name.to_s.strip
      family = family_name.to_s.strip.sub(/\?\z/, "")
      return "" if given.empty? && family.empty?

      "#{given}-#{family}".strip.downcase.tr(" ", "-").gsub(/\A-+|-+\z/, "")
    end
  end
  ResolvedManifest = Struct.new(
    :identity, :kind, :build_runner, :build_main, :artifact_binary, :artifact_primary, :source_path,
    keyword_init: true
  )

  module Identity
    PROTO_MANIFEST_FILE_NAME = "holon.proto"

    def self.parse(path)
      parse_holon(path)
    end

    def self.resolve(root)
      resolve_proto_file(resolve_manifest_path(root))
    end

    def self.resolve_manifest(root)
      resolved = resolve(root)
      [resolved.identity, resolved.source_path]
    end

    def self.resolve_proto_file(path)
      expanded = File.expand_path(path.to_s)
      unless File.basename(expanded) == PROTO_MANIFEST_FILE_NAME
        raise "#{expanded} is not a #{PROTO_MANIFEST_FILE_NAME} file"
      end

      parse_manifest(expanded)
    end

    # Parse a holon.proto manifest file.
    def self.parse_holon(path)
      parse_manifest(path).identity
    end

    def self.parse_manifest(path)
      expanded = File.expand_path(path.to_s)
      text = File.read(expanded)
      manifest_block = extract_manifest_block(text)
      raise "#{expanded}: missing holons.v1.manifest option in holon.proto" if manifest_block.to_s.empty?

      identity_block = extract_named_block(manifest_block, "identity")
      lineage_block = extract_named_block(manifest_block, "lineage")
      build_block = extract_named_block(manifest_block, "build")
      artifacts_block = extract_named_block(manifest_block, "artifacts")

      ResolvedManifest.new(
        identity: HolonIdentity.new(
          uuid: extract_string(identity_block, "uuid"),
          given_name: extract_string(identity_block, "given_name"),
          family_name: extract_string(identity_block, "family_name"),
          motto: extract_string(identity_block, "motto"),
          composer: extract_string(identity_block, "composer"),
          clade: extract_string(identity_block, "clade"),
          status: extract_string(identity_block, "status"),
          born: extract_string(identity_block, "born"),
          lang: extract_string(manifest_block, "lang"),
          parents: extract_list(lineage_block, "parents"),
          reproduction: extract_string(lineage_block, "reproduction"),
          generated_by: extract_string(lineage_block, "generated_by"),
          proto_status: extract_string(identity_block, "proto_status"),
          aliases: extract_list(identity_block, "aliases")
        ),
        kind: extract_string(manifest_block, "kind"),
        build_runner: extract_string(build_block, "runner"),
        build_main: extract_string(build_block, "main"),
        artifact_binary: extract_string(artifacts_block, "binary"),
        artifact_primary: extract_string(artifacts_block, "primary"),
        source_path: expanded
      )
    end

    def self.find_holon_proto(root)
      expanded = File.expand_path(root.to_s)
      return expanded if File.file?(expanded) && File.basename(expanded) == PROTO_MANIFEST_FILE_NAME
      return nil unless File.directory?(expanded)

      direct = File.join(expanded, PROTO_MANIFEST_FILE_NAME)
      return direct if File.file?(direct)

      api_v1 = File.join(expanded, "api", "v1", PROTO_MANIFEST_FILE_NAME)
      return api_v1 if File.file?(api_v1)

      Dir.glob(File.join(expanded, "**", PROTO_MANIFEST_FILE_NAME)).sort.first
    end

    def self.resolve_manifest_path(root)
      expanded = File.expand_path(root.to_s)
      search_roots = [expanded]
      parent = File.dirname(expanded)
      if File.basename(expanded) == "protos"
        search_roots << parent
      elsif !search_roots.include?(parent)
        search_roots << parent
      end

      search_roots.each do |candidate_root|
        candidate = find_holon_proto(candidate_root)
        return candidate unless candidate.nil?
      end

      raise "no holon.proto found near #{expanded}"
    end

    def self.extract_manifest_block(text)
      match = text.match(/option\s*\(\s*holons\.v1\.manifest\s*\)\s*=\s*\{/m)
      return "" if match.nil?

      brace_index = text.index("{", match.begin(0))
      return "" if brace_index.nil?

      extract_braced_block(text, "{", brace_index)
    end

    def self.extract_named_block(text, field_name)
      match = text.match(/^\s*#{Regexp.escape(field_name)}\s*:\s*\{/m)
      return "" if match.nil?

      extract_braced_block(text, "{", match.begin(0) + match[0].index("{"))
    end

    def self.extract_string(text, field_name)
      match = text.match(/^\s*#{Regexp.escape(field_name)}\s*:\s*"([^"]*)"/m)
      return "" if match.nil?

      match[1].to_s
    end

    def self.extract_list(text, field_name)
      match = text.match(/^\s*#{Regexp.escape(field_name)}\s*:\s*\[(.*?)\]/m)
      return [] if match.nil?

      match[1].scan(/"((?:[^"\\]|\\.)*)"|([^\s,\]]+)/).map do |quoted, bare|
        value = quoted.to_s.empty? ? bare.to_s : quoted.to_s
        value.gsub("\\\"", "\"").gsub("\\\\", "\\")
      end
    end

    def self.extract_braced_block(text, needle, offset = nil)
      start = offset || text.index(needle)
      raise "#{needle} not found" if start.nil?

      brace_start = offset || text.index("{", start)
      raise "opening brace not found after #{needle}" if brace_start.nil?

      depth = 0
      in_string = false
      escaped = false

      text.each_char.with_index do |char, index|
        next if index < brace_start

        if in_string
          if escaped
            escaped = false
          elsif char == "\\"
            escaped = true
          elsif char == "\""
            in_string = false
          end
          next
        end

        case char
        when "\""
          in_string = true
        when "{"
          depth += 1
        when "}"
          depth -= 1
          return text[(brace_start + 1)...index] if depth.zero?
        end
      end

      raise "unterminated brace block for #{needle}"
    end
  end
end
