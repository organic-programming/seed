cmake_minimum_required(VERSION 3.16)
project(charon-direct-c-go-orchestrator C)
set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)
# We don't link the c-holons SDK mostly because we only need plain POSIX, but we can if we want holons.
add_executable(charon-direct-c-go-orchestrator src/main.c)
