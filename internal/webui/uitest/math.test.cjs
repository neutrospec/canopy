const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const vm = require('node:vm');
const vendor = join(__dirname, '../static/vendor');
const manifest = JSON.parse(readFileSync(join(vendor, 'assets.json'), 'utf8'));

// Execute the actual pinned TeX processor and extension modules. Disable DOM
// startup/timers: these tests compile TeX to its MathML tree, not to painted SVG,
// and make no browser, font, accessibility, or network-loading claim.
const context = {
  MathJax: { startup: { ready() {}, typeset: false } },
  document: { getElementById: () => null, getElementsByTagName: () => [], documentElement: { querySelectorAll: () => [] } },
  console, setTimeout() {}, clearTimeout() {},
};
context.window = context;
vm.createContext(context);
for (const name of ['tex-svg.js', 'mathjax-color.js', 'mathjax-physics.js']) {
  vm.runInContext(readFileSync(join(vendor, name), 'utf8'), context, { filename: name });
}
function compile(source) {
  const modules = context.MathJax._;
  const tex = new modules.input.tex_ts.TeX({ packages: manifest.mathjax.packages });
  tex.setMmlFactory(new modules.core.MmlTree.MmlFactory.MmlFactory());
  const tree = tex.compile({ math: source, display: true, inputData: {} }, { options: {} });
  return { tree, error: tex.parseOptions.error, source: tex.latex };
}
for (const source of manifest.mathjax.corpus) {
  test(`pinned local TeX corpus compiles: ${source}`, () => {
    const result = compile(source);
    assert.equal(result.error, false);
    assert.equal(result.tree.kind, 'math');
    assert.equal(result.source, source);
    assert.doesNotMatch(result.tree.toString(), /merror/);
  });
}
test('undeclared TeX extensions fail instead of triggering an autoload', () => {
  const result = compile(manifest.mathjax.unsupported_example);
  assert.equal(result.error, true);
  assert.match(result.tree.toString(), /merror/);
  assert.equal(result.source, manifest.mathjax.unsupported_example);
});
