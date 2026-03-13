# UI Experience Vision: Professional, Minimalist, Consistent

> [!NOTE]
> This document defines the standard UI/UX experience for all `hostui` applications in the Organic Programming ecosystem (SwiftUI, Flutter, Kotlin, .NET, Qt, Web). The goal is to establish a **professional, minimalist, and consistent** interface across all technologies.

## Core Philosophy
We will prioritize functionality, usability, and native feel over flashy cosmetics. The interface should feel like a robust, no-nonsense utility designed for professionals.

### Principles
1. **Clarity over Flourish:** Avoid unnecessary gradients, animations, or glassmorphism. Use solid colors and clear boundaries.
2. **Platform Consistency:** While the exact rendering (e.g., standard text box borders) is left to the native framework, the *layout*, *spacing*, and *flow* must be identical across all hosts.
3. **High Contrast:** Ensure text is easily readable. Dark mode is preferred as the default, but it should be high-contrast dark (e.g., dark gray/black backgrounds with crisp white text).
4. **Predictability:** The user flow is strictly top-down. The user views the status, inputs their target language and name, clicks the action button, and views the result below.

## Standard Layout Structure

Every UI host will implement a **Unified Landscape Layout with a Top Header**. 
The application window is wide (e.g., 16:9 ratio, ~800px min-width) and composed of two primary vertical regions: a full-width top header, and a split body.

### 1. Top Row: App Header
A distinct horizontal bar at the very top of the application window spanning the entire width.
- **App Title:** "Gudule [Technology]" (e.g., "Gudule SwiftUI", "Gudule Flutter"). Rendered in a large, bold, sans-serif system font aligned to the top-left.
- **Subtitle / Meta:** A small, muted string indicating the daemon connection info (e.g., "Gudule Greeting SwiftUI connected to C++ daemon.") placed directly below or beside the title.

### 2. Main Body: Left Side (Context & Input Flow)
The left portion of the main body (approx. 40% width) handles the interactive state.

#### Status Indicator
A clear, single line of text or a colored dot + text at the top of the left column indicating daemon status:
- 🟡 **Yellow/Orange:** "Starting daemon..."
- 🟢 **Green:** "Ready"
- 🔴 **Red:** "Offline - [Detailed Error Reason]"

#### Input Controls
A simple, unboxed vertical form directly below the status:
1. **Language Picker:** A standard native dropdown/combobox. Defaults to English.
2. **Name Input:** A standard native single-line text field labeled "Name". 
   - **Default Value:** "World!" (with an exclamation mark).

### 3. Main Body: Right Side (The Result Stage)
The right portion (approx. 60% width) acts as the main stage. 
It must be enclosed in a **Smart Frame**—an elegant, subtle border or slightly elevated panel container that clearly defines the result area without feeling heavy.

- **Initial State:** Displays the default greeting enclosed in quotes: `"Hello World!"`. The text should be bold, attractive, and perfectly centered within the smart frame.
- **Success State:** Displays the localized `<Greeting>` **enclosed in quote marks** in massive, highly legible text, followed by the localized `<Language Name>` in smaller text below it.
- **Error State:** If the gRPC call fails, the quoted greeting **must not display**. Instead, display the detailed, selectable error message.

### 4. Bottom Action Bar 
**The action button is placed at the bottom right corner of the entire application frame**.

- **Placement:** Pinned to the `bottom-right` edge of the window, sitting below the main body columns.
- **Action Button ("Greet"):** 
  - A prominent native button.
  - Must be disabled if the daemon is offline, loading, or if Name is empty.
  - **Color:** A clean call to action readable with high contrast matching the platform convention.

## Aesthetic Tokens

- **Background:** Solid dark theme `#121212` or native dark window background.
- **Panels/Cards:** Solid `#1E1E1E` or slightly elevated native surface color, with a 1px solid border (`#333333`) to delineate sections cleanly without shadows.
- **Typography:** Exclusively native UI system fonts (San Francisco on Apple, Roboto on Android/Chrome, Segoe UI on Windows).
- **Corner Radius:** Small and functional. 6px to 8px max (or sticking to the framework's native default). No massive 24px rounded curves.
- **Padding & Margins:** Use generous, consistent whitespace. e.g., 24px between major sections, 16px between input fields.

## Expected User Flow
1. **Launch:** App opens. Status shows "Starting daemon...". Language picker is disabled.
2. **Ready:** Daemon connects. Language list populates. Status updates to "Ready". The Result block shows "Hello World".
3. **Interaction:** User changes the language or types a name and clicks "Greet".
4. **Processing:** Action button briefly indicates progress (e.g., text changes to "Greeting...", or button disables to prevent double-click).
5. **Result:** The Result Area immediately renders the response or a copyable technical error.

## Candidate UI Mockups

### macOS (SwiftUI)
![macOS SwiftUI Mockup](/Users/bpds/.gemini/antigravity/brain/b9d77c70-ad3f-415d-a0aa-01548f73e537/mockup_header_swiftui_1773399785569.png)

### Material (Flutter)
![Material Flutter Mockup](/Users/bpds/.gemini/antigravity/brain/b9d77c70-ad3f-415d-a0aa-01548f73e537/mockup_header_flutter_1773399799343.png)

### Windows UI (.NET MAUI)
![Windows MAUI Mockup](/Users/bpds/.gemini/antigravity/brain/b9d77c70-ad3f-415d-a0aa-01548f73e537/mockup_header_maui_1773399835052.png)

### Qt / C++
![Qt C++ Mockup](/Users/bpds/.gemini/antigravity/brain/b9d77c70-ad3f-415d-a0aa-01548f73e537/mockup_header_qt_1773399849768.png)
