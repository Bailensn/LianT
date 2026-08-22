import java.io.File

plugins {
    kotlin("jvm")
    id("org.jetbrains.compose")
    id("org.jetbrains.kotlin.plugin.compose")
}

repositories {
    google()
    mavenCentral()
}

dependencies {
    implementation(compose.desktop.currentOs)
    implementation(compose.components.resources)
    implementation(libs.material3)
    implementation(libs.backdrop)
}

val iconWindows: File = file("../../resources/windows/LianT.ico")
val iconMac: File = file("../../resources/darwin/LianT.icns")
val iconLinux: File = file("../../resources/linux/LianT-linux.png")

compose.desktop {
    application {
        mainClass = "com.bailensn.liant.MainKt"
        nativeDistributions {
            packageName = "LianT"
            packageVersion = "1.0.0"
            windows {
                iconFile.set(iconWindows)
            }
            macOS {
                iconFile.set(iconMac)
                bundleID = "com.bailensn.liant"
            }
            linux {
                iconFile.set(iconLinux)
            }
        }
    }
}

compose.resources {
    publicResClass = true
    packageOfResClass = "com.bailensn.liant"
}