package greeting.gokotlin

import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application
import greeting.gokotlin.grpc.DaemonProcess
import greeting.gokotlin.ui.ContentView
import greeting.gokotlin.ui.GokotlinTheme

fun main() = application {
    val daemon = DaemonProcess()
    Window(
        onCloseRequest = {
            daemon.stop()
            exitApplication()
        },
        title = "Gudule Greeting Gokotlin",
    ) {
        GokotlinTheme {
            ContentView(daemon)
        }
    }
}
