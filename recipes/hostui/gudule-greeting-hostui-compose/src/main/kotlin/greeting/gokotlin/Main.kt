package greeting.gokotlin

import androidx.compose.ui.window.Window
import androidx.compose.ui.window.application
import greeting.gokotlin.grpc.DaemonProcess
import greeting.gokotlin.ui.ContentView
import greeting.gokotlin.ui.GokotlinTheme

fun main() = application {
    val daemon = DaemonProcess()
    val assemblyFamily = daemon.displayAssemblyTitleFamily()
    Window(
        onCloseRequest = {
            daemon.stop()
            exitApplication()
        },
        title = "Gudule $assemblyFamily",
    ) {
        GokotlinTheme {
            ContentView(daemon, assemblyFamily)
        }
    }
}
