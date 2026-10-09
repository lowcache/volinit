# volinit-m3 Noctalia Template

volinit reads its palette from Material 3 roles rendered by Noctalia, the desktop
theme engine. This template syncs the cockpit colors with your wallpaper and accent
choices.

## Installation

Copy the template to your Noctalia templates directory:

```bash
cp -r . ~/.config/noctalia/templates/volinit-m3
```

Then enable it by adding `"volinit-m3"` to the `community_ids` list under
`[theme.templates]` in `~/.local/state/noctalia/settings.toml`:

```toml
[theme.templates]
community_ids = [
  # … other templates
  "volinit-m3"
]
```

Apply it by changing the wallpaper, re-applying the active scheme, or running:

```bash
noctalia msg templates-apply
```

## Usage

Once enabled, volinit reads Material 3 neutrals, surface colors, and accents
from `~/.config/volinit/palette.toml` whenever Noctalia regenerates the theme.
The file (and its parent directory) is created automatically the first time the
template is applied.

To disable, remove `"volinit-m3"` from `community_ids`. This stops future
regeneration but does not delete the file already written — volinit will keep
using the last-generated colors until you remove
`~/.config/volinit/palette.toml` yourself, at which point it falls back to its
compiled-in default palette.
