package com.nocturneosint.app

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import com.nocturneosint.app.presentation.NocturneApp
import com.nocturneosint.app.presentation.theme.NocturneTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent { NocturneTheme { NocturneApp() } }
    }
}
