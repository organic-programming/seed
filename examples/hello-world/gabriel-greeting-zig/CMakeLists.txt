cmake_minimum_required(VERSION 3.20)

project(gabriel_greeting_zig NONE)

find_program(ZIG_EXECUTABLE zig REQUIRED)

if(NOT CMAKE_RUNTIME_OUTPUT_DIRECTORY)
  set(CMAKE_RUNTIME_OUTPUT_DIRECTORY "${CMAKE_BINARY_DIR}/bin")
endif()

set(ZIG_PREFIX "${CMAKE_BINARY_DIR}/zig-prefix")
set(ZIG_BINARY "${ZIG_PREFIX}/bin/gabriel-greeting-zig")
set(OP_BINARY "${CMAKE_RUNTIME_OUTPUT_DIRECTORY}/gabriel-greeting-zig")

add_custom_target(gabriel-greeting-zig ALL
  COMMAND "${ZIG_EXECUTABLE}" build --prefix "${ZIG_PREFIX}"
  COMMAND "${CMAKE_COMMAND}" -E make_directory "${CMAKE_RUNTIME_OUTPUT_DIRECTORY}"
  COMMAND "${CMAKE_COMMAND}" -E copy "${ZIG_BINARY}" "${OP_BINARY}"
  WORKING_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}"
  VERBATIM
)

enable_testing()
add_test(
  NAME zig-unit
  COMMAND "${ZIG_EXECUTABLE}" build test
  WORKING_DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}"
)
