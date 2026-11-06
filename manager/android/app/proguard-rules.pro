# Compose 与协程不需要额外 keep；这里只保留反射/序列化的钩子。
-keepattributes *Annotation*

# JNI 符号与 NativeBridge 必须保留，否则 release 包会 UnsatisfiedLinkError。
-keep class com.bailensn.liant.NativeBridge { *; }
-keepclasseswithmembers class * { native <methods>; }
