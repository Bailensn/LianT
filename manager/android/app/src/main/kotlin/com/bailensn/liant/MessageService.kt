package com.bailensn.liant

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.content.pm.ServiceInfo
import android.media.AudioAttributes
import android.media.RingtoneManager
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.embedding.engine.dart.DartExecutor
import io.flutter.embedding.engine.loader.FlutterLoader
import io.flutter.plugin.common.MethodChannel

class MessageService : Service() {
    private var flutterEngine: FlutterEngine? = null
    private val handler = Handler(Looper.getMainLooper())
    private val repost = object : Runnable {
        override fun run() {
            post()
            handler.postDelayed(this, 60_000)
        }
    }

    override fun onCreate() {
        super.onCreate()
        createChannel()
        post()
        startBackgroundEngine()
        handler.postDelayed(repost, 60_000)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_REPOST) post()
        return START_STICKY
    }

    override fun onDestroy() {
        handler.removeCallbacks(repost)
        flutterEngine?.destroy()
        flutterEngine = null
        super.onDestroy()
    }

    private fun startBackgroundEngine() {
        try {
            val loader = FlutterLoader()
            loader.startInitialization(applicationContext)
            loader.ensureInitializationComplete(applicationContext, null)
            val engine = FlutterEngine(applicationContext)
            MethodChannel(engine.dartExecutor.binaryMessenger, "liant/background_notify")
                .setMethodCallHandler { call, result ->
                    if (call.method == "notify") {
                        val id = call.argument<Int>("id") ?: 0
                        val title = call.argument<String>("title") ?: "LianT"
                        val body = call.argument<String>("body") ?: ""
                        showMessageNotification(id, title, body)
                        result.success(null)
                    } else {
                        result.notImplemented()
                    }
                }
            engine.dartExecutor.executeDartEntrypoint(
                DartExecutor.DartEntrypoint("lib/background_main.dart", "backgroundMain"),
            )
            flutterEngine = engine
        } catch (e: Exception) {
            android.util.Log.w("LianT", "background engine failed", e)
        }
    }

    private fun showMessageNotification(id: Int, title: String, body: String) {
        val manager = if (Build.VERSION.SDK_INT >= 23) {
            getSystemService(NotificationManager::class.java)
        } else {
            getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        }
        val sound: Uri? = RingtoneManager.getDefaultUri(RingtoneManager.TYPE_NOTIFICATION)
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
            .setAutoCancel(true)
            .build()
        manager.notify(id, notification)
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun post() {
        val notification = buildNotification()
        if (Build.VERSION.SDK_INT >= 29) {
            startForeground(
                NOTIFICATION_ID,
                notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC,
            )
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun buildNotification(): Notification {
        val open = Intent(this, MainActivity::class.java)
        val flags = if (Build.VERSION.SDK_INT >= 23) {
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        } else {
            PendingIntent.FLAG_UPDATE_CURRENT
        }
        val pending = PendingIntent.getActivity(this, 0, open, flags)
        val delete = PendingIntent.getBroadcast(
            this,
            1,
            Intent(this, NotificationRestartReceiver::class.java),
            flags,
        )
        val builder = if (Build.VERSION.SDK_INT >= 26) {
            Notification.Builder(this, CHANNEL)
        } else {
            Notification.Builder(this)
        }
        val notification = builder
            .setContentTitle("LianT")
            .setContentText("正在后台接收消息")
            .setSmallIcon(R.mipmap.ic_launcher_foreground)
            .setContentIntent(pending)
            .setDeleteIntent(delete)
            .setOngoing(true)
            .setAutoCancel(false)
            .setOnlyAlertOnce(true)
            .build()
        notification.flags = notification.flags or Notification.FLAG_NO_CLEAR
        return notification
    }

    private fun createChannel() {
        if (Build.VERSION.SDK_INT < 26) return
        val manager = getSystemService(NotificationManager::class.java)
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

    companion object {
        const val CHANNEL = "liant.messages"
        const val NOTIFICATION_ID = 1001
        const val ACTION_REPOST = "com.bailensn.liant.REPOST"
        private const val MESSAGE_CHANNEL = "liant.notifications.v2"
    }
}
