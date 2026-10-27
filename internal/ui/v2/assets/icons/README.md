# Paw Lucide icon assets

These PNGs are generated from official [Lucide Icons](https://github.com/lucide-icons/lucide) SVGs at release `1.54.0`. The SVG path geometry is preserved; only the stroke color is changed before rendering.

- PNG: 64 x 64 pixels, 8-bit RGBA, transparent background.
- Renderer: [resvg](https://github.com/linebender/resvg) 0.48.1 with geometric precision.
- Lossless PNG optimization: oxipng 10.2.1.
- Light stroke: #565656.
- Dark stroke: #F3F3F3.
- Active stroke: #0B8FF3.
- Danger stroke: #E5484D.
Regenerate from the repository root with the mise run generate-icons task. To choose another Lucide release, pass --version TAG to internal/ui/v2/assets/icons/generate-icons.sh.

The Lucide license and copyright notices are preserved in [LICENSE](LICENSE).
