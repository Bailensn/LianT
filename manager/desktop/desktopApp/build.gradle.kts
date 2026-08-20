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
val iconMac:     File = file("../../resources/darwin/LianT.icns")
val iconLinux:   File = file("../../resources/linux/LianT-linux.png")

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

val outdatedDistDir = file("build/compose/binaries/main/app")
tasks.configureEach {
    if (name == "createDistributable") {
        doFirst {
            if (outdatedDistDir.exists()) {
                logger.lifecycle("清理旧打包产物，避免 .exe 图标残留: ${outdatedDistDir.absolutePath}")
                outdatedDistDir.deleteRecursively()
            }
        }
        doLast {
            val exe = file("build/compose/binaries/main/app/LianT/LianT.exe")
            if (OsCheck.isWindows() && !exe.exists()) {
                throw GradleException("打包后未找到 ${exe.absolutePath}")
            }
        }
    }
}

object OsCheck {
    fun isWindows(): Boolean =
        System.getProperty("os.name").lowercase().contains("win")
}