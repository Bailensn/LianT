import java.util.Properties
import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

// 显示版本固定 1.0.0；versionCode 每次发布 +1。
val liantVersionCode = 119
val liantVersionName = "1.0.0"

val keystoreProperties = Properties()
val keystorePropertiesFile = rootProject.file("key.properties")
if (keystorePropertiesFile.exists()) {
    keystorePropertiesFile.inputStream().use { keystoreProperties.load(it) }
}

android {
    namespace = "com.bailensn.liant"
    compileSdk = 37

    defaultConfig {
        applicationId = "com.bailensn.liant"
        // Backdrop 的 Liquid Glass 与 miuix-blur 都依赖 RuntimeShader / 13+ 的图形能力；
        // 从 118 起只支持 Android 13 及以上。
        minSdk = 33
        targetSdk = 36
        versionCode = liantVersionCode
        versionName = liantVersionName
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
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
            signingConfig = if (keystorePropertiesFile.exists()) {
                signingConfigs.getByName("release")
            } else {
                signingConfigs.getByName("debug")
            }
            isMinifyEnabled = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2026.09.00")
    implementation(composeBom)
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-graphics")
    implementation("androidx.compose.foundation:foundation")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-core")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.activity:activity-compose:1.13.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")

    // Liquid Glass（Kyant0/AndroidLiquidGlass）：Backdrop 效果 + 示例底栏
    implementation("io.github.kyant0:backdrop:2.0.1")
    implementation("io.github.kyant0:shapes:1.2.1")

    // MIUI/HyperOS 风格组件库（Compose Multiplatform）
    implementation("top.yukonga.miuix.kmp:miuix-ui:0.9.4")
    implementation("top.yukonga.miuix.kmp:miuix-preference:0.9.4")
    implementation("top.yukonga.miuix.kmp:miuix-icons:0.9.4")
    implementation("top.yukonga.miuix.kmp:miuix-blur:0.9.4")
    implementation("top.yukonga.miuix.kmp:miuix-squircle:0.9.4")
    implementation("top.yukonga.miuix.kmp:miuix-nav:0.9.4")
    implementation("top.yukonga.miuix.kmp:miuix-shader:0.9.4")
    debugImplementation("androidx.compose.ui:ui-tooling")
    testImplementation("junit:junit:4.13.2")
}
