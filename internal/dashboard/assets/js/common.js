// Shared helpers for the dashboard's read-only project views (Project Brain,
// Memory, Impact, Optimization). Kept dependency-free like the rest of the
// dashboard. tasks.js/sessions.js predate this file and keep their own local
// copies rather than being refactored to depend on it.
(function () {
  "use strict";

  var token = new URLSearchParams(location.search).get("token") || "";

  function api(path) {
    var sep = path.indexOf("?") >= 0 ? "&" : "?";
    return fetch(path + sep + "token=" + encodeURIComponent(token)).then(function (r) {
      if (!r.ok) throw new Error("HTTP " + r.status);
      return r.json();
    });
  }

  function esc(s) {
    return String(s == null ? "" : s).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  function card(title, body) { return '<div class="card"><h2>' + title + "</h2>" + body + "</div>"; }
  function kv(k, v) { return "<span><b>" + esc(k) + ":</b> " + v + "</span>"; }

  function stateClass(s) { return "state-" + esc(s || "unverified"); }

  window.AuxCommon = { api: api, esc: esc, card: card, kv: kv, stateClass: stateClass };
})();
