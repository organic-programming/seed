cmake_minimum_required(VERSION 3.21)
project(observability-cascade-cpp LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

set(RELAY_CASCADE_CPP_PREBUILT_ROOT "")
if(DEFINED ENV{OP_SDK_CPP_PATH} AND NOT "$ENV{OP_SDK_CPP_PATH}" STREQUAL "")
  file(TO_CMAKE_PATH "$ENV{OP_SDK_CPP_PATH}" RELAY_CASCADE_CPP_PREBUILT_ROOT)
  set(CMAKE_PREFIX_PATH "${RELAY_CASCADE_CPP_PREBUILT_ROOT}" ${CMAKE_PREFIX_PATH})
  set(CMAKE_INCLUDE_PATH "${RELAY_CASCADE_CPP_PREBUILT_ROOT}/include" ${CMAKE_INCLUDE_PATH})
  set(CMAKE_LIBRARY_PATH "${RELAY_CASCADE_CPP_PREBUILT_ROOT}/lib" ${CMAKE_LIBRARY_PATH})
  set(CMAKE_PROGRAM_PATH "${RELAY_CASCADE_CPP_PREBUILT_ROOT}/bin" ${CMAKE_PROGRAM_PATH})
  set(Protobuf_DIR "${RELAY_CASCADE_CPP_PREBUILT_ROOT}/lib/cmake/protobuf" CACHE PATH "" FORCE)
  set(gRPC_DIR "${RELAY_CASCADE_CPP_PREBUILT_ROOT}/lib/cmake/grpc" CACHE PATH "" FORCE)
endif()

find_package(Protobuf CONFIG QUIET)
find_package(gRPC CONFIG QUIET)
if(NOT Protobuf_FOUND OR NOT gRPC_FOUND)
  message(FATAL_ERROR "observability-cascade-cpp requires the C++ SDK prebuilts. Run `op sdk install cpp` or build through `op build`.")
endif()

if(TARGET protobuf::protoc)
  set(RELAY_CASCADE_CPP_PROTOC $<TARGET_FILE:protobuf::protoc>)
elseif(DEFINED Protobuf_PROTOC_EXECUTABLE)
  set(RELAY_CASCADE_CPP_PROTOC "${Protobuf_PROTOC_EXECUTABLE}")
else()
  find_program(RELAY_CASCADE_CPP_PROTOC protoc REQUIRED)
endif()

if(TARGET gRPC::grpc_cpp_plugin)
  set(RELAY_CASCADE_CPP_GRPC_PLUGIN $<TARGET_FILE:gRPC::grpc_cpp_plugin>)
else()
  find_program(RELAY_CASCADE_CPP_GRPC_PLUGIN grpc_cpp_plugin
    PATHS "${RELAY_CASCADE_CPP_PREBUILT_ROOT}/bin" /opt/homebrew/bin /usr/local/bin
    REQUIRED)
endif()

set(RELAY_CASCADE_CPP_GENERATED_DIR "${CMAKE_CURRENT_BINARY_DIR}/generated")
set(HOLONS_PROTO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../holons/grace-op/_protos")
set(RELAY_PROTO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../_protos")
set(HOLONS_SDK_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../sdk/cpp-holons")

function(observability_cascade_generate_grpc_cpp target_name)
  set(options)
  set(one_value_args OUT_DIR)
  set(multi_value_args PROTOS IMPORT_DIRS)
  cmake_parse_arguments(RG "${options}" "${one_value_args}" "${multi_value_args}" ${ARGN})
  file(MAKE_DIRECTORY "${RG_OUT_DIR}")
  set(generated_sources)
  set(generated_headers)
  foreach(proto IN LISTS RG_PROTOS)
    set(proto_relative "")
    foreach(import_dir IN LISTS RG_IMPORT_DIRS)
      file(RELATIVE_PATH candidate "${import_dir}" "${proto}")
      if(NOT candidate MATCHES "^\\.\\.")
        set(proto_relative "${candidate}")
        break()
      endif()
    endforeach()
    get_filename_component(proto_dir "${proto_relative}" DIRECTORY)
    get_filename_component(proto_name "${proto}" NAME_WE)
    list(APPEND generated_sources
      "${RG_OUT_DIR}/${proto_dir}/${proto_name}.pb.cc"
      "${RG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.cc")
    list(APPEND generated_headers
      "${RG_OUT_DIR}/${proto_dir}/${proto_name}.pb.h"
      "${RG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.h")
    set(import_args)
    foreach(import_dir IN LISTS RG_IMPORT_DIRS)
      list(APPEND import_args -I "${import_dir}")
    endforeach()
    add_custom_command(
      OUTPUT
        "${RG_OUT_DIR}/${proto_dir}/${proto_name}.pb.cc"
        "${RG_OUT_DIR}/${proto_dir}/${proto_name}.pb.h"
        "${RG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.cc"
        "${RG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.h"
      COMMAND "${RELAY_CASCADE_CPP_PROTOC}"
        --cpp_out=${RG_OUT_DIR}
        --grpc_out=${RG_OUT_DIR}
        --plugin=protoc-gen-grpc=${RELAY_CASCADE_CPP_GRPC_PLUGIN}
        ${import_args}
        "${proto}"
      DEPENDS "${proto}"
      VERBATIM)
  endforeach()
  add_library(${target_name} STATIC ${generated_sources} ${generated_headers})
  target_include_directories(${target_name} PUBLIC "${RG_OUT_DIR}")
  target_link_libraries(${target_name} PUBLIC gRPC::grpc++ protobuf::libprotobuf)
endfunction()

observability_cascade_generate_grpc_cpp(observability_cascade_cpp_holons_proto_grpc
  OUT_DIR "${RELAY_CASCADE_CPP_GENERATED_DIR}"
  PROTOS
    "${HOLONS_PROTO_ROOT}/holons/v1/manifest.proto"
    "${HOLONS_PROTO_ROOT}/holons/v1/describe.proto"
    "${HOLONS_PROTO_ROOT}/holons/v1/session.proto"
    "${HOLONS_PROTO_ROOT}/holons/v1/observability.proto"
  IMPORT_DIRS "${HOLONS_PROTO_ROOT}")

observability_cascade_generate_grpc_cpp(observability_cascade_cpp_relay_proto_grpc
  OUT_DIR "${RELAY_CASCADE_CPP_GENERATED_DIR}"
  PROTOS "${RELAY_PROTO_ROOT}/relay/v1/relay.proto"
  IMPORT_DIRS "${RELAY_PROTO_ROOT}")

observability_cascade_generate_grpc_cpp(observability_cascade_cpp_service_proto_grpc
  OUT_DIR "${RELAY_CASCADE_CPP_GENERATED_DIR}"
  PROTOS "${RELAY_PROTO_ROOT}/observability_cascade/v1/service.proto"
  IMPORT_DIRS "${RELAY_PROTO_ROOT}")

add_executable(observability-cascade-cpp
  src/main.cpp
)

target_include_directories(observability-cascade-cpp
  PRIVATE
    "${RELAY_CASCADE_CPP_GENERATED_DIR}"
    "${HOLONS_SDK_ROOT}/include"
    "${HOLONS_SDK_ROOT}/gen/cpp"
)

if(RELAY_CASCADE_CPP_PREBUILT_ROOT)
  target_include_directories(observability-cascade-cpp PRIVATE "${RELAY_CASCADE_CPP_PREBUILT_ROOT}/include")
endif()

target_link_libraries(observability-cascade-cpp
  PRIVATE
    observability_cascade_cpp_holons_proto_grpc
    observability_cascade_cpp_relay_proto_grpc
    observability_cascade_cpp_service_proto_grpc
    gRPC::grpc++
    gRPC::grpc++_reflection
    protobuf::libprotobuf)
