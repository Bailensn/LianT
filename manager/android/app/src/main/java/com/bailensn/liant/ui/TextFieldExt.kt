package com.bailensn.liant.ui

import androidx.compose.foundation.text.input.TextFieldState

/** 整体替换 TextFieldState 的内容，并把光标放到末尾。 */
fun TextFieldState.setAll(value: String) {
    edit {
        replace(0, length, value)
        placeCursorAfterCharAt(length)
    }
}
