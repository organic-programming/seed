package org.organicprogramming.gabriel.greeting.kotlincompose

import androidx.compose.ui.unit.DpSize
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.WindowState
import androidx.compose.ui.window.application
import kotlinx.coroutines.runBlocking
import org.organicprogramming.gabriel.greeting.kotlincompose.controller.CoaxController
import org.organicprogramming.gabriel.greeting.kotlincompose.controller.GreetingController
import org.organicprogramming.gabriel.greeting.kotlincompose.runtime.AppPaths
import org.organicprogramming.gabriel.greeting.kotlincompose.runtime.DesktopHolonCatalog
import org.organicprogramming.gabriel.greeting.kotlincompose.runtime.DesktopHolonConnector
import org.organicprogramming.gabriel.greeting.kotlincompose.settings.FileSettingsStore
import org.organicprogramming.gabriel.greeting.kotlincompose.settings.applyLaunchEnvironmentOverrides
import org.organicprogramming.gabriel.greeting.kotlincompose.ui.GabrielGreetingApp

fun main() {
    AppPaths.configureRuntimeEnvironment()
    val settingsStore = FileSettingsStore.create()
    applyLaunchEnvironmentOverrides(settingsStore)
    val greetingController = GreetingController(
        catalog = DesktopHolonCatalog(),
        connector = DesktopHolonConnector(),
    )
    val coaxController = CoaxController(
        greetingController = greetingController,
        settingsStore = settingsStore,
    )

    application {
        Window(
            onCloseRequest = {
                runBlocking {
                    coaxController.shutdown()
                    greetingController.shutdown()
                }
                exitApplication()
            },
            title = "Gabriel Greeting",
            state = WindowState(size = DpSize(1100.dp, 760.dp)),
        ) {
            window.minimumSize = java.awt.Dimension(800, 600)
            GabrielGreetingApp(
                greetingController = greetingController,
                coaxController = coaxController,
            )
        }
    }
}
