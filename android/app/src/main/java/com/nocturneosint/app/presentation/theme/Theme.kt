package com.nocturneosint.app.presentation.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

/** Tokens from docs/Design-System.md (dark-first). */
object NocturneColors {
    val BackgroundDeep = Color(0xFF0B0D10)
    val Surface = Color(0xFF11151A)
    val SurfaceElevated = Color(0xFF171C22)
    val SurfaceInteractive = Color(0xFF1D232B)
    val Border = Color(0xFF29313A)
    val BorderStrong = Color(0xFF37414C)
    val TextPrimary = Color(0xFFF1F3F5)
    val TextSecondary = Color(0xFFB3BAC3)
    val TextTertiary = Color(0xFF7D8792)
    val Accent = Color(0xFF7C9CFF)
    val Success = Color(0xFF6FCF97)
    val Warning = Color(0xFFE8B86D)
    val Error = Color(0xFFE07878)
}

/** The Design System's 4 dp grid. */
object Spacing {
    val xs = 4.dp
    val s = 8.dp
    val m = 16.dp // default content padding
    val l = 24.dp
    val xl = 32.dp
}

private val DarkColors = darkColorScheme(
    primary = NocturneColors.Accent,
    onPrimary = NocturneColors.BackgroundDeep,
    background = NocturneColors.BackgroundDeep,
    onBackground = NocturneColors.TextPrimary,
    surface = NocturneColors.Surface,
    onSurface = NocturneColors.TextPrimary,
    surfaceVariant = NocturneColors.SurfaceElevated,
    onSurfaceVariant = NocturneColors.TextSecondary,
    surfaceContainer = NocturneColors.SurfaceInteractive,
    outline = NocturneColors.BorderStrong,
    outlineVariant = NocturneColors.Border,
    error = NocturneColors.Error,
    onError = NocturneColors.BackgroundDeep,
)

// ponytail: the Design System defines only the dark palette ("dark theme
// first"); light mode is Material's baseline until a tested light palette exists.
private val LightColors = lightColorScheme()

private val NocturneShapes = Shapes(
    small = RoundedCornerShape(6.dp),
    medium = RoundedCornerShape(10.dp),
    large = RoundedCornerShape(14.dp),
    extraLarge = RoundedCornerShape(20.dp),
)

/** Typography is Material 3's default: Roboto, the Design System's font. */
@Composable
fun NocturneTheme(darkTheme: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = if (darkTheme) DarkColors else LightColors,
        shapes = NocturneShapes,
        content = content,
    )
}
