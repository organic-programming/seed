package greeting.gokotlin.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import greeting.gokotlin.grpc.DaemonProcess
import greeting.gokotlin.grpc.GreetingLanguage
import kotlinx.coroutines.launch

@Composable
fun ContentView(daemon: DaemonProcess) {
    val scope = rememberCoroutineScope()
    val languages = remember { mutableStateListOf<GreetingLanguage>() }
    var selectedLanguage by remember { mutableStateOf<GreetingLanguage?>(null) }
    var languageMenuExpanded by remember { mutableStateOf(false) }
    var userName by remember { mutableStateOf("World") }
    var greeting by remember { mutableStateOf("Select a language and press Greet.") }
    var status by remember { mutableStateOf("Loading languages…") }

    LaunchedEffect(Unit) {
        runCatching { daemon.client().listLanguages() }
            .onSuccess {
                languages.clear()
                languages.addAll(it)
                selectedLanguage = it.firstOrNull { language -> language.code == "en" } ?: it.firstOrNull()
                status = ""
            }
            .onFailure {
                status = it.message ?: "Failed to load languages"
            }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .padding(24.dp),
    ) {
        Row(modifier = Modifier.fillMaxSize()) {
            Card(modifier = Modifier.width(300.dp).fillMaxHeight()) {
                Column(modifier = Modifier.padding(20.dp)) {
                    Text("Languages", style = MaterialTheme.typography.titleMedium)
                    Spacer(Modifier.height(16.dp))
                    LazyColumn(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        items(languages) { language ->
                            Card(
                                modifier = Modifier.fillMaxWidth(),
                                onClick = {
                                    selectedLanguage = language
                                },
                            ) {
                                Column(modifier = Modifier.padding(14.dp)) {
                                    Text(language.native, fontWeight = FontWeight.Bold)
                                    Text(language.name, style = MaterialTheme.typography.bodySmall)
                                }
                            }
                        }
                    }
                }
            }

            Spacer(Modifier.width(20.dp))

            Card(modifier = Modifier.weight(1f).fillMaxHeight()) {
                Column(modifier = Modifier.fillMaxSize().padding(28.dp)) {
                    Text("Gudule Greeting Gokotlin", style = MaterialTheme.typography.headlineMedium)
                    Spacer(Modifier.height(12.dp))
                    Text(
                        "Compose Desktop talking to the Go daemon over localhost gRPC.",
                        style = MaterialTheme.typography.bodyMedium,
                    )
                    Spacer(Modifier.height(20.dp))
                    Box {
                        OutlinedTextField(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable { languageMenuExpanded = true },
                            value = selectedLanguage?.let { "${it.native} (${it.name})" } ?: "",
                            onValueChange = {},
                            readOnly = true,
                            label = { Text("Language") },
                        )
                        DropdownMenu(
                            expanded = languageMenuExpanded,
                            onDismissRequest = { languageMenuExpanded = false },
                        ) {
                            languages.forEach { language ->
                                DropdownMenuItem(
                                    text = { Text("${language.native} (${language.name})") },
                                    onClick = {
                                        selectedLanguage = language
                                        languageMenuExpanded = false
                                    },
                                )
                            }
                        }
                    }
                    Spacer(Modifier.height(28.dp))
                    Card(modifier = Modifier.fillMaxWidth().weight(1f)) {
                        Box(modifier = Modifier.fillMaxSize().padding(24.dp)) {
                            Text(greeting, style = MaterialTheme.typography.headlineLarge)
                        }
                    }
                    Spacer(Modifier.height(18.dp))
                    OutlinedTextField(
                        modifier = Modifier.fillMaxWidth(),
                        value = userName,
                        onValueChange = { userName = it },
                        label = { Text("Your name") },
                    )
                    Spacer(Modifier.height(12.dp))
                    Button(
                        onClick = {
                            val language = selectedLanguage ?: return@Button
                            scope.launch {
                                greeting = daemon.client().sayHello(userName, language.code)
                            }
                        },
                    ) {
                        Text("Greet")
                    }
                    Spacer(Modifier.height(12.dp))
                    Text(status, style = MaterialTheme.typography.bodySmall)
                }
            }
        }
    }
}
