plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
}

// 版本由发布流水线用 -PversionName/-PversionCode 注入；本地默认 v1.0.0-Alpha。
val appVersionName = providers.gradleProperty("versionName").getOrElse("v1.0.0-Alpha")
val appVersionCode = providers.gradleProperty("versionCode").getOrElse("10000").toInt()

android {
    namespace = "com.bailensn.liant"
    // API 37.1：Android 17 的稳定 minor 平台（37.2 还是 beta）。
    // compileSdk 只决定编译器可见的 API，运行时行为仍由 targetSdk 决定。
    compileSdk {
        version = release(37) {
            minorApiLevel = 1
        }
    }

    defaultConfig {
        applicationId = "com.bailensn.liant"
        // miuix-blur 依赖 RuntimeShader（API 33），这是本项目的硬下限。
        minSdk = 33
        targetSdk = 37
        versionCode = appVersionCode
        versionName = appVersionName

    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }

    buildFeatures {
        compose = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_21
        targetCompatibility = JavaVersion.VERSION_21
    }

    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1}"
        }
    }
}

dependencies {
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.graphics)
    implementation(libs.androidx.compose.ui.tooling.preview)
    implementation(libs.androidx.compose.material3)
    debugImplementation(libs.androidx.compose.ui.tooling)

    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.process)

    implementation(libs.miuix.ui)
    implementation(libs.miuix.icons)
    implementation(libs.miuix.nav)
    implementation(libs.miuix.blur)
    implementation(libs.kotlinx.coroutines.android)
    implementation(libs.okhttp)
    implementation(libs.zxing.embedded)

    testImplementation(libs.junit)
}
