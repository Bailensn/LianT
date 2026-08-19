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

            windows {
                createMsi = false
                iconFile.set(file("../../resources/windows/LianT.ico"))
            }
            macOS {
                iconFile.set(file("../../resources/darwin/LianT.icns"))
                bundleID = "com.bailensn.liant"
            }
            linux {
                // 不写死图标，后面你自己的deb脚本控
            }
        }
    }
}
