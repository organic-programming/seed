cmake_minimum_required(VERSION 3.14)
project(cpp-holons LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

include(CMakeParseArguments)

add_library(holons INTERFACE)
target_include_directories(holons INTERFACE include)
target_include_directories(holons INTERFACE "${CMAKE_CURRENT_SOURCE_DIR}/gen/cpp")
target_include_directories(holons INTERFACE /opt/homebrew/include /usr/local/include)

add_executable(test_runner test/holons_test.cpp)
target_link_libraries(test_runner PRIVATE holons)

find_package(gRPC CONFIG QUIET)
find_package(Protobuf CONFIG QUIET)
find_package(OpenSSL QUIET)
if (NOT Protobuf_FOUND)
  find_package(Protobuf QUIET)
endif()

set(HOLONS_OPENSSL_LIBRARIES)
if (OpenSSL_FOUND)
  list(APPEND HOLONS_OPENSSL_LIBRARIES OpenSSL::SSL OpenSSL::Crypto)
else()
  find_library(HOLONS_SSL_LIBRARY NAMES ssl libssl PATHS /opt/homebrew/lib /usr/local/lib)
  find_library(HOLONS_CRYPTO_LIBRARY NAMES crypto libcrypto PATHS /opt/homebrew/lib /usr/local/lib)
  if (HOLONS_SSL_LIBRARY AND HOLONS_CRYPTO_LIBRARY)
    list(APPEND HOLONS_OPENSSL_LIBRARIES "${HOLONS_SSL_LIBRARY}" "${HOLONS_CRYPTO_LIBRARY}")
  endif()
endif()

if (gRPC_FOUND)
  target_link_libraries(test_runner PRIVATE gRPC::grpc++)
else()
  find_library(GRPCPP_LIBRARY NAMES grpc++ PATHS /opt/homebrew/lib /usr/local/lib)
  if (GRPCPP_LIBRARY)
    target_link_libraries(test_runner PRIVATE "${GRPCPP_LIBRARY}")
  endif()
endif()

if (Protobuf_FOUND AND TARGET protobuf::libprotobuf)
  target_link_libraries(test_runner PRIVATE protobuf::libprotobuf)
elseif (Protobuf_FOUND AND Protobuf_LIBRARIES)
  target_link_libraries(test_runner PRIVATE ${Protobuf_LIBRARIES})
endif()

if (HOLONS_OPENSSL_LIBRARIES)
  target_link_libraries(test_runner PRIVATE ${HOLONS_OPENSSL_LIBRARIES})
endif()

if (TARGET gRPC::grpc++_reflection)
  target_link_libraries(test_runner PRIVATE gRPC::grpc++_reflection)
else()
  find_library(GRPCPP_REFLECTION_LIBRARY NAMES grpc++_reflection PATHS /opt/homebrew/lib /usr/local/lib)
  if (GRPCPP_REFLECTION_LIBRARY)
    target_link_libraries(test_runner PRIVATE "${GRPCPP_REFLECTION_LIBRARY}")
  endif()
endif()

if (WIN32)
  target_link_libraries(test_runner PRIVATE ws2_32)
endif()

set(HOLONS_CANONICAL_PROTO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../_protos")
if (NOT EXISTS "${HOLONS_CANONICAL_PROTO_ROOT}")
  set(HOLONS_CANONICAL_PROTO_ROOT
      "${CMAKE_CURRENT_SOURCE_DIR}/../../holons/grace-op/_protos")
endif()
set(HOLONS_LOCAL_PROTO_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/protos")
set(HOLONS_GENERATED_DIR "${CMAKE_CURRENT_BINARY_DIR}/generated")

if (TARGET protobuf::protoc)
  set(HOLONS_PROTOC_EXECUTABLE $<TARGET_FILE:protobuf::protoc>)
elseif (DEFINED Protobuf_PROTOC_EXECUTABLE)
  set(HOLONS_PROTOC_EXECUTABLE "${Protobuf_PROTOC_EXECUTABLE}")
else()
  find_program(HOLONS_PROTOC_EXECUTABLE protoc)
endif()

if (TARGET gRPC::grpc_cpp_plugin)
  set(HOLONS_GRPC_CPP_PLUGIN $<TARGET_FILE:gRPC::grpc_cpp_plugin>)
else()
  find_program(HOLONS_GRPC_CPP_PLUGIN grpc_cpp_plugin PATHS /opt/homebrew/bin /usr/local/bin)
endif()

function(holons_generate_grpc_cpp target_name)
  set(options)
  set(one_value_args OUT_DIR)
  set(multi_value_args PROTOS IMPORT_DIRS)
  cmake_parse_arguments(HG "${options}" "${one_value_args}" "${multi_value_args}" ${ARGN})

  if (NOT HG_OUT_DIR)
    set(HG_OUT_DIR "${HOLONS_GENERATED_DIR}")
  endif()

  if (NOT gRPC_FOUND OR NOT Protobuf_FOUND OR NOT HOLONS_PROTOC_EXECUTABLE OR
      NOT HOLONS_GRPC_CPP_PLUGIN)
    message(STATUS "Skipping ${target_name}: protoc/grpc_cpp_plugin or gRPC CMake package unavailable")
    return()
  endif()

  file(MAKE_DIRECTORY "${HG_OUT_DIR}")

  set(generated_sources)
  set(generated_headers)
  foreach(proto IN LISTS HG_PROTOS)
    set(proto_relative "")
    foreach(import_dir IN LISTS HG_IMPORT_DIRS)
      file(RELATIVE_PATH candidate "${import_dir}" "${proto}")
      if (NOT candidate MATCHES "^\\.\\.")
        set(proto_relative "${candidate}")
        break()
      endif()
    endforeach()
    if (NOT proto_relative)
      file(RELATIVE_PATH proto_relative "${CMAKE_CURRENT_SOURCE_DIR}" "${proto}")
    endif()

    get_filename_component(proto_dir "${proto_relative}" DIRECTORY)
    get_filename_component(proto_name "${proto}" NAME_WE)

    list(APPEND generated_sources
         "${HG_OUT_DIR}/${proto_dir}/${proto_name}.pb.cc"
         "${HG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.cc")
    list(APPEND generated_headers
         "${HG_OUT_DIR}/${proto_dir}/${proto_name}.pb.h"
         "${HG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.h")

    set(import_args)
    foreach(import_dir IN LISTS HG_IMPORT_DIRS)
      list(APPEND import_args -I "${import_dir}")
    endforeach()

    add_custom_command(
      OUTPUT
        "${HG_OUT_DIR}/${proto_dir}/${proto_name}.pb.cc"
        "${HG_OUT_DIR}/${proto_dir}/${proto_name}.pb.h"
        "${HG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.cc"
        "${HG_OUT_DIR}/${proto_dir}/${proto_name}.grpc.pb.h"
      COMMAND "${HOLONS_PROTOC_EXECUTABLE}"
        --cpp_out=${HG_OUT_DIR}
        --grpc_out=${HG_OUT_DIR}
        --plugin=protoc-gen-grpc=${HOLONS_GRPC_CPP_PLUGIN}
        ${import_args}
        "${proto}"
      DEPENDS "${proto}"
      VERBATIM
    )
  endforeach()

  add_library(${target_name} STATIC ${generated_sources} ${generated_headers})
  target_include_directories(${target_name} PUBLIC "${HG_OUT_DIR}")
  target_link_libraries(${target_name} PUBLIC gRPC::grpc++)
  if (TARGET protobuf::libprotobuf)
    target_link_libraries(${target_name} PUBLIC protobuf::libprotobuf)
  elseif (Protobuf_LIBRARIES)
    target_link_libraries(${target_name} PUBLIC ${Protobuf_LIBRARIES})
  endif()
endfunction()

holons_generate_grpc_cpp(holons_proto_grpc
  OUT_DIR "${HOLONS_GENERATED_DIR}"
  PROTOS
    "${HOLONS_CANONICAL_PROTO_ROOT}/holons/v1/manifest.proto"
    "${HOLONS_CANONICAL_PROTO_ROOT}/holons/v1/describe.proto"
    "${HOLONS_CANONICAL_PROTO_ROOT}/holons/v1/coax.proto"
  IMPORT_DIRS "${HOLONS_CANONICAL_PROTO_ROOT}"
)

holons_generate_grpc_cpp(echo_grpc
  OUT_DIR "${HOLONS_GENERATED_DIR}"
  PROTOS "${HOLONS_LOCAL_PROTO_ROOT}/echo/v1/echo.proto"
  IMPORT_DIRS "${HOLONS_LOCAL_PROTO_ROOT}"
)

if (TARGET holons_proto_grpc)
  target_link_libraries(test_runner PRIVATE holons_proto_grpc)
endif()

if (TARGET holons_proto_grpc AND TARGET echo_grpc)
  add_executable(cpp_echo_server examples/echo_server.cpp)
  target_link_libraries(cpp_echo_server PRIVATE holons holons_proto_grpc echo_grpc)
  if (HOLONS_OPENSSL_LIBRARIES)
    target_link_libraries(cpp_echo_server PRIVATE ${HOLONS_OPENSSL_LIBRARIES})
  endif()
  target_include_directories(cpp_echo_server PRIVATE "${HOLONS_GENERATED_DIR}")
  if (TARGET gRPC::grpc++_reflection)
    target_link_libraries(cpp_echo_server PRIVATE gRPC::grpc++_reflection)
  else()
    find_library(GRPCPP_REFLECTION_LIBRARY NAMES grpc++_reflection PATHS /opt/homebrew/lib /usr/local/lib)
    if (GRPCPP_REFLECTION_LIBRARY)
      target_link_libraries(cpp_echo_server PRIVATE "${GRPCPP_REFLECTION_LIBRARY}")
    endif()
  endif()
endif()
