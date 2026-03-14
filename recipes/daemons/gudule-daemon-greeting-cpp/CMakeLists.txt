cmake_minimum_required(VERSION 3.21)
project(gudule-daemon-greeting-cpp LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(gRPC CONFIG REQUIRED)
find_package(Protobuf CONFIG REQUIRED)

# Try to find the greeting interface proto.
# The `cpp-holons` SDK requires we include it.
# We also need to compile the proto.
add_executable(gudule-daemon-greeting-cpp
  src/main.cpp
)

set(HOLONS_SDK_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../../../sdk/cpp-holons")
set(HOLONS_SDK_GEN_DIR "${HOLONS_SDK_ROOT}/gen/cpp")
set(HOLONMETA_PROTO_SRC "${HOLONS_SDK_GEN_DIR}/holonmeta/v1/holonmeta.pb.cc")
set(HOLONMETA_GRPC_SRC "${HOLONS_SDK_GEN_DIR}/holonmeta/v1/holonmeta.grpc.pb.cc")

# Target include directories for the SDK
target_include_directories(gudule-daemon-greeting-cpp PRIVATE
  "${HOLONS_SDK_ROOT}/include"
  "${HOLONS_SDK_GEN_DIR}"
  ${CMAKE_CURRENT_BINARY_DIR}
)

# Protobuf compilation setup
set(PROTO_FILE "${CMAKE_CURRENT_SOURCE_DIR}/../../protos/greeting/v1/greeting.proto")

# Generate protoc files
get_target_property(grpc_cpp_plugin_location gRPC::grpc_cpp_plugin LOCATION)

set(PROTO_GEN_DIR "${CMAKE_CURRENT_BINARY_DIR}")
set(PROTO_SRC "${PROTO_GEN_DIR}/greeting/v1/greeting.pb.cc")
set(PROTO_HDR "${PROTO_GEN_DIR}/greeting/v1/greeting.pb.h")
set(GRPC_SRC "${PROTO_GEN_DIR}/greeting/v1/greeting.grpc.pb.cc")
set(GRPC_HDR "${PROTO_GEN_DIR}/greeting/v1/greeting.grpc.pb.h")

add_custom_command(
    OUTPUT "${PROTO_SRC}" "${PROTO_HDR}" "${GRPC_SRC}" "${GRPC_HDR}"
    COMMAND ${CMAKE_COMMAND} -E make_directory "${PROTO_GEN_DIR}/greeting/v1"
    COMMAND $<TARGET_FILE:protobuf::protoc>
    --cpp_out="${PROTO_GEN_DIR}"
    --grpc_out="${PROTO_GEN_DIR}"
    --plugin=protoc-gen-grpc="${grpc_cpp_plugin_location}"
    -I"${CMAKE_CURRENT_SOURCE_DIR}/../../protos"
    "${PROTO_FILE}"
    DEPENDS "${PROTO_FILE}"
)

# Add the generated source files to the executable
target_sources(gudule-daemon-greeting-cpp PRIVATE
  "${PROTO_SRC}"
  "${GRPC_SRC}"
  "${HOLONMETA_PROTO_SRC}"
  "${HOLONMETA_GRPC_SRC}"
)

target_link_libraries(gudule-daemon-greeting-cpp PRIVATE
  gRPC::grpc++
  gRPC::grpc++_reflection
  protobuf::libprotobuf
)
