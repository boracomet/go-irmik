(() => {
  if (window.__IRMIK_DEV) return;
  window.__IRMIK_DEV = true;

  const PREFIX = "/_irmik/dev";
  const errors = [];
  let open = false;
  let tab = "errors";
  let connected = false;
  let info = { env: "development", addr: "", routes: [], liveReload: true };

  const style = document.createElement("style");
  style.textContent = `
    #irmik-dev-root{all:initial;position:fixed;left:16px;bottom:16px;z-index:2147483646;font-family:ui-sans-serif,system-ui,sans-serif;font-size:13px;line-height:1.45;color:#e8e8e8}
    #irmik-dev-root *{box-sizing:border-box}
    #irmik-dev-badge{position:relative;width:44px;height:44px;padding:0;border:0;background:transparent;cursor:pointer;border-radius:50%}
    #irmik-dev-badge img{width:44px;height:44px;display:block;border-radius:50%}
    #irmik-dev-dot{position:absolute;right:1px;top:1px;width:10px;height:10px;border-radius:50%;background:#3dd68c;border:2px solid #0b0b0c}
    #irmik-dev-dot.warn{background:#f5a524}
    #irmik-dev-dot.err{background:#f31260}
    #irmik-dev-panel{display:none;position:absolute;left:0;bottom:56px;width:360px;max-height:min(70vh,480px);background:#121214;border:1px solid #2a2a2e;border-radius:12px;overflow:hidden}
    #irmik-dev-panel.open{display:flex;flex-direction:column}
    #irmik-dev-head{display:flex;align-items:center;justify-content:space-between;padding:10px 12px;border-bottom:1px solid #2a2a2e;font-weight:600;font-size:12px;letter-spacing:.02em}
    #irmik-dev-tabs{display:flex;gap:4px;padding:8px 10px;border-bottom:1px solid #2a2a2e}
    #irmik-dev-tabs button{background:transparent;border:0;color:#9a9aa3;padding:4px 8px;border-radius:6px;cursor:pointer;font:inherit}
    #irmik-dev-tabs button.on{background:#1e1e22;color:#fff}
    #irmik-dev-body{padding:10px 12px;overflow:auto;flex:1;color:#c4c4cc}
    #irmik-dev-body ul{margin:0;padding:0;list-style:none}
    #irmik-dev-body li{padding:8px 0;border-bottom:1px solid #232327}
    #irmik-dev-body li:last-child{border:0}
    #irmik-dev-body .muted{color:#8b8b94}
    #irmik-dev-body .err{color:#ff7a9c;white-space:pre-wrap;word-break:break-word}
    #irmik-dev-foot{padding:8px 12px;border-top:1px solid #2a2a2e;display:flex;justify-content:space-between;align-items:center}
    #irmik-dev-foot button{background:#1e1e22;border:1px solid #2a2a2e;color:#e8e8e8;border-radius:6px;padding:4px 8px;cursor:pointer;font:inherit}
  `;
  document.documentElement.appendChild(style);

  const root = document.createElement("div");
  root.id = "irmik-dev-root";
  root.innerHTML = `
    <div id="irmik-dev-panel">
      <div id="irmik-dev-head"><span>Irmik Dev</span><span id="irmik-dev-status" class="muted">connecting</span></div>
      <div id="irmik-dev-tabs">
        <button type="button" data-tab="errors" class="on">Errors</button>
        <button type="button" data-tab="routes">Routes</button>
        <button type="button" data-tab="server">Server</button>
      </div>
      <div id="irmik-dev-body"></div>
      <div id="irmik-dev-foot">
        <span class="muted">development only</span>
        <button type="button" id="irmik-dev-clear">Clear</button>
      </div>
    </div>
    <button type="button" id="irmik-dev-badge" title="Irmik Dev" aria-label="Irmik Dev">
      <img alt="" width="44" height="44" src="${PREFIX}/logo.png">
      <span id="irmik-dev-dot"></span>
    </button>
  `;
  document.documentElement.appendChild(root);

  const panel = root.querySelector("#irmik-dev-panel");
  const body = root.querySelector("#irmik-dev-body");
  const statusEl = root.querySelector("#irmik-dev-status");
  const dot = root.querySelector("#irmik-dev-dot");

  function paint() {
    dot.className = "";
    if (errors.length) dot.classList.add("err");
    else if (!connected) dot.classList.add("warn");
    statusEl.textContent = connected ? "live" : "offline";

    root.querySelectorAll("#irmik-dev-tabs button").forEach((b) => {
      b.classList.toggle("on", b.dataset.tab === tab);
    });

    if (tab === "errors") {
      if (!errors.length) {
        body.innerHTML = `<p class="muted">No errors. Template save reloads this tab. Island compile errors stay in the Vite overlay.</p>`;
      } else {
        body.innerHTML = "<ul>" + errors.map((e) =>
          `<li><div class="err"></div><div class="muted"></div></li>`
        ).join("") + "</ul>";
        [...body.querySelectorAll("li")].forEach((li, i) => {
          li.querySelector(".err").textContent = errors[i].message;
          li.querySelector(".muted").textContent = (errors[i].source || "server") + (errors[i].at ? " · " + errors[i].at : "");
        });
      }
    } else if (tab === "routes") {
      const routes = info.routes || [];
      if (!routes.length) {
        body.innerHTML = `<p class="muted">No file routes yet. MountPages first.</p>`;
      } else {
        body.innerHTML = "<ul>" + routes.map(() => `<li><code></code> <span class="muted"></span></li>`).join("") + "</ul>";
        [...body.querySelectorAll("li")].forEach((li, i) => {
          li.querySelector("code").textContent = routes[i].path;
          li.querySelector("span").textContent = routes[i].mode || "ssr";
        });
      }
    } else {
      body.innerHTML = `<p>env <code></code></p><p>listen <code></code></p><p class="muted">Live reload watches app/ and templates/. Go files need a process restart. Vite HMR handles islands.</p>`;
      const codes = body.querySelectorAll("code");
      codes[0].textContent = info.env || "development";
      codes[1].textContent = info.addr || "";
    }
  }

  function pushError(item) {
    errors.unshift(item);
    if (errors.length > 50) errors.pop();
    open = true;
    panel.classList.add("open");
    tab = "errors";
    paint();
  }

  root.querySelector("#irmik-dev-badge").addEventListener("click", () => {
    open = !open;
    panel.classList.toggle("open", open);
    if (open) paint();
  });
  root.querySelectorAll("#irmik-dev-tabs button").forEach((b) => {
    b.addEventListener("click", () => { tab = b.dataset.tab; paint(); });
  });
  root.querySelector("#irmik-dev-clear").addEventListener("click", () => {
    errors.length = 0;
    paint();
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && open) {
      open = false;
      panel.classList.remove("open");
    }
  });

  window.addEventListener("error", (e) => {
    pushError({ source: "window", message: e.message || "error", at: e.filename ? e.filename + ":" + e.lineno : "" });
  });
  window.addEventListener("unhandledrejection", (e) => {
    const reason = e.reason;
    const message = reason && reason.message ? reason.message : String(reason || "unhandledrejection");
    pushError({ source: "promise", message, at: "" });
  });

  fetch(PREFIX + "/info")
    .then((r) => r.ok ? r.json() : null)
    .then((data) => { if (data) { info = data; if (open) paint(); } })
    .catch(() => {});

  try {
    const es = new EventSource(PREFIX + "/events");
    es.addEventListener("open", () => { connected = true; paint(); });
    es.onopen = () => { connected = true; paint(); };
    es.onerror = () => { connected = false; paint(); };
    es.addEventListener("reload", () => { location.reload(); });
    es.addEventListener("problem", (ev) => {
      let payload = { source: "template", message: ev.data || "error", at: "" };
      try { payload = Object.assign(payload, JSON.parse(ev.data)); } catch (_) {}
      pushError(payload);
    });
    es.addEventListener("info", (ev) => {
      try { info = Object.assign(info, JSON.parse(ev.data)); paint(); } catch (_) {}
    });
  } catch (_) {
    connected = false;
    paint();
  }

  paint();
})();
