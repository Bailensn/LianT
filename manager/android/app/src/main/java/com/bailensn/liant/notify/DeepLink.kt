package com.bailensn.liant.notify

import androidx.compose.runtime.mutableStateOf

/** 通知点进来的目标会话；消费一次就清空。 */
data class DeepLinkTarget(val botId: String, val peerId: String)

object DeepLink {
    val pending = mutableStateOf<DeepLinkTarget?>(null)

    fun open(botId: String, peerId: String) {
        pending.value = DeepLinkTarget(botId, peerId)
    }

    fun consume() {
        pending.value = null
    }
}
