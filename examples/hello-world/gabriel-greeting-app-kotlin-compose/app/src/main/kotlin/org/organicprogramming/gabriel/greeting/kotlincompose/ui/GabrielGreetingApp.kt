package org.organicprogramming.gabriel.greeting.kotlincompose.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowDropDown
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.WarningAmber
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import org.organicprogramming.gabriel.greeting.kotlincompose.controller.CoaxController
import org.organicprogramming.gabriel.greeting.kotlincompose.controller.GreetingController
import org.organicprogramming.gabriel.greeting.kotlincompose.model.CoaxServerTransport
import org.organicprogramming.gabriel.greeting.kotlincompose.model.CoaxSurfaceState
import org.organicprogramming.gabriel.greeting.kotlincompose.model.CoaxUiState
import org.organicprogramming.gabriel.greeting.kotlincompose.model.GabrielHolonIdentity
import org.organicprogramming.gabriel.greeting.kotlincompose.model.GreetingUiState
import org.organicprogramming.gabriel.greeting.kotlincompose.model.LanguageOption
import org.organicprogramming.gabriel.greeting.kotlincompose.model.transportTitle

private val headerPadding = 32.dp
private val workspacePadding = 32.dp
private val holonPickerWidth = 260.dp
private val holonGroupWidth = 360.dp
private val runtimePickerWidth = 140.dp
private val runtimeGroupWidth = 240.dp
private val languagePickerWidth = 260.dp
private val surfaceShape = RoundedCornerShape(10.dp)
private val fieldShape = RoundedCornerShape(10.dp)
private val borderColor = Color(0xFFD8DCE3)
private val mutedTextColor = Color(0xFF6B7280)
private val offlineColor = Color(0xFFD96A6A)
private val readyColor = Color(0xFF66B85E)
private val loadingColor = Color(0xFFD2A243)

@Composable
fun GabrielGreetingApp(
    greetingController: GreetingController,
    coaxController: CoaxController,
) {
    val greetingState by greetingController.state.collectAsState()
    val coaxState by coaxController.state.collectAsState()
    val scope = rememberCoroutineScope()
    var showCoaxSettings by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        greetingController.initialize()
        coaxController.startIfEnabled()
    }

    MaterialTheme(colorScheme = GabrielColors) {
        GreetingScreen(
            greetingState = greetingState,
            coaxState = coaxState,
            onToggleCoax = { enabled ->
                scope.launch { coaxController.setIsEnabled(enabled) }
            },
            onOpenCoaxSettings = { showCoaxSettings = true },
            onSelectHolon = { slug ->
                scope.launch { greetingController.selectHolonBySlug(slug) }
            },
            onSelectTransport = { transport ->
                scope.launch { greetingController.setTransport(transport) }
            },
            onNameChange = { value ->
                scope.launch { greetingController.setUserName(value) }
            },
            onSelectLanguage = { code ->
                scope.launch { greetingController.setSelectedLanguage(code) }
            },
        )

        if (showCoaxSettings) {
            CoaxSettingsDialog(
                state = coaxState,
                onDismiss = { showCoaxSettings = false },
                onToggleServer = { enabled ->
                    scope.launch { coaxController.setServerEnabled(enabled) }
                },
                onTransportChange = { transport ->
                    scope.launch { coaxController.setServerTransport(transport) }
                },
                onHostChange = { host ->
                    scope.launch { coaxController.setServerHost(host) }
                },
                onPortChange = { port ->
                    scope.launch { coaxController.setServerPortText(port) }
                },
                onUnixPathChange = { path ->
                    scope.launch { coaxController.setServerUnixPath(path) }
                },
            )
        }
    }
}

@Composable
fun GreetingScreen(
    greetingState: GreetingUiState,
    coaxState: CoaxUiState,
    onToggleCoax: (Boolean) -> Unit,
    onOpenCoaxSettings: () -> Unit,
    onSelectHolon: (String) -> Unit,
    onSelectTransport: (String) -> Unit,
    onNameChange: (String) -> Unit,
    onSelectLanguage: (String) -> Unit,
) {
    Surface(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .testTag("app-root"),
        color = MaterialTheme.colorScheme.background,
    ) {
        Column(modifier = Modifier.fillMaxSize()) {
            TopHeader(
                coaxState = coaxState,
                onToggleCoax = onToggleCoax,
                onOpenCoaxSettings = onOpenCoaxSettings,
            )
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.75f))
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(workspacePadding),
            ) {
                WorkspaceBar(
                    state = greetingState,
                    onSelectHolon = onSelectHolon,
                    onSelectTransport = onSelectTransport,
                )
                Spacer(modifier = Modifier.height(32.dp))
                BoxWithConstraints(
                    modifier = Modifier
                        .weight(1f)
                        .fillMaxWidth(),
                ) {
                    val inputWidth = (maxWidth * 0.27f).coerceIn(220.dp, 320.dp)
                    val gapWidth = (maxWidth * 0.03f).coerceIn(18.dp, 32.dp)
                    Row(
                        modifier = Modifier.fillMaxSize(),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        InputColumn(
                            state = greetingState,
                            width = inputWidth,
                            onNameChange = onNameChange,
                        )
                        Spacer(modifier = Modifier.width(gapWidth))
                        BubbleColumn(greetingState = greetingState)
                    }
                }
                Spacer(modifier = Modifier.height(28.dp))
                Box(
                    modifier = Modifier.fillMaxWidth(),
                    contentAlignment = Alignment.Center,
                ) {
                    LanguagePicker(
                        greetingState = greetingState,
                        onSelectLanguage = onSelectLanguage,
                    )
                }
            }
        }
    }
}

@Composable
private fun TopHeader(
    coaxState: CoaxUiState,
    onToggleCoax: (Boolean) -> Unit,
    onOpenCoaxSettings: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = headerPadding, vertical = 16.dp),
        horizontalArrangement = Arrangement.End,
        verticalAlignment = Alignment.Top,
    ) {
        Column(
            horizontalAlignment = Alignment.End,
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                Text(
                    text = "COAX",
                    style = MaterialTheme.typography.bodySmall.copy(
                        fontWeight = FontWeight.SemiBold,
                        fontFamily = FontFamily.Monospace,
                    ),
                )
                Switch(
                    checked = coaxState.isEnabled,
                    onCheckedChange = onToggleCoax,
                    modifier = Modifier.testTag("coax-toggle"),
                )
                Surface(
                    color = MaterialTheme.colorScheme.surfaceVariant,
                    shape = CircleShape,
                    modifier = Modifier.size(32.dp),
                ) {
                    IconButton(onClick = onOpenCoaxSettings) {
                        Icon(
                            imageVector = Icons.Filled.Settings,
                            contentDescription = "Open COAX settings",
                            tint = mutedTextColor,
                        )
                    }
                }
            }

            coaxState.serverStatus.endpoint?.let { endpoint ->
                Row(
                    modifier = Modifier.widthIn(max = 520.dp),
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalAlignment = Alignment.Top,
                ) {
                    Text(
                        text = "${coaxState.serverStatus.title}:",
                        style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.SemiBold),
                        color = mutedTextColor,
                    )
                    SelectionContainer {
                        Text(
                            text = endpoint,
                            style = MaterialTheme.typography.labelSmall.copy(fontFamily = FontFamily.Monospace),
                            color = mutedTextColor,
                            textAlign = TextAlign.End,
                        )
                    }
                    Text(
                        text = coaxState.serverStatus.state.badgeTitle,
                        style = MaterialTheme.typography.labelSmall.copy(
                            fontWeight = FontWeight.Bold,
                            fontFamily = FontFamily.Monospace,
                            fontSize = 9.sp,
                        ),
                        color = surfaceBadgeColor(coaxState.serverStatus.state),
                    )
                }
            }

            coaxState.statusDetail?.let { detail ->
                Text(
                    text = detail,
                    modifier = Modifier.width(320.dp),
                    style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.Medium),
                    color = loadingColor,
                    textAlign = TextAlign.End,
                )
            }
        }
    }
}

@Composable
private fun WorkspaceBar(
    state: GreetingUiState,
    onSelectHolon: (String) -> Unit,
    onSelectTransport: (String) -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.Bottom,
    ) {
        HolonHeaderGroup(
            state = state,
            onSelectHolon = onSelectHolon,
        )
        Spacer(modifier = Modifier.weight(1f))
        RuntimeHeaderGroup(
            state = state,
            onSelectTransport = onSelectTransport,
        )
    }
}

@Composable
private fun HolonHeaderGroup(
    state: GreetingUiState,
    onSelectHolon: (String) -> Unit,
) {
    Column(
        modifier = Modifier
            .width(holonGroupWidth)
            .testTag("holon-list"),
            verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        DropdownField(
            value = state.selectedHolon,
            options = state.availableHolons,
            placeholder = "Loading holons...",
            width = holonPickerWidth,
            labelFor = { it.displayName },
            onSelect = { onSelectHolon(it.slug) },
        )
        state.selectedHolon?.let { holon ->
            SelectionContainer {
                Text(
                    text = holon.slug,
                    style = MaterialTheme.typography.labelSmall.copy(fontFamily = FontFamily.Monospace),
                    color = mutedTextColor,
                )
            }
        }
    }
}

@Composable
private fun RuntimeHeaderGroup(
    state: GreetingUiState,
    onSelectTransport: (String) -> Unit,
) {
    Column(
        modifier = Modifier.width(runtimeGroupWidth),
        horizontalAlignment = Alignment.End,
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(
                text = "mode:",
                style = MaterialTheme.typography.bodySmall.copy(fontWeight = FontWeight.SemiBold),
            )
            DropdownField(
                value = state.transport,
                options = listOf("stdio", "unix", "tcp"),
                placeholder = "stdio",
                width = runtimePickerWidth,
                labelFor = { transportTitle(it) },
                onSelect = onSelectTransport,
            )
        }
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(
                text = state.statusTitle,
                style = MaterialTheme.typography.bodySmall.copy(fontWeight = FontWeight.SemiBold),
            )
            RuntimeDot(
                isLoading = state.isLoading,
                isRunning = state.isRunning,
            )
        }
    }
}

@Composable
private fun InputColumn(
    state: GreetingUiState,
    width: androidx.compose.ui.unit.Dp,
    onNameChange: (String) -> Unit,
) {
    Box(
        modifier = Modifier.width(width),
        contentAlignment = Alignment.CenterStart,
    ) {
        OutlinedTextField(
            value = state.userName,
            onValueChange = onNameChange,
            modifier = Modifier
                .width(width)
                .testTag("name-input"),
            placeholder = { Text("World") },
            singleLine = true,
            shape = fieldShape,
        )
    }
}

@Composable
private fun BubbleColumn(greetingState: GreetingUiState) {
    val bubbleFillColor = MaterialTheme.colorScheme.surface
    val bubbleStrokeColor = MaterialTheme.colorScheme.outline.copy(alpha = 0.75f)
    Box(
        modifier = Modifier
            .fillMaxSize()
            .testTag("greeting-bubble")
            .drawBehind {
                val path = leftPointerBubblePath(
                    size = size,
                    pointerSize = 14.dp.toPx(),
                    cornerRadius = 16.dp.toPx(),
                )
                drawPath(path = path, color = bubbleFillColor)
                drawPath(
                    path = path,
                    color = bubbleStrokeColor,
                    style = Stroke(
                        width = 1.5.dp.toPx(),
                        pathEffect = PathEffect.dashPathEffect(
                            intervals = floatArrayOf(1f, 5.dp.toPx()),
                        ),
                    ),
                )
            }
            .padding(start = 40.dp, top = 32.dp, end = 32.dp, bottom = 32.dp),
        contentAlignment = Alignment.Center,
    ) {
        BubbleContent(greetingState = greetingState)
    }
}

@Composable
private fun BubbleContent(greetingState: GreetingUiState) {
    when {
        greetingState.connectionError != null -> {
            ErrorPanel(
                title = "Holon Offline",
                message = greetingState.connectionError,
            )
        }
        greetingState.error != null -> {
            ErrorPanel(
                title = "Error",
                message = greetingState.error,
            )
        }
        else -> {
            val greeting = when {
                greetingState.greeting.isNotBlank() -> greetingState.greeting
                greetingState.isGreeting -> "..."
                else -> ""
            }
            SelectionContainer {
                Text(
                    text = greeting,
                    style = MaterialTheme.typography.headlineMedium.copy(
                        fontWeight = FontWeight.Medium,
                        fontSize = 42.sp,
                    ),
                    textAlign = TextAlign.Center,
                )
            }
        }
    }
}

@Composable
private fun ErrorPanel(
    title: String,
    message: String?,
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(12.dp),
        horizontalAlignment = Alignment.Start,
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Icon(
                imageVector = Icons.Filled.WarningAmber,
                contentDescription = null,
                tint = offlineColor,
                modifier = Modifier.size(20.dp),
            )
            Text(
                text = title,
                style = MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.Bold),
                color = offlineColor,
            )
        }
        SelectionContainer {
            Text(
                text = message.orEmpty(),
                style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
            )
        }
    }
}

@Composable
private fun LanguagePicker(
    greetingState: GreetingUiState,
    onSelectLanguage: (String) -> Unit,
) {
    val selectedLanguage = greetingState.availableLanguages.firstOrNull {
        it.code == greetingState.selectedLanguageCode
    }
    DropdownField(
        value = selectedLanguage,
        options = greetingState.availableLanguages,
        placeholder = if (greetingState.isLoading) "Loading..." else "Select language",
        width = languagePickerWidth,
        labelFor = { languageTitle(it) },
        onSelect = { onSelectLanguage(it.code) },
        modifier = Modifier.testTag("language-row"),
    )
}

@Composable
private fun <T> DropdownField(
    value: T?,
    options: List<T>,
    placeholder: String,
    width: androidx.compose.ui.unit.Dp,
    labelFor: (T) -> String,
    onSelect: (T) -> Unit,
    modifier: Modifier = Modifier,
) {
    var expanded by remember { mutableStateOf(false) }
    val interactionSource = remember { MutableInteractionSource() }

    Box(modifier = modifier.width(width)) {
        Surface(
            modifier = Modifier
                .fillMaxWidth()
                .clickable(
                    interactionSource = interactionSource,
                    indication = null,
                ) {
                    expanded = true
                },
            shape = fieldShape,
            color = MaterialTheme.colorScheme.surface,
            border = BorderStroke(1.dp, borderColor),
        ) {
            Row(
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                Text(
                    text = value?.let(labelFor) ?: placeholder,
                    modifier = Modifier.weight(1f),
                    style = MaterialTheme.typography.bodyMedium,
                    color = if (value == null) mutedTextColor else MaterialTheme.colorScheme.onSurface,
                )
                Icon(
                    imageVector = Icons.Filled.ArrowDropDown,
                    contentDescription = null,
                    tint = mutedTextColor,
                )
            }
        }
        DropdownMenu(
            expanded = expanded,
            onDismissRequest = { expanded = false },
            modifier = Modifier
                .widthIn(min = width, max = width)
                .heightIn(max = 320.dp),
        ) {
            options.forEach { option ->
                DropdownMenuItem(
                    text = { Text(labelFor(option)) },
                    onClick = {
                        expanded = false
                        onSelect(option)
                    },
                )
            }
        }
    }
}

@Composable
private fun RuntimeDot(
    isLoading: Boolean,
    isRunning: Boolean,
) {
    val color = when {
        isLoading -> loadingColor
        isRunning -> readyColor
        else -> offlineColor
    }
    Box(
        modifier = Modifier
            .size(10.dp)
            .background(color = color, shape = CircleShape),
    )
}

@Composable
private fun CoaxSettingsDialog(
    state: CoaxUiState,
    onDismiss: () -> Unit,
    onToggleServer: (Boolean) -> Unit,
    onTransportChange: (CoaxServerTransport) -> Unit,
    onHostChange: (String) -> Unit,
    onPortChange: (String) -> Unit,
    onUnixPathChange: (String) -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        modifier = Modifier.widthIn(min = 720.dp, max = 720.dp),
        confirmButton = {
            TextButton(onClick = onDismiss) {
                Text("Done")
            }
        },
        title = {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Text(
                    text = "COAX",
                    style = MaterialTheme.typography.headlineSmall.copy(fontWeight = FontWeight.SemiBold),
                )
                Text(
                    text = "Configure the server surface.",
                    style = MaterialTheme.typography.bodySmall,
                    color = mutedTextColor,
                )
            }
        },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    Text(
                        text = "Enabled",
                        style = MaterialTheme.typography.bodySmall.copy(fontWeight = FontWeight.Medium),
                        color = mutedTextColor,
                    )
                    Switch(checked = state.isEnabled, onCheckedChange = {})
                }

                SettingsSection(
                    title = "Server",
                    subtitle = "Expose the embedded runtime directly.",
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        Text(
                            text = "Enable this surface",
                            modifier = Modifier.width(110.dp),
                            style = MaterialTheme.typography.bodySmall.copy(fontWeight = FontWeight.SemiBold),
                        )
                        Switch(
                            checked = state.serverEnabled,
                            onCheckedChange = onToggleServer,
                        )
                    }

                    SettingRow(label = "Transport") {
                        DropdownField(
                            value = state.serverTransport,
                            options = state.capabilities.coaxServerTransports,
                            placeholder = "TCP",
                            width = 250.dp,
                            labelFor = { it.title },
                            onSelect = onTransportChange,
                        )
                    }

                    if (state.serverTransport == CoaxServerTransport.TCP) {
                        SettingRow(label = "Host") {
                            OutlinedTextField(
                                value = state.serverHost,
                                onValueChange = onHostChange,
                                singleLine = true,
                                placeholder = { Text("127.0.0.1") },
                                shape = fieldShape,
                            )
                        }
                        SettingRow(label = "Port") {
                            OutlinedTextField(
                                value = state.serverPortText,
                                onValueChange = onPortChange,
                                singleLine = true,
                                shape = fieldShape,
                                placeholder = { Text(state.serverTransport.defaultPort.toString()) },
                                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                            )
                        }
                    } else {
                        SettingRow(label = "Socket path") {
                            OutlinedTextField(
                                value = state.serverUnixPath,
                                onValueChange = onUnixPathChange,
                                singleLine = true,
                                shape = fieldShape,
                                placeholder = { Text("/tmp/gabriel-greeting-coax.sock") },
                            )
                        }
                    }

                    state.serverPortValidationMessage?.let { message ->
                        Text(
                            text = message,
                            style = MaterialTheme.typography.bodySmall,
                            color = mutedTextColor,
                        )
                    }

                    SettingsSection(title = "Endpoint", subtitle = null) {
                        SelectionContainer {
                            Text(
                                text = state.serverStatus.endpoint ?: state.serverPreviewEndpoint,
                                style = MaterialTheme.typography.bodyMedium.copy(fontFamily = FontFamily.Monospace),
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .background(
                                        color = MaterialTheme.colorScheme.surfaceVariant,
                                        shape = surfaceShape,
                                    )
                                    .padding(horizontal = 12.dp, vertical = 14.dp),
                            )
                        }
                    }
                }
            }
        },
    )
}

@Composable
private fun SettingsSection(
    title: String,
    subtitle: String?,
    content: @Composable () -> Unit,
) {
    Surface(
        shape = RoundedCornerShape(14.dp),
        color = MaterialTheme.colorScheme.surface,
        border = BorderStroke(1.dp, borderColor),
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                text = title,
                style = MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.SemiBold),
            )
            subtitle?.let {
                Text(
                    text = it,
                    style = MaterialTheme.typography.bodySmall,
                    color = mutedTextColor,
                )
            }
            content()
        }
    }
}

@Composable
private fun SettingRow(
    label: String,
    content: @Composable () -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text(
            text = label,
            modifier = Modifier.width(110.dp),
            style = MaterialTheme.typography.bodySmall.copy(fontWeight = FontWeight.SemiBold),
        )
        Box(modifier = Modifier.weight(1f)) {
            content()
        }
    }
}

private fun leftPointerBubblePath(
    size: Size,
    pointerSize: Float,
    cornerRadius: Float,
): Path =
    Path().apply {
        val minX = pointerSize
        val maxX = size.width
        val minY = 0f
        val maxY = size.height
        val pointerCenterY = size.height / 2f

        moveTo(minX + cornerRadius, minY)
        lineTo(maxX - cornerRadius, minY)
        arcTo(
            rect = Rect(maxX - (cornerRadius * 2), minY, maxX, minY + (cornerRadius * 2)),
            startAngleDegrees = -90f,
            sweepAngleDegrees = 90f,
            forceMoveTo = false,
        )
        lineTo(maxX, maxY - cornerRadius)
        arcTo(
            rect = Rect(maxX - (cornerRadius * 2), maxY - (cornerRadius * 2), maxX, maxY),
            startAngleDegrees = 0f,
            sweepAngleDegrees = 90f,
            forceMoveTo = false,
        )
        lineTo(minX + cornerRadius, maxY)
        arcTo(
            rect = Rect(minX, maxY - (cornerRadius * 2), minX + (cornerRadius * 2), maxY),
            startAngleDegrees = 90f,
            sweepAngleDegrees = 90f,
            forceMoveTo = false,
        )
        lineTo(minX, pointerCenterY + pointerSize)
        lineTo(0f, pointerCenterY)
        lineTo(minX, pointerCenterY - pointerSize)
        lineTo(minX, minY + cornerRadius)
        arcTo(
            rect = Rect(minX, minY, minX + (cornerRadius * 2), minY + (cornerRadius * 2)),
            startAngleDegrees = 180f,
            sweepAngleDegrees = 90f,
            forceMoveTo = false,
        )
        close()
    }

private fun surfaceBadgeColor(state: CoaxSurfaceState): Color =
    when (state) {
        CoaxSurfaceState.OFF -> mutedTextColor
        CoaxSurfaceState.SAVED -> Color.Gray
        CoaxSurfaceState.ANNOUNCED -> loadingColor
        CoaxSurfaceState.LIVE -> readyColor
        CoaxSurfaceState.ERROR -> offlineColor
    }

private fun languageTitle(language: LanguageOption): String =
    when {
        language.nativeName.isNotBlank() && language.name.isNotBlank() ->
            "${language.nativeName} (${language.name})"
        language.nativeName.isNotBlank() -> language.nativeName
        language.name.isNotBlank() -> language.name
        else -> language.code
    }
