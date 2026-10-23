// Shows a session's display (Xpra's HTML5 client) as the app itself: no
// menu, no desktop, no window frames, the app's main window filling the
// enowX window and its title telling enowX what to show or hide.
(() => {
  const css = `
    #float_menu, #float_tray, #open_windows, #window_preview, #xpramenu, #startmenu,
    #shadow_pointer, #upload_form, #about, #sessioninfo, #progress, #login_form { display: none !important; }
    html, body, #screen { background: #000 !important; margin: 0; overflow: hidden; }
    .windowhead { display: none !important; }
    .window { border: 0 !important; box-shadow: none !important; border-radius: 0 !important; }
    #enowx-wait { position: fixed; inset: 0; display: flex; align-items: center; justify-content: center;
      color: #8a8f98; font: 13px -apple-system, system-ui, sans-serif; z-index: 100000; pointer-events: none; }
  `;
  const style = document.createElement("style");
  style.textContent = css;
  (document.head || document.documentElement).appendChild(style);

  // The title enowX reads: TITLE_APP + the app's title while it shows a
  // window, TITLE_WAIT while it shows none.
  const TITLE_APP = "enowx-app:";
  const TITLE_WAIT = "enowx-wait";
  const main = (w) =>
    !w.override_redirect && !w.tray &&
    !(w.windowtype || []).some((t) => t !== "NORMAL");

  const wait = document.createElement("div");
  wait.id = "enowx-wait";
  wait.textContent = "Waiting for the app to open a window\u2026";
  const showWait = (on) => {
    if (on && !wait.isConnected && document.body) document.body.appendChild(wait);
    if (!on && wait.isConnected) wait.remove();
  };

  setInterval(() => {
    const c = window.client;
    const wins = c && c.id_to_window ? Object.values(c.id_to_window).filter(main) : [];
    showWait(wins.length === 0);
    if (wins.length === 0) {
      if (document.title !== TITLE_WAIT) document.title = TITLE_WAIT;
      return;
    }
    wins.sort((a, b) => a.wid - b.wid);
    const first = wins[0];
    // The client keeps room for the frame it would draw: none here.
    const framed = first.topoffset || first.leftoffset || first.rightoffset || first.bottomoffset;
    if (framed) {
      first.topoffset = first.leftoffset = first.rightoffset = first.bottomoffset = 0;
      if (first.maximized) first.set_maximized(false);
    }
    if (!first.maximized && typeof first.set_maximized === "function") first.set_maximized(true);
    const title = TITLE_APP + (first.title || "");
    if (document.title !== title) document.title = title;
  }, 300);
})();
