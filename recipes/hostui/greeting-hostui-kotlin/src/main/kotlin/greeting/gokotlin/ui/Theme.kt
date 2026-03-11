package greeting.gokotlin.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

private val GokotlinColors = darkColorScheme(
    primary = Color(0xFF7CE5FF),
    secondary = Color(0xFFB0F89A),
    background = Color(0xFF0A1220),
    surface = Color(0xFF122038),
    surfaceVariant = Color(0xFF0E1A2D),
)

@Composable
fun GokotlinTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = if (isSystemInDarkTheme()) GokotlinColors else GokotlinColors,
        content = content,
    )
}
