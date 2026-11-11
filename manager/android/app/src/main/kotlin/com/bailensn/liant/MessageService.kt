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
import android.media.AudioAttributes
import android.media.RingtoneManager
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.util.Log
import com.bailensn.liant.core.CoreFiles
import com.bailensn.liant.core.CoreLog
import com.bailensn.liant.core.net.LiantSocket
import com.bailensn.liant.core.state.StateStore
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.embedding.engine.dart.DartExecutor
import io.flutter.plugin.common.MethodChannel
import org.json.JSONObject
import java.io.File

/**
 * 常驻消息服务，跑在独立的 `:background` 进程里（见 AndroidManifest 的 android:process）。
 *
 * 这里不依赖 Activity，也不依赖 UI isolate：自己创建一个 FlutterEngine，执行 Dart 侧的
 * `backgroundMain` 入口。那个入口独立连 WebSocket、收消息、发通知；UI 进程被系统杀掉
 * 之后它仍然活着（前台服务 + START_STICKY）。
 *
 * 通知统一走 [CHANNEL_NOTIFY]，共享状态文件路径走 [CHANNEL_STATE]。
 */
class MessageService : Service() {
    private var flutterEngine: FlutterEngine? = null

    /** 第一步迁移的验证探针：纯 Kotlin 的连接 + 认证，暂不接管 Dart 链路。 */
    @Volatile
    private var coreSocket: LiantSocket? = null
    private var coreThread: Thread? = null

    @Volatile
    private var coreGeneration = 0

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
        startBackgroundEngine()
        startKotlinCore()
        handler.postDelayed(repost, 60_000)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_REPOST -> postForeground()
            ACTION_WAKE -> postForeground()
        }
        // 系统把进程杀掉后按 START_STICKY 重启：intent 可能为 null，引擎要在 onCreate 恢复。
        if (flutterEngine == null) startBackgroundEngine()
        if (coreSocket == null) startKotlinCore()
        return START_STICKY
    }

    override fun onTaskRemoved(rootIntent: Intent?) {
        // 划掉任务时不停止服务（stopWithTask=false），这里再排一个兜底闹钟：
        // 万一进程随后被厂商清理，闹钟到点会尝试把服务拉回来。
        scheduleWakeup(60_000L)
        super.onTaskRemoved(rootIntent)
    }

    override fun onDestroy() {
        handler.removeCallbacks(repost)
        stopKotlinCore()
        flutterEngine?.destroy()
        flutterEngine = null
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    // -------------------------------------------------------------------------
    // Kotlin 核心（迁移第一步：只验证连接 + 认证）
    // -------------------------------------------------------------------------

    /**
     * 用纯 Kotlin 走一遍「读身份 → 连 WSS → 验服务端自证 → 回 auth.response」。
     *
     * 这一步**不接管**现有 Dart 后台链路：两条连接同时存在是预期现象，第二步
     * 把桥接搬过来后才会只剩一条。结果只写日志（logcat 的 LianT 标签；失败另
     * 外落 liant_crash.log），不影响任何现有功能。
     */
    private fun startKotlinCore() {
        if (coreThread != null) return
        val generation = ++coreGeneration
        val thread = Thread({ runKotlinCore(generation) }, "liant-core")
        coreThread = thread
        thread.start()
    }

    private fun runKotlinCore(generation: Int) {
        try {
            val state = StateStore.read(applicationContext)
            if (state == null) {
                Log.i(TAG, "Kotlin 核心：未配对（没有状态文件），跳过")
                return
            }
            if (coreGeneration != generation) return
            val socket = LiantSocket(
                state.identity,
                object : LiantSocket.Listener {
                    override fun onState(state: LiantSocket.State, detail: String) {
                        Log.i(TAG, "Kotlin 核心状态：${state.name} $detail")
                        if (state == LiantSocket.State.error) {
                            CoreLog.record(applicationContext, "Kotlin 核心：$detail")
                        }
                    }

                    override fun onEvent(type: String, payload: JSONObject) {
                        Log.i(TAG, "Kotlin 核心事件：$type")
                    }
                },
            )
            if (coreGeneration != generation) return
            coreSocket = socket
            socket.start()
        } catch (error: Exception) {
            // 探针失败不能把 :background 进程带走。
            Log.w(TAG, "Kotlin 核心启动失败", error)
            CoreLog.record(applicationContext, "Kotlin 核心启动失败：$error")
        }
    }

    private fun stopKotlinCore() {
        coreGeneration++
        coreSocket?.stop()
        coreSocket = null
        coreThread = null
    }

    // -------------------------------------------------------------------------
    // 后台 Flutter 引擎
    // -------------------------------------------------------------------------

    private fun startBackgroundEngine() {
        if (flutterEngine != null) return
        try {
            // 构造函数会完成 FlutterLoader 初始化并按 GeneratedPluginRegistrant 注册插件。
            val engine = FlutterEngine(applicationContext)
            attachChannels(engine)
            val bundle = DartExecutor.DartEntrypoint.createDefault().pathToBundle
            engine.dartExecutor.executeDartEntrypoint(
                DartExecutor.DartEntrypoint(bundle, ENTRYPOINT),
            )
            flutterEngine = engine
        } catch (e: Exception) {
            // 起不来要能在通知上看出来，不然只会表现为「收不到通知」。
            statusText = "后台引擎启动失败"
            postForeground()
            Log.w(TAG, "后台引擎启动失败", e)
        }
    }

    private fun attachChannels(engine: FlutterEngine) {
        // 共享状态文件路径：UI 进程与后台进程都从这里取，保证是同一个文件。
        MethodChannel(engine.dartExecutor.binaryMessenger, CHANNEL_STATE)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "statePath" -> result.success(stateFile(this).absolutePath)
                    "bridgePath" -> result.success(bridgeFile(this).absolutePath)
                    "logPath" -> result.success(logFile(this).absolutePath)
                    else -> result.notImplemented()
                }
            }
        MethodChannel(engine.dartExecutor.binaryMessenger, CHANNEL_NOTIFY)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "notify" -> {
                        val id = call.argument<Int>("id") ?: 0
                        val title = call.argument<String>("title") ?: "LianT"
                        val body = call.argument<String>("body") ?: ""
                        showMessageNotification(id, title, body)
                        result.success(null)
                    }
                    "status" -> {
                        val text = call.argument<String>("text")
                        if (!text.isNullOrEmpty() && text != statusText) {
                            statusText = text
                            postForeground()
                        }
                        result.success(null)
                    }
                    else -> result.notImplemented()
                }
            }
    }

    // -------------------------------------------------------------------------
    // 通知
    // -------------------------------------------------------------------------

    private fun showMessageNotification(id: Int, title: String, body: String) {
        val manager = notificationManager()
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
            .setContentIntent(openAppIntent(id))
            .setSound(sound)
            // 同一条消息可能被 UI 进程和后台进程各弹一次，同 ID + 只响一次即可去重。
            .setOnlyAlertOnce(true)
            .setAutoCancel(true)
            .build()
        manager.notify(id, notification)
    }

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

        /** Dart 侧的后台入口函数名（定义在 lib/main.dart 的根库里）。 */
        const val ENTRYPOINT = "backgroundMain"

        /** 后台进程与 UI 进程共用的原生通道。 */
        const val CHANNEL_NOTIFY = "liant/background_notify"
        const val CHANNEL_STATE = "liant/background_state"

        private const val MESSAGE_CHANNEL = "liant.notifications.v2"

        /**
         * UI 进程写、后台进程读的共享状态文件。
         *
         * 存在即代表「已配对」：BootReceiver 靠它决定开机要不要拉起服务。
         */
        fun stateFile(context: Context): File = CoreFiles.stateFile(context)

        /** 后台进程写给 UI 的桥接信息（本地端口 + 令牌）。 */
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
