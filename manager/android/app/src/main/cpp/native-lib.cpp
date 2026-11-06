// JNI 桥的 native 实现。
//
// 函数名遵循 Java_<包名>_<类名>_<方法名>，包名/类名的下划线要转义成 _1。
// 改动 Kotlin 侧的包名或 NativeBridge 类名时，这里的符号要同步改，
// 否则运行时只会在 Logcat 里看到 UnsatisfiedLinkError。

#include <jni.h>

#include <android/log.h>

#include <string>

namespace {

constexpr const char* kLogTag = "LianTNative";

const char* abiName() {
#if defined(__aarch64__)
    return "arm64-v8a";
#elif defined(__arm__)
    return "armeabi-v7a";
#elif defined(__x86_64__)
    return "x86_64";
#elif defined(__i386__)
    return "x86";
#else
    return "unknown";
#endif
}

const char* cppStandardName() {
#if __cplusplus >= 202302L
    return "C++23";
#elif __cplusplus >= 202002L
    return "C++20";
#elif __cplusplus >= 201703L
    return "C++17";
#else
    return "C++14";
#endif
}

std::string buildInfo() {
    std::string info;
    info.reserve(160);

    info += "ABI      : ";
    info += abiName();
    info += '\n';

    info += "Standard : ";
    info += cppStandardName();
    info += '\n';

    info += "Compiler : ";
#if defined(__clang__)
    info += "Clang ";
    info += __clang_version__;
#elif defined(__GNUC__)
    info += "GCC ";
    info += __VERSION__;
#else
    info += "unknown";
#endif
    info += '\n';

    info += "Built at : ";
    info += __DATE__;
    info += ' ';
    info += __TIME__;

    return info;
}

}  // namespace

extern "C" JNIEXPORT jstring JNICALL
Java_com_bailensn_liant_NativeBridge_greeting(JNIEnv* env, jobject /*thiz*/) {
    const std::string message = std::string("Hello from C++ on ") + abiName();
    return env->NewStringUTF(message.c_str());
}

extern "C" JNIEXPORT jstring JNICALL
Java_com_bailensn_liant_NativeBridge_buildInfo(JNIEnv* env, jobject /*thiz*/) {
    const std::string info = buildInfo();
    return env->NewStringUTF(info.c_str());
}

extern "C" JNIEXPORT jint JNICALL JNI_OnLoad(JavaVM* vm, void* /*reserved*/) {
    JNIEnv* env = nullptr;
    if (vm->GetEnv(reinterpret_cast<void**>(&env), JNI_VERSION_1_6) != JNI_OK) {
        return JNI_ERR;
    }
    __android_log_print(ANDROID_LOG_INFO, kLogTag, "libLianT loaded (%s, %s)",
                        abiName(), cppStandardName());
    return JNI_VERSION_1_6;
}
