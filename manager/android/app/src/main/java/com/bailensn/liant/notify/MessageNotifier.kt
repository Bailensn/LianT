package com.bailensn.liant.notify

import android.Manifest
import android.annotation.SuppressLint
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import com.bailensn.liant.MainActivity
import com.bailensn.liant.R
import java.util.concurrent.atomic.AtomicInteger

/**
 * 当前活跃 Bot 的新消息通知。
 *
 * 只负责「一条通知怎么发」；是否该发由 ChatController 按 D9 判断
 * （活跃 Bot + App 不在前台）。
 */
class MessageNotifier(private val context: Context) {

    init {
        createChannel()
    }

    /** 权限未授予时静默返回，不抛异常、不打扰用户。 */
    @SuppressLint("MissingPermission")
    fun notify(title: String, body: String) {
        if (!canPost()) return
        val intent = Intent(context, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP
            putExtra(EXTRA_FROM_NOTIFICATION, true)
        }
        val pendingIntent = PendingIntent.getActivity(
            context,
            0,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val notification = NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_launcher_foreground)
            .setContentTitle(title)
            .setContentText(body)
            .setStyle(NotificationCompat.BigTextStyle().bigText(body))
            .setAutoCancel(true)
            .setContentIntent(pendingIntent)
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)
            .build()
        try {
            NotificationManagerCompat.from(context).notify(nextId(), notification)
        } catch (_: SecurityException) {
            // 权限在判断之后被撤销：当作静默跳过。
        }
    }

    private fun canPost(): Boolean {
        if (!NotificationManagerCompat.from(context).areNotificationsEnabled()) return false
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            return ContextCompat.checkSelfPermission(
                context,
                Manifest.permission.POST_NOTIFICATIONS,
            ) == PackageManager.PERMISSION_GRANTED
        }
        return true
    }

    private fun createChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val manager = context.getSystemService(NotificationManager::class.java) ?: return
        if (manager.getNotificationChannel(CHANNEL_ID) != null) return
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL_ID, "消息", NotificationManager.IMPORTANCE_DEFAULT).apply {
                description = "当前活跃 Bot 的新消息"
            },
        )
    }

    private companion object {
        const val CHANNEL_ID = "messages"
        const val EXTRA_FROM_NOTIFICATION = "from_notification"
        val counter = AtomicInteger(1000)

        fun nextId(): Int = counter.incrementAndGet()
    }
}
