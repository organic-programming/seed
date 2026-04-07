cmake_minimum_required(VERSION 3.21)
project(gabriel-greeting-c LANGUAGES C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)
set(CMAKE_C_EXTENSIONS OFF)

enable_testing()

find_package(Protobuf CONFIG REQUIRED)
find_package(utf8_range CONFIG REQUIRED)
find_program(GO_EXECUTABLE NAMES go REQUIRED)
find_program(BASH_EXECUTABLE NAMES bash REQUIRED)

get_target_property(PROTOBUF_INCLUDE_DIRS protobuf::libprotobuf INTERFACE_INCLUDE_DIRECTORIES)
find_library(UPB_LIBRARY NAMES upb REQUIRED)

set(HOLONS_SDK_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../sdk/c-holons")
if(CMAKE_RUNTIME_OUTPUT_DIRECTORY)
  set(GABRIEL_GREETING_C_RUNTIME_DIR "${CMAKE_RUNTIME_OUTPUT_DIRECTORY}")
else()
  set(GABRIEL_GREETING_C_RUNTIME_DIR "${CMAKE_CURRENT_BINARY_DIR}")
endif()
set(GABRIEL_GREETING_C_BACKEND_BINARY "${GABRIEL_GREETING_C_RUNTIME_DIR}/gabriel-greeting-c-backend")
set(GABRIEL_GREETING_C_BRIDGE_BINARY "${GABRIEL_GREETING_C_RUNTIME_DIR}/grpc-bridge")
set(GABRIEL_GREETING_C_PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../_protos")
set(GABRIEL_GREETING_C_MANIFEST_PATH "${CMAKE_CURRENT_SOURCE_DIR}/api/v1/holon.proto")
set(GABRIEL_GREETING_C_DESCRIBE_STATIC_PATH "${GABRIEL_GREETING_C_RUNTIME_DIR}/describe_generated.json")
set(GABRIEL_GREETING_C_RELOCATE_SCRIPT "${CMAKE_CURRENT_SOURCE_DIR}/../_scripts/relocate-macos-dylibs.sh")

file(MAKE_DIRECTORY "${GABRIEL_GREETING_C_RUNTIME_DIR}")
add_custom_command(
  OUTPUT "${GABRIEL_GREETING_C_BRIDGE_BINARY}"
  COMMAND "${GO_EXECUTABLE}" build -o "${GABRIEL_GREETING_C_BRIDGE_BINARY}"
          "${HOLONS_SDK_ROOT}/cmd/grpc-bridge-go/main.go"
          "${HOLONS_SDK_ROOT}/cmd/grpc-bridge-go/static_registry.go"
  DEPENDS
    "${HOLONS_SDK_ROOT}/cmd/grpc-bridge-go/main.go"
    "${HOLONS_SDK_ROOT}/cmd/grpc-bridge-go/static_registry.go"
  WORKING_DIRECTORY "${HOLONS_SDK_ROOT}/../go-holons"
  VERBATIM
)
add_custom_target(gabriel_greeting_c_bridge ALL DEPENDS "${GABRIEL_GREETING_C_BRIDGE_BINARY}")

add_custom_command(
  OUTPUT "${GABRIEL_GREETING_C_DESCRIBE_STATIC_PATH}"
  COMMAND "${GO_EXECUTABLE}" run "${HOLONS_SDK_ROOT}/cmd/describe-json-go/main.go"
          --proto-dir "${GABRIEL_GREETING_C_PROTO_DIR}"
          --manifest "${GABRIEL_GREETING_C_MANIFEST_PATH}"
          --out "${GABRIEL_GREETING_C_DESCRIBE_STATIC_PATH}"
  DEPENDS
    "${HOLONS_SDK_ROOT}/cmd/describe-json-go/main.go"
    "${CMAKE_CURRENT_SOURCE_DIR}/api/v1/holon.proto"
  WORKING_DIRECTORY "${HOLONS_SDK_ROOT}/../go-holons"
  VERBATIM
)
add_custom_target(gabriel_greeting_c_static_describe ALL DEPENDS "${GABRIEL_GREETING_C_DESCRIBE_STATIC_PATH}")

add_library(gabriel_greeting_c_lib
  api/public.c
  api/cli.c
  internal/greetings.c
  internal/server.c
  gen/describe_generated.c
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
    GABRIEL_GREETING_C_DESCRIBE_STATIC_PATH="${GABRIEL_GREETING_C_DESCRIBE_STATIC_PATH}"
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
add_dependencies(gabriel-greeting-c gabriel-greeting-c-backend gabriel_greeting_c_bridge gabriel_greeting_c_static_describe)
add_dependencies(gabriel-greeting-c-backend gabriel_greeting_c_static_describe)

if(APPLE)
  set(GABRIEL_GREETING_C_RELOCATE_STAMP "${GABRIEL_GREETING_C_RUNTIME_DIR}/.dylibs-relocated")
  add_custom_command(
    OUTPUT "${GABRIEL_GREETING_C_RELOCATE_STAMP}"
    COMMAND "${BASH_EXECUTABLE}" "${GABRIEL_GREETING_C_RELOCATE_SCRIPT}" "${GABRIEL_GREETING_C_RUNTIME_DIR}" gabriel-greeting-c gabriel-greeting-c-backend
    COMMAND "${CMAKE_COMMAND}" -E touch "${GABRIEL_GREETING_C_RELOCATE_STAMP}"
    DEPENDS gabriel-greeting-c gabriel-greeting-c-backend "${GABRIEL_GREETING_C_RELOCATE_SCRIPT}"
    VERBATIM
  )
  add_custom_target(gabriel_greeting_c_relocate ALL DEPENDS "${GABRIEL_GREETING_C_RELOCATE_STAMP}")
endif()

add_executable(gabriel_greeting_c_public_test api/public_test.c)
target_link_libraries(gabriel_greeting_c_public_test PRIVATE gabriel_greeting_c_lib)

add_executable(gabriel_greeting_c_cli_test api/cli_test.c)
target_link_libraries(gabriel_greeting_c_cli_test PRIVATE gabriel_greeting_c_lib)

add_executable(gabriel_greeting_c_server_test internal/server_test.c)
target_compile_definitions(gabriel_greeting_c_server_test
  PRIVATE
    GABRIEL_GREETING_C_PUBLIC_BINARY="${GABRIEL_GREETING_C_RUNTIME_DIR}/gabriel-greeting-c"
    GABRIEL_GREETING_C_PROTO_DIR="${GABRIEL_GREETING_C_PROTO_DIR}"
)
target_link_libraries(gabriel_greeting_c_server_test PRIVATE gabriel_greeting_c_lib)
add_dependencies(gabriel_greeting_c_server_test gabriel-greeting-c gabriel_greeting_c_bridge gabriel_greeting_c_static_describe)

add_test(NAME gabriel_greeting_c_public_test COMMAND gabriel_greeting_c_public_test)
add_test(NAME gabriel_greeting_c_cli_test COMMAND gabriel_greeting_c_cli_test)
add_test(NAME gabriel_greeting_c_server_test COMMAND gabriel_greeting_c_server_test)
