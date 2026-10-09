# Third-party web assets

Canopy embeds these upstream JavaScript distributions. Their licenses apply to
those assets separately from canopy's license; provenance hashes do not replace
license text or attribution.

| Asset | Pinned release | License and retained attribution |
| --- | --- | --- |
| force-graph | [1.51.4](https://github.com/vasturiano/force-graph/tree/v1.51.4) | [MIT license](internal/webui/static/vendor/LICENSE-force-graph.txt), Copyright (c) 2018 Vasco Asturiano; [redistribution note](internal/webui/static/vendor/NOTICE-force-graph.txt) |
| MathJax tex-svg, color, physics | [3.2.2](https://github.com/mathjax/MathJax/tree/3.2.2) | [Apache License 2.0](internal/webui/static/vendor/LICENSE-mathjax.txt), Copyright (c) 2017-2022 The MathJax Consortium; [redistribution note](internal/webui/static/vendor/NOTICE-mathjax.txt) |

The [asset manifest](internal/webui/static/vendor/assets.json) records exact
source URLs, bytes, processing, hashes, and the declared TeX package set. The
existing tex-svg and force-graph bundles match their pinned upstream bytes.
Color and physics keep upstream bytes under local filenames. Canopy's MathJax
configuration supports `base`, `ams`, `newcommand`, `color`, and `physics`;
`autoload`, `require`, remote extensions, and the optional MathJax menu are
disabled. This is a bounded TeX corpus, not full LaTeX support.

GoReleaser archives carry this file and the license/redistribution notes under
`internal/webui/static/vendor/`. The Homebrew source formula installs the same
texts under `pkgshare` with the same relative paths, alongside the manifest and
font license. Run `python3 scripts/check-assets.py` to check
source bytes and packaging declarations, and append `--archive PATH` to inspect
an actual `.tar.gz` or `.zip` archive. The latter verifies the texts inside the
archive; checking the source declaration alone does not verify a release.

This manifest covers the two selected JavaScript vendors, not a complete SBOM or
vulnerability audit. Existing font terms remain in
[LICENSE-fonts.txt](internal/webui/static/vendor/fonts/LICENSE-fonts.txt);
Mermaid's separately embedded bundle retains its upstream license banner in
`internal/mermaid/mermaid.min.js`.
