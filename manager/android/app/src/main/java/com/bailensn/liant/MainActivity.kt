package com.bailensn.liant

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import com.bailensn.liant.ui.AppRoot
import com.bailensn.liant.ui.theme.LianTTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        setContent {
            LianTTheme {
                AppRoot()
            }
        }
    }
}
