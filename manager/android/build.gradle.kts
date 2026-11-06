// AGP 9 自带「内置 Kotlin」，随插件携带的 KGP 是 2.2.10。
// 想用更新的 Kotlin，官方做法是把高版本 KGP 放到根工程 buildscript classpath，
// AGP 会以它为准（版本冲突取高者）。不要再应用 org.jetbrains.kotlin.android。
buildscript {
    repositories {
        google()
        mavenCentral()
    }
    dependencies {
        classpath("org.jetbrains.kotlin:kotlin-gradle-plugin:2.4.21")
    }
}

plugins {
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.kotlin.compose) apply false
}
