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

	// The ADB bridge: a device on one host used from a session on
	// another. android.adb.open is a tunnel (its frames as tunnel.open's)
	// to the host's ADB server; android.adb.listen listens on a loopback
	// port of the session's host, telling the client of each connection
	// (EventAndroidADBAccepted), which android.adb.accept makes a tunnel;
	// the client carries the bytes between the two.
	MethodAndroidADBOpen   = "android.adb.open"   // → TunnelInfo
	MethodAndroidADBListen = "android.adb.listen" // → AndroidADBListener
	MethodAndroidADBAccept = "android.adb.accept" // AndroidADBConn → TunnelInfo
	MethodAndroidADBClose  = "android.adb.close"  // AndroidADBListener
	// android.forward.open is a tunnel to a port adb forwards on the
	// device's host (adb forward tcp:Port …), as scrcpy's video takes.
	MethodAndroidForwardOpen = "android.forward.open" // AndroidForwardParams → TunnelInfo
	// android.proxy sends a device's HTTP to a port of its host (a
	// traffic source's), and has it trust the interceptor's CA.
	MethodAndroidProxy = "android.proxy" // AndroidProxyParams → AndroidProxyResult
	// android.mirror is a tunnel carrying a device's screen as scrcpy's
	// server sends it (core/android.Mirror); android.mirror.input touches
	// and types on it. Closing the tunnel stops the screen.
	MethodAndroidMirror      = "android.mirror"       // AndroidMirrorParams → TunnelInfo
	MethodAndroidMirrorInput = "android.mirror.input" // AndroidMirrorInput
	// What an agent uses a device with: its screen as a PNG, its controls
	// as numbered text, and input by adb (no screen shown needed).
	MethodAndroidScreenshot = "android.screenshot"   // AndroidDeviceParams → AndroidScreenshot
	MethodAndroidControls   = "android.controls"     // AndroidDeviceParams → AndroidControls
	MethodAndroidAct        = "android.act"          // AndroidAct
	EventAndroidADBAccepted = "android.adb.accepted" // AndroidADBConn, to the client that listens
)

// AndroidADBListener is a loopback port of the session's host that leads
// to a device host's ADB server.
type AndroidADBListener struct {
	ID   uint32 `json:"id"`
	Port int    `json:"port"`
}

// AndroidADBConn is a connection accepted on a listener.
type AndroidADBConn struct {
	Listener uint32 `json:"listener"`
	ID       uint32 `json:"id"`
}

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

// AndroidForwardParams names a port adb forwards to a device.
type AndroidForwardParams struct {
	Port int `json:"port"`
}

// AndroidProxyParams point device Serial's HTTP proxy at Port of its
// host (through adb reverse), and add CA (PEM) to what it trusts; Off
// undoes it.
type AndroidProxyParams struct {
	Serial string `json:"serial"`
	Port   int    `json:"port"`
	CA     string `json:"ca,omitempty"`
	Off    bool   `json:"off,omitempty"`
}

// AndroidProxyResult says what was done, and what the user must do.
type AndroidProxyResult struct {
	Note string `json:"note,omitempty"`
}

// AndroidMirrorParams ask for device Serial's screen, its longer side at
// most MaxSize pixels (0: the device's).
type AndroidMirrorParams struct {
	Serial  string `json:"serial"`
	MaxSize int    `json:"maxSize,omitempty"`
}

// AndroidMirrorInput is input for a device's screen shown: a touch (Kind
// "touch", Action 0 down, 1 up, 2 move, at X, Y of a video Width by
// Height), a key pressed (Kind "key", Key an Android key code), or text
// typed (Kind "text").
type AndroidMirrorInput struct {
	Serial string `json:"serial"`
	Kind   string `json:"kind"`
	Action int    `json:"action,omitempty"`
	X      int    `json:"x,omitempty"`
	Y      int    `json:"y,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Key    int    `json:"key,omitempty"`
	Text   string `json:"text,omitempty"`
}

// AndroidScreenshot is a device's screen as a PNG.
type AndroidScreenshot struct {
	PNG []byte `json:"png"`
}

// AndroidControls are a device's controls now, as lines numbered for
// android.act's Ref.
type AndroidControls struct {
	Text  string `json:"text"`
	Count int    `json:"count"`
}

// Actions of android.act.
const (
	AndroidTap       = "tap"       // at Ref, or X, Y
	AndroidLongPress = "longpress" // at Ref, or X, Y, for Ms (default 800)
	AndroidSwipe     = "swipe"     // from X, Y to X2, Y2 over Ms (default 300)
	AndroidType      = "type"      // Text into the field focused, or Ref's tapped first
	AndroidPress     = "press"     // Key: a name (back, home, enter…) or an Android key code
)

// AndroidAct is input for device Serial. Ref is a control's number from
// the last android.controls of the device.
type AndroidAct struct {
	Serial string `json:"serial"`
	Action string `json:"action"`
	Ref    int    `json:"ref,omitempty"`
	X      int    `json:"x,omitempty"`
	Y      int    `json:"y,omitempty"`
	X2     int    `json:"x2,omitempty"`
	Y2     int    `json:"y2,omitempty"`
	Ms     int    `json:"ms,omitempty"`
	Key    string `json:"key,omitempty"`
	Text   string `json:"text,omitempty"`
}
