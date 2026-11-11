package com.bailensn.liant.core

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.media.AudioAttributes
import android.media.RingtoneManager
import android.net.Uri
import android.os.Build
import com.bailensn.liant.MainActivity
import com.bailensn.liant.R

/**
 * 后台进程的消息通知。
 *
 * 与 Dart 的 `runtime_io.dart` 和 UI 进程的 `MainActivity.showNotification`
 * 使用同一个渠道、同一个 `notificationIdFor`：
 * 同一条消息两边各自弹一次时，系统按 ID 覆盖 + `setOnlyAlertOnce` 只响一次。
 */
class Notifier(private val context: Context) {
    fun notify(msgId: String, title: String, body: String) {
        val manager = context.getSystemService(NotificationManager::class.java) ?: return
        val sound: Uri = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION)
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
        val id = notificationIdFor(msgId)
        val pending = PendingIntent.getActivity(
            context,
            id,
            Intent(context, MainActivity::class.java),
            if (Build.VERSION.SDK_INT >= 23) {
                PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
            } else {
                PendingIntent.FLAG_UPDATE_CURRENT
            },
        )
        val builder = if (Build.VERSION.SDK_INT >= 26) {
            Notification.Builder(context, MESSAGE_CHANNEL)
        } else {
            Notification.Builder(context)
        }
        val notification = builder
            .setContentTitle(title)
            .setContentText(body)
            .setStyle(Notification.BigTextStyle().bigText(body))
            .setSmallIcon(R.mipmap.ic_launcher_foreground)
            .setContentIntent(pending)
            .setSound(sound)
            .setOnlyAlertOnce(true)
            .setAutoCancel(true)
            .build()
        try {
            manager.notify(id, notification)
        } catch (_: SecurityException) {
            // 没给通知权限（Android 13+）：静默失败，界面里仍能看到消息。
        }
    }

    private companion object {
        const val MESSAGE_CHANNEL = "liant.notifications.v2"
        const val FOREGROUND_ID = 1001

        /**
         * 同一条消息在 UI 进程与 `:background` 进程里必须算出同一个值。
         * 算法与 Dart 的 `notificationIdFor` 完全相同（FNV-1a，按 UTF-16 码元）。
         */
        fun notificationIdFor(msgId: String): Int {
            var hash = 0x811c9dc5.toInt()
            for (char in msgId) {
                hash = hash xor char.code
                hash = (hash * 0x01000193) and 0x7fffffff
            }
            // 0 或常驻通知 ID 撞上就顺延，否则会把「正在后台接收消息」那条盖掉。
            if (hash == 0 || hash == FOREGROUND_ID) hash += 1
            return hash
        }
    }
}
