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

// 各平台图标资源（相对 desktop/ 的路径）
val iconWindows: File = file("../../resources/windows/LianT.ico")
val iconMac:     File = file("../../resources/darwin/LianT.icns")
// Linux 用标准 8-bit RGBA PNG（LianT.png 原图是 16-bit 灰度，个别桌面环境不认）
val iconLinux:   File = file("../../resources/linux/LianT-linux.png")

compose.desktop {
    application {
        mainClass = "com.bailensn.liant.MainKt"
        nativeDistributions {
            packageName = "LianT"
            packageVersion = "1.0.0"
            // --- 方式一（CMP 官方）：iconFile 会交给 jpackage 的 --icon 处理 ---
            // Windows: .ico → 嵌入 LianT.exe（JDK 19+ 直接内嵌进 exe）
            // macOS:   .icns → 放进 .app/Contents/Resources
            // Linux:   .png  → 用于 .desktop 桌面图标
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

// ============================================================
// 方式二（兜底 + 双保险，仅 Windows）：
// 在 createDistributable 之后，用 rcedit 把 .ico 强制写进
// LianT.exe 二进制本身，保证 exe 本体 100% 带你的图标，
// 不依赖 jpackage 行为 / 构建缓存 / Inno 复制时机。
//
// 说明：macOS 的 .app 和 Linux 的应用，用上面的「方式一」iconFile
// 由 jpackage 自动处理即可，没有 Windows 这种 exe 图标残留问题，
// 所以这里只管 Windows。rcedit 自动下载一次并缓存。
// ============================================================

// rcedit：Windows 下编辑 exe 资源（设置图标）的官方工具。
// 本地已存在的 rcedit-x64.exe 优先（放哪都行），否则尝试自动下载一次。
val rceditLocal: File = file("../../tools/rcedit-x64.exe") // 可选：你自己放的
val rceditUrl = "https://github.com/electron/rcedit/releases/download/v2.0.0/rcedit-x64.exe"
val rceditFile = layout.buildDirectory.file("rcedit/rcedit-x64.exe")

// 返回最终使用的 rcedit 可执行文件：本地优先，否则 build 缓存里的
fun resolveRcedit(): File = if (rceditLocal.exists()) rceditLocal else rceditFile.get().asFile

/**
 * 下载一个 .exe 文件到 dest（纯 Kotlin，不依赖 Gradle DSL）。
 */
fun downloadFile(url: String, dest: File) {
    val connection: URLConnection = URL(url).openConnection()
    connection.connectTimeout = 30_000
    connection.readTimeout = 60_000
    connection.getInputStream().use { input ->
        dest.outputStream().use { output -> input.copyTo(output) }
    }
}

/**
 * 用 ProcessBuilder 运行一条命令并等待结束（纯 Kotlin，不依赖 Gradle DSL exec）。
 * 返回进程退出码；非 0 会抛异常。
 */
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
                    "rcedit 自动下载失败（网络可能无法访问 GitHub）：${e.message}\n" +
                    "解决办法：手动下载 https://github.com/electron/rcedit/releases/download/v2.0.0/rcedit-x64.exe，" +
                    "放到 manager/tools/rcedit-x64.exe（脚本会自动优先用它）。",
                    e
                )
            }
            if (tmp.length() < 100_000L) {
                tmp.delete()
                throw GradleException("rcedit 下载文件不完整（长度 ${tmp.length()}）")
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
        // createDistributable 产物：app/<packageName>/<packageName>.exe
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

// 打包完成后自动补一次图标：让 createDistributable 结束时触发 setExeIcon
tasks.named("createDistributable") {
    finalizedBy("setExeIcon")
}