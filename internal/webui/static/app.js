// Instant search dropdown + wikilink hover popovers. No dependencies.
(function () {
  "use strict";
  const esc = (s) => s.replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));

  // --- instant search ------------------------------------------------
  document.querySelectorAll(".searchbar").forEach((form) => {
    const input = form.querySelector("input[name=q]");
    if (!input) return;
    const box = document.createElement("div");
    box.className = "ac";
    box.hidden = true;
    form.style.position = "relative";
    form.appendChild(box);

    let items = [];
    let sel = -1;
    let timer = null;
    let seq = 0;

    function render() {
      box.innerHTML = items.map((it, i) =>
        `<a href="/page/${encodeURIComponent(it.Slug)}" class="${i === sel ? "sel" : ""}">` +
        `<b>${esc(it.Title || it.Slug)}</b> <span class="muted">${esc(it.Slug)}</span></a>`
      ).join("");
      box.hidden = items.length === 0;
    }

    input.addEventListener("input", () => {
      clearTimeout(timer);
      const q = input.value.trim();
      if (!q) { items = []; render(); return; }
      timer = setTimeout(async () => {
        const my = ++seq;
        try {
          const r = await fetch(`/api/search?q=${encodeURIComponent(q)}&k=8`);
          const d = await r.json();
          if (my !== seq) return; // stale response
          items = d.results || [];
          sel = -1;
          render();
        } catch { /* server gone; plain form submit still works */ }
      }, 150);
    });

    input.addEventListener("keydown", (e) => {
      if (box.hidden) return;
      if (e.key === "ArrowDown") { sel = Math.min(sel + 1, items.length - 1); render(); e.preventDefault(); }
      else if (e.key === "ArrowUp") { sel = Math.max(sel - 1, -1); render(); e.preventDefault(); }
      else if (e.key === "Enter" && sel >= 0) { location.href = "/page/" + encodeURIComponent(items[sel].Slug); e.preventDefault(); }
      else if (e.key === "Escape") { box.hidden = true; }
    });

    document.addEventListener("click", (e) => {
      if (!form.contains(e.target)) box.hidden = true;
    });
  });

  // --- wikilink popover preview -------------------------------------
  const cache = new Map();
  let pop = null;
  let ptimer = null;

  function hidePop() {
    clearTimeout(ptimer);
    if (pop) { pop.remove(); pop = null; }
  }

  async function showPop(a) {
    const slug = a.dataset.slug;
    let d = cache.get(slug);
    if (!d) {
      try {
        d = await (await fetch(`/api/preview/${encodeURIComponent(slug)}`)).json();
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
    const w = pop.offsetWidth;
    pop.style.left = Math.max(8, Math.min(r.left + scrollX, innerWidth + scrollX - w - 8)) + "px";
    pop.style.top = (r.bottom + scrollY + 6) + "px";
  }

  document.addEventListener("mouseover", (e) => {
    const a = e.target.closest("a.wikilink[data-slug]:not(.missing)");
    if (!a) return;
    clearTimeout(ptimer);
    ptimer = setTimeout(() => showPop(a), 250);
  });
  document.addEventListener("mouseout", (e) => {
    if (e.target.closest && e.target.closest("a.wikilink[data-slug]")) hidePop();
  });
})();
