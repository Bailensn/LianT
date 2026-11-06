package com.bailensn.liant

/**
 * JNI 入口。当前只有自检用的 greeting / buildInfo，
 * 后续协议或媒体相关的 native 实现挂在这里。
 */
object NativeBridge {

    private const val LIBRARY_NAME = "LianT"

    private val loadFailure: Throwable? = runCatching {
        System.loadLibrary(LIBRARY_NAME)
    }.exceptionOrNull()

    val isLoaded: Boolean
        get() = loadFailure == null

    val loadFailureMessage: String?
        get() = loadFailure?.message

    external fun greeting(): String

    external fun buildInfo(): String
}
