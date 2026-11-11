import java.util.Properties
import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("com.android.application")
    id("kotlin-android")
    id("org.jetbrains.kotlin.plugin.compose")
    id("dev.flutter.flutter-gradle-plugin")
}

// 显示版本固定 1.0.0；versionCode 每次发布 +1。
// 同时同步 pubspec.yaml 的 +build（当前 111）。
val liantVersionCode = 111

val keystoreProperties = Properties()
val keystorePropertiesFile = rootProject.file("key.properties")
if (keystorePropertiesFile.exists()) {
    keystorePropertiesFile.inputStream().use { keystoreProperties.load(it) }
}

android {
    namespace = "com.bailensn.liant"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        applicationId = "com.bailensn.liant"
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = liantVersionCode
        versionName = "1.0.0"
    }

    packaging {
        // AGP 9 默认不压缩 .so（extractNativeLibs=false），APK 直接大一倍。
        // 这里换回压缩，安装时解压；侧载分发更在意下载体积。
        jniLibs {
            useLegacyPackaging = true
            // Flutter 插件会把自己支持的 ABI 全塞进 abiFilters（含 x86_64/armeabi-v7a），
            // 而依赖 AAR（ML Kit 条码、CameraX、DataStore、dartjni）自带三套 .so。
            // 本项目只发 aarch64，这里直接按 APK 内的路径把它们剔掉。
            excludes.addAll(
                listOf(
                    "lib/armeabi-v7a/**",
                    "lib/x86/**",
                    "lib/x86_64/**",
                )
            )
        }
    }

    buildFeatures {
        // 原生 UI（迁移试水）：与 Flutter 并存，试水屏在 NativeSettingsActivity。
        compose = true
    }

    signingConfigs {
        if (keystorePropertiesFile.exists()) {
            create("release") {
                storeFile = file(keystoreProperties.getProperty("storeFile"))
                storePassword = keystoreProperties.getProperty("storePassword")
                keyAlias = keystoreProperties.getProperty("keyAlias")
                keyPassword = keystoreProperties.getProperty("keyPassword")
            }
        }
    }

    buildTypes {
        named<com.android.build.api.dsl.ApplicationBuildType>("release") {
            // 有 key.properties 就用正式签名；没有时保留 debug 回退，便于本地测试。
            // v1.0.0 正式发布必须在 CI 注入 key.properties。
            signingConfig = if (keystorePropertiesFile.exists()) {
                signingConfigs.getByName("release")
            } else {
                signingConfigs.getByName("debug")
            }

        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

// Flutter 的 Gradle 插件会在 afterEvaluate 里把 defaultConfig.ndk.abiFilters
// 重置成「Flutter 支持的全部 ABI」（armeabi-v7a / arm64-v8a / x86_64，见
// FlutterPlugin.configureAbiWithoutSplits），.so 为 ML Kit 条码（mobile_scanner）、
// CameraX、DataStore 这些 AAR 自带，不裁就会各带三套。这里在它之后再改回 arm64。
afterEvaluate {
    val filters = android.defaultConfig.ndk.abiFilters
    filters.clear()
    filters.add("arm64-v8a")
}

flutter {
    source = "../.."
}

dependencies {
    // 迁移第一步：后台 Kotlin 核心直接用 OkHttp 连 WSS（自签证书、重连、心跳）。
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    // 只跑 JVM 单测（P-256 与 openssl 向量的互操作），不参与打包。
    testImplementation("junit:junit:4.13.2")

    // 原生 UI 试水（与 Flutter 并存）：版本由 BOM 统一。
    val composeBom = platform("androidx.compose:compose-bom:2026.06.01")
    implementation(composeBom)
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-graphics")
    implementation("androidx.compose.foundation:foundation")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-core")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.activity:activity-compose:1.13.0")
    debugImplementation("androidx.compose.ui:ui-tooling")
}
