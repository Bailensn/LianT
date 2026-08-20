import org.apache.tools.ant.taskdefs.condition.Os

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


val rceditLocal: File = file("../../tools/rcedit-x64.exe") // 可选：你自己放的
val rceditUrl = "https://github.com/electron/rcedit/releases/download/v2.0.0/rcedit-x64.exe"
val rceditFile = layout.buildDirectory.file("rcedit/rcedit-x64.exe")

fun resolveRcedit(): File = if (rceditLocal.exists()) rceditLocal else rceditFile.get().asFile

tasks.register("downloadRcedit") {
    onlyIf { Os.isFamily(Os.FAMILY_WINDOWS) && !rceditLocal.exists() }
    outputs.file(rceditFile)
    doLast {
        val dest = rceditFile.get().asFile
        if (!dest.exists()) {
            dest.parentFile.mkdirs()
            logger.lifecycle("下载 rcedit: $rceditUrl")
            val tmp = File(dest.parentFile, dest.name + ".part")
            val conn = java.net.URI(rceditUrl).toURL().openConnection()
            conn.connectTimeout = 30_000
            conn.readTimeout = 60_000
            conn.getInputStream().use { input ->
                tmp.outputStream().use { output -> input.copyTo(output) }
            }
            if (tmp.length() < 100_000L) {
                tmp.delete()
                throw GradleException(
                    "rcedit 自动下载失败（网络可能无法访问 GitHub）。\n" +
                    "解决办法：手动下载 https://github.com/electron/rcedit/releases/download/v2.0.0/rcedit-x64.exe ，" +
                    "放到 resources/windows/ 同级，并告诉我或放进 tools/rcedit-x64.exe。",
                )
            }
            if (!tmp.renameTo(dest)) {
                tmp.copyTo(dest, overwrite = true)
                tmp.delete()
            }
        }
    }
}

tasks.register("setExeIcon") {
    group = "distribution"
    description = "用 rcedit 把图标强制写入生成的 LianT.exe，保证二进制本身带图标"
    dependsOn("downloadRcedit")
    onlyIf { Os.isFamily(Os.FAMILY_WINDOWS) }
    doLast {
        val rcedit = resolveRcedit()
        val exe = file("build/compose/binaries/main/app/LianT/LianT.exe")
        if (!exe.exists()) {
            throw GradleException("找不到 LianT.exe: ${exe.absolutePath}，请先运行 createDistributable。")
        }
        if (!iconWindows.exists()) {
            throw GradleException("找不到图标: ${iconWindows.absolutePath}")
        }
        logger.lifecycle("写入图标到 $exe （来自 ${iconWindows.absolutePath}）")
        exec {
            commandLine(
                rcedit.absolutePath,
                exe.absolutePath,
                "--set-icon", iconWindows.absolutePath
            )
        }
        logger.lifecycle("LianT.exe 图标写入完成 ✔")
    }
}

tasks.named("createDistributable") {
    finalizedBy("setExeIcon")
}