package org.organicprogramming.gabriel.greeting.kotlincompose.ui

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import org.junit.Rule
import org.junit.Test
import org.organicprogramming.gabriel.greeting.kotlincompose.model.AppPlatformCapabilities
import org.organicprogramming.gabriel.greeting.kotlincompose.model.CoaxSettingsSnapshot
import org.organicprogramming.gabriel.greeting.kotlincompose.model.CoaxUiState
import org.organicprogramming.gabriel.greeting.kotlincompose.model.GreetingUiState
import org.organicprogramming.gabriel.greeting.kotlincompose.support.holon
import org.organicprogramming.gabriel.greeting.kotlincompose.support.language

class ComposeSmokeTest {
    @get:Rule
    val rule = createComposeRule()

    @Test
    fun rendersPrimaryWorkspaceRegions() {
        rule.setContent {
            GreetingScreen(
                greetingState = GreetingUiState(
                    isRunning = true,
                    isLoading = false,
                    greeting = "Bonjour Alice from Gabriel",
                    availableHolons = listOf(holon("gabriel-greeting-swift")),
                    selectedHolon = holon("gabriel-greeting-swift"),
                    availableLanguages = listOf(language("fr", "French", "Francais")),
                    selectedLanguageCode = "fr",
                ),
                coaxState = CoaxUiState(
                    isEnabled = true,
                    snapshot = CoaxSettingsSnapshot.defaults,
                    capabilities = AppPlatformCapabilities.desktopCurrent(),
                    listenUri = "tcp://127.0.0.1:60000",
                ),
                onToggleCoax = {},
                onOpenCoaxSettings = {},
                onSelectHolon = {},
                onSelectTransport = {},
                onNameChange = {},
                onSelectLanguage = {},
            )
        }

        rule.onNodeWithTag("app-root").assertIsDisplayed()
        rule.onNodeWithTag("coax-toggle").assertIsDisplayed()
        rule.onNodeWithTag("holon-list").assertIsDisplayed()
        rule.onNodeWithTag("name-input").assertIsDisplayed()
        rule.onNodeWithTag("greeting-bubble").assertIsDisplayed()
        rule.onNodeWithTag("language-row").assertIsDisplayed()
    }
}
