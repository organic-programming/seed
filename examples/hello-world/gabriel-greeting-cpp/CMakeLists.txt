cmake_minimum_required(VERSION 3.21)
project(gabriel-greeting-cpp LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

enable_testing()

find_package(Protobuf CONFIG REQUIRED)
find_package(gRPC CONFIG REQUIRED)

set(HOLONS_SDK_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../sdk/cpp-holons")

add_library(gabriel_greeting_cpp_lib
  api/public.cpp
  api/cli.cpp
  internal/greetings.cpp
  internal/server.cpp
  gen/cpp/greeting/v1/greeting.pb.cc
  gen/cpp/greeting/v1/greeting.grpc.pb.cc
  "${HOLONS_SDK_ROOT}/gen/cpp/holonmeta/v1/holonmeta.pb.cc"
  "${HOLONS_SDK_ROOT}/gen/cpp/holonmeta/v1/holonmeta.grpc.pb.cc"
)

target_include_directories(gabriel_greeting_cpp_lib
  PUBLIC
    "${CMAKE_CURRENT_SOURCE_DIR}"
    "${CMAKE_CURRENT_SOURCE_DIR}/gen/cpp/greeting/v1"
    "${HOLONS_SDK_ROOT}/include"
    "${HOLONS_SDK_ROOT}/gen/cpp"
)

target_link_libraries(gabriel_greeting_cpp_lib
  PUBLIC
    gRPC::grpc++
    gRPC::grpc++_reflection
    protobuf::libprotobuf
)

add_executable(gabriel-greeting-cpp
  cmd/main.cpp
)
target_link_libraries(gabriel-greeting-cpp PRIVATE gabriel_greeting_cpp_lib)

add_executable(gabriel_greeting_cpp_public_test api/public_test.cpp)
target_link_libraries(gabriel_greeting_cpp_public_test PRIVATE gabriel_greeting_cpp_lib)

add_executable(gabriel_greeting_cpp_cli_test api/cli_test.cpp)
target_link_libraries(gabriel_greeting_cpp_cli_test PRIVATE gabriel_greeting_cpp_lib)

add_executable(gabriel_greeting_cpp_server_test internal/server_test.cpp)
target_link_libraries(gabriel_greeting_cpp_server_test PRIVATE gabriel_greeting_cpp_lib)

add_test(NAME gabriel_greeting_cpp_public_test COMMAND gabriel_greeting_cpp_public_test)
add_test(NAME gabriel_greeting_cpp_cli_test COMMAND gabriel_greeting_cpp_cli_test)
add_test(NAME gabriel_greeting_cpp_server_test COMMAND gabriel_greeting_cpp_server_test)
