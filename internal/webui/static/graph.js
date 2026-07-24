// Interactive knowledge graph (Obsidian-style): force layout with
// zoom/pan, node dragging, neighbor highlighting on hover, click to
// open the page, and a find box that flies to a node.
(function () {
  "use strict";
  const el = document.getElementById("graph");
  if (!el || typeof ForceGraph === "undefined") return;

  const css = getComputedStyle(document.documentElement);
  const C = {
    bg: css.getPropertyValue("--bg").trim(),
    fg: css.getPropertyValue("--fg").trim(),
    muted: css.getPropertyValue("--muted").trim(),
    line: css.getPropertyValue("--line").trim(),
    accent: css.getPropertyValue("--accent").trim(),
    link: css.getPropertyValue("--link").trim(),
    missing: css.getPropertyValue("--missing").trim(),
  };
  const dirColor = { entities: C.link, concepts: C.accent, comparisons: "#c586c0" };

  const graph = ForceGraph()(el)
    .backgroundColor(C.bg)
    .nodeId("id")
    .nodeLabel(null)
    .linkColor(() => C.line)
    .linkWidth((l) => (highlightLinks.has(l) ? 2 : 1))
    .linkDirectionalParticles((l) => (highlightLinks.has(l) ? 2 : 0))
    .linkDirectionalParticleWidth(2.5)
    .cooldownTicks(200)
    .d3VelocityDecay(0.25);

  let hoverNode = null;
  const highlightNodes = new Set();
  const highlightLinks = new Set();
  const neighbors = new Map(); // id -> Set(ids)
  const nodeLinks = new Map(); // id -> Set(link objects)

  // Rich hover tooltip: zoomed out the dots carry no information, so
  // the hovered node always gets a card (instant metadata, excerpt
  // filled async from /api/preview).
  const tip = document.createElement("div");
  tip.className = "graphtip";
  tip.hidden = true;
  el.appendChild(tip);
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));
  let mouse = { x: 0, y: 0 };
  el.addEventListener("mousemove", (e) => {
    const r = el.getBoundingClientRect();
    mouse = { x: e.clientX - r.left, y: e.clientY - r.top };
    if (!tip.hidden) placeTip();
  });
  function placeTip() {
    const pad = 14;
    let x = mouse.x + pad, y = mouse.y + pad;
    const w = tip.offsetWidth, h = tip.offsetHeight;
    if (x + w > el.clientWidth - 8) x = mouse.x - w - pad;
    if (y + h > el.clientHeight - 8) y = mouse.y - h - pad;
    tip.style.left = Math.max(8, x) + "px";
    tip.style.top = Math.max(8, y) + "px";
  }
  function showTip(n) {
    const meta = [n.dir, `링크 ${n.deg}`, n.read ? "읽음" : "안 읽음"];
    if (n.island) meta.push("🏝 섬");
    tip.innerHTML = `<b>${esc(n.title)}</b><span class="tipmeta">${esc(meta.join(" · "))}</span>` +
      (n.excerpt ? `<p>${esc(n.excerpt)}</p>` : "");
    tip.hidden = false;
    placeTip();
  }

  const radius = (n) => 2.5 + Math.sqrt(n.deg || 0) * 1.6;

  graph
    .nodeCanvasObject((n, ctx, scale) => {
      const r = radius(n);
      const dim = highlightNodes.size > 0 && !highlightNodes.has(n.id);
      ctx.globalAlpha = dim ? 0.15 : 1;
      ctx.beginPath();
      ctx.arc(n.x, n.y, r, 0, 2 * Math.PI);
      ctx.fillStyle = dirColor[n.dir] || C.muted;
      ctx.fill();
      if (!n.read) { // unread pages get a ring — the discovery loop, visible
        ctx.strokeStyle = C.missing;
        ctx.lineWidth = 1 / scale;
        ctx.stroke();
      }
      if (n.island) { // disconnected clusters get a dashed halo
        ctx.beginPath();
        ctx.setLineDash([2.5, 2]);
        ctx.arc(n.x, n.y, r + 3, 0, 2 * Math.PI);
        ctx.strokeStyle = "#d29922";
        ctx.lineWidth = 1.2 / scale;
        ctx.stroke();
        ctx.setLineDash([]);
      }
      // labels appear as you zoom in (Obsidian behavior), always on hover
      if (scale > 2 || (hoverNode && highlightNodes.has(n.id))) {
        const size = Math.max(11 / scale, 1.6);
        ctx.font = `${size}px -apple-system, "Apple SD Gothic Neo", sans-serif`;
        ctx.textAlign = "center";
        ctx.textBaseline = "top";
        ctx.fillStyle = dim ? C.muted : C.fg;
        ctx.fillText(n.title, n.x, n.y + r + 1.5 / scale);
      }
      ctx.globalAlpha = 1;
    })
    .nodePointerAreaPaint((n, color, ctx) => {
      ctx.beginPath();
      ctx.arc(n.x, n.y, radius(n) + 3, 0, 2 * Math.PI);
      ctx.fillStyle = color;
      ctx.fill();
    })
    .onNodeHover((n) => {
      highlightNodes.clear();
      highlightLinks.clear();
      hoverNode = n || null;
      if (n) {
        highlightNodes.add(n.id);
        (neighbors.get(n.id) || []).forEach((id) => highlightNodes.add(id));
        (nodeLinks.get(n.id) || []).forEach((l) => highlightLinks.add(l));
        showTip(n);
      } else {
        tip.hidden = true;
      }
      el.style.cursor = n ? "pointer" : "";
    })
    .onNodeClick((n) => { location.href = "/page/" + encodeURIComponent(n.id); })
    .onBackgroundClick(() => { highlightNodes.clear(); highlightLinks.clear(); });

  function fit() {
    graph.width(el.clientWidth).height(el.clientHeight);
  }
  fit();
  addEventListener("resize", fit);

  fetch("/api/graph").then((r) => r.json()).then((data) => {
    for (const l of data.links) {
      if (!neighbors.has(l.source)) neighbors.set(l.source, new Set());
      if (!neighbors.has(l.target)) neighbors.set(l.target, new Set());
      neighbors.get(l.source).add(l.target);
      neighbors.get(l.target).add(l.source);
    }
    graph.graphData(data);
    // graphData materializes link endpoints into node objects
    for (const l of graph.graphData().links) {
      const a = l.source.id ?? l.source, b = l.target.id ?? l.target;
      if (!nodeLinks.has(a)) nodeLinks.set(a, new Set());
      if (!nodeLinks.has(b)) nodeLinks.set(b, new Set());
      nodeLinks.get(a).add(l);
      nodeLinks.get(b).add(l);
    }
    graph.onEngineStop(() => graph.zoomToFit(400, 40));

    // find box: fly to the best-matching node and light up its neighborhood
    const find = document.getElementById("graphfind");
    find?.addEventListener("keydown", (e) => {
      if (e.key !== "Enter") return;
      const q = find.value.trim().toLowerCase();
      if (!q) return;
      const nodes = graph.graphData().nodes;
      const hit = nodes.find((n) => n.id === q) ||
        nodes.find((n) => n.title.toLowerCase().includes(q) || n.id.includes(q));
      if (!hit) return;
      graph.centerAt(hit.x, hit.y, 600);
      graph.zoom(4, 600);
      highlightNodes.clear();
      highlightLinks.clear();
      hoverNode = hit;
      highlightNodes.add(hit.id);
      (neighbors.get(hit.id) || []).forEach((id) => highlightNodes.add(id));
      (nodeLinks.get(hit.id) || []).forEach((l) => highlightLinks.add(l));
    });
  }).catch(() => {
    el.textContent = "그래프 데이터를 불러오지 못했습니다";
  });

  // Debug/power-user hook (also used by automated UI tests).
  window.__canopyGraph = graph;
})();
