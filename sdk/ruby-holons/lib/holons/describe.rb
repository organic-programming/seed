# frozen_string_literal: true

require "set"
require_relative "identity"

module Holons
  module Describe
    HOLON_META_SERVICE = "holons.v1.HolonMeta"
    SCALAR_TYPES = %w[
      double float int64 uint64 int32 fixed64 fixed32 bool string bytes
      uint32 sfixed32 sfixed64 sint32 sint64
    ].to_set.freeze

    PACKAGE_PATTERN = /^package\s+([A-Za-z0-9_.]+)\s*;$/
    SERVICE_PATTERN = /^service\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?/
    MESSAGE_PATTERN = /^message\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?/
    ENUM_PATTERN = /^enum\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?/
    RPC_PATTERN = /^rpc\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)\s*returns\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)\s*;?/
    MAP_FIELD_PATTERN = /^(repeated\s+)?map\s*<\s*([.A-Za-z0-9_]+)\s*,\s*([.A-Za-z0-9_]+)\s*>\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;/
    FIELD_PATTERN = /^(optional\s+|repeated\s+)?([.A-Za-z0-9_]+)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;/
    ENUM_VALUE_PATTERN = /^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(-?\d+)\s*;/

    module FieldLabel
      UNSPECIFIED = 0
      OPTIONAL = 1
      REPEATED = 2
      MAP = 3
      REQUIRED = 4
    end

    class NoIncodeDescriptionError < StandardError
      def initialize
        super("no Incode Description registered — run op build")
      end
    end

    ErrNoIncodeDescription = NoIncodeDescriptionError

    @static_response = nil
    @grpc_describe_loaded = nil
    @holon_meta_service_class = nil

    class DescribeRequest
    end

    class EnumValueDoc
      attr_reader :name, :number, :description

      def initialize(name:, number:, description:)
        @name = name
        @number = number
        @description = description
      end

      def to_h
        {
          name: name,
          number: number,
          description: description
        }
      end
    end

    class FieldDoc
      attr_reader :name, :type, :number, :description, :label, :map_key_type,
                  :map_value_type, :nested_fields, :enum_values, :required, :example

      def initialize(
        name:,
        type:,
        number:,
        description:,
        label:,
        map_key_type: "",
        map_value_type: "",
        nested_fields: [],
        enum_values: [],
        required: false,
        example: ""
      )
        @name = name
        @type = type
        @number = number
        @description = description
        @label = label
        @map_key_type = map_key_type
        @map_value_type = map_value_type
        @nested_fields = nested_fields
        @enum_values = enum_values
        @required = required
        @example = example
      end

      def to_h
        {
          name: name,
          type: type,
          number: number,
          description: description,
          label: label,
          map_key_type: map_key_type,
          map_value_type: map_value_type,
          nested_fields: nested_fields.map(&:to_h),
          enum_values: enum_values.map(&:to_h),
          required: required,
          example: example
        }
      end
    end

    class MethodDoc
      attr_reader :name, :description, :input_type, :output_type, :input_fields,
                  :output_fields, :client_streaming, :server_streaming, :example_input

      def initialize(
        name:,
        description:,
        input_type:,
        output_type:,
        input_fields: [],
        output_fields: [],
        client_streaming: false,
        server_streaming: false,
        example_input: ""
      )
        @name = name
        @description = description
        @input_type = input_type
        @output_type = output_type
        @input_fields = input_fields
        @output_fields = output_fields
        @client_streaming = client_streaming
        @server_streaming = server_streaming
        @example_input = example_input
      end

      def to_h
        {
          name: name,
          description: description,
          input_type: input_type,
          output_type: output_type,
          input_fields: input_fields.map(&:to_h),
          output_fields: output_fields.map(&:to_h),
          client_streaming: client_streaming,
          server_streaming: server_streaming,
          example_input: example_input
        }
      end
    end

    class ServiceDoc
      attr_reader :name, :description, :methods

      def initialize(name:, description:, methods:)
        @name = name
        @description = description
        @methods = methods
      end

      def to_h
        {
          name: name,
          description: description,
          methods: methods.map(&:to_h)
        }
      end
    end

    class DescribeResponse
      attr_reader :manifest, :services

      def initialize(manifest:, services:)
        @manifest = manifest
        @services = services
      end

      def to_h
        {
          manifest: Describe.manifest_to_h(manifest),
          services: services.map(&:to_h)
        }
      end
    end

    class Provider
      def initialize(*_unused, **_unused)
      end

      def describe(_request = DescribeRequest.new)
        response = Describe.static_response
        raise ErrNoIncodeDescription if response.nil?

        response
      end
    end

    class << self
      def use_static_response(response)
        if response.nil?
          @static_response = nil
          return
        end

        require_grpc_describe_support!
        unless response.is_a?(::Holons::V1::DescribeResponse)
          raise ArgumentError, "static response must be a Holons::V1::DescribeResponse"
        end

        @static_response = clone_proto_message(response)
      end

      def static_response
        clone_proto_message(@static_response)
      end

      def register(server, **_unused)
        raise ArgumentError, "grpc server is required" if server.nil?

        response = static_response
        raise ErrNoIncodeDescription if response.nil?

        server.handle(holon_meta_service_class.new(response))
      end

      def build_response(proto_dir:, manifest_path: nil)
        resolved_manifest_path = resolve_manifest_path(proto_dir, manifest_path)
        manifest = Identity.parse_manifest(resolved_manifest_path)
        index = parse_proto_directory(proto_dir)

        DescribeResponse.new(
          manifest: manifest,
          services: index.services.each_with_object([]) do |service, docs|
            next if service.full_name == HOLON_META_SERVICE

            docs << service_doc(service, index)
          end
        )
      end

      def service(*_unused, **_unused)
        Provider.new
      end

      private

      def clone_proto_message(message)
        return nil if message.nil?

        message.class.decode(message.class.encode(message))
      end

      def proto_response(proto_dir:, manifest_path:)
        require_grpc_describe_support!

        response = build_response(
          proto_dir: proto_dir,
          manifest_path: manifest_path
        )
        ::Holons::V1::DescribeResponse.new(
          manifest: proto_manifest(response.manifest),
          services: response.services.map { |service| proto_service_doc(service) }
        )
      end

      def manifest_to_h(manifest)
        return {} if manifest.nil?

        {
          identity: manifest.identity.nil? ? {} : {
            uuid: manifest.identity.uuid,
            given_name: manifest.identity.given_name,
            family_name: manifest.identity.family_name,
            motto: manifest.identity.motto,
            composer: manifest.identity.composer,
            status: manifest.identity.status,
            born: manifest.identity.born,
            aliases: manifest.identity.aliases
          },
          kind: manifest.kind,
          lang: manifest.identity.lang,
          build: {
            runner: manifest.build_runner,
            main: manifest.build_main
          },
          artifacts: {
            binary: manifest.artifact_binary,
            primary: manifest.artifact_primary
          }
        }
      end

      def proto_manifest(manifest)
        ::Holons::V1::HolonManifest.new(
          identity: ::Holons::V1::HolonManifest::Identity.new(
            schema: "holon/v1",
            uuid: manifest.identity.uuid.to_s,
            given_name: manifest.identity.given_name.to_s,
            family_name: manifest.identity.family_name.to_s,
            motto: manifest.identity.motto.to_s,
            composer: manifest.identity.composer.to_s,
            status: manifest.identity.status.to_s,
            born: manifest.identity.born.to_s,
            version: manifest.identity.respond_to?(:version) ? manifest.identity.version.to_s : "",
            aliases: manifest.identity.aliases
          ),
          lang: manifest.identity.lang.to_s,
          kind: manifest.kind.to_s,
          build: ::Holons::V1::HolonManifest::Build.new(
            runner: manifest.build_runner.to_s,
            main: manifest.build_main.to_s
          ),
          artifacts: ::Holons::V1::HolonManifest::Artifacts.new(
            binary: manifest.artifact_binary.to_s,
            primary: manifest.artifact_primary.to_s
          )
        )
      end

      def resolve_manifest_path(proto_dir, manifest_path)
        candidate = manifest_path
        return candidate unless candidate.to_s.strip.empty?

        Identity.resolve_manifest_path(proto_dir)
      end

      def proto_service_doc(service)
        ::Holons::V1::ServiceDoc.new(
          name: service.name,
          description: service.description,
          methods: service.methods.map { |method| proto_method_doc(method) }
        )
      end

      def proto_method_doc(method)
        ::Holons::V1::MethodDoc.new(
          name: method.name,
          description: method.description,
          input_type: method.input_type,
          output_type: method.output_type,
          input_fields: method.input_fields.map { |field| proto_field_doc(field) },
          output_fields: method.output_fields.map { |field| proto_field_doc(field) },
          client_streaming: method.client_streaming,
          server_streaming: method.server_streaming,
          example_input: method.example_input
        )
      end

      def proto_field_doc(field)
        ::Holons::V1::FieldDoc.new(
          name: field.name,
          type: field.type,
          number: field.number,
          description: field.description,
          label: field.label,
          map_key_type: field.map_key_type,
          map_value_type: field.map_value_type,
          nested_fields: field.nested_fields.map { |nested| proto_field_doc(nested) },
          enum_values: field.enum_values.map { |value| proto_enum_value_doc(value) },
          required: field.required,
          example: field.example
        )
      end

      def proto_enum_value_doc(value)
        ::Holons::V1::EnumValueDoc.new(
          name: value.name,
          number: value.number,
          description: value.description
        )
      end

      def holon_meta_service_class
        return @holon_meta_service_class unless @holon_meta_service_class.nil?

        require_grpc_describe_support!

        @holon_meta_service_class = Class.new(::Holons::V1::HolonMeta::Service) do
          def initialize(response)
            @response = response
          end

          def describe(_request, _call)
            @response
          end
        end
      end

      def require_grpc_describe_support!
        return unless @grpc_describe_loaded.nil?

        ensure_generated_proto_load_path!
        require_relative "../gen/holons/v1/manifest_pb"
        require_relative "../gen/holons/v1/describe_pb"
        require_relative "../gen/holons/v1/describe_services_pb"
        @grpc_describe_loaded = true
      end

      def ensure_generated_proto_load_path!
        gen_root = File.expand_path("../gen", __dir__)
        $LOAD_PATH.unshift(gen_root) unless $LOAD_PATH.include?(gen_root)
      end

      def service_doc(service, index)
        ServiceDoc.new(
          name: service.full_name,
          description: service.comment.description,
          methods: service.methods.map { |method| method_doc(method, index) }
        )
      end

      def method_doc(method, index)
        input = index.messages[method.input_type]
        output = index.messages[method.output_type]

        MethodDoc.new(
          name: method.name,
          description: method.comment.description,
          input_type: method.input_type,
          output_type: method.output_type,
          input_fields: input.nil? ? [] : input.fields.map { |field| field_doc(field, index, Set.new) },
          output_fields: output.nil? ? [] : output.fields.map { |field| field_doc(field, index, Set.new) },
          client_streaming: method.client_streaming,
          server_streaming: method.server_streaming,
          example_input: method.comment.example
        )
      end

      def field_doc(field, index, seen)
        nested_fields = []
        enum_values = []

        if field.cardinality == :map
          resolved_type = field.resolved_map_value_type(index)
          nested_fields = nested_field_docs(resolved_type, index, seen)
          enum_values = enum_value_docs(resolved_type, index)
        else
          resolved_type = field.resolved_type(index)
          nested_fields = nested_field_docs(resolved_type, index, seen)
          enum_values = enum_value_docs(resolved_type, index)
        end

        FieldDoc.new(
          name: field.name,
          type: field.type_name,
          number: field.number,
          description: field.comment.description,
          label: field.label,
          map_key_type: field.map_key_type.to_s,
          map_value_type: field.map_value_type.to_s,
          nested_fields: nested_fields,
          enum_values: enum_values,
          required: field.comment.required,
          example: field.comment.example
        )
      end

      def nested_field_docs(type_name, index, seen)
        message = index.messages[type_name]
        return [] if message.nil?
        return [] unless seen.add?(message.full_name)

        message.fields.map { |field| field_doc(field, index, seen.dup) }
      end

      def enum_value_docs(type_name, index)
        enum_def = index.enums[type_name]
        return [] if enum_def.nil?

        enum_def.values.map do |value|
          EnumValueDoc.new(
            name: value.name,
            number: value.number,
            description: value.comment.description
          )
        end
      end

      def parse_proto_directory(proto_dir)
        index = ProtoIndex.new
        return index unless File.directory?(proto_dir)

        Dir.glob(File.join(proto_dir, "**", "*.proto")).sort.each do |path|
          parse_proto_file(path, index)
        end
        index
      end

      def parse_proto_file(path, index)
        package_name = ""
        stack = []
        pending_comments = []

        File.readlines(path, chomp: true).each do |raw_line|
          line = raw_line.strip
          if line.start_with?("//")
            pending_comments << line.delete_prefix("//").strip
            next
          end
          next if line.empty?

          if (match = line.match(PACKAGE_PATTERN))
            package_name = match[1]
            pending_comments.clear
            next
          end

          if (match = line.match(SERVICE_PATTERN))
            comment = CommentMeta.parse(pending_comments)
            pending_comments.clear
            service = ServiceDef.new(
              name: match[1],
              full_name: qualify(package_name, match[1]),
              comment: comment
            )
            index.services << service
            stack << Block.new(:service, service, nil, nil)
            trim_closed_blocks(line, stack)
            next
          end

          if (match = line.match(MESSAGE_PATTERN))
            scope = message_scope(stack)
            name = match[1]
            comment = CommentMeta.parse(pending_comments)
            pending_comments.clear
            message = MessageDef.new(
              name: name,
              full_name: qualify(package_name, qualify_scope(scope, name)),
              package_name: package_name,
              scope: scope.dup,
              comment: comment
            )
            index.messages[message.full_name] = message
            index.simple_types[message.simple_key] ||= message.full_name
            stack << Block.new(:message, nil, message, nil)
            trim_closed_blocks(line, stack)
            next
          end

          if (match = line.match(ENUM_PATTERN))
            scope = message_scope(stack)
            name = match[1]
            comment = CommentMeta.parse(pending_comments)
            pending_comments.clear
            enum_def = EnumDef.new(
              name: name,
              full_name: qualify(package_name, qualify_scope(scope, name)),
              package_name: package_name,
              scope: scope.dup,
              comment: comment
            )
            index.enums[enum_def.full_name] = enum_def
            index.simple_types[enum_def.simple_key] ||= enum_def.full_name
            stack << Block.new(:enum, nil, nil, enum_def)
            trim_closed_blocks(line, stack)
            next
          end

          if (match = line.match(RPC_PATTERN))
            current_service = current_service(stack)
            if current_service
              current_service.methods << MethodDef.new(
                name: match[1],
                input_type: resolve_type(match[3], package_name, [], index),
                output_type: resolve_type(match[5], package_name, [], index),
                client_streaming: !match[2].nil?,
                server_streaming: !match[4].nil?,
                comment: CommentMeta.parse(pending_comments)
              )
            end
            pending_comments.clear
            trim_closed_blocks(line, stack)
            next
          end

          if (match = line.match(MAP_FIELD_PATTERN))
            current_message = current_message(stack)
            if current_message
              current_message.fields << FieldDef.new(
                name: match[4],
                type_name: "map<#{match[2]},#{match[3]}>",
                raw_type: "map",
                number: match[5].to_i,
                comment: CommentMeta.parse(pending_comments),
                cardinality: :map,
                package_name: package_name,
                scope: current_message.scope_with_self,
                map_key_type: match[2],
                map_value_type: match[3]
              )
            end
            pending_comments.clear
            trim_closed_blocks(line, stack)
            next
          end

          if (match = line.match(FIELD_PATTERN))
            current_message = current_message(stack)
            if current_message
              qualifier = match[1].to_s.strip
              raw_type = match[2]
              current_message.fields << FieldDef.new(
                name: match[3],
                type_name: resolve_type(raw_type, package_name, current_message.scope_with_self, index),
                raw_type: raw_type,
                number: match[4].to_i,
                comment: CommentMeta.parse(pending_comments),
                cardinality: qualifier == "repeated" ? :repeated : :optional,
                package_name: package_name,
                scope: current_message.scope_with_self
              )
            end
            pending_comments.clear
            trim_closed_blocks(line, stack)
            next
          end

          if (match = line.match(ENUM_VALUE_PATTERN))
            current_enum = current_enum(stack)
            if current_enum
              current_enum.values << EnumValueDef.new(
                name: match[1],
                number: match[2].to_i,
                comment: CommentMeta.parse(pending_comments)
              )
            end
            pending_comments.clear
            trim_closed_blocks(line, stack)
            next
          end

          pending_comments.clear unless line == "}"
          trim_closed_blocks(line, stack)
        end
      end

      def current_service(stack)
        stack.reverse_each.find { |block| block.kind == :service }&.service
      end

      def current_message(stack)
        stack.reverse_each.find { |block| block.kind == :message }&.message
      end

      def current_enum(stack)
        stack.reverse_each.find { |block| block.kind == :enum }&.enum_def
      end

      def trim_closed_blocks(line, stack)
        line.count("}").times { stack.pop unless stack.empty? }
      end

      def message_scope(stack)
        stack.each_with_object([]) do |block, names|
          names << block.message.name if block.kind == :message
        end
      end

      def qualify(package_name, name)
        return name if package_name.to_s.empty?

        "#{package_name}.#{name}"
      end

      def qualify_scope(scope, name)
        return name if scope.empty?

        "#{scope.join('.')}.#{name}"
      end

      def resolve_type(type_name, package_name, scope, index)
        type_name = type_name.to_s.strip
        return type_name.delete_prefix(".") if type_name.start_with?(".")
        return type_name if SCALAR_TYPES.include?(type_name)

        search_scope = scope.dup
        until search_scope.empty?
          candidate = qualify(package_name, qualify_scope(search_scope, type_name))
          return candidate if index.messages.key?(candidate) || index.enums.key?(candidate)

          search_scope.pop
        end

        package_candidate = qualify(package_name, type_name)
        return package_candidate if index.messages.key?(package_candidate) || index.enums.key?(package_candidate)

        index.simple_types[type_name] || package_candidate
      end

      def slug_for(identity)
        [identity.given_name, identity.family_name]
          .map { |value| value.to_s.strip }
          .reject(&:empty?)
          .join("-")
          .downcase
          .tr(" ", "-")
          .gsub(/\A-+|-+\z/, "")
      end
    end

    class CommentMeta
      attr_reader :description, :required, :example

      def initialize(description:, required:, example:)
        @description = description
        @required = required
        @example = example
      end

      def self.parse(lines)
        description_lines = []
        required = false
        example = ""

        lines.each do |line|
          value = line.to_s.strip
          case value
          when "@required"
            required = true
          when /\A@example\s+(.+)\z/
            example = Regexp.last_match(1).strip
          else
            description_lines << value unless value.empty?
          end
        end

        new(
          description: description_lines.join(" ").strip,
          required: required,
          example: example
        )
      end
    end

    class ProtoIndex
      attr_reader :services, :messages, :enums, :simple_types

      def initialize
        @services = []
        @messages = {}
        @enums = {}
        @simple_types = {}
      end
    end

    class ServiceDef
      attr_reader :name, :full_name, :comment, :methods

      def initialize(name:, full_name:, comment:)
        @name = name
        @full_name = full_name
        @comment = comment
        @methods = []
      end
    end

    class MethodDef
      attr_reader :name, :input_type, :output_type, :client_streaming,
                  :server_streaming, :comment

      def initialize(name:, input_type:, output_type:, client_streaming:, server_streaming:, comment:)
        @name = name
        @input_type = input_type
        @output_type = output_type
        @client_streaming = client_streaming
        @server_streaming = server_streaming
        @comment = comment
      end
    end

    class MessageDef
      attr_reader :name, :full_name, :package_name, :scope, :comment, :fields

      def initialize(name:, full_name:, package_name:, scope:, comment:)
        @name = name
        @full_name = full_name
        @package_name = package_name
        @scope = scope
        @comment = comment
        @fields = []
      end

      def simple_key
        ([package_name] + scope + [name]).reject(&:empty?).join(".")
      end

      def scope_with_self
        scope + [name]
      end
    end

    class EnumDef
      attr_reader :name, :full_name, :package_name, :scope, :comment, :values

      def initialize(name:, full_name:, package_name:, scope:, comment:)
        @name = name
        @full_name = full_name
        @package_name = package_name
        @scope = scope
        @comment = comment
        @values = []
      end

      def simple_key
        ([package_name] + scope + [name]).reject(&:empty?).join(".")
      end
    end

    class EnumValueDef
      attr_reader :name, :number, :comment

      def initialize(name:, number:, comment:)
        @name = name
        @number = number
        @comment = comment
      end
    end

    class FieldDef
      attr_reader :name, :type_name, :raw_type, :number, :comment, :cardinality,
                  :package_name, :scope, :map_key_type, :map_value_type

      def initialize(
        name:,
        type_name:,
        raw_type:,
        number:,
        comment:,
        cardinality:,
        package_name:,
        scope:,
        map_key_type: nil,
        map_value_type: nil
      )
        @name = name
        @type_name = type_name
        @raw_type = raw_type
        @number = number
        @comment = comment
        @cardinality = cardinality
        @package_name = package_name
        @scope = scope
        @map_key_type = map_key_type
        @map_value_type = map_value_type
      end

      def label
        return FieldLabel::MAP if cardinality == :map
        return FieldLabel::REPEATED if cardinality == :repeated
        return FieldLabel::REQUIRED if comment.required

        FieldLabel::OPTIONAL
      end

      def resolved_type(index)
        return raw_type if SCALAR_TYPES.include?(raw_type)
        return type_name if type_name.include?(".")

        Describe.send(:resolve_type, raw_type, package_name, scope, index)
      end

      def resolved_map_value_type(index)
        return map_value_type if map_value_type.nil? || SCALAR_TYPES.include?(map_value_type)

        Describe.send(:resolve_type, map_value_type, package_name, scope, index)
      end
    end

    Block = Struct.new(:kind, :service, :message, :enum_def)
  end
end
