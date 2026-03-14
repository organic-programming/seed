cmake_minimum_required(VERSION 3.21)
project(gudule-daemon-greeting-c LANGUAGES C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)

# The C business logic stays in a backend binary. The public daemon artifact
# is an SDK-owned gRPC bridge wrapper so the holon satisfies connect(slug).
set(BACKEND_NAME gudule-daemon-greeting-c-backend)
set(WRAPPER_NAME gudule-daemon-greeting-c)
set(SDK_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../sdk/c-holons")
set(GREETING_PROTO_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../protos/greeting")
set(HOLON_YAML "${CMAKE_CURRENT_SOURCE_DIR}/holon.yaml")
set(WRAPPER_TEMPLATE "${CMAKE_CURRENT_SOURCE_DIR}/src/gudule-daemon-greeting-c.sh.in")
set(WRAPPER_OUTPUT "${CMAKE_RUNTIME_OUTPUT_DIRECTORY}/${WRAPPER_NAME}")

add_executable(${BACKEND_NAME}
  src/main.c
  ${SDK_ROOT}/src/holons.c
)

target_include_directories(${BACKEND_NAME} PRIVATE
  ${SDK_ROOT}/include
)

file(MAKE_DIRECTORY "${CMAKE_RUNTIME_OUTPUT_DIRECTORY}")
configure_file(
  ${WRAPPER_TEMPLATE}
  ${WRAPPER_OUTPUT}
  @ONLY
  FILE_PERMISSIONS
    OWNER_READ OWNER_WRITE OWNER_EXECUTE
    GROUP_READ GROUP_EXECUTE
    WORLD_READ WORLD_EXECUTE
)
