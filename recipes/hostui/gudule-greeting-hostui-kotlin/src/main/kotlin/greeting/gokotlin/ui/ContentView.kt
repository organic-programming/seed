package greeting.gokotlin.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.RoundRect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import greeting.gokotlin.grpc.DaemonProcess
import greeting.gokotlin.grpc.GreetingLanguage
import kotlinx.coroutines.launch

@Composable
fun ContentView(daemon: DaemonProcess, assemblyFamily: String) {
    val scope = rememberCoroutineScope()
    val languages = remember { mutableStateListOf<GreetingLanguage>() }
    val subtitle = remember { daemon.subtitleText() }
    var selectedLanguage by remember { mutableStateOf<GreetingLanguage?>(null) }
    var languageMenuExpanded by remember { mutableStateOf(false) }
    var userName by remember { mutableStateOf("World!") }
    var greeting by remember { mutableStateOf("Select a language and type a name") }
    var status by remember { mutableStateOf("Loading languages…") }
    var errorMsg by remember { mutableStateOf<String?>(null) }

    val mainAppColor = Color(0xFF141414)
    val headerColor = Color(0xFF212121)

    // Helper to trigger greeting load
    fun triggerGreeting() {
        val lang = selectedLanguage ?: return
        if (userName.isBlank()) return
        scope.launch {
            try {
                greeting = daemon.client().sayHello(userName, lang.code)
                errorMsg = null
            } catch (e: Exception) {
                errorMsg = e.message ?: "Unknown error calling Daemon"
            }
        }
    }

    LaunchedEffect(Unit) {
        runCatching { daemon.client().listLanguages() }
            .onSuccess {
                languages.clear()
                languages.addAll(it)
                selectedLanguage = it.firstOrNull { language -> language.code == "en" } ?: it.firstOrNull()
                status = "Ready"
                triggerGreeting()
            }
            .onFailure {
                status = "Failed to connect"
                errorMsg = it.message ?: "Failed to load languages"
            }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(mainAppColor)
    ) {
        Column(modifier = Modifier.fillMaxSize()) {
            // == Top Header Area ==
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(headerColor)
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 32.dp, vertical = 20.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.Top
                ) {
                    // Left Side
                    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Text(
                            text = "Gudule : $assemblyFamily",
                            style = TextStyle(
                                fontSize = 24.sp,
                                fontWeight = FontWeight.Bold,
                                color = Color.White
                            )
                        )
                        Text(
                            text = subtitle,
                            style = TextStyle(
                                fontSize = 14.sp,
                                color = Color.White.copy(alpha = 0.5f)
                            )
                        )
                    }

                    // Right Side (Mode & Status)
                    Column(
                        horizontalAlignment = Alignment.End,
                        verticalArrangement = Arrangement.spacedBy(4.dp)
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                            Text(
                                "mode:",
                                style = TextStyle(fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = Color.White)
                            )
                            Box(
                                modifier = Modifier
                                    .background(Color.White.copy(alpha = 0.15f), RoundedCornerShape(4.dp))
                                    .padding(horizontal = 12.dp, vertical = 4.dp)
                            ) {
                                Text(
                                    "stdio", // hardcoded matching SwiftUI behaviour for stdio/network distinction
                                    style = TextStyle(fontSize = 13.sp, color = Color.White.copy(alpha = 0.6f))
                                )
                            }
                        }

                        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                            Text(
                                status,
                                style = TextStyle(fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = Color.White)
                            )
                            Box(
                                modifier = Modifier
                                    .size(10.dp)
                                    .background(if (status == "Ready") Color(0xFF34C759) else Color.Red, RoundedCornerShape(5.dp))
                            )
                        }
                    }
                }
                
                // Bottom border line for header
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(1.dp)
                        .background(Color.White.copy(alpha = 0.06f))
                        .align(Alignment.BottomCenter)
                )
            }

            // == Main Content split ==
            Box(modifier = Modifier.fillMaxSize()) {
                // Center Alignment Wrapper
                Row(
                    modifier = Modifier.fillMaxSize().padding(32.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(32.dp)
                ) {
                    // Controls Column
                    Column(
                        verticalArrangement = Arrangement.spacedBy(5.dp),
                        modifier = Modifier.width(260.dp)
                    ) {
                        BasicTextField(
                            value = userName,
                            onValueChange = { userName = it; triggerGreeting() },
                            textStyle = TextStyle(color = Color.White.copy(alpha = 0.9f), fontSize = 14.sp),
                            cursorBrush = SolidColor(Color.White),
                            modifier = Modifier
                                .fillMaxWidth()
                                .heightIn(min = 100.dp, max = 200.dp)
                                .background(Color.White.copy(alpha = 0.05f), RoundedCornerShape(6.dp))
                                .padding(12.dp)
                        )
                    }

                    // Result Bubble Wrapper
                    Box(modifier = Modifier.weight(1f).fillMaxHeight()) {
                        BubbleShape {
                            if (errorMsg != null) {
                                Column(modifier = Modifier.padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                                        Text("⚠️ Daemon Offline", color = Color(0xFFFF6666), fontWeight = FontWeight.Bold, fontSize = 18.sp)
                                    }
                                    Text(errorMsg!!, color = Color.White.copy(alpha = 0.85f), fontSize = 13.sp)
                                }
                            } else {
                                Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                                    Text(
                                        text = greeting,
                                        style = TextStyle(fontSize = 32.sp, fontWeight = FontWeight.Medium, color = Color.White),
                                        modifier = Modifier.padding(start = 20.dp, end = 16.dp, top = 16.dp, bottom = 16.dp)
                                    )
                                }
                            }
                        }
                    }
                }

                // Bottom Left Absolute position Language Picker
                Column(
                    modifier = Modifier.align(Alignment.BottomStart).padding(32.dp),
                    verticalArrangement = Arrangement.spacedBy(5.dp)
                ) {
                    Box {
                        Box(
                            modifier = Modifier
                                .background(Color.White.copy(alpha = 0.15f), RoundedCornerShape(6.dp))
                                .clickable { languageMenuExpanded = true }
                                .padding(horizontal = 12.dp, vertical = 6.dp)
                                .widthIn(min = 160.dp)
                        ) {
                            Text(
                                text = selectedLanguage?.let { "${it.native} (${it.name})" } ?: if (status == "Ready") "Select..." else "Loading...",
                                color = Color.White,
                                fontSize = 13.sp
                            )
                        }
                        DropdownMenu(
                            expanded = languageMenuExpanded,
                            onDismissRequest = { languageMenuExpanded = false },
                            modifier = Modifier.background(headerColor)
                        ) {
                            languages.forEach { lang ->
                                DropdownMenuItem(
                                    text = { Text("${lang.native} (${lang.name})", color = Color.White) },
                                    onClick = {
                                        selectedLanguage = lang
                                        languageMenuExpanded = false
                                        triggerGreeting()
                                    }
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun BubbleShape(content: @Composable () -> Unit) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .drawBehind {
                val cornerRadius = 24.dp.toPx()
                val pointerWidth = 20.dp.toPx()
                val pointerHeight = 24.dp.toPx()

                val path = Path().apply {
                    val rect = RoundRect(
                        rect = Rect(
                            left = pointerWidth,
                            top = 0f,
                            right = size.width,
                            bottom = size.height
                        ),
                        cornerRadius = CornerRadius(cornerRadius)
                    )
                    addRoundRect(rect)

                    val midY = size.height / 2f
                    moveTo(pointerWidth, midY - pointerHeight / 2f)
                    lineTo(0f, midY)
                    lineTo(pointerWidth, midY + pointerHeight / 2f)
                    close()
                }

                drawPath(
                    path = path,
                    color = Color.White.copy(alpha = 0.4f),
                    style = Stroke(
                        width = 1.5.dp.toPx(),
                        pathEffect = PathEffect.dashPathEffect(floatArrayOf(0.1f, 15f), 0f),
                        cap = StrokeCap.Round,
                        join = StrokeJoin.Round
                    )
                )
            }
    ) {
        // We push content inside so it doesn't overlap the left pointer padding
        Box(modifier = Modifier.padding(start = 20.dp).fillMaxSize()) {
            content()
        }
    }
}
