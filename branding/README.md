# L5S1 brand assets

Primary mark: three soft teal “vertebra / disc” forms — calm clinical health product identity for lumbar-focused tracking (L5–S1) without clinical gore.

## Files

| File | Use |
|------|-----|
| `logo-mark.png` / `logo-mark-512.png` | Full-resolution mark on light tile |
| `app-icon.png` | Teal rounded app icon (PWA / homescreen) — transparent outside the squircle |
| `frontend/assets/brand/favicon.svg` (+ `favicon-32.png`, `/favicon.ico`) | Tab favicon: mark on **transparent** background (no white tile) |
| `logo-lockup.png` / `logo-lockup-readme.png` | App icon + **L5S1 Health Registry** wordmark (transparent PNG for GitHub README) |
| `logo-mark.svg` / `logo-lockup.svg` | Vector versions (crisp at any size; no baked background) |
| `logo-monogram-alt.jpg` | Alternate LS monogram exploration (not primary) |

App-served copies live under `frontend/assets/brand/`.

## Usage

**GitHub README** — use the transparent lockup (avoid the old square white tile):

```markdown
<p align="center">
  <img src="branding/logo-lockup-readme.png" alt="L5S1 Health Registry" width="480" />
</p>
```

Vector: `logo-lockup.svg`.

**HTML / PWA**

- Icons: `/assets/brand/app-icon-192.png`, `app-icon-512.png`
- Inline header: `/assets/brand/app-icon-192.png` or SVG mark

## Do / don’t

- Prefer teal clinical palette (`#0d9488` / `#14b8a6`) with the disc stack mark  
- Keep ample padding on app icons  
- Don’t recolor the mark to warning red or harsh clinical red for the default brand  
