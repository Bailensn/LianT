cmake_minimum_required(VERSION 3.22.1)

# target 名决定产物名：LianT -> libLianT.so，必须和 Kotlin 侧 System.loadLibrary("LianT") 一致。
project(LianT LANGUAGES CXX)

add_library(${CMAKE_PROJECT_NAME} SHARED native-lib.cpp)

target_compile_features(${CMAKE_PROJECT_NAME} PRIVATE cxx_std_23)

set_target_properties(${CMAKE_PROJECT_NAME} PROPERTIES
    CXX_EXTENSIONS OFF
    POSITION_INDEPENDENT_CODE ON)

target_compile_options(${CMAKE_PROJECT_NAME} PRIVATE
    -Wall
    -Wextra
    -Wpedantic
    $<$<CONFIG:Debug>:-O0>
    $<$<CONFIG:Debug>:-g>)

find_library(android-lib android)
find_library(log-lib log)

target_link_libraries(${CMAKE_PROJECT_NAME}
    ${android-lib}
    ${log-lib})
