package com.bailensn.liant

import android.Manifest
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.media.AudioAttributes
import android.media.RingtoneManager
import android.net.Uri
import android.os.Build
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        // 共享状态文件路径：UI 进程写、:background 进程读，必须是同一个文件。
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, MessageService.CHANNEL_STATE)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "statePath" ->
                        result.success(MessageService.stateFile(this).absolutePath)
                    "bridgePath" ->
                        result.success(MessageService.bridgeFile(this).absolutePath)
                    "logPath" ->
                        result.success(MessageService.logFile(this).absolutePath)
                    else -> result.notImplemented()
                }
            }
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, "liant/background")
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "start" -> {
                        try {
                            startMessageService()
                            result.success(null)
                        } catch (e: Exception) {
                            result.error("start_failed", e.message, null)
                        }
                    }
                    "stop" -> {
                        stopService(Intent(this, MessageService::class.java))
                        result.success(null)
                    }
                    "notify" -> {
                        val id = call.argument<Int>("id") ?: 0
                        val title = call.argument<String>("title") ?: "LianT"
                        val body = call.argument<String>("body") ?: ""
                        showNotification(id, title, body)
                        result.success(null)
                    }
                    "requestNotifications" -> {
                        requestNotificationPermission()
                        result.success(null)
                    }
                    "openUrl" -> {
                        val url = call.argument<String>("url")
                        if (url.isNullOrEmpty()) {
                            result.error("bad_url", "URL 为空", null)
                        } else {
                            try {
                                startActivity(
                                    Intent(Intent.ACTION_VIEW, Uri.parse(url))
                                        .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
                                )
                                result.success(null)
                            } catch (e: Exception) {
                                result.error("open_failed", e.message, null)
                            }
                        }
                    }
                    else -> result.notImplemented()
                }
            }
    }

    private fun requestNotificationPermission() {
        if (Build.VERSION.SDK_INT >= 33 &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 42)
        }
    }

    private fun startMessageService() {
        requestNotificationPermission()
        MessageService.start(this)
    }

    private fun showNotification(id: Int, title: String, body: String) {
        val manager = if (Build.VERSION.SDK_INT >= 23) {
            getSystemService(NotificationManager::class.java)
        } else {
            getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        }
        val sound = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION)
        if (Build.VERSION.SDK_INT >= 26) {
            manager.createNotificationChannel(
                NotificationChannel(
                    MESSAGE_CHANNEL,
                    "消息通知",
                    NotificationManager.IMPORTANCE_HIGH,
                ).apply {
                    description = "LianT 新消息"
                    setSound(
                        sound,
                        AudioAttributes.Builder()
                            .setUsage(AudioAttributes.USAGE_NOTIFICATION)
                            .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
                            .build(),
                    )
                    enableVibration(true)
                },
            )
        }
        val flags = if (Build.VERSION.SDK_INT >= 23) {
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        } else {
            PendingIntent.FLAG_UPDATE_CURRENT
        }
        val pending = PendingIntent.getActivity(
            this,
            id,
            Intent(this, MainActivity::class.java),
            flags,
        )
        val builder = if (Build.VERSION.SDK_INT >= 26) {
            Notification.Builder(this, MESSAGE_CHANNEL)
        } else {
            Notification.Builder(this)
        }
        val notification = builder
            .setContentTitle(title)
            .setContentText(body)
            .setStyle(Notification.BigTextStyle().bigText(body))
            .setSmallIcon(R.mipmap.ic_launcher_foreground)
            .setContentIntent(pending)
            .setSound(sound)
            .setDefaults(Notification.DEFAULT_SOUND or Notification.DEFAULT_VIBRATE)
            .setOnlyAlertOnce(true)
            .setAutoCancel(true)
            .build()
        manager.notify(id, notification)
    }

    companion object {
        private const val MESSAGE_CHANNEL = "liant.notifications.v2"
    }
}
