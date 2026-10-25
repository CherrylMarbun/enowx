# Plan: running mobile apps (Android)

Status: planned, nothing built yet.

## Goal

Run the app a session builds on an Android device and debug it, nothing more, from one place: an **Android** tool in the side panel.

- Any framework that ends in an APK: Flutter, React Native (and Expo), Capacitor, Kotlin or Java, NativePHP Mobile.
- The device may be on another host than the session: the project on a server, the emulator on the laptop, or both on a server.
- Its screen shows on this device; its log (logcat) shows in the tool.

## Non-goals

Kept out to stay small; each can come later if needed:

- Redroid or other container runtimes, a device farm, cloud devices.
- Previews without Android (web, desktop), screenshot tests.
- Commands for agents (`enowx-app android …`), building and downloading APKs from the UI.
- iOS.
- Shipping the emulator or Android images: they come from the Android SDK, downloaded on the user's machine by `sdkmanager`, under its license, which the user accepts.

## The Android tool (side panel)

One tool, after Browser, for the selected session:

```
Android                                     
Device   [ Pixel (emulator) · This Mac   v ]  [Start]
         [ Run app ]   [ Show screen ]
Logcat   [ app only v ] [ search      ]  [Clear]
  10:42:01 I flutter: Hello
  10:42:03 E AndroidRuntime: FATAL EXCEPTION …
```

- **Device:** every device of every connected host: emulators (AVDs) that can start, running ones, phones plugged in by USB or connected over wireless debugging (`adb connect ip:port`, typed in). **Start** boots a stopped emulator; a host that can't run one says why (no acceleration) in one line.
- **Run app:** runs the project's command in a tab of the session, so hot reload keys work:

| Project (by its files) | Command |
|---|---|
| Flutter | `flutter run -d $ANDROID_SERIAL` |
| Expo | `npx expo run:android --device` |
| React Native | `npx react-native run-android --deviceId $ANDROID_SERIAL` |
| Capacitor | `npx cap run android --target $ANDROID_SERIAL` |
| NativePHP Mobile | `php artisan native:run android` |
| Gradle (Kotlin, Java) | `./gradlew installDebug`, then launch the app |

  The command shows before it runs, editable, and is kept in the project's Run menu.
- **Show screen:** the device's screen in a window of its own on this device, with touch and keyboard (scrcpy, see below). An emulator on this laptop can show its own window instead.
- **Logcat:** the device's log, the app's own lines by default (by its package), levels colored, search, clear, crashes easy to spot.
- **Empty states:** no Android SDK on the host ("Set up Android" opens Setup's Android group), no device ("Start an emulator" or "Connect a phone").

## How it works

### Runtimes (`core/android`)

Each daemon finds the SDK (`ANDROID_HOME`, `ANDROID_SDK_ROOT`, the usual folders), its AVDs (`emulator -list-avds`), whether the emulator is accelerated (`emulator -accel-check`: Hypervisor.framework on macOS, WHPX on Windows, `/dev/kvm` on Linux) and its devices (`adb devices -l`). The emulator runs on the host itself, never in a container: on macOS and Windows a container has no acceleration.

### Setup (`core/setup`, an "Android" group)

JDK 17, the SDK's command-line tools, `platform-tools`, `emulator`, one system image for the host's architecture (arm64 on Apple Silicon and arm64 Linux, x86_64 elsewhere), licenses accepted by the user, and one AVD ("enowX Phone"). Sessions get `ANDROID_HOME` and the SDK's tools on their `PATH`. Framework tools (Flutter, Node, PHP) come from the setup that exists.

### The device on another host: the ADB bridge

Tools in the session talk to an ADB server on port 5037. When the device's host isn't the session's:

- The session's host listens on a loopback port (`android.adb.listen`); for each connection the app opens a tunnel to the device host's ADB server (`android.adb.open`, on the tunnel frames `tunnel.open` uses) and carries the bytes between the two daemons. One path for laptop and server, or two servers. When the device is on this laptop and the server is reached over SSH, an SSH remote forward does it directly.
- The session's tab gets `ADB_SERVER_SOCKET=tcp:127.0.0.1:<port>` and `ANDROID_SERIAL=<device>`, so `adb`, `flutter`, `gradlew` see the device as if it were plugged in there.
- **Dev servers the app calls** (Metro, Expo, Capacitor's live reload, a Laravel server): `adb reverse tcp:<port>` sends the phone's `localhost:<port>` to the device's host, and enowX forwards that port there from the session's host, as it forwards ports to this device today.
- Loopback only, and only while the session uses the device.

### The screen: scrcpy

scrcpy streams the device's screen as H.264 and sends touch and keys back, light for this device. It runs here and reaches the device through the same bridge. enowX uses the scrcpy installed, or downloads a pinned release for this platform into `~/.enowx-app/tools` on first use, its checksum checked (Apache-2.0).

## Protocol additions (`daemon/rpc_android.go`)

- `android.status`: the SDK, the emulator's acceleration, AVDs and devices; event `android.devices.changed`.
- `android.start` / `android.stop` (an AVD), `android.connect` (wireless debugging, `ip:port`).
- `android.logcat`: the device's log as a stream, filtered by package.
- `android.adb.open`, `android.adb.listen`, `android.adb.close`, event `android.adb.accepted`: the bridge.

An older daemon answers them as unknown; the tool says the host needs updating.

## Phases

1. **Devices on any host.** Detection, Setup's Android group, the tool's device list, Start and Stop, wireless connect. Done when Setup then Start boots an emulator on a Mac and on a Linux host with KVM, `adb` lists it, and a host without acceleration says why.
2. **Run app and logcat.** Project detection, Run app in a tab, logcat filtered to the app. Done when a Flutter app and an Expo app on the laptop run with hot reload and their logs show.
3. **Across hosts and the screen.** The ADB bridge, `adb reverse` for dev servers, scrcpy. Done when a Flutter project on a server runs on the laptop's emulator with hot reload, and an Expo project on a server reaches its Metro through `adb reverse`; an end-to-end test with two daemons checks the bridge's bytes both ways.

## Risks

- **No acceleration on many VPSes** (no nested virtualization): an emulator there is too slow. The tool says so and points to the laptop's emulator or a phone.
- **x86_64 images and arm64-only apps:** an app with only arm64 native libraries won't run on an x86_64 emulator (NativePHP's bundled PHP to check). Apple Silicon runs arm64 images.
- **ADB over a far link** is slow to install large APKs; acceptable for debug builds, revisit if it hurts.
