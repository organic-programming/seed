cmake_minimum_required(VERSION 3.21)
project(gabriel-greeting-c LANGUAGES C CXX)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)
set(CMAKE_C_EXTENSIONS OFF)
set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

enable_testing()

find_package(Protobuf CONFIG REQUIRED)
find_package(gRPC CONFIG REQUIRED)
find_package(utf8_range CONFIG REQUIRED)
find_program(BASH_EXECUTABLE NAMES bash REQUIRED)

get_target_property(PROTOBUF_INCLUDE_DIRS protobuf::libprotobuf INTERFACE_INCLUDE_DIRECTORIES)
find_library(UPB_LIBRARY NAMES upb REQUIRED)

set(HOLONS_SDK_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../sdk/c-holons")
set(CPP_HOLONS_SDK_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../sdk/cpp-holons")
set(GABRIEL_GREETING_C_RELOCATE_SCRIPT "${CMAKE_CURRENT_SOURCE_DIR}/../_scripts/relocate-macos-dylibs.sh")

if(CMAKE_RUNTIME_OUTPUT_DIRECTORY)
  set(GABRIEL_GREETING_C_RUNTIME_DIR "${CMAKE_RUNTIME_OUTPUT_DIRECTORY}")
else()
  set(GABRIEL_GREETING_C_RUNTIME_DIR "${CMAKE_CURRENT_BINARY_DIR}")
endif()

if(NOT DEFINED OP_PACKAGE_ARCH OR OP_PACKAGE_ARCH STREQUAL "")
  string(TOLOWER "${CMAKE_SYSTEM_NAME}" OP_PACKAGE_OS)
  set(OP_PACKAGE_ARCH "${OP_PACKAGE_OS}_${CMAKE_SYSTEM_PROCESSOR}")
endif()
if(NOT DEFINED OP_USE_INSTALL_LAYOUT)
  set(OP_USE_INSTALL_LAYOUT OFF)
endif()
set(GABRIEL_GREETING_C_INSTALL_RUNTIME_DIR "${CMAKE_INSTALL_PREFIX}/bin/${OP_PACKAGE_ARCH}")

add_library(gabriel_greeting_c_lib
  api/public.c
  api/cli.c
  internal/greetings.c
  internal/server.c
  gen/describe_generated.c
  gen/serve_generated.c
  gen/c/greeting/v1/greeting.upb.c
  gen/c/greeting/v1/greeting.upb_minitable.c
  "${HOLONS_SDK_ROOT}/src/holons.c"
  "${HOLONS_SDK_ROOT}/src/serve_grpc.cpp"
)

target_include_directories(gabriel_greeting_c_lib
  PUBLIC
    "${CMAKE_CURRENT_SOURCE_DIR}"
    "${CMAKE_CURRENT_SOURCE_DIR}/gen/c/greeting/v1"
    "${HOLONS_SDK_ROOT}/include"
    "${CPP_HOLONS_SDK_ROOT}/include"
    ${PROTOBUF_INCLUDE_DIRS}
)

target_link_libraries(gabriel_greeting_c_lib
  PUBLIC
    "${UPB_LIBRARY}"
    gRPC::grpc++
    gRPC::grpc++_reflection
    protobuf::libprotobuf
    utf8_range::utf8_range
)

add_executable(gabriel-greeting-c cmd/main.c)
target_link_libraries(gabriel-greeting-c PRIVATE gabriel_greeting_c_lib)
set_target_properties(gabriel-greeting-c PROPERTIES LINKER_LANGUAGE CXX)

if(APPLE AND NOT OP_USE_INSTALL_LAYOUT)
  set(GABRIEL_GREETING_C_RELOCATE_STAMP "${GABRIEL_GREETING_C_RUNTIME_DIR}/.dylibs-relocated")
  add_custom_command(
    OUTPUT "${GABRIEL_GREETING_C_RELOCATE_STAMP}"
    COMMAND "${BASH_EXECUTABLE}" "${GABRIEL_GREETING_C_RELOCATE_SCRIPT}" "${GABRIEL_GREETING_C_RUNTIME_DIR}" gabriel-greeting-c
    COMMAND "${CMAKE_COMMAND}" -E touch "${GABRIEL_GREETING_C_RELOCATE_STAMP}"
    DEPENDS gabriel-greeting-c "${GABRIEL_GREETING_C_RELOCATE_SCRIPT}"
    VERBATIM
  )
  add_custom_target(gabriel_greeting_c_relocate ALL DEPENDS "${GABRIEL_GREETING_C_RELOCATE_STAMP}")
endif()

add_executable(gabriel_greeting_c_public_test api/public_test.c)
target_link_libraries(gabriel_greeting_c_public_test PRIVATE gabriel_greeting_c_lib)
set_target_properties(gabriel_greeting_c_public_test PROPERTIES LINKER_LANGUAGE CXX)

add_executable(gabriel_greeting_c_cli_test api/cli_test.c)
target_link_libraries(gabriel_greeting_c_cli_test PRIVATE gabriel_greeting_c_lib)
set_target_properties(gabriel_greeting_c_cli_test PROPERTIES LINKER_LANGUAGE CXX)

add_executable(gabriel_greeting_c_server_test internal/server_test.c)
target_compile_definitions(gabriel_greeting_c_server_test
  PRIVATE
    GABRIEL_GREETING_C_PUBLIC_BINARY="$<TARGET_FILE:gabriel-greeting-c>"
    GABRIEL_GREETING_C_PROTO_DIR="${CMAKE_CURRENT_SOURCE_DIR}/../../_protos"
)
target_link_libraries(gabriel_greeting_c_server_test PRIVATE gabriel_greeting_c_lib)
add_dependencies(gabriel_greeting_c_server_test gabriel-greeting-c)
set_target_properties(gabriel_greeting_c_server_test PROPERTIES LINKER_LANGUAGE CXX)

add_test(NAME gabriel_greeting_c_public_test COMMAND gabriel_greeting_c_public_test)
add_test(NAME gabriel_greeting_c_cli_test COMMAND gabriel_greeting_c_cli_test)
add_test(NAME gabriel_greeting_c_server_test COMMAND gabriel_greeting_c_server_test)

install(TARGETS gabriel-greeting-c
  RUNTIME DESTINATION "bin/${OP_PACKAGE_ARCH}"
)

if(APPLE)
  install(CODE "
    execute_process(
      COMMAND \"${BASH_EXECUTABLE}\" \"${GABRIEL_GREETING_C_RELOCATE_SCRIPT}\" \"${GABRIEL_GREETING_C_INSTALL_RUNTIME_DIR}\" gabriel-greeting-c
      COMMAND_ERROR_IS_FATAL ANY
    )
  ")
endif()
