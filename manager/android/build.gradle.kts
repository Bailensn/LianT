allprojects {
    repositories {
        google()
        mavenCentral()
    }
}

// 构建产物统一放 manager/build/<module>：Flutter 工具链按这个路径找 APK。
rootProject.layout.buildDirectory.value(
    rootProject.layout.buildDirectory.dir("../../build").get()
)

subprojects {
    project.layout.buildDirectory.value(
        rootProject.layout.buildDirectory.dir(project.name).get()
    )
}

subprojects {
    project.evaluationDependsOn(":app")
}

// 老插件的 Android 模块还写着 compileSdk 34，但 AndroidX / flutter_plugin_android_lifecycle
// 已经要求 36，AGP 9 的 AAR metadata 检查会直接失败。
// 统一把插件工程的 compileSdk 拉到 36（Flutter 自己以前也这么覆盖过）。
subprojects {
    afterEvaluate {
        (extensions.findByName("android") as? com.android.build.api.dsl.LibraryExtension)
            ?.compileSdk = 36
    }
}

tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}
