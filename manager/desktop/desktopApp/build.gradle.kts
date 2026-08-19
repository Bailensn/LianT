plugins {
    kotlin("jvm")
    id("org.jetbrains.compose")
}

repositories {
    google()
    mavenCentral()
}

dependencies {
    implementation(compose.desktop.currentOs)
}

compose.desktop {
    application {
        mainClass = "com.bailensn.liant.MainKt"

        nativeDistributions {
            packageName = "LianT"
            packageVersion = "1.0.0"
            description = "LianT Manager"
            vendor = "LianT Team"

            windows {
                createMsi = false
                iconFile.set(file("../../resources/windows/LianT.ico"))
                menu = true
                perUserInstall = true
            }
            macOS {
                iconFile.set(file("../../resources/darwin/LianT.icns"))
                bundleID = "com.bailensn.liant"
                minimumSystemVersion = "11.0"
            }
            linux {
                // 图标 + .desktop 由 CI 脚本动态处理
            }
        }
    }
}
