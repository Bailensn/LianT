package com.bailensn.liant

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log

/**
 * 把常驻服务拉回来。三个来源共用：
 *
 * - 通知被划掉（delete intent，没有 action）：立刻重发常驻通知；
 * - MessageService.onTaskRemoved 排的兜底闹钟（[MessageService.ACTION_WAKE]）；
 * - 开机（BOOT_COMPLETED / QUICKBOOT_POWERON）：只有已经配对过才拉起。
 *
 * Android 12+ 从后台启动前台服务会被系统拒绝（ForegroundServiceStartNotAllowedException），
 * 这里只能尽力而为；真正可靠的路径是用户打开 App 时由 MainActivity 拉起。
 */
class RestartReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val action = intent.action
        val boot = action == Intent.ACTION_BOOT_COMPLETED ||
            action == "android.intent.action.QUICKBOOT_POWERON"
        if (boot && !MessageService.stateFile(context).exists()) return
        try {
            MessageService.start(context, MessageService.ACTION_WAKE)
        } catch (e: Exception) {
            Log.w("LianT", "拉起后台服务失败（action=$action）", e)
        }
    }
}
