const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const vm = require('node:vm');
const source = readFileSync(join(__dirname, '../static/app.js'), 'utf8');

// A small event/clock adapter runs the shipped script without npm or a
// browser. Layout, native focus, and real markup are checked in the browser.
class Element {
  constructor(tag = 'DIV') {
    this.tagName = tag.toUpperCase();
    this.children = [];
    this.listeners = new Map();
    this.attributes = new Map();
    this.selectors = new Map();
    this.hidden = false;
    this.value = '';
    this.textContent = '';
    this.innerHTML = '';
    this.dataset = {};
    this.className = '';
    this.classes = new Set();
    this.classList = { add: (name) => this.classes.add(name), contains: (name) => this.classes.has(name) };
  }
  get childNodes() { return this.children; }
  get firstElementChild() { return this.children[0] || null; }
  appendChild(child) { this.children.push(child); return child; }
  setAttribute(name, value) { this.attributes.set(name, String(value)); }
  getAttribute(name) { return this.attributes.get(name); }
  removeAttribute(name) { this.attributes.delete(name); }
  addEventListener(name, callback) {
    if (!this.listeners.has(name)) this.listeners.set(name, []);
    this.listeners.get(name).push(callback);
  }
  fire(name, values = {}) {
    const event = { target: this, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, ...values };
    const results = (this.listeners.get(name) || []).map((fn) => fn(event));
    return Promise.all(results);
  }
  contains(node) { return node === this || this.children.some((child) => child.contains(node)); }
  querySelector(selector) { return this.selectors.get(selector) || null; }
  querySelectorAll(selector) { return this.selectors.get(selector) || []; }
  focus() { this.focusCount = (this.focusCount || 0) + 1; }
  click() { this.clickCount = (this.clickCount || 0) + 1; return this.fire('click'); }
  cloneNode() { const clone = new Element(this.tagName); clone.textContent = this.textContent; return clone; }
}

function harness(options = {}) {
  const document = new Element('DOCUMENT');
  document.documentElement = new Element('HTML');
  document.documentElement.scrollHeight = options.longPage ? 3000 : 700;
  document.visibilityState = 'visible';
  document.body = new Element('BODY');
  document.head = new Element('HEAD');
  document.createElement = (tag) => new Element(tag);
  const ids = new Map();
  document.getElementById = (id) => ids.get(id) || null;
  const messages = new Element('SCRIPT');
  messages.textContent = JSON.stringify({
    copy_button: 'Copy', copy_done: 'Copied', copy_failed: 'Copy failed',
    heading_link: 'Link to this section', page_read_label: 'read', page_auto: 'auto',
    page_reread: 'Read again', page_reread_title: 'Mark as read again',
    page_cancel: 'Cancel', page_unread_title: 'Undo read',
  });
  ids.set('ui-messages', messages);
  const form = new Element('FORM');
  const input = new Element('INPUT');
  form.appendChild(input);
  form.selectors.set('input[name=q]', input);
  document.selectors.set('.searchbar', options.search === false ? [] : [form]);
  document.selectors.set('.topbar input[name="q"]', input);

  const mobile = new Element();
  mobile.matches = !!options.mobile;
  const context = {
    document, navigator: {}, location: { href: 'initial' },
    innerHeight: 800, innerWidth: 1280, scrollY: 0, scrollX: 0,
    AbortController, addEventListener: (name, fn) => document.addEventListener(name, fn),
    matchMedia: (query) => query === '(max-width: 800px)' ? mobile : { matches: false },
  };
  context.window = context;
  const requests = [];
  context.fetch = (url, init = {}) => {
    if (url.startsWith('/api/read/')) {
      requests.push({ url, init });
      return options.readNetworkFailure ? Promise.reject(new Error('offline')) : Promise.resolve({ ok: options.readOK !== false });
    }
    return new Promise((resolve) => requests.push({ url, init, resolve }));
  };
  const timers = new Map();
  let nextTimer = 1;
  const addTimer = (callback, interval) => {
    const id = nextTimer++;
    timers.set(id, { callback, interval });
    return id;
  };
  context.setTimeout = (fn) => addTimer(fn, false);
  context.setInterval = (fn) => addTimer(fn, true);
  context.clearTimeout = context.clearInterval = (id) => timers.delete(id);
  const runTimeouts = () => {
    for (const [id, timer] of [...timers]) {
      if (!timer.interval) { timers.delete(id); timer.callback(); }
    }
  };
  const tick = async () => {
    for (const timer of [...timers.values()]) if (timer.interval) await timer.callback();
  };

  let sidebar, toggle;
  if (options.nav) {
    sidebar = new Element('NAV'); toggle = new Element('BUTTON');
    ids.set('site-nav', sidebar); ids.set('navtoggle', toggle);
  }
  let pre, code, heading, outline, toc, bodyTitle;
  if (options.prose) {
    const prose = new Element();
    prose.textContent = 'Ordinary prose';
    if (options.bodyTitle) {
      const pageTitle = new Element('H1'); pageTitle.textContent = 'Page title';
      bodyTitle = new Element('H1'); bodyTitle.textContent = options.bodyTitle;
      prose.appendChild(bodyTitle);
      document.selectors.set('article > .pagehead h1', pageTitle);
    }
    pre = new Element('PRE'); code = new Element('CODE');
    code.textContent = 'printf "<path> $value"\n';
    pre.selectors.set('code', code);
    pre.appendChild(code);
    heading = new Element('H2'); heading.id = 'deploy'; heading.textContent = 'Deployment';
    prose.selectors.set('pre', [pre]);
    prose.selectors.set('h2[id], h3[id], h4[id]', [heading]);
    document.selectors.set('.prose', prose);
    outline = new Element('ASIDE'); outline.hidden = true;
    toc = new Element('OL');
    ids.set('pageoutline', outline); ids.set('page-toc', toc);
    context.navigator.clipboard = { writeText: async (text) => {
      context.copiedText = text;
      if (options.copyFailure) throw new Error('clipboard denied');
    } };
  }
  let readform, readbutton;
  if (options.article) {
    const article = new Element('ARTICLE');
    article.dataset = { slug: 'alpha', readSecs: '1' };
    document.selectors.set('article[data-slug]', article);
    readform = new Element('FORM'); readform.innerHTML = 'explicit controls';
    readbutton = new Element('BUTTON'); readform.appendChild(readbutton);
    ids.set('readform', readform);
    document.selectors.set('#readform button[value="read"]', readbutton);
  }
  vm.runInNewContext(source, context);
  const box = form.children[1];
  return { document, input, form, box, requests, context, runTimeouts, tick, mobile, sidebar, toggle, pre, code, heading, outline, toc, bodyTitle, readform, readbutton };
}
const settle = () => new Promise((resolve) => setImmediate(resolve));
function search(env, value) { env.input.value = value; env.input.fire('input'); env.runTimeouts(); }
async function results(request, slug) {
  request.resolve({ ok: true, json: async () => ({ results: [{ Slug: slug, Title: slug }] }) });
  await settle();
}

// The adapter deliberately ignores abort signals: sequence checks must still
// protect the screen if the server or response decoding finishes after abort.
test('a cleared query cannot be reopened by a late response', async () => {
  const env = harness();
  search(env, 'old');
  env.input.value = ''; await env.input.fire('input');
  await results(env.requests[0], 'old-hit');
  assert.equal(env.box.hidden, true);
  assert.equal(env.input.getAttribute('aria-expanded'), 'false');
});
test('responses from before the current debounce stay hidden', async () => {
  const env = harness();
  search(env, 'old');
  env.input.value = 'new'; await env.input.fire('input');
  await results(env.requests[0], 'old-hit');
  assert.equal(env.box.hidden, true);
  assert.equal(env.requests.length, 1);
  env.runTimeouts();
  await results(env.requests[1], 'new-hit');
  assert.match(env.box.innerHTML, /new-hit/);
});
test('out-of-order search responses never replace current results', async () => {
  const env = harness();
  search(env, 'old'); search(env, 'new');
  await results(env.requests[1], 'new-hit');
  await results(env.requests[0], 'old-hit');
  assert.match(env.box.innerHTML, /new-hit/);
  assert.doesNotMatch(env.box.innerHTML, /old-hit/);
});
test('Escape cancels pending results and keyboard selection', async () => {
  const env = harness();
  search(env, 'alpha');
  await env.input.fire('keydown', { key: 'Escape' });
  await results(env.requests[0], 'alpha');
  assert.equal(env.box.hidden, true);
  assert.equal(env.input.getAttribute('aria-activedescendant'), undefined);
});
test('IME composition defers search and prevents suggestion navigation', async () => {
  const env = harness();
  await env.input.fire('compositionstart');
  search(env, '한');
  await env.input.fire('keydown', { key: 'Enter', isComposing: true });
  assert.equal(env.requests.length, 0);
  assert.equal(env.context.location.href, 'initial');
  env.input.value = '한국어';
  await env.input.fire('compositionend'); env.runTimeouts();
  assert.match(env.requests[0].url, /%ED%95%9C%EA%B5%AD%EC%96%B4/);
  await results(env.requests[0], 'korean');
  assert.equal(env.box.hidden, false);
});
test('keyboard selection is exposed to assistive technology and opens the selected page', async () => {
  const env = harness();
  search(env, 'alpha'); await results(env.requests[0], 'alpha');
  await env.input.fire('keydown', { key: 'ArrowDown' });
  assert.equal(env.input.getAttribute('aria-activedescendant'), 'search-options-0-0');
  assert.match(env.box.innerHTML, /aria-selected="true"/);
  await env.input.fire('keydown', { key: 'Enter' });
  assert.equal(env.context.location.href, '/page/alpha');
});
test('failed search responses leave the ordinary form available', async () => {
  const env = harness();
  search(env, 'alpha');
  env.requests[0].resolve({ ok: false }); await settle();
  assert.equal(env.box.hidden, true);
  assert.equal(env.input.value, 'alpha');
});
test('mobile navigation opens, closes with Escape, and adapts to desktop', async () => {
  const env = harness({ nav: true, mobile: true });
  assert.equal(env.sidebar.hidden, true);
  assert.equal(env.toggle.hidden, false);
  await env.toggle.click();
  assert.equal(env.sidebar.hidden, false);
  assert.equal(env.toggle.getAttribute('aria-expanded'), 'true');
  await env.document.fire('keydown', { key: 'Escape' });
  assert.equal(env.sidebar.hidden, true);
  assert.equal(env.toggle.focusCount, 1);
  env.mobile.matches = false; await env.mobile.fire('change');
  assert.equal(env.sidebar.hidden, false);
  assert.equal(env.toggle.hidden, true);
});
test('search shortcuts respect typing, while Ctrl K can focus search from an editor', async () => {
  const env = harness();
  await env.document.fire('keydown', { key: '/', target: new Element('TEXTAREA') });
  assert.equal(env.input.focusCount, undefined);
  await env.document.fire('keydown', { key: '/' });
  await env.document.fire('keydown', { key: 'k', ctrlKey: true, target: new Element('TEXTAREA') });
  assert.equal(env.input.focusCount, 2);
});
test('code copy preserves literal source text and displays localized success', async () => {
  const env = harness({ prose: true, search: false });
  const button = env.pre.children[1];
  assert.equal(button.textContent, 'Copy');
  await button.click();
  assert.equal(env.context.copiedText, env.code.textContent);
  assert.equal(button.textContent, 'Copied');
  env.runTimeouts();
  assert.equal(button.textContent, 'Copy');
});
test('clipboard failure never displays a success state', async () => {
  const env = harness({ prose: true, copyFailure: true, search: false });
  const button = env.pre.children[1];
  await button.click();
  assert.equal(button.textContent, 'Copy failed');
  assert.equal(button.disabled, false);
});
test('the page outline uses real heading targets without including the anchor marker', () => {
  const env = harness({ prose: true, search: false });
  assert.equal(env.outline.hidden, false);
  const link = env.toc.children[0].children[0];
  assert.equal(link.href, '#deploy');
  assert.equal(link.textContent, 'Deployment');
  assert.equal(env.heading.children[0].getAttribute('aria-label'), 'Link to this section: Deployment');
});
test('only an exact duplicate body title is collapsed, with its source preserved', () => {
  const same = harness({ prose: true, bodyTitle: 'Page title', search: false });
  assert.equal(same.bodyTitle.hidden, true);
  assert.equal(same.bodyTitle.textContent, 'Page title');
  const different = harness({ prose: true, bodyTitle: 'Another heading', search: false });
  assert.equal(different.bodyTitle.hidden, false);
});
test('auto read requires visible dwell and scroll progress', async () => {
  const env = harness({ article: true, longPage: true, search: false });
  env.document.visibilityState = 'hidden'; await env.tick();
  env.context.scrollY = 2200; await env.document.fire('scroll');
  assert.equal(env.requests.length, 0);
  env.document.visibilityState = 'visible'; await env.tick();
  assert.equal(env.requests.length, 1);
  assert.match(env.readform.innerHTML, /readmark/);
  assert.match(env.readform.innerHTML, /Read again/);
  assert.match(env.readform.innerHTML, /auto/);
});
test('dwell without sufficient scroll does not mark a long page read', async () => {
  const env = harness({ article: true, longPage: true, search: false });
  await env.tick(); await env.tick();
  assert.equal(env.requests.length, 0);
  assert.equal(env.readform.innerHTML, 'explicit controls');
});
for (const failure of ['HTTP', 'network']) {
  test(`${failure} failure keeps explicit read controls without showing auto success`, async () => {
    const env = harness({ article: true, search: false, readOK: false, readNetworkFailure: failure === 'network' });
    await env.tick();
    assert.equal(env.readform.innerHTML, 'explicit controls');
    await env.document.fire('keydown', { key: 'r' });
    assert.equal(env.readbutton.clickCount, 1);
  });
}
test('read shortcut respects editable content and key repeats', async () => {
  const env = harness({ article: true, search: false });
  const editable = new Element(); editable.isContentEditable = true;
  await env.document.fire('keydown', { key: 'r', target: editable });
  await env.document.fire('keydown', { key: 'r', repeat: true });
  assert.equal(env.readbutton.clickCount, undefined);
});
