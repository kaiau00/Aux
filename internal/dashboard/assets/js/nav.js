// Shared top navigation, injected into every dashboard page (roadmapplan.md
// §13.12 "shared nav/shell"). No page owns its own copy of this markup, so
// adding a view means adding one entry here.
(function () {
  "use strict";

  var LINKS = [
    { href: "tasks", label: "Tasks" },
    { href: "project", label: "Project Brain" },
    { href: "memory", label: "Memory" },
    { href: "impact", label: "Impact" },
    { href: "optimization", label: "Optimization" },
    { href: "sessions", label: "Sessions" }
  ];

  function currentPage() {
    var path = location.pathname.replace(/^\//, "").replace(/\/$/, "");
    return path === "" ? "tasks" : path;
  }

  function withQuery(href) {
    return href + location.search;
  }

  window.AuxNav = {
    mount: function (subtitle) {
      var header = document.createElement("header");
      header.className = "topbar";
      var active = currentPage();
      var links = LINKS.map(function (l) {
        var cls = "navlink" + (l.href === active ? " active" : "");
        return '<a class="' + cls + '" href="' + withQuery(l.href) + '">' + l.label + "</a>";
      }).join("");
      header.innerHTML =
        '<span class="brand">Aux</span>' +
        (subtitle ? '<span class="subtitle">' + subtitle + "</span>" : "") +
        '<nav class="navlinks">' + links + "</nav>";
      document.body.insertBefore(header, document.body.firstChild);
    }
  };
})();
