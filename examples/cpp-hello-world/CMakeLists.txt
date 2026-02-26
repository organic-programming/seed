cmake_minimum_required(VERSION 3.16)

project(cpp_hello_world LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

enable_testing()

add_executable(test_hello test_hello.cpp)
add_test(NAME hello_unit_tests COMMAND test_hello)
