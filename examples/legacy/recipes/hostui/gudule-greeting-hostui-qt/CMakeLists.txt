cmake_minimum_required(VERSION 3.21)
project(gudule-greeting-goqt LANGUAGES CXX)

set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(Qt6 6.5 COMPONENTS Widgets REQUIRED)
find_package(nlohmann_json CONFIG QUIET)
if(NOT nlohmann_json_FOUND)
  include(FetchContent)
  FetchContent_Declare(
    nlohmann_json
    URL https://github.com/nlohmann/json/releases/download/v3.11.3/json.tar.xz
  )
  FetchContent_MakeAvailable(nlohmann_json)
endif()

add_executable(gudule-greeting-goqt
  src/main.cpp
  src/MainWindow.cpp
  src/DaemonProcess.cpp
  src/GreetingClient.cpp
)

target_include_directories(gudule-greeting-goqt PRIVATE
  src
  ../../../sdk/cpp-holons/include
)
target_compile_definitions(gudule-greeting-goqt PRIVATE NOMINMAX)
target_link_libraries(gudule-greeting-goqt PRIVATE Qt6::Widgets nlohmann_json::nlohmann_json)
