# Plan: mobile development (Android first)

Status: planned, nothing built yet.

## Goal

Build and run mobile apps from any session, on any device the user has, wherever either one is:

1. **Any framework that ends in an APK:** Flutter, React Native (and Expo), Capacitor (and Ionic), Kotlin or Java (Jetpack Compose, Views), NativePHP Mobile. The app runs on a real Android, so every native feature works: camera, location, notifications, intents, Play services.
2. **A device is apart from the session.** A device lives on a host: an emulator on this laptop, an emulator or a Redroid container on a server, a phone plugged into any of them. A session on another host uses it as if it were plugged in there. The common case: the project and its agent on a server, the emulator on the laptop; or both on a server and only the screen on the laptop.
3. **Its screen shows on this device**, wherever the device runs, with touch, keyboard and clipboard.
4. **Agents use it too:** install, launch, screenshot, tap, type, read logcat, to check what they build.
5. **A light preview without Android** where the framework has one (web, desktop), for the many hours of UI work that need no phone.

## Non-goals

- Shipping an emulator or Android images with enowX. They come from the Android SDK, downloaded on the user's machine by `sdkmanager`, under its license, which the user accepts.
- A device farm, or devices of a cloud service.
- iOS in this plan's phases (see "Later: iOS").
- A generic remote desktop of the device beyond its screen.

## What exists today

| Piece | Where | Use here |
|---|---|---|
| A daemon on every host, the app connected to several | `daemon/`, `hostconn`, `client` | Each host reports and runs its devices |
| A tunnel to a session's ports in the protocol | `tunnel.open`, `TypeTunnelData` frames (`proto/tunnel.go`) | Carries ADB, extended to the ADB server's port |
| Forwarding a server's port to this device, on the same port | `transport.Forward`, `ui/forwards.go` | Dev servers (Metro, Vite) a phone reaches through `adb reverse` |
| Desktop apps of a server on this screen | `core/display` (Xpra), `ui/appwindow` | Fallback display of an emulator's own window |
| One-click setup of a server | `core/setup` | Installs JDK, SDK, emulator, framework tools |
| Agents driving a tool from the command line | `enowx-app browser …` (`cli/`, `browser.act`) | The same shape for `enowx-app android …` |
| Side panel tools, offer cards | `ui/sidepanel.go`, `ui/portoffers.go` | An Android tool, "a device is ready" offers |

## Design

### 1. Runtimes: what runs Android on a host (`core/android`)

Each daemon finds what its host can run, and says why when it can't:

| Host | Runtime | Acceleration it needs |
|---|---|---|
| macOS (Apple Silicon) | Android Emulator, arm64 images | Hypervisor.framework (always there) |
| macOS (Intel) | Android Emulator, x86_64 images | Hypervisor.framework |
| Windows | Android Emulator, x86_64 images | Windows Hypervisor Platform, or AEHD |
| Linux | Android Emulator, x86_64 (arm64 on arm64 hosts) | `/dev/kvm` readable and writable |
| Linux without KVM | Redroid, in Docker or Podman | The `binder` driver (`binder_linux`, or binderfs), a container runtime |
| Linux with neither | Android Emulator without acceleration | None: it runs, slowly; offered last, with that warning |
| Any host | A physical device | `adb` sees it: USB, or wireless debugging |

- **Detection** (`android.runtimes`): the SDK's location (`ANDROID_HOME`, `ANDROID_SDK_ROOT`, the usual folders), `emulator -accel-check`, `/dev/kvm`, `binder` in `/proc/filesystems` or `/dev/binder*`, a container runtime, the host's architecture and memory. Each runtime comes with "ready", "can be set up" (what's missing) or "can't" (why, and what would fix it: "enable KVM for this VM", "load binder_linux on the host").
- **Not Docker everywhere.** On macOS and Windows a container runs inside a Linux VM, with neither KVM nor binder, so Android there would be slow or wouldn't start; the emulator on the host is fast. Images that run the emulator in a container still need `/dev/kvm`. Docker is for Redroid only.
- **Images.** The emulator uses the host's architecture: arm64 on Apple Silicon and arm64 Linux, x86_64 elsewhere. Flutter, React Native, Capacitor and Gradle builds include x86_64 by default. NativePHP bundles its own PHP binary: check it ships for x86_64 before relying on x86_64 runtimes (open question).

### 2. Setup (`core/setup`, a new "Android" group)

One click brings a host what building and running need, as a dry run first like the rest of Setup:

- A JDK (17 or 21, by the Android Gradle Plugin's need), the Android SDK's command-line tools, `platform-tools` (adb), a `platforms;android-NN` and `build-tools;NN`, accepting the licenses with the user's consent (shown, not silently accepted).
- To run devices: `emulator` and a system image (`system-images;android-NN;google_apis;<arch>`, with Play Store when wanted), and an AVD made with `avdmanager` ("enowX Pixel", a phone profile, sized to the host's memory). Or Redroid's image pulled.
- Per framework, when a project needs it: the Flutter SDK, Node and the package manager, PHP and Composer for NativePHP.
- Where the SDK goes: `~/Android/Sdk` (Linux), `~/Library/Android/sdk` (macOS), `%LOCALAPPDATA%\Android\Sdk` (Windows), or the one found. Sessions get `ANDROID_HOME` and the SDK's tools on their `PATH`.

### 3. Devices (`android.devices`)

A device is what `adb devices -l` lists on a host, plus the AVDs and Redroid containers that can start:

```
Device {
  Host     // the host it is on (the app knows which)
  Serial   // adb's serial: emulator-5554, a USB serial, ip:port
  Kind     // emulator | redroid | physical
  Name     // "Pixel 8 (emulator)", the phone's model
  State    // stopped | starting | online | offline | unauthorized
  ABI, API // to warn when an app has no matching ABI
}
```

- **Lifecycle:** `android.start` (an AVD: `emulator -avd … -no-snapshot-save`, headless `-no-window` when this device is a server; Redroid: `docker run --privileged redroid/redroid:<api>` with its binder devices), `android.stop`, and `android.wait` until `sys.boot_completed`. Each started device is tagged like a session's processes, so it stops with enowX's daemon unless kept.
- **Events:** `android.devices.changed` when a device comes or goes (a phone plugged in, an emulator up), so every app connected sees it.
- **Physical devices:** plugged in over USB, or **Pair a phone…** for wireless debugging: the phone's pairing code and port, or a QR code the app shows (`adb pair`), then `adb connect`. "Unauthorized" says to accept the prompt on the phone.
- **Resources:** an emulator takes 2–4 GB and a few cores; Redroid 1–2 GB. The devices show in Resource usage, and hibernation leaves sessions using a running device alone. A device nobody used for a while can stop (a setting, off by default).

### 4. The ADB bridge: a device on one host, a session on another

The session runs `adb`, `flutter`, `gradlew`; they talk to an ADB server on port 5037. enowX makes the device's host's ADB server reachable on the session's host:

- **Same host:** nothing to do; the session's `adb` uses the local server.
- **Different hosts:** the session gets `ADB_SERVER_SOCKET=tcp:127.0.0.1:<port>` (and `ANDROID_ADB_SERVER_PORT` for tools that read that), `<port>` a loopback port on the session's host that leads to the device host's ADB server:
  - **`android.adb.open`** on the device's host: like `tunnel.open`, but to its ADB server (started if needed), on the same `TypeTunnelData` frames and flow control.
  - **`android.adb.listen`** on the session's host: the daemon listens on a loopback port and, for each connection, tells the app (`android.adb.accepted`), which opens `android.adb.open` on the device's host and carries the bytes between the two daemons. One code path whatever the hosts are: laptop and server, two servers, or a server and a laptop.
  - When the device is on this laptop and the session's host is reached over SSH, an SSH remote forward does the same with fewer hops; the relay stays the fallback.
- **Which device:** the session's `ANDROID_SERIAL` names the device picked, so tools that take the first device take that one. Several sessions can share a device.
- **The phone reaching the session's dev servers.** React Native's Metro, Expo, a Capacitor live reload, a NativePHP or Laravel server: the phone calls `localhost:<port>`, which `adb reverse` sends to the device's host. enowX forwards that port from the session's host to the device's host (today's forwarding, generalized from "to this device" to "to the device's host"), and runs `adb reverse tcp:<port> tcp:<port>` when the session's ports change. Flutter's hot reload goes the other way (`adb forward`, host to device) and needs nothing more.
- **Security:** the bridge listens on loopback only, lives while its session uses the device, and the ADB server is never exposed beyond the connection. A device on another host is used only once the user picks it for the session.

### 5. Running an app (`android.run`, the Run on Android button)

- **Run on Android** (the session's toolbar and the command palette) picks a device (the last one, else a list of every host's devices, starting one if needed), sets up the bridge, and runs the project's command:

| Project (found by its files) | Command |
|---|---|
| Flutter (`pubspec.yaml` with `flutter`) | `flutter run -d $ANDROID_SERIAL` |
| Expo (`app.json` with `expo`) | `npx expo run:android --device` |
| React Native (`android/` and `react-native`) | `npx react-native run-android --deviceId $ANDROID_SERIAL` |
| Capacitor (`capacitor.config.*`) | `npx cap run android --target $ANDROID_SERIAL` |
| NativePHP Mobile (`nativephp/mobile`) | `php artisan native:run android` |
| Gradle Android (`app/build.gradle*` with `com.android.application`) | `./gradlew installDebug`, then `adb shell monkey -p <applicationId> 1` |

- It runs in a tab of the session (a terminal, so hot reload keys work), and joins the project's Run menu (`core/commands`), editable there.
- **Build APK**: `assembleDebug` / `flutter build apk` / the framework's, the APK offered to download to this device.

### 6. The screen, on this device

- **scrcpy** shows any device's screen in a window of its own on this device, with touch, keyboard, clipboard and rotation: it streams H.264 from the phone, which costs this device little. It reaches the device through the same ADB bridge, toward this device (`ADB_SERVER_SOCKET` for scrcpy). enowX finds scrcpy, or downloads its release for this platform into `~/.enowx-app/tools` on first use, checking its checksum (scrcpy is Apache-2.0).
- An emulator on this laptop can also show its own window (the user's choice).
- **Fallback:** an emulator on a server run with a window on the session's Xpra display, shown as desktop apps are, when scrcpy can't run here.
- The window is titled with the device and its host, opened from the device list, the Run button, or a "Pixel 8 is ready" offer card.

### 7. The Android tool, and agents

- **Side panel → Android:** the devices of every host (start, stop, open the screen, pair a phone), the session's device, **logcat** filtered to the app's package (levels, search, clear), install an APK, a screenshot sent to the agent like the Browser's.
- **Command line** for agents and scripts (`enowx-app android …`, through the daemon like `enowx-app browser`): `devices`, `start <avd>`, `use <device>`, `install <apk>`, `launch <package>`, `screenshot [file]`, `tap x y`, `swipe`, `type <text>`, `key BACK`, `logcat [--package] [--since]`, `ui-dump` (the view hierarchy, `uiautomator dump`, so an agent finds a button by its text). The agent guide (`core/agent` guidance) gets a short section, as the browser has.

### 8. Preview without Android

For UI work, a framework's own light target, shown with what enowX already has:

| Framework | Preview | Shown in |
|---|---|---|
| Flutter | `flutter run -d web-server`, or `-d linux` | Browser tool, or a desktop window (Xpra) |
| Expo / React Native | `npx expo start --web` (react-native-web) | Browser tool |
| Capacitor / Ionic | the web app's dev server | Browser tool |
| NativePHP | `php artisan serve` | Browser tool |
| Compose Multiplatform | the desktop target | a desktop window |
| Kotlin or Java (Jetpack) | screenshots of each screen, Paparazzi or Roborazzi | the Android tool, as images |

The Run menu offers **Preview** next to Run on Android, saying what won't work there (native modules, Play services).

## Protocol additions

New methods in `daemon/rpc_android.go`, recorded in `proto/testdata/api.txt`; an older daemon answers them as unknown, and the app says the host needs updating:

- `android.runtimes` → what the host can run, and why not.
- `android.devices` → its devices; event `android.devices.changed`.
- `android.start`, `android.stop`, `android.wait`.
- `android.pair`, `android.connect` (wireless debugging).
- `android.adb.open` (a tunnel to the ADB server), `android.adb.listen` / `android.adb.close` and event `android.adb.accepted` (the bridge's side on the session's host).
- `android.act` (screenshot, tap, type, key, ui dump, logcat), what `enowx-app android` and the Android tool use.

No new frame types: the bridge reuses tunnel frames.

## Phases

### Phase 1: runtimes, setup, devices
- `core/android`: detection on macOS, Windows and Linux; the SDK found or set up; AVDs listed, made and started; `adb devices` read.
- Setup's Android group (JDK, SDK, platform-tools, emulator, an image, an AVD), dry run first.
- The Android tool's device list for every connected host; start and stop.
- Done when: on a Mac, a Linux server with KVM and Windows, Setup then Start brings up an emulator that `adb devices` lists as online, and a host without acceleration says why and what would fix it.

### Phase 2: the bridge and Run on Android
- `android.adb.open`, `android.adb.listen`, the app's relay, `ANDROID_SERIAL` and `ADB_SERVER_SOCKET` in the session.
- Ports forwarded to the device's host and `adb reverse`.
- Project detection and Run on Android for the six project kinds; Build APK.
- Done when: a Flutter project on a server runs on an emulator on the laptop with hot reload, and an Expo project on a server runs on an emulator on another server with Metro reached through `adb reverse`; an end-to-end test with two daemons and a fake ADB server checks the bridge's bytes both ways.

### Phase 3: the screen, logcat, agents
- scrcpy found or downloaded, opened through the bridge; the Xpra fallback.
- Logcat in the Android tool; screenshots to the agent.
- `enowx-app android …` and its guide for agents.
- Done when: an agent on a server installs its app on a device on another host, taps through a screen by `ui-dump` and checks a screenshot.

### Phase 4: physical devices and Redroid
- Wireless pairing (code and QR), USB devices on any host, "unauthorized" handled.
- Redroid on Linux with binder: started, stopped, its image pulled; shown and bridged like an emulator.
- Done when: a phone paired with the laptop runs a server session's app, and a Redroid device on a server without KVM boots and runs a Flutter app.

### Phase 5: previews
- Preview per framework (table above) in the Run menu; Paparazzi screenshots in the Android tool.

## Risks

- **Acceleration is often missing on VPSes** (no nested virtualization, no binder). Detection must say so plainly and point to what works: the laptop's emulator, a phone, or a server that has KVM.
- **ADB over a relayed connection** is chatty; installing a large APK over a far link is slow. Pushing the APK from the session's host straight to the device's host first, then `adb install` there, is the fallback if it matters.
- **x86_64 vs arm64:** apps with only arm64 native libraries won't run on x86_64 images (NativePHP's PHP binary to check). Apple Silicon and arm64 servers run arm64 images.
- **scrcpy downloads** depend on its GitHub releases; pin a version and its checksums.
- **Licenses:** SDK packages are accepted by the user, never redistributed by enowX.

## Open questions

- NativePHP Mobile on x86_64 images: does its bundled PHP run there?
- Keep devices running when the app quits, as sessions do, or stop them? (Default proposed: stop emulators enowX started when no session used them for 30 minutes, a setting.)
- Play Store images (needed for Play services) are larger and can't be rooted: offer both, or default to Google APIs images?

## Later: iOS

The iOS Simulator and Xcode run only on macOS, so iOS follows the same model with a Mac as the device's host: a session on any host builds with a Mac (its own host, or a Mac reached over SSH) for Flutter, React Native, Capacitor and NativePHP; the simulator's screen shows on this device (a Mac's window, or a stream); physical iPhones through the Mac's `devicectl`. A plan of its own once Android has shipped.
