import org.apache.tools.ant.taskdefs.condition.Os
import java.net.URL
import java.net.URLConnection
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

val rceditLocal: File = file("../../tools/rcedit-x64.exe") 
val rceditUrl = "https:
val rceditFile = layout.buildDirectory.file("rcedit/rcedit-x64.exe")

fun resolveRcedit(): File = if (rceditLocal.exists()) rceditLocal else rceditFile.get().asFile

fun downloadFile(url: String, dest: File) {
    val connection: URLConnection = URL(url).openConnection()
    connection.connectTimeout = 30_000
    connection.readTimeout = 60_000
    connection.getInputStream().use { input ->
        dest.outputStream().use { output -> input.copyTo(output) }
    }
}


fun runProcess(vararg command: String): Int {
    val builder = ProcessBuilder(*command)
        .redirectErrorStream(true)
    val process = builder.start()
    process.inputStream.bufferedReader().useLines { lines ->
        lines.forEach { line -> println(line) }
    }
    val exit = process.waitFor()
    if (exit != 0) {
        throw GradleException("命令执行失败，退出码 $exit：${command.joinToString(" ")}")
    }
    return exit
}

tasks.register("downloadRcedit") {
    onlyIf { Os.isFamily(Os.FAMILY_WINDOWS) && !rceditLocal.exists() }
    outputs.file(rceditFile)
    doLast {
        val dest = rceditFile.get().asFile
        if (!dest.exists()) {
            dest.parentFile.mkdirs()
            logger.lifecycle("下载 rcedit: $rceditUrl")
            val tmp = File(dest.parentFile, dest.name + ".part")
            try {
                downloadFile(rceditUrl, tmp)
            } catch (e: Exception) {
                tmp.delete()
                throw GradleException(
                    "rcedit 自动下载失败：${e.message}\n" +
                    e
                )
            }
            if (tmp.length() < 100_000L) {
                tmp.delete()
                throw GradleException("rcedit 文件不完整（长度 ${tmp.length()}）")
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
        runProcess(
            rcedit.absolutePath,
            exe.absolutePath,
            "--set-icon", iconWindows.absolutePath
        )
        logger.lifecycle("LianT.exe 图标写入完成 ✔")
    }
}

tasks.configureEach {
    if (name == "createDistributable") {
        finalizedBy("setExeIcon")
    }
}
