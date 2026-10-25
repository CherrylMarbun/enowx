package proto

// Android: devices on a host (emulators, phones) and running a session's
// app on one (docs/plans/mobile.md).
const (
	MethodAndroidStatus  = "android.status"  // → AndroidStatus
	MethodAndroidSetup   = "android.setup"   // AndroidSetupParams → SessionInfo (the tab that sets it up)
	MethodAndroidStart   = "android.start"   // AndroidDeviceParams
	MethodAndroidStop    = "android.stop"    // AndroidDeviceParams
	MethodAndroidConnect = "android.connect" // AndroidConnectParams
	MethodAndroidLogcat  = "android.logcat"  // AndroidLogcatParams → AndroidLog
	MethodAndroidRun     = "android.run"     // AndroidRunParams → AndroidRun
	MethodAndroidProject = "android.project" // AndroidProjectParams → AndroidProject
)

// AndroidStatus is what a host has for Android: the SDK, whether its
// emulator is accelerated, the virtual devices it can start and the
// devices online.
type AndroidStatus struct {
	SDK      string          `json:"sdk,omitempty"` // "" without one
	ADB      bool            `json:"adb,omitempty"`
	Emulator bool            `json:"emulator,omitempty"`
	Java     string          `json:"java,omitempty"`
	Accel    bool            `json:"accel,omitempty"`
	AccelWhy string          `json:"accelWhy,omitempty"` // why not, and what fixes it
	AVDs     []string        `json:"avds,omitempty"`
	Devices  []AndroidDevice `json:"devices,omitempty"`
	Display  bool            `json:"display,omitempty"` // windows need a virtual display here
	Problem  string          `json:"problem,omitempty"`
}

// AndroidDevice is a device adb sees, or a virtual one starting.
type AndroidDevice struct {
	Serial string `json:"serial"`
	Kind   string `json:"kind"`  // emulator, physical
	Name   string `json:"name"`  // the AVD's or the phone's model
	State  string `json:"state"` // device, offline, unauthorized, starting
	ABI    string `json:"abi,omitempty"`
	API    string `json:"api,omitempty"`
}

// AndroidSetupParams sets Android up on the host, in a tab of Session.
type AndroidSetupParams struct {
	Session string `json:"session"`
}

// AndroidDeviceParams names a virtual device to start (AVD) or a device
// to stop (Serial).
type AndroidDeviceParams struct {
	AVD    string `json:"avd,omitempty"`
	Serial string `json:"serial,omitempty"`
	// Window shows the emulator's own window, where the host has a
	// screen; else it runs headless and shows through scrcpy.
	Window bool `json:"window,omitempty"`
}

// AndroidConnectParams connects a phone over wireless debugging: its
// pairing address and code (Android 11+) when Code is set, then its
// address.
type AndroidConnectParams struct {
	Address     string `json:"address"`
	PairAddress string `json:"pairAddress,omitempty"`
	Code        string `json:"code,omitempty"`
}

// AndroidLogcatParams reads device Serial's log after line After, only
// Package's lines when set.
type AndroidLogcatParams struct {
	Serial  string `json:"serial"`
	Package string `json:"package,omitempty"`
	After   int64  `json:"after,omitempty"`
}

// AndroidLog is a part of a device's log.
type AndroidLog struct {
	Lines []AndroidLogLine `json:"lines"`
	Last  int64            `json:"last"` // the last line's number, for After
}

// AndroidLogLine is a line of logcat.
type AndroidLogLine struct {
	N       int64  `json:"n"`
	Time    string `json:"time"`
	PID     int    `json:"pid"`
	Level   string `json:"level"` // V D I W E F
	Tag     string `json:"tag"`
	Message string `json:"message"`
}

// AndroidProjectParams asks what kind of app the project at Dir is.
type AndroidProjectParams struct {
	Dir string `json:"dir"`
}

// AndroidProject is a project's app: its framework, the command that
// runs it on a device, and its package when known.
type AndroidProject struct {
	Kind    string `json:"kind"` // flutter, expo, react-native, capacitor, nativephp, gradle, "" for none
	Command string `json:"command,omitempty"`
	Package string `json:"package,omitempty"`
}

// AndroidRunParams runs session Session's app on device Serial of host
// DeviceHost ("" for the session's), in a new tab, with Command (the
// project's own when "").
type AndroidRunParams struct {
	Session string `json:"session"`
	Serial  string `json:"serial"`
	Command string `json:"command,omitempty"`
	// ADB is the address of the device's host's ADB server reached from the
	// session's host (the bridge), "" when the device is on it.
	ADB string `json:"adb,omitempty"`
}

// AndroidRun is the tab running the app.
type AndroidRun struct {
	Tab     string `json:"tab"`
	Command string `json:"command"`
	Package string `json:"package,omitempty"`
}
