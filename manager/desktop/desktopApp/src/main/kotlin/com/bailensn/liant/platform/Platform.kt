package com.bailensn.liant.platform

enum class Platform {
    Windows,
    Linux,
    Mac
}

val currentPlatform: Platform
    get(){
        val os =
            System.getProperty("os.name")
                .lowercase()
        return when{
            os.contains("mac") ->
                Platform.Mac
            os.contains("win") ->
                Platform.Windows
            else ->
                Platform.Linux
        }
    }