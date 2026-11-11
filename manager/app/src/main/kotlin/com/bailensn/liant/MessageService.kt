package com.bailensn.liant

import android.app.AlarmManager
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.util.Log
import com.bailensn.liant.core.BackgroundRuntime
import com.bailensn.liant.core.CoreFiles
import java.io.File

/**
 * 常驻消息服务，跑在独立的 `:background` 进程里（见 AndroidManifest 的 android:process）。
 *
 * 迁移第二步之后这里**不再创建 FlutterEngine**：WebSocket、协议、签名、通知
 * 都在纯 Kotlin 的 [BackgroundRuntime] 里。Service 自己只负责：
 * 尽快 startForeground、维护常驻通知文案、被系统重启后把运行时拉回来。
 *
 * UI 进程被系统杀掉之后它仍然活着（前台服务 + START_STICKY）。
 */
class MessageService : Service() {
    private var runtime: BackgroundRuntime? = null

    @Volatile
    private var destroyed = false

    private val handler = Handler(Looper.getMainLooper())
    private var statusText = "正在后台接收消息"
    private val repost = object : Runnable {
        override fun run() {
            postForeground()
            handler.postDelayed(this, 60_000)
        }
    }

    override fun onCreate() {
        super.onCreate()
        createChannels()
        // 必须在 startForegroundService 之后尽快调用，Android 只给 10 秒。
        postForeground()
        startRuntime()
        handler.postDelayed(repost, 60_000)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_REPOST -> postForeground()
            ACTION_WAKE -> postForeground()
        }
        // 系统把进程杀掉后按 START_STICKY 重启：intent 可能为 null，运行时在 onCreate 恢复。
        if (runtime == null) startRuntime()
        return START_STICKY
    }

    override fun onTaskRemoved(rootIntent: Intent?) {
        // 划掉任务时不停止服务（stopWithTask=false），这里再排一个兜底闹钟：
        // 万一进程随后被厂商清理，闹钟到点会尝试把服务拉回来。
        scheduleWakeup(60_000L)
        super.onTaskRemoved(rootIntent)
    }

    override fun onDestroy() {
        destroyed = true
        handler.removeCallbacks(repost)
        handler.removeCallbacksAndMessages(null)
        runtime?.stop()
        runtime = null
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    // -------------------------------------------------------------------------
    // Kotlin 运行时
    // -------------------------------------------------------------------------

    private fun startRuntime() {
        if (runtime != null) return
        val created = try {
            BackgroundRuntime(applicationContext) { text ->
                // 回调可能来自 OkHttp/调度线程；通知更新放回主线程。
                handler.post { updateStatus(text) }
            }
        } catch (error: Exception) {
            Log.w(TAG, "Kotlin 运行时创建失败", error)
            return
        }
        runtime = created
        created.start()
    }

    private fun updateStatus(text: String) {
        // 服务已经销毁时不能再 startForeground；停止运行时的回调会落到这里。
        if (destroyed || text.isEmpty() || text == statusText) return
        statusText = text
        postForeground()
    }

    // -------------------------------------------------------------------------
    // 通知
    // -------------------------------------------------------------------------

    private fun postForeground() {
        val notification = buildForegroundNotification()
        if (Build.VERSION.SDK_INT >= 34) {
            startForeground(
                NOTIFICATION_ID,
                notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE,
            )
        } else {
            // 29~33 不认识 specialUse，两参版本会按清单里的类型提交。
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun buildForegroundNotification(): Notification {
        val builder = if (Build.VERSION.SDK_INT >= 26) {
            Notification.Builder(this, CHANNEL)
        } else {
            Notification.Builder(this)
        }
        val delete = PendingIntent.getBroadcast(
            this,
            1,
            Intent(this, RestartReceiver::class.java),
            pendingFlags(),
        )
        val notification = builder
            .setContentTitle("LianT")
            .setContentText(statusText)
            .setSmallIcon(R.mipmap.ic_launcher_foreground)
            .setContentIntent(openAppIntent(0))
            .setDeleteIntent(delete)
            .setOngoing(true)
            .setAutoCancel(false)
            .setOnlyAlertOnce(true)
            .build()
        notification.flags = notification.flags or Notification.FLAG_NO_CLEAR
        return notification
    }

    private fun openAppIntent(requestCode: Int): PendingIntent = PendingIntent.getActivity(
        this,
        requestCode,
        Intent(this, MainActivity::class.java),
        pendingFlags(),
    )

    private fun pendingFlags(): Int =
        if (Build.VERSION.SDK_INT >= 23) {
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        } else {
            PendingIntent.FLAG_UPDATE_CURRENT
        }

    private fun notificationManager(): NotificationManager =
        if (Build.VERSION.SDK_INT >= 23) {
            getSystemService(NotificationManager::class.java)
        } else {
            getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        }

    private fun createChannels() {
        if (Build.VERSION.SDK_INT < 26) return
        val manager = notificationManager()
        val channel = NotificationChannel(
            CHANNEL,
            "后台消息",
            NotificationManager.IMPORTANCE_LOW,
        ).apply {
            description = "保持连接，接收 LianT 消息"
            setShowBadge(false)
        }
        manager.createNotificationChannel(channel)
    }

    // -------------------------------------------------------------------------
    // 兜底拉起
    // -------------------------------------------------------------------------

    private fun scheduleWakeup(delayMs: Long) {
        try {
            val alarm = getSystemService(Context.ALARM_SERVICE) as AlarmManager
            val intent = Intent(this, RestartReceiver::class.java)
                .setAction(ACTION_WAKE)
            val pending = PendingIntent.getBroadcast(this, 2, intent, pendingFlags())
            // 不申请精确闹钟权限：用不精确的 setAndAllowWhileIdle，待机时也能唤醒。
            val at = System.currentTimeMillis() + delayMs
            if (Build.VERSION.SDK_INT >= 23) {
                alarm.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, pending)
            } else {
                alarm.set(AlarmManager.RTC_WAKEUP, at, pending)
            }
        } catch (e: Exception) {
            Log.w(TAG, "安排兜底闹钟失败", e)
        }
    }

    companion object {
        private const val TAG = "LianT"

        const val CHANNEL = "liant.messages"
        const val NOTIFICATION_ID = 1001
        const val ACTION_REPOST = "com.bailensn.liant.REPOST"
        const val ACTION_WAKE = "com.bailensn.liant.WAKE"

        /** UI 进程与后台进程共用的原生通道（状态文件/桥接文件/日志路径）。 */
        const val CHANNEL_STATE = "liant/background_state"

        /**
         * UI 进程写、后台进程读的共享状态文件。
         *
         * 存在即代表「已配对」：RestartReceiver 靠它决定开机要不要拉起服务。
         */
        fun stateFile(context: Context): File = CoreFiles.stateFile(context)

        /** 后台进程写给 UI 的桥接信息（本地端口 + 令牌 + 心跳）。 */
        fun bridgeFile(context: Context): File = CoreFiles.bridgeFile(context)

        /** 未捕获异常落盘的位置（UI 与 :background 进程共用）。 */
        fun logFile(context: Context): File = CoreFiles.logFile(context)

        /** 用前台服务的方式拉起；Android 12+ 后台可能被拒绝，调用方自己兜住异常。 */
        fun start(context: Context, action: String? = null) {
            val intent = Intent(context, MessageService::class.java)
            if (action != null) intent.action = action
            if (Build.VERSION.SDK_INT >= 26) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }
    }
}
