cmake_minimum_required(VERSION 3.21)
project(gudule-daemon-greeting-c LANGUAGES C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)

# Instead of fetching grpc, we compile the C code directly bridging libholons
add_executable(gudule-daemon-greeting-c
  src/main.c
  ../../../sdk/c-holons/src/holons.c
)

target_include_directories(gudule-daemon-greeting-c PRIVATE
  ../../../sdk/c-holons/include
)
