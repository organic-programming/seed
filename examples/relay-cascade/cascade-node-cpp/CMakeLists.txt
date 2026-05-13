cmake_minimum_required(VERSION 3.21)
project(cascade-node-cpp LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

set(CASCADE_NODE_CPP_PREBUILT_ROOT "")
if(DEFINED ENV{OP_SDK_CPP_PATH} AND NOT "$ENV{OP_SDK_CPP_PATH}" STREQUAL "")
  file(TO_CMAKE_PATH "$ENV{OP_SDK_CPP_PATH}" CASCADE_NODE_CPP_PREBUILT_ROOT)
  if(NOT EXISTS "${CASCADE_NODE_CPP_PREBUILT_ROOT}/include" OR
      NOT EXISTS "${CASCADE_NODE_CPP_PREBUILT_ROOT}/lib")
    message(FATAL_ERROR
      "OP_SDK_CPP_PATH=${CASCADE_NODE_CPP_PREBUILT_ROOT} is not a C++ SDK prebuilt root; run `op sdk install cpp`")
  endif()
  set(CMAKE_PREFIX_PATH "${CASCADE_NODE_CPP_PREBUILT_ROOT}" ${CMAKE_PREFIX_PATH})
  set(CMAKE_INCLUDE_PATH "${CASCADE_NODE_CPP_PREBUILT_ROOT}/include" ${CMAKE_INCLUDE_PATH})
  set(CMAKE_LIBRARY_PATH "${CASCADE_NODE_CPP_PREBUILT_ROOT}/lib" ${CMAKE_LIBRARY_PATH})
  set(CMAKE_PROGRAM_PATH "${CASCADE_NODE_CPP_PREBUILT_ROOT}/bin" ${CMAKE_PROGRAM_PATH})
  set(Protobuf_DIR "${CASCADE_NODE_CPP_PREBUILT_ROOT}/lib/cmake/protobuf" CACHE PATH "" FORCE)
  set(gRPC_DIR "${CASCADE_NODE_CPP_PREBUILT_ROOT}/lib/cmake/grpc" CACHE PATH "" FORCE)
endif()

find_package(Protobuf CONFIG QUIET)
find_package(gRPC CONFIG QUIET)
if(NOT Protobuf_FOUND OR NOT gRPC_FOUND)
  if(CASCADE_NODE_CPP_PREBUILT_ROOT)
    message(FATAL_ERROR
      "OP_SDK_CPP_PATH=${CASCADE_NODE_CPP_PREBUILT_ROOT} is missing gRPC or Protobuf CMake packages; run `op sdk verify cpp` or reinstall with `op sdk install cpp`")
  else()
    message(FATAL_ERROR
      "cascade-node-cpp requires the C++ SDK prebuilts. Run `op sdk install cpp` or install gRPC system-wide.")
  endif()
endif()
find_program(BASH_EXECUTABLE NAMES bash REQUIRED)

if(TARGET protobuf::protoc)
  set(CASCADE_NODE_CPP_PROTOC $<TARGET_FILE:protobuf::protoc>)
elseif(DEFINED Protobuf_PROTOC_EXECUTABLE)
  set(CASCADE_NODE_CPP_PROTOC "${Protobuf_PROTOC_EXECUTABLE}")
else()
  find_program(CASCADE_NODE_CPP_PROTOC protoc REQUIRED)
endif()

if(TARGET gRPC::grpc_cpp_plugin)
  set(CASCADE_NODE_CPP_GRPC_PLUGIN $<TARGET_FILE:gRPC::grpc_cpp_plugin>)
else()
  find_program(CASCADE_NODE_CPP_GRPC_PLUGIN grpc_cpp_plugin
    PATHS "${CASCADE_NODE_CPP_PREBUILT_ROOT}/bin" /opt/homebrew/bin /usr/local/bin
    REQUIRED)
endif()

set(HOLONS_SDK_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../sdk/cpp-holons")
set(HOLONS_PROTO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../holons/grace-op/_protos")
set(RELAY_PROTO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../_protos")
set(CASCADE_NODE_CPP_GENERATED_DIR "${CMAKE_CURRENT_BINARY_DIR}/generated")
set(CASCADE_NODE_CPP_RELOCATE_SCRIPT "${CMAKE_CURRENT_SOURCE_DIR}/../../../sdk/scripts/relocate-macos-dylibs.sh")

function(cascade_generate_grpc_cpp target_name)
  set(options)
  set(one_value_args OUT_DIR)
  set(multi_value_args PROTOS IMPORT_DIRS)
  cmake_parse_arguments(CG "${options}" "${one_value_args}" "${multi_value_args}" ${ARGN})
  file(MAKE_DIRECTORY "${CG_OUT_DIR}")
  set(generated_sources)
  set(generated_headers)
  foreach(proto IN LISTS CG_PROTOS)
    set(proto_relative "")
    foreach(import_dir IN LISTS CG_IMPORT_DIRS)
      file(RELATIVE_PATH candidate "${import_dir}" "${proto}")
      if(NOT candidate MATCHES "^\\.\\.")
        set(proto_relative "${candidate}")
        break()
      endif()
    endforeach()
    get_filename_component(proto_dir "${proto_relative}" DIRECTORY)
    get_filename_component(proto_name "${proto}" NAME_WE)
    list(APPEND generated_sources
      "${CG_OUT_DIR}/${proto_dir}/${proto_name}.pb.cc"
      "${CG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.cc")
    list(APPEND generated_headers
      "${CG_OUT_DIR}/${proto_dir}/${proto_name}.pb.h"
      "${CG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.h")
    set(import_args)
    foreach(import_dir IN LISTS CG_IMPORT_DIRS)
      list(APPEND import_args -I "${import_dir}")
    endforeach()
    add_custom_command(
      OUTPUT
        "${CG_OUT_DIR}/${proto_dir}/${proto_name}.pb.cc"
        "${CG_OUT_DIR}/${proto_dir}/${proto_name}.pb.h"
        "${CG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.cc"
        "${CG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.h"
      COMMAND "${CASCADE_NODE_CPP_PROTOC}"
        --cpp_out=${CG_OUT_DIR}
        --grpc_out=${CG_OUT_DIR}
        --plugin=protoc-gen-grpc=${CASCADE_NODE_CPP_GRPC_PLUGIN}
        ${import_args}
        "${proto}"
      DEPENDS "${proto}"
      VERBATIM)
  endforeach()
  add_library(${target_name} STATIC ${generated_sources} ${generated_headers})
  target_include_directories(${target_name} PUBLIC "${CG_OUT_DIR}")
  target_link_libraries(${target_name} PUBLIC gRPC::grpc++ protobuf::libprotobuf)
endfunction()

cascade_generate_grpc_cpp(cascade_node_cpp_holons_proto_grpc
  OUT_DIR "${CASCADE_NODE_CPP_GENERATED_DIR}"
  PROTOS
    "${HOLONS_PROTO_ROOT}/holons/v1/manifest.proto"
    "${HOLONS_PROTO_ROOT}/holons/v1/describe.proto"
    "${HOLONS_PROTO_ROOT}/holons/v1/session.proto"
    "${HOLONS_PROTO_ROOT}/holons/v1/observability.proto"
  IMPORT_DIRS "${HOLONS_PROTO_ROOT}")

cascade_generate_grpc_cpp(cascade_node_cpp_relay_grpc
  OUT_DIR "${CASCADE_NODE_CPP_GENERATED_DIR}"
  PROTOS "${RELAY_PROTO_ROOT}/relay/v1/relay.proto"
  IMPORT_DIRS "${RELAY_PROTO_ROOT}")

if(CMAKE_RUNTIME_OUTPUT_DIRECTORY)
  set(CASCADE_NODE_CPP_RUNTIME_DIR "${CMAKE_RUNTIME_OUTPUT_DIRECTORY}")
else()
  set(CASCADE_NODE_CPP_RUNTIME_DIR "${CMAKE_CURRENT_BINARY_DIR}")
endif()

if(NOT DEFINED OP_PACKAGE_ARCH OR OP_PACKAGE_ARCH STREQUAL "")
  string(TOLOWER "${CMAKE_SYSTEM_NAME}" OP_PACKAGE_OS)
  set(OP_PACKAGE_ARCH "${OP_PACKAGE_OS}_${CMAKE_SYSTEM_PROCESSOR}")
endif()
if(NOT DEFINED OP_USE_INSTALL_LAYOUT)
  set(OP_USE_INSTALL_LAYOUT OFF)
endif()
set(CASCADE_NODE_CPP_INSTALL_RUNTIME_DIR "${CMAKE_INSTALL_PREFIX}/bin/${OP_PACKAGE_ARCH}")

add_library(cascade_node_cpp_lib
  api/public.cpp
  api/cli.cpp
  internal/server.cpp
)

target_include_directories(cascade_node_cpp_lib
  PUBLIC
    "${CMAKE_CURRENT_SOURCE_DIR}"
    "${CASCADE_NODE_CPP_GENERATED_DIR}"
    "${HOLONS_SDK_ROOT}/include"
    "${HOLONS_SDK_ROOT}/gen/cpp"
)
if(CASCADE_NODE_CPP_PREBUILT_ROOT)
  target_include_directories(cascade_node_cpp_lib
    PUBLIC
      "${CASCADE_NODE_CPP_PREBUILT_ROOT}/include")
endif()

target_link_libraries(cascade_node_cpp_lib
  PUBLIC
    cascade_node_cpp_holons_proto_grpc
    cascade_node_cpp_relay_grpc
    gRPC::grpc++
    gRPC::grpc++_reflection
    protobuf::libprotobuf)

add_executable(cascade-node-cpp cmd/main.cpp)
target_link_libraries(cascade-node-cpp PRIVATE cascade_node_cpp_lib)

if(APPLE AND NOT OP_USE_INSTALL_LAYOUT)
  set(CASCADE_NODE_CPP_RELOCATE_STAMP "${CASCADE_NODE_CPP_RUNTIME_DIR}/.dylibs-relocated")
  add_custom_command(
    OUTPUT "${CASCADE_NODE_CPP_RELOCATE_STAMP}"
    COMMAND "${BASH_EXECUTABLE}" "${CASCADE_NODE_CPP_RELOCATE_SCRIPT}" "${CASCADE_NODE_CPP_RUNTIME_DIR}" cascade-node-cpp
    COMMAND "${CMAKE_COMMAND}" -E touch "${CASCADE_NODE_CPP_RELOCATE_STAMP}"
    DEPENDS cascade-node-cpp "${CASCADE_NODE_CPP_RELOCATE_SCRIPT}"
    VERBATIM)
  add_custom_target(cascade_node_cpp_relocate ALL DEPENDS "${CASCADE_NODE_CPP_RELOCATE_STAMP}")
endif()

install(TARGETS cascade-node-cpp
  RUNTIME DESTINATION "bin/${OP_PACKAGE_ARCH}")

if(APPLE)
  install(CODE "
    execute_process(
      COMMAND \"${BASH_EXECUTABLE}\" \"${CASCADE_NODE_CPP_RELOCATE_SCRIPT}\" \"${CASCADE_NODE_CPP_INSTALL_RUNTIME_DIR}\" cascade-node-cpp
      COMMAND_ERROR_IS_FATAL ANY
    )
  ")
endif()
