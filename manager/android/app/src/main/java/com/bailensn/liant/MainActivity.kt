package com.bailensn.liant

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.ContextCompat
import com.bailensn.liant.notify.DeepLink
import com.bailensn.liant.notify.MessageNotifier
import com.bailensn.liant.ui.AppRoot
import com.bailensn.liant.ui.theme.LianTTheme

class MainActivity : ComponentActivity() {

    /** Android 13+ 的通知权限；拒绝后仍可收到应用内消息，只是不弹系统通知。 */
    private val requestNotifications =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { /* 结果不影响主流程 */ }

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        handleDeepLink(intent)
        ensureNotificationPermission()
        setContent {
            LianTTheme {
                AppRoot()
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleDeepLink(intent)
    }

    private fun handleDeepLink(intent: Intent?) {
        val botId = intent?.getStringExtra(MessageNotifier.EXTRA_BOT_ID) ?: return
        val peerId = intent.getStringExtra(MessageNotifier.EXTRA_PEER_ID) ?: return
        DeepLink.open(botId, peerId)
    }

    private fun ensureNotificationPermission() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return
        val granted = ContextCompat.checkSelfPermission(
            this,
            Manifest.permission.POST_NOTIFICATIONS,
        ) == PackageManager.PERMISSION_GRANTED
        if (!granted) {
            requestNotifications.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
    }
}
