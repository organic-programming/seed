cmake_minimum_required(VERSION 3.21)
project(gabriel-greeting-c LANGUAGES C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)
set(CMAKE_C_EXTENSIONS OFF)

enable_testing()

find_package(Protobuf CONFIG REQUIRED)
find_package(utf8_range CONFIG REQUIRED)

get_target_property(PROTOBUF_INCLUDE_DIRS protobuf::libprotobuf INTERFACE_INCLUDE_DIRECTORIES)
find_library(UPB_LIBRARY NAMES upb REQUIRED)

set(HOLONS_SDK_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../sdk/c-holons")
set(GABRIEL_GREETING_C_BACKEND_BINARY "${CMAKE_CURRENT_BINARY_DIR}/gabriel-greeting-c-backend")
set(GABRIEL_GREETING_C_BRIDGE_BINARY "${HOLONS_SDK_ROOT}/bin/grpc-bridge")
set(GABRIEL_GREETING_C_PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../_protos")
set(GABRIEL_GREETING_C_MANIFEST_PATH "${CMAKE_CURRENT_SOURCE_DIR}/api/v1/holon.proto")

add_library(gabriel_greeting_c_lib
  api/public.c
  api/cli.c
  internal/greetings.c
  internal/server.c
  gen/c/greeting/v1/greeting.upb.c
  gen/c/greeting/v1/greeting.upb_minitable.c
  "${HOLONS_SDK_ROOT}/src/holons.c"
)

target_include_directories(gabriel_greeting_c_lib
  PUBLIC
    "${CMAKE_CURRENT_SOURCE_DIR}"
    "${CMAKE_CURRENT_SOURCE_DIR}/gen/c/greeting/v1"
    "${HOLONS_SDK_ROOT}/include"
    ${PROTOBUF_INCLUDE_DIRS}
)

target_compile_definitions(gabriel_greeting_c_lib
  PRIVATE
    GABRIEL_GREETING_C_BACKEND_BINARY="${GABRIEL_GREETING_C_BACKEND_BINARY}"
    GABRIEL_GREETING_C_BRIDGE_BINARY="${GABRIEL_GREETING_C_BRIDGE_BINARY}"
    GABRIEL_GREETING_C_PROTO_DIR="${GABRIEL_GREETING_C_PROTO_DIR}"
    GABRIEL_GREETING_C_MANIFEST_PATH="${GABRIEL_GREETING_C_MANIFEST_PATH}"
)

target_link_libraries(gabriel_greeting_c_lib
  PUBLIC
    "${UPB_LIBRARY}"
    utf8_range::utf8_range
)

add_executable(gabriel-greeting-c cmd/main.c)
target_link_libraries(gabriel-greeting-c PRIVATE gabriel_greeting_c_lib)

add_executable(gabriel-greeting-c-backend cmd/backend_main.c)
target_link_libraries(gabriel-greeting-c-backend PRIVATE gabriel_greeting_c_lib)

add_executable(gabriel_greeting_c_public_test api/public_test.c)
target_link_libraries(gabriel_greeting_c_public_test PRIVATE gabriel_greeting_c_lib)

add_executable(gabriel_greeting_c_cli_test api/cli_test.c)
target_link_libraries(gabriel_greeting_c_cli_test PRIVATE gabriel_greeting_c_lib)

add_executable(gabriel_greeting_c_server_test internal/server_test.c)
target_compile_definitions(gabriel_greeting_c_server_test
  PRIVATE
    GABRIEL_GREETING_C_PUBLIC_BINARY="${CMAKE_CURRENT_BINARY_DIR}/gabriel-greeting-c"
    GABRIEL_GREETING_C_PROTO_DIR="${GABRIEL_GREETING_C_PROTO_DIR}"
)
target_link_libraries(gabriel_greeting_c_server_test PRIVATE gabriel_greeting_c_lib)

add_test(NAME gabriel_greeting_c_public_test COMMAND gabriel_greeting_c_public_test)
add_test(NAME gabriel_greeting_c_cli_test COMMAND gabriel_greeting_c_cli_test)
add_test(NAME gabriel_greeting_c_server_test COMMAND gabriel_greeting_c_server_test)
