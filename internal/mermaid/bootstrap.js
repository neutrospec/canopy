// Runs after the bundle is loaded. The bundled DOMPurify built itself against
// our fake DOM and came out without sanitize/addHook; backfill them through
// Object.prototype so mermaid's sanitizeText becomes identity. Acceptable only
// because this VM is a throwaway parse sandbox — nothing renders, nothing
// escapes to a browser.
(function () {
  var stub = {
    sanitize: function (t) { return t; },
    addHook: function () {},
    removeHook: function () {},
    removeAllHooks: function () {},
    setConfig: function () {},
    clearConfig: function () {},
    isValidAttribute: function () { return true; },
  };
  for (var k in stub) {
    if (!(k in Object.prototype)) {
      Object.defineProperty(Object.prototype, k, {
        value: stub[k],
        writable: true,
        configurable: true,
        enumerable: false,
      });
    }
  }
})();

mermaid.initialize({ startOnLoad: false, securityLevel: "loose" });

// mermaid.parse is async, but goja drains the microtask queue before returning
// control to Go, so `out` is settled by the time the caller reads it. A still-
// "pending" state therefore signals an environment fault, not a bad diagram.
function __canopyValidate(src) {
  var out = { state: "pending", name: "", msg: "" };
  try {
    mermaid.parse(src).then(
      function () { out.state = "ok"; },
      function (e) {
        out.state = "err";
        out.name = String((e && e.name) || "");
        out.msg = String((e && e.message) || e);
      }
    );
  } catch (e) {
    out.state = "err";
    out.name = String((e && e.name) || "");
    out.msg = String((e && e.message) || e);
  }
  return out;
}
