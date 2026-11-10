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

tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}
