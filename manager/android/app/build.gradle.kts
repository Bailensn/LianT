import java.util.Properties
import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("com.android.application")
    id("kotlin-android")
    id("dev.flutter.flutter-gradle-plugin")
}

// 显示版本固定 1.0.0；versionCode 每次发布 +1。
// 同时同步 pubspec.yaml 的 +build（当前 103）。
val liantVersionCode = 103

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

        // 只打 arm64-v8a（本项目只发 aarch64）。
        // --target-platform 只管 Flutter 引擎/libapp.so，管不到依赖 AAR 自带的 .so：
        // ML Kit 条码（mobile_scanner）、CameraX、DataStore 每个都带四个 ABI，
        // 不裁的话 APK 会白胖约 6 MB。
        ndk {
            abiFilters.add("arm64-v8a")
        }
    }

    packaging {
        // AGP 9 默认不压缩 .so（extractNativeLibs=false），APK 直接大一倍。
        // 这里换回压缩，安装时解压；侧载分发更在意下载体积。
        jniLibs {
            useLegacyPackaging = true
        }
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

flutter {
    source = "../.."
}
