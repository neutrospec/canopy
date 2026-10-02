// Progressive enhancements for the server-rendered wiki. No dependencies.
(function () {
  "use strict";
  const messages = JSON.parse(document.getElementById("ui-messages")?.textContent || "{}");
  const t = (id) => messages[id] || id;
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));
  const typing = (el) => el?.isContentEditable || /^(input|textarea|select)$/i.test(el?.tagName || "");

  // --- workspace navigation and search shortcuts ----------------------
  const sidebar = document.getElementById("site-nav");
  const toggle = document.getElementById("navtoggle");
  const mobile = matchMedia("(max-width: 800px)");
  if (sidebar && toggle) {
    document.documentElement.classList.add("js-nav");
    function setNav(open) {
      sidebar.hidden = !open;
      toggle.setAttribute("aria-expanded", String(open));
    }
    function adaptNav() {
      toggle.hidden = !mobile.matches;
      setNav(!mobile.matches);
    }
    adaptNav();
    mobile.addEventListener("change", adaptNav);
    toggle.addEventListener("click", () => setNav(sidebar.hidden));
    document.addEventListener("click", (e) => {
      if (mobile.matches && !sidebar.contains(e.target) && !toggle.contains(e.target)) setNav(false);
    });
    document.addEventListener("keydown", (e) => {
      if (e.key === "Escape" && mobile.matches && !sidebar.hidden) {
        setNav(false);
        toggle.focus();
      }
    });
  }
  const searchInput = document.querySelector('.topbar input[name="q"]');
  document.querySelector(".searchfocus")?.addEventListener("click", () => searchInput?.focus());
  document.addEventListener("keydown", (e) => {
    if (e.defaultPrevented || e.isComposing) return;
    if (((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") ||
        (e.key === "/" && !e.metaKey && !e.ctrlKey && !e.altKey && !typing(e.target))) {
      if (searchInput) { e.preventDefault(); searchInput.focus(); }
    }
  });

  // --- instant search --------------------------------------------------
  document.querySelectorAll(".searchbar").forEach((form, index) => {
    const input = form.querySelector("input[name=q]");
    if (!input) return;
    const box = document.createElement("div");
    box.className = "ac";
    box.id = `search-options-${index}`;
    box.setAttribute("role", "listbox");
    box.hidden = true;
    form.appendChild(box);
    input.setAttribute("role", "combobox");
    input.setAttribute("aria-autocomplete", "list");
    input.setAttribute("aria-controls", box.id);
    input.setAttribute("aria-expanded", "false");

    let items = [], sel = -1, timer = null, seq = 0, request = null, composing = false;
    function invalidate() {
      clearTimeout(timer);
      seq++;
      request?.abort();
      request = null;
    }
    function render() {
      box.innerHTML = items.map((it, i) =>
        `<a id="${box.id}-${i}" role="option" aria-selected="${i === sel}" tabindex="-1" ` +
        `href="/page/${encodeURIComponent(it.Slug)}" class="${i === sel ? "sel" : ""}">` +
        `<b>${esc(it.Title || it.Slug)}</b><span class="muted">${esc(it.Slug)}</span></a>`
      ).join("");
      box.hidden = items.length === 0;
      input.setAttribute("aria-expanded", String(!box.hidden));
      if (sel >= 0) input.setAttribute("aria-activedescendant", `${box.id}-${sel}`);
      else input.removeAttribute("aria-activedescendant");
      box.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
    }
    function close() {
      invalidate();
      items = []; sel = -1;
      render();
    }
    function schedule() {
      // Invalidate at the keystroke, before the debounce: an older response
      // must not reopen results while the new request is still waiting.
      close();
      const q = input.value.trim();
      if (!q || composing) return;
      const my = seq;
      timer = setTimeout(async () => {
        request = new AbortController();
        try {
          const r = await fetch(`/api/search?q=${encodeURIComponent(q)}&k=8`, { signal: request.signal });
          if (!r.ok) throw new Error("search failed");
          const d = await r.json();
          if (my !== seq) return;
          items = Array.isArray(d.results) ? d.results.filter((it) => typeof it.Slug === "string") : [];
          sel = -1;
          render();
        } catch {
          if (my === seq) { items = []; sel = -1; render(); }
          // The ordinary search form remains usable when the API is down.
        }
      }, 150);
    }
    input.addEventListener("input", schedule);
    input.addEventListener("compositionstart", () => { composing = true; close(); });
    input.addEventListener("compositionend", () => { composing = false; schedule(); });
    input.addEventListener("keydown", (e) => {
      if (composing || e.isComposing) return;
      if (e.key === "Escape" || e.key === "Tab") { close(); return; }
      if (box.hidden) return;
      if (e.key === "ArrowDown") { sel = Math.min(sel + 1, items.length - 1); render(); e.preventDefault(); }
      else if (e.key === "ArrowUp") { sel = Math.max(sel - 1, -1); render(); e.preventDefault(); }
      else if (e.key === "Enter" && sel >= 0) { location.href = "/page/" + encodeURIComponent(items[sel].Slug); e.preventDefault(); }
    });
    document.addEventListener("click", (e) => { if (!form.contains(e.target)) close(); });
  });

  // --- wikilink previews (exposure, never a read) -----------------------
  const cache = new Map();
  let pop = null, ptimer = null;
  function hidePop() {
    clearTimeout(ptimer);
    if (pop) { pop.remove(); pop = null; }
  }
  async function showPop(a) {
    const slug = a.dataset.slug;
    let d = cache.get(slug);
    if (!d) {
      try {
        const response = await fetch(`/api/preview/${encodeURIComponent(slug)}`);
        if (!response.ok) return;
        d = await response.json();
      } catch { return; }
      cache.set(slug, d);
    }
    if (!d.exists || !d.excerpt || !a.matches(":hover")) return;
    hidePop();
    pop = document.createElement("div");
    pop.className = "popover";
    pop.innerHTML = `<b>${esc(d.title)}</b><p>${esc(d.excerpt)}</p>`;
    document.body.appendChild(pop);
    const r = a.getBoundingClientRect();
    pop.style.left = Math.max(8, Math.min(r.left + scrollX, innerWidth + scrollX - pop.offsetWidth - 8)) + "px";
    const y = r.bottom + 6 + pop.offsetHeight > innerHeight ? r.top - pop.offsetHeight - 6 : r.bottom + 6;
    pop.style.top = (Math.max(8, y) + scrollY) + "px";
  }
  document.addEventListener("mouseover", (e) => {
    const a = e.target.closest?.("a.wikilink[data-slug]:not(.missing)");
    if (!a) return;
    clearTimeout(ptimer);
    ptimer = setTimeout(() => showPop(a), 250);
  });
  document.addEventListener("mouseout", (e) => {
    if (e.target.closest?.("a.wikilink[data-slug]")) hidePop();
  });

  // --- code copy, section links and outline, diagrams, math ------------
  const loadScript = (src) => new Promise((ok, bad) => {
    const s = document.createElement("script");
    s.src = src; s.onload = ok; s.onerror = bad;
    document.head.appendChild(s);
  });
  const dark = matchMedia("(prefers-color-scheme: dark)").matches;
  const prose = document.querySelector(".prose");
  if (prose) {
    // new pages include a body H1 as well as the frontmatter title. Keep
    // the source intact and collapse only an exact duplicate in the view.
    const title = document.querySelector("article > .pagehead h1");
    const first = prose.firstElementChild;
    if (title && first?.tagName === "H1" && first.textContent.trim() === title.textContent.trim()) first.hidden = true;
    prose.querySelectorAll("pre").forEach((pre) => {
      if (pre.classList.contains("mermaid")) return;
      const btn = document.createElement("button");
      btn.className = "copybtn";
      btn.type = "button";
      btn.textContent = t("copy_button");
      let reset = null;
      btn.addEventListener("click", async () => {
        clearTimeout(reset);
        btn.disabled = true;
        try {
          const code = pre.querySelector("code");
          const text = code ? code.textContent : Array.from(pre.childNodes).filter((n) => n !== btn).map((n) => n.textContent).join("");
          await navigator.clipboard.writeText(text);
          btn.textContent = t("copy_done");
        } catch { btn.textContent = t("copy_failed"); }
        finally { btn.disabled = false; }
        reset = setTimeout(() => { btn.textContent = t("copy_button"); }, 1500);
      });
      pre.appendChild(btn);
    });

    const toc = document.getElementById("page-toc");
    prose.querySelectorAll("h2[id], h3[id], h4[id]").forEach((h) => {
      const title = h.textContent;
      const a = document.createElement("a");
      a.className = "hanchor";
      a.href = "#" + h.id;
      a.textContent = "#";
      a.setAttribute("aria-label", `${t("heading_link")}: ${title}`);
      h.appendChild(a);
      if (toc) {
        const li = document.createElement("li");
        if (h.tagName !== "H2") li.className = "toc-sub";
        const link = document.createElement("a");
        link.href = "#" + h.id;
        link.textContent = title;
        li.appendChild(link);
        toc.appendChild(li);
      }
    });
    if (toc?.children.length) document.getElementById("pageoutline").hidden = false;
    prose.querySelectorAll('a[href^="http"]').forEach((a) => {
      a.target = "_blank"; a.rel = "noopener"; a.classList.add("external");
    });
    const diagrams = prose.querySelectorAll("pre.mermaid, div.mermaid");
    if (diagrams.length) {
      loadScript("/static/vendor/mermaid.min.js").then(() => {
        window.mermaid.initialize({ startOnLoad: false, theme: dark ? "dark" : "default", securityLevel: "strict" });
        return window.mermaid.run({ nodes: diagrams });
      }).catch(() => {});
    }
    const textOnly = (() => {
      const c = prose.cloneNode(true);
      c.querySelectorAll("pre, code").forEach((n) => n.remove());
      return c.textContent;
    })();
    if (/\$\$[^$]+\$\$|(^|[^\\$])\$[^\s$][^$\n]*\$|\\\(|\\\[/.test(textOnly)) {
      window.MathJax = {
        tex: { inlineMath: [["$", "$"], ["\\(", "\\)"]], displayMath: [["$$", "$$"], ["\\[", "\\]"]] },
        svg: { fontCache: "global" },
      };
      loadScript("/static/vendor/tex-svg.js").catch(() => {});
    }
  }

  // --- read tracking ---------------------------------------------------
  const article = document.querySelector("article[data-slug]");
  if (article) {
    document.addEventListener("keydown", (e) => {
      if (e.defaultPrevented || e.isComposing || e.repeat || e.key !== "r" || e.metaKey || e.ctrlKey || e.altKey || typing(e.target)) return;
      document.querySelector('#readform button[value="read"]')?.click();
    });
    const secs = +article.dataset.readSecs;
    if (secs) {
      const slug = article.dataset.slug;
      let visibleMs = 0, maxScroll = 0, sent = false;
      const scrolled = () => {
        const h = document.documentElement.scrollHeight;
        return h <= innerHeight + 50 ? 1 : (scrollY + innerHeight) / h;
      };
      const onScroll = () => { maxScroll = Math.max(maxScroll, scrolled()); };
      onScroll();
      addEventListener("scroll", onScroll, { passive: true });
      const tick = setInterval(async () => {
        if (document.visibilityState !== "visible" || sent) return;
        visibleMs += 1000;
        if (visibleMs >= secs * 1000 && maxScroll >= .7) {
          sent = true;
          clearInterval(tick);
          try {
            const response = await fetch(`/api/read/${encodeURIComponent(slug)}?source=auto`, { method: "POST" });
            if (!response.ok) return;
            const form = document.getElementById("readform");
            if (form) form.innerHTML =
              `<span class="chip readmark">✓ ${esc(t("page_read_label"))} <small>${esc(t("page_auto"))}</small></span>` +
              `<button class="chip" name="action" value="read" title="${esc(t("page_reread_title"))}">${esc(t("page_reread"))}</button>` +
              `<button class="chip" name="action" value="unread" title="${esc(t("page_unread_title"))}">${esc(t("page_cancel"))}</button>`;
          } catch { /* Explicit read controls remain usable. */ }
        }
      }, 1000);
    }
  }

  const langsel = document.querySelector("details.langsel");
  if (langsel) {
    document.addEventListener("click", (e) => { if (langsel.open && !langsel.contains(e.target)) langsel.open = false; });
    document.addEventListener("keydown", (e) => { if (e.key === "Escape" && langsel.open) langsel.open = false; });
  }
})();
