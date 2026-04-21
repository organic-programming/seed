#pragma once

#include "holons.hpp"

#if __has_include("holons/v1/describe.pb.h") &&                                \
    __has_include("holons/v1/describe.grpc.pb.h")
#include "holons/v1/describe.pb.h"
#include "holons/v1/describe.grpc.pb.h"
#define HOLONS_HAS_STATIC_DESCRIBE_PROTO 1
#else
#define HOLONS_HAS_STATIC_DESCRIBE_PROTO 0
#endif

#include <cctype>
#include <functional>
#include <memory>
#include <regex>
#include <set>
#include <unordered_map>

namespace holons::describe {

constexpr std::string_view kHolonMetaServiceName = "holons.v1.HolonMeta";
constexpr std::string_view kDescribeMethodName = "Describe";
constexpr std::string_view kNoIncodeDescriptionRegistered =
    "no Incode Description registered — run op build";

enum class field_label {
  unspecified = 0,
  optional = 1,
  repeated = 2,
  map = 3,
  required = 4,
};

struct enum_value_doc {
  std::string name;
  int number = 0;
  std::string description;
};

struct field_doc {
  std::string name;
  std::string type;
  int number = 0;
  std::string description;
  field_label label = field_label::unspecified;
  std::string map_key_type;
  std::string map_value_type;
  std::vector<field_doc> nested_fields;
  std::vector<enum_value_doc> enum_values;
  bool required = false;
  std::string example;
};

struct method_doc {
  std::string name;
  std::string description;
  std::string input_type;
  std::string output_type;
  std::vector<field_doc> input_fields;
  std::vector<field_doc> output_fields;
  bool client_streaming = false;
  bool server_streaming = false;
  std::string example_input;
};

struct service_doc {
  std::string name;
  std::string description;
  std::vector<method_doc> methods;
};

struct describe_request {};

struct describe_response {
  HolonManifest manifest;
  std::vector<service_doc> services;
};

struct registration {
  std::string service_name;
  std::string method_name;
#if HOLONS_HAS_STATIC_DESCRIBE_PROTO
  std::function<holons::v1::DescribeResponse(
      const holons::v1::DescribeRequest &)> handler;
#else
  std::function<describe_response(const describe_request &)> handler;
#endif
};

namespace detail {

inline const std::regex kPackagePattern(R"(^package\s+([A-Za-z0-9_.]+)\s*;$)");
inline const std::regex kServicePattern(R"(^service\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?)");
inline const std::regex kMessagePattern(R"(^message\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?)");
inline const std::regex kEnumPattern(R"(^enum\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?)");
inline const std::regex kRpcPattern(
    R"(^rpc\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)\s*returns\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)\s*;?)");
inline const std::regex kMapFieldPattern(
    R"(^(repeated\s+)?map\s*<\s*([.A-Za-z0-9_]+)\s*,\s*([.A-Za-z0-9_]+)\s*>\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;)");
inline const std::regex kFieldPattern(
    R"(^(optional\s+|repeated\s+)?([.A-Za-z0-9_]+)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;)");
inline const std::regex kEnumValuePattern(
    R"(^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(-?\d+)\s*;)");
inline const std::set<std::string> kScalarTypes = {
    "double",  "float",   "int64",   "uint64",  "int32",   "fixed64",
    "fixed32", "bool",    "string",  "bytes",   "uint32",  "sfixed32",
    "sfixed64","sint32",  "sint64",
};

struct comment_meta {
  std::string description;
  bool required = false;
  std::string example;

  static comment_meta parse(const std::vector<std::string> &lines) {
    comment_meta out;
    for (const auto &line : lines) {
      if (line == "@required") {
        out.required = true;
        continue;
      }
      if (line.rfind("@example ", 0) == 0) {
        out.example = trim_copy(line.substr(9));
        continue;
      }
      if (!line.empty()) {
        if (!out.description.empty()) {
          out.description += ' ';
        }
        out.description += line;
      }
    }
    return out;
  }

private:
  static std::string trim_copy(const std::string &value) {
    size_t start = 0;
    while (start < value.size() &&
           std::isspace(static_cast<unsigned char>(value[start]))) {
      ++start;
    }
    size_t end = value.size();
    while (end > start &&
           std::isspace(static_cast<unsigned char>(value[end - 1]))) {
      --end;
    }
    return value.substr(start, end - start);
  }
};

struct enum_value_def {
  std::string name;
  int number = 0;
  comment_meta comment;
};

struct field_def {
  std::string name;
  std::string type_name;
  std::string raw_type;
  int number = 0;
  comment_meta comment;
  enum class cardinality { optional, repeated, map } cardinality =
      cardinality::optional;
  std::string package_name;
  std::vector<std::string> scope;
  std::string map_key_type;
  std::string map_value_type;

  field_label label() const {
    if (cardinality == cardinality::map) {
      return field_label::map;
    }
    if (cardinality == cardinality::repeated) {
      return field_label::repeated;
    }
    if (comment.required) {
      return field_label::required;
    }
    return field_label::optional;
  }
};

struct message_def {
  std::string name;
  std::string full_name;
  std::string package_name;
  std::vector<std::string> scope;
  comment_meta comment;
  std::vector<field_def> fields;

  std::string simple_key() const {
    std::string out = package_name;
    for (const auto &part : scope) {
      if (!out.empty()) {
        out += '.';
      }
      out += part;
    }
    if (!out.empty()) {
      out += '.';
    }
    out += name;
    return out;
  }

  std::vector<std::string> scope_with_self() const {
    auto result = scope;
    result.push_back(name);
    return result;
  }
};

struct enum_def {
  std::string name;
  std::string full_name;
  std::string package_name;
  std::vector<std::string> scope;
  comment_meta comment;
  std::vector<enum_value_def> values;

  std::string simple_key() const {
    std::string out = package_name;
    for (const auto &part : scope) {
      if (!out.empty()) {
        out += '.';
      }
      out += part;
    }
    if (!out.empty()) {
      out += '.';
    }
    out += name;
    return out;
  }
};

struct method_def {
  std::string name;
  std::string input_type;
  std::string output_type;
  bool client_streaming = false;
  bool server_streaming = false;
  comment_meta comment;
};

struct service_def {
  std::string name;
  std::string full_name;
  comment_meta comment;
  std::vector<method_def> methods;
};

struct proto_index {
  std::vector<service_def> services;
  std::unordered_map<std::string, message_def> messages;
  std::unordered_map<std::string, enum_def> enums;
  std::unordered_map<std::string, std::string> simple_types;
};

struct block {
  enum class kind { service, message, enum_type } kind;
  service_def *service = nullptr;
  message_def *message = nullptr;
  enum_def *enum_value = nullptr;
};

inline std::string trim_copy(const std::string &value) {
  size_t start = 0;
  while (start < value.size() &&
         std::isspace(static_cast<unsigned char>(value[start]))) {
    ++start;
  }
  size_t end = value.size();
  while (end > start &&
         std::isspace(static_cast<unsigned char>(value[end - 1]))) {
    --end;
  }
  return value.substr(start, end - start);
}

inline std::string qualify(const std::string &package_name,
                           const std::string &name) {
  if (package_name.empty()) {
    return name;
  }
  return package_name + "." + name;
}

inline std::string qualify_scope(const std::vector<std::string> &scope,
                                 const std::string &name) {
  if (scope.empty()) {
    return name;
  }
  std::string out;
  for (const auto &part : scope) {
    if (!out.empty()) {
      out += '.';
    }
    out += part;
  }
  out += '.';
  out += name;
  return out;
}

inline std::vector<std::string>
message_scope(const std::vector<block> &stack) {
  std::vector<std::string> scope;
  for (const auto &entry : stack) {
    if (entry.kind == block::kind::message && entry.message != nullptr) {
      scope.push_back(entry.message->name);
    }
  }
  return scope;
}

inline void trim_closed_blocks(const std::string &line,
                               std::vector<block> &stack) {
  size_t closes = static_cast<size_t>(std::count(line.begin(), line.end(), '}'));
  while (closes-- > 0 && !stack.empty()) {
    stack.pop_back();
  }
}

inline service_def *current_service(std::vector<block> &stack) {
  for (auto it = stack.rbegin(); it != stack.rend(); ++it) {
    if (it->kind == block::kind::service) {
      return it->service;
    }
  }
  return nullptr;
}

inline message_def *current_message(std::vector<block> &stack) {
  for (auto it = stack.rbegin(); it != stack.rend(); ++it) {
    if (it->kind == block::kind::message) {
      return it->message;
    }
  }
  return nullptr;
}

inline enum_def *current_enum(std::vector<block> &stack) {
  for (auto it = stack.rbegin(); it != stack.rend(); ++it) {
    if (it->kind == block::kind::enum_type) {
      return it->enum_value;
    }
  }
  return nullptr;
}

inline std::string resolve_type(const std::string &type_name,
                                const std::string &package_name,
                                std::vector<std::string> scope,
                                const proto_index &index) {
  std::string trimmed = trim_copy(type_name);
  if (trimmed.empty()) {
    return trimmed;
  }
  if (trimmed.front() == '.') {
    return trimmed.substr(1);
  }
  if (kScalarTypes.find(trimmed) != kScalarTypes.end()) {
    return trimmed;
  }

  while (!scope.empty()) {
    auto candidate = qualify(package_name, qualify_scope(scope, trimmed));
    if (index.messages.find(candidate) != index.messages.end() ||
        index.enums.find(candidate) != index.enums.end()) {
      return candidate;
    }
    scope.pop_back();
  }

  auto package_candidate = qualify(package_name, trimmed);
  if (index.messages.find(package_candidate) != index.messages.end() ||
      index.enums.find(package_candidate) != index.enums.end()) {
    return package_candidate;
  }

  auto simple = index.simple_types.find(trimmed);
  if (simple != index.simple_types.end()) {
    return simple->second;
  }
  return package_candidate;
}

inline void parse_proto_file(const std::filesystem::path &path,
                             proto_index &index) {
  std::ifstream input(path);
  if (!input) {
    throw std::runtime_error("failed to open proto file: " + path.string());
  }

  std::string package_name;
  std::vector<block> stack;
  std::vector<std::string> pending_comments;
  std::string raw_line;
  std::smatch match;

  while (std::getline(input, raw_line)) {
    auto line = trim_copy(raw_line);
    if (line.rfind("//", 0) == 0) {
      pending_comments.push_back(trim_copy(line.substr(2)));
      continue;
    }
    if (line.empty()) {
      continue;
    }

    if (std::regex_match(line, match, kPackagePattern)) {
      package_name = match[1].str();
      pending_comments.clear();
      continue;
    }

    if (std::regex_match(line, match, kServicePattern)) {
      auto comment = comment_meta::parse(pending_comments);
      pending_comments.clear();
      index.services.push_back(
          service_def{match[1].str(), qualify(package_name, match[1].str()),
                      comment, {}});
      stack.push_back(block{block::kind::service, &index.services.back(),
                            nullptr, nullptr});
      trim_closed_blocks(line, stack);
      continue;
    }

    if (std::regex_match(line, match, kMessagePattern)) {
      auto scope = message_scope(stack);
      auto name = match[1].str();
      auto comment = comment_meta::parse(pending_comments);
      pending_comments.clear();
      message_def message{name,
                          qualify(package_name, qualify_scope(scope, name)),
                          package_name,
                          scope,
                          comment,
                          {}};
      auto full_name = message.full_name;
      index.messages.emplace(full_name, std::move(message));
      index.simple_types.emplace(index.messages.at(full_name).simple_key(),
                                 full_name);
      stack.push_back(
          block{block::kind::message, nullptr, &index.messages.at(full_name),
                nullptr});
      trim_closed_blocks(line, stack);
      continue;
    }

    if (std::regex_match(line, match, kEnumPattern)) {
      auto scope = message_scope(stack);
      auto name = match[1].str();
      auto comment = comment_meta::parse(pending_comments);
      pending_comments.clear();
      enum_def enum_type{name,
                         qualify(package_name, qualify_scope(scope, name)),
                         package_name,
                         scope,
                         comment,
                         {}};
      auto full_name = enum_type.full_name;
      index.enums.emplace(full_name, std::move(enum_type));
      index.simple_types.emplace(index.enums.at(full_name).simple_key(),
                                 full_name);
      stack.push_back(block{block::kind::enum_type, nullptr, nullptr,
                            &index.enums.at(full_name)});
      trim_closed_blocks(line, stack);
      continue;
    }

    if (std::regex_match(line, match, kRpcPattern)) {
      auto *service = current_service(stack);
      if (service != nullptr) {
        service->methods.push_back(method_def{
            match[1].str(),
            resolve_type(match[3].str(), package_name, {}, index),
            resolve_type(match[5].str(), package_name, {}, index),
            match[2].matched,
            match[4].matched,
            comment_meta::parse(pending_comments),
        });
      }
      pending_comments.clear();
      trim_closed_blocks(line, stack);
      continue;
    }

    if (std::regex_match(line, match, kMapFieldPattern)) {
      auto *message = current_message(stack);
      if (message != nullptr) {
        message->fields.push_back(field_def{
            match[4].str(),
            "map<" + match[2].str() + "," + match[3].str() + ">",
            "map",
            std::stoi(match[5].str()),
            comment_meta::parse(pending_comments),
            field_def::cardinality::map,
            package_name,
            message->scope_with_self(),
            match[2].str(),
            match[3].str(),
        });
      }
      pending_comments.clear();
      trim_closed_blocks(line, stack);
      continue;
    }

    if (std::regex_match(line, match, kFieldPattern)) {
      auto *message = current_message(stack);
      if (message != nullptr) {
        auto qualifier = trim_copy(match[1].str());
        auto raw_type = match[2].str();
        message->fields.push_back(field_def{
            match[3].str(),
            resolve_type(raw_type, package_name, message->scope_with_self(),
                         index),
            raw_type,
            std::stoi(match[4].str()),
            comment_meta::parse(pending_comments),
            qualifier == "repeated" ? field_def::cardinality::repeated
                                     : field_def::cardinality::optional,
            package_name,
            message->scope_with_self(),
            "",
            "",
        });
      }
      pending_comments.clear();
      trim_closed_blocks(line, stack);
      continue;
    }

    if (std::regex_match(line, match, kEnumValuePattern)) {
      auto *enum_type = current_enum(stack);
      if (enum_type != nullptr) {
        enum_type->values.push_back(enum_value_def{
            match[1].str(),
            std::stoi(match[2].str()),
            comment_meta::parse(pending_comments),
        });
      }
      pending_comments.clear();
      trim_closed_blocks(line, stack);
      continue;
    }

    if (line != "}") {
      pending_comments.clear();
    }
    trim_closed_blocks(line, stack);
  }
}

inline proto_index parse_proto_directory(const std::filesystem::path &proto_dir) {
  proto_index index;
  if (!std::filesystem::is_directory(proto_dir)) {
    return index;
  }

  std::vector<std::filesystem::path> files;
  for (const auto &entry :
       std::filesystem::recursive_directory_iterator(proto_dir)) {
    if (!entry.is_regular_file() || entry.path().extension() != ".proto") {
      continue;
    }
    files.push_back(entry.path());
  }
  std::sort(files.begin(), files.end());

  for (const auto &file : files) {
    parse_proto_file(file, index);
  }
  return index;
}

inline std::string slug_for(const HolonIdentity &identity) {
  auto given_name = trim_copy(identity.given_name);
  auto family_name = trim_copy(identity.family_name);
  std::string slug;
  if (given_name.empty()) {
    slug = family_name;
  } else if (family_name.empty()) {
    slug = given_name;
  } else {
    slug = given_name + "-" + family_name;
  }
  std::transform(slug.begin(), slug.end(), slug.begin(),
                 [](unsigned char ch) { return static_cast<char>(std::tolower(ch)); });
  return slug;
}

inline std::vector<enum_value_doc>
enum_value_docs(const std::string &type_name, const proto_index &index) {
  std::vector<enum_value_doc> docs;
  auto found = index.enums.find(type_name);
  if (found == index.enums.end()) {
    return docs;
  }
  for (const auto &value : found->second.values) {
    docs.push_back(enum_value_doc{value.name, value.number,
                                  value.comment.description});
  }
  return docs;
}

inline std::vector<field_doc>
nested_field_docs(const std::string &type_name, const proto_index &index,
                  std::set<std::string> seen);

inline field_doc to_field_doc(const field_def &field, const proto_index &index,
                              std::set<std::string> seen) {
  std::string resolved_type;
  if (field.cardinality == field_def::cardinality::map) {
    resolved_type = resolve_type(field.map_value_type, field.package_name,
                                 field.scope, index);
  } else {
    resolved_type = resolve_type(field.raw_type, field.package_name, field.scope,
                                 index);
  }

  field_doc doc;
  doc.name = field.name;
  doc.type = field.type_name;
  doc.number = field.number;
  doc.description = field.comment.description;
  doc.label = field.label();
  doc.map_key_type = field.map_key_type;
  doc.map_value_type = field.map_value_type;
  doc.nested_fields = nested_field_docs(resolved_type, index, std::move(seen));
  doc.enum_values = enum_value_docs(resolved_type, index);
  doc.required = field.comment.required;
  doc.example = field.comment.example;
  return doc;
}

inline std::vector<field_doc>
nested_field_docs(const std::string &type_name, const proto_index &index,
                  std::set<std::string> seen) {
  std::vector<field_doc> docs;
  auto found = index.messages.find(type_name);
  if (found == index.messages.end()) {
    return docs;
  }
  if (!seen.insert(found->second.full_name).second) {
    return docs;
  }
  for (const auto &field : found->second.fields) {
    docs.push_back(to_field_doc(field, index, seen));
  }
  return docs;
}

inline method_doc to_method_doc(const method_def &method,
                                const proto_index &index) {
  method_doc doc;
  doc.name = method.name;
  doc.description = method.comment.description;
  doc.input_type = method.input_type;
  doc.output_type = method.output_type;
  doc.client_streaming = method.client_streaming;
  doc.server_streaming = method.server_streaming;
  doc.example_input = method.comment.example;

  if (auto found = index.messages.find(method.input_type);
      found != index.messages.end()) {
    for (const auto &field : found->second.fields) {
      doc.input_fields.push_back(to_field_doc(field, index, {}));
    }
  }
  if (auto found = index.messages.find(method.output_type);
      found != index.messages.end()) {
    for (const auto &field : found->second.fields) {
      doc.output_fields.push_back(to_field_doc(field, index, {}));
    }
  }
  return doc;
}

inline service_doc to_service_doc(const service_def &service,
                                  const proto_index &index) {
  service_doc doc;
  doc.name = service.full_name;
  doc.description = service.comment.description;
  for (const auto &method : service.methods) {
    doc.methods.push_back(to_method_doc(method, index));
  }
  return doc;
}

} // namespace detail

#if HOLONS_HAS_STATIC_DESCRIBE_PROTO
namespace detail {

inline std::mutex &static_response_mutex() {
  static std::mutex mu;
  return mu;
}

inline std::shared_ptr<holons::v1::DescribeResponse> &static_response() {
  static std::shared_ptr<holons::v1::DescribeResponse> response;
  return response;
}

} // namespace detail

inline void use_static_response(const holons::v1::DescribeResponse *response) {
  std::lock_guard<std::mutex> lock(detail::static_response_mutex());
  if (response == nullptr) {
    detail::static_response().reset();
    return;
  }

  auto copy = std::make_shared<holons::v1::DescribeResponse>();
  copy->CopyFrom(*response);
  detail::static_response() = std::move(copy);
}

inline void use_static_response(const holons::v1::DescribeResponse &response) {
  use_static_response(&response);
}

inline void clear_static_response() { use_static_response(nullptr); }

inline std::shared_ptr<holons::v1::DescribeResponse>
registered_static_response() {
  std::lock_guard<std::mutex> lock(detail::static_response_mutex());
  if (!detail::static_response()) {
    return nullptr;
  }

  auto copy = std::make_shared<holons::v1::DescribeResponse>();
  copy->CopyFrom(*detail::static_response());
  return copy;
}
#endif

inline describe_response build_response(const std::filesystem::path &proto_dir) {
  auto resolved = parse_resolved_manifest(resolve_manifest_path(proto_dir).string());
  auto index = detail::parse_proto_directory(proto_dir);

  describe_response response;
  response.manifest = resolved.manifest;

  for (const auto &service : index.services) {
    if (service.full_name == kHolonMetaServiceName) {
      continue;
    }
    response.services.push_back(detail::to_service_doc(service, index));
  }

  return response;
}

inline registration make_registration() {
  registration reg;
  reg.service_name = std::string(kHolonMetaServiceName);
  reg.method_name = std::string(kDescribeMethodName);
#if HOLONS_HAS_STATIC_DESCRIBE_PROTO
  reg.handler = [](const holons::v1::DescribeRequest &) {
    auto response = registered_static_response();
    if (!response) {
      throw std::runtime_error(
          std::string(kNoIncodeDescriptionRegistered));
    }
    return *response;
  };
#else
  reg.handler = [](const describe_request &) { return describe_response{}; };
#endif
  return reg;
}

} // namespace holons::describe
