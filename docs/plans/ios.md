# Plan: running iOS apps

Status: planned, nothing built yet.

## Goal

Run the app a session builds on an iOS Simulator or an iPhone and debug it, as the Android tool does for Android ([docs/android.md](../android.md)), from the same place: the Android tool becomes the **Mobile** tool, its device list holding Android and iOS devices together.

- Any framework that builds for iOS: Flutter, React Native (and Expo), Capacitor, NativePHP Mobile, Swift and Objective-C (Xcode projects).
- The device's host is a Mac, the only machine Apple's tools run on: this laptop when it is a Mac, or a Mac reached as a server (one's own Mac mini, or one rented: AWS EC2 Mac, Scaleway, MacStadium).
- Its screen shows on this device; its log shows in the tool; its traffic can be caught.

## Non-goals

- Running iOS anywhere but on a Mac. There is no iOS emulator outside Apple's tools that runs today's apps; iOS's license allows it only on Apple hardware.
- Installing Xcode for the user: it comes from Apple (App Store, or developer.apple.com) with the user's Apple ID. enowX checks for it and says what is missing.
- Signing for the App Store, TestFlight, provisioning profiles: Xcode's.

## What exists today, reused

| Piece | Use here |
|---|---|
| The Android tool (`ui/androidview`), `core/android` | The tool's layout, device list, Run app, logcat view, across hosts |
| Project detection and run commands (`android.Project`) | Their iOS commands beside the Android ones |
| Traffic (`docs/traffic.md`) | A simulator's or an iPhone's HTTP caught |
| Project sync (`docs/transfer.md#syncing-back`) | A project on a Linux server built on a Mac |
| Hosts over SSH, the daemon, Setup | A Mac as a host, once the daemon runs on macOS servers |

## Design

### 1. iOS on a Mac host (`core/ios`)

The daemon of a Mac finds:

- **Xcode:** `xcode-select -p`, `xcodebuild -version`; missing, the tool says to install Xcode, then `sudo xcodebuild -license accept` and `xcodebuild -runFirstLaunch`.
- **Simulator runtimes and devices:** `xcrun simctl list --json` (runtimes, device types, devices and their state); a missing runtime is downloaded with `xcodebuild -downloadPlatform iOS`, a simulator made with `xcrun simctl create`.
- **iPhones:** `xcrun devicectl list devices --json-output` (Xcode 15 and later): connected by USB or the network, paired, developer mode on or not.
- Boot and shut down a simulator (`simctl boot`, `shutdown`), opening Simulator.app on a Mac with a screen.

### 2. The Mobile tool

The Android tool's list grows iOS devices: "iPhone 16 (Simulator) · This Mac", "Ada's iPhone · mac-mini". The rest stays: Start or Stop, Run app, Show screen, the log, Catch its traffic. On a host without iOS (Linux, Windows), iOS devices simply aren't there; when the session's project builds for iOS and no Mac is known, the tool says a Mac is needed and how to add one.

### 3. Run app

| Project | Command, with `$IOS_DEVICE` the device's UDID |
|---|---|
| Flutter | `flutter run -d "$IOS_DEVICE"` |
| Expo | `npx expo run:ios --device "$IOS_DEVICE"` |
| React Native | `npx react-native run-ios --udid "$IOS_DEVICE"` |
| Capacitor | `npx cap run ios --target "$IOS_DEVICE"` |
| NativePHP Mobile | `php artisan native:run ios` |
| Xcode project | `xcodebuild -scheme … -destination id=$IOS_DEVICE build`, then `simctl install` and `launch` (or `devicectl device install app` and `process launch` on an iPhone) |

As for Android, it runs in a tab of the session, so hot reload's keys work, the command editable before it runs. CocoaPods (`pod install`) runs first when the project has a Podfile and its pods aren't installed.

A **project on a Linux server** can't build for iOS there. **Run on iOS** then offers to sync it to a Mac host (the project sync, both ways), run it there in a session of the Mac, and sync the changes back; or to start the session on the Mac to begin with.

### 4. The screen

- **This Mac:** Simulator.app's own window; an iPhone shows itself.
- **A Mac server:** the simulator's screen streamed here: `xcrun simctl io <udid> recordVideo --codec=h264` to a pipe, carried as a tunnel and shown in a window of the app; touch, swipe and text sent back with Meta's `idb` (MIT: `idb_companion` on the Mac), which Setup brings. A first version can show screenshots (`simctl io screenshot`) a few times a second.

### 5. The log

`xcrun simctl spawn <udid> log stream --style compact --predicate 'process == "<app>"'` for a simulator; an iPhone's through `devicectl` (Xcode 16's device log) or `idevicesyslog`. Shown as logcat is, by level, searchable.

### 6. Traffic

- **Simulator:** enowX's CA added with `xcrun simctl keychain <udid> add-root-cert`, no root needed. A simulator uses the Mac's network settings: the proxy is set on the Mac for the time it is caught (a network service's web proxy, put back after), or apps that read `HTTP_PROXY` get it through `SIMCTL_CHILD_` variables. To decide while building (open question).
- **iPhone:** the CA as a configuration profile to install and trust in Settings, and the Wi-Fi's proxy pointed at the Mac's interceptor, both by the user, the tool saying how.

### 7. A Mac as a server

Today servers are Linux. A Mac host needs:

- the daemon built for macOS (darwin/arm64, darwin/amd64) and installed over SSH as on Linux;
- run as a launchd agent (`~/Library/LaunchAgents`) rather than a systemd service, kept running and started at login;
- Setup's steps for macOS (Homebrew instead of a distribution's packages; Xcode checked, not installed);
- the rented Macs' quirks: a minimum of 24 hours a lease (Apple's terms), and a user signed in for the Simulator to run.

## Phases

1. **This Mac.** `core/ios` detection, simulators listed, booted and shut down, Run app for the six project kinds on a simulator, the log, in the Mobile tool. Done when a Flutter app and an Expo app run on a simulator of the laptop from a session on it, with hot reload and their logs.
2. **iPhones and traffic.** Devices through `devicectl`, Run app on an iPhone, the CA on simulators and the profile for iPhones, the simulator's proxy. Done when an app on a simulator shows its HTTPS calls in Traffic.
3. **A Mac as a server.** The daemon on macOS over SSH, launchd, Setup for macOS. Done when a Mac mini added as a host runs sessions that survive the laptop closing.
4. **A remote simulator here.** Its screen streamed, touch and text sent back with `idb`. Done when an app on a rented Mac's simulator is used from a Linux or Windows laptop.
5. **Linux projects on a Mac.** Run on iOS syncing a server's project to a Mac host and back. Done when a Flutter project on a Linux server runs on a Mac's simulator and an edit made on the server reloads it.

## Risks

- **Rented Macs** cost by the day at least, and their simulators need a signed-in user and a GPU session; some offers are headless only.
- **iPhones** need the user's Apple ID in Xcode, a development team for signing, and developer mode on the phone: enowX can only say what is missing.
- **Xcode versions:** `devicectl` and the device log need Xcode 15 and 16; older ones get simulators only.
- **Streaming a simulator** costs the Mac encoding and the network bandwidth; screenshots are the fallback.

## Open questions

- The simulator's proxy: the Mac's network settings for a while, or only apps that read the variables?
- `idb` for input, or the simulator's own accessibility hooks through `simctl`?
- Renaming the Android tool to Mobile now, or when iOS devices first appear in it?
