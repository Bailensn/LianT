package com.bailensn.liant

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.Build

class NotificationRestartReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val service = Intent(context, MessageService::class.java)
            .setAction(MessageService.ACTION_REPOST)
        try {
            if (Build.VERSION.SDK_INT >= 26) {
                context.startForegroundService(service)
            } else {
                context.startService(service)
            }
        } catch (_: Exception) {
            // 服务通常还在运行，下一次定时重发会补上通知。
        }
    }
}
