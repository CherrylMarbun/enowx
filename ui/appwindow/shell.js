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
    .window-DIALOG, .window-UTILITY, .window-POPUP_MENU, .window-DROPDOWN_MENU, .window-MENU {
      box-shadow: 0 10px 34px rgba(0, 0, 0, 0.32), 0 0 0 1px rgba(0, 0, 0, 0.12) !important; }
    .window-DIALOG { border-radius: 8px !important; overflow: hidden; }
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
    const all = c && c.id_to_window ? Object.values(c.id_to_window) : [];
    // The client keeps room for the frames it would draw around windows:
    // none here, so a dialog would sit off where its clicks land.
    for (const w of all) {
      if (w.topoffset || w.leftoffset || w.rightoffset || w.bottomoffset) {
        w.topoffset = w.leftoffset = w.rightoffset = w.bottomoffset = 0;
        if (w.maximized) w.set_maximized(false);
        else if (typeof w.updateCSSGeometry === "function") w.updateCSSGeometry();
      }
    }
    // A dialog opens centered over the app, as on a desktop, once.
    for (const w of all) {
      if (main(w) || w.override_redirect || w._enowxPlaced || !w.w || !w.h) continue;
      w._enowxPlaced = true;
      const left = Math.max(0, Math.round((c.desktop_width - w.w) / 2));
      const top = Math.max(0, Math.round((c.desktop_height - w.h) / 3));
      if (typeof w.handle_moved === "function") {
        w.handle_moved({ position: { left, top } });
        if (typeof w.updateCSSGeometry === "function") w.updateCSSGeometry();
      }
    }
    const wins = all.filter(main);
    showWait(wins.length === 0);
    if (wins.length === 0) {
      if (document.title !== TITLE_WAIT) document.title = TITLE_WAIT;
      return;
    }
    wins.sort((a, b) => a.wid - b.wid);
    const first = wins[0];
    // The window's size changed (a zoom, a resize the client missed): the
    // client learns it, and the main window fills it again, once per size.
    if (c.connected && c.container &&
        (c.container.clientWidth !== c.desktop_width || c.container.clientHeight !== c.desktop_height)) {
      c._screen_resized();
    }
    const size = c.desktop_width + "x" + c.desktop_height;
    if (first.maximized && first._enowxSize !== size) first.set_maximized(false);
    if (!first.maximized && typeof first.set_maximized === "function") {
      first.set_maximized(true);
      first._enowxSize = size;
    }
    const title = TITLE_APP + (first.title || "");
    if (document.title !== title) document.title = title;
  }, 300);
})();
