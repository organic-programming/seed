cmake_minimum_required(VERSION 3.16)

project(cpp_hello_world LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

find_package(Threads REQUIRED)
find_package(gRPC CONFIG QUIET)

add_library(cpp_hello_support INTERFACE)
target_include_directories(
  cpp_hello_support
  INTERFACE
    ${CMAKE_CURRENT_SOURCE_DIR}
    ${CMAKE_CURRENT_SOURCE_DIR}/../../sdk/cpp-holons/include
    /opt/homebrew/include
    /usr/local/include
)
target_link_libraries(cpp_hello_support INTERFACE Threads::Threads)

if (gRPC_FOUND)
  target_link_libraries(cpp_hello_support INTERFACE gRPC::grpc++)
else()
  find_library(GRPCPP_LIBRARY NAMES grpc++ PATHS /opt/homebrew/lib /usr/local/lib)
  if (GRPCPP_LIBRARY)
    target_link_libraries(cpp_hello_support INTERFACE "${GRPCPP_LIBRARY}")
  endif()
endif()

enable_testing()

add_executable(cpp-hello-world hello.cpp)
target_link_libraries(cpp-hello-world PRIVATE cpp_hello_support)

add_executable(connect_example connect_example.cpp)
target_link_libraries(connect_example PRIVATE cpp_hello_support)

add_executable(test_hello test_hello.cpp)
target_link_libraries(test_hello PRIVATE cpp_hello_support)
add_test(NAME hello_unit_tests COMMAND test_hello)
