package com.bailensn.liant.ui

import androidx.compose.foundation.text.input.TextFieldState
import androidx.compose.foundation.text.input.edit

/** 整体替换 TextFieldState 的内容，并把光标放到末尾。 */
fun TextFieldState.setAll(value: String) {
    edit {
        replace(0, length, value)
        placeCursorAtEnd()
    }
}
