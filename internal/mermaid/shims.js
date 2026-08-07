// Minimal browser-environment shims so the vendored mermaid bundle can be
// *loaded* under goja. We only ever call mermaid.parse() — nothing renders,
// so DOM objects just need to exist, not behave.
var window = globalThis;
var self = globalThis;
var document = {
  createElement: function () {
    return {
      style: {},
      setAttribute: function () {},
      appendChild: function () {},
      classList: { add: function () {} },
    };
  },
  createTextNode: function () { return {}; },
  querySelector: function () { return null; },
  querySelectorAll: function () { return []; },
  body: { appendChild: function () {}, removeChild: function () {} },
  documentElement: { style: {} },
  addEventListener: function () {},
};
var navigator = { userAgent: "goja" };
var location = { href: "http://localhost/", protocol: "http:", host: "localhost" };
window.addEventListener = function () {};
// mermaid.parse never schedules timers; running callbacks synchronously keeps
// its internal promises resolvable without an event loop.
var setTimeout = function (fn) { fn(); return 0; };
var clearTimeout = function () {};
var setInterval = function () { return 0; };
var clearInterval = function () {};
var console = { log: function () {}, warn: function () {}, error: function () {}, debug: function () {}, info: function () {}, trace: function () {} };
// goja has no TextEncoder/structuredClone (pie, gitGraph need them at init).
function TextEncoder() {}
TextEncoder.prototype.encode = function (s) {
  s = String(s);
  var out = [];
  for (var i = 0; i < s.length; i++) out.push(s.charCodeAt(i) & 0xff);
  return new Uint8Array(out);
};
function TextDecoder() {}
TextDecoder.prototype.decode = function (a) {
  var s = "";
  for (var i = 0; i < a.length; i++) s += String.fromCharCode(a[i]);
  return s;
};
var structuredClone = function (o) {
  return o === undefined ? o : JSON.parse(JSON.stringify(o));
};
