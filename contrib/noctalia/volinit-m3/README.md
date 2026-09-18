# volinit-m3 Noctalia Template

volinit reads its palette from Material 3 roles rendered by Noctalia, the desktop
theme engine. This template syncs the cockpit colors with your wallpaper and accent
choices.

## Installation

Copy the template to your Noctalia templates directory:

```bash
cp -r . ~/.config/noctalia/templates/volinit-m3
```

Then enable the template in `~/.local/state/noctalia/settings.toml`:

```toml
theme.templates.enabled = [
  # … other templates
  "volinit-m3"
]
```

Apply the template by changing the wallpaper or re-applying the active scheme:

```bash
noctalia apply
```

## Usage

Once enabled, volinit will read Material 3 neutrals, surface colors, and accents
from `~/.config/volinit/palette.toml` whenever noctalia regenerates the theme.
The file is auto-created on first apply. Disable the template to revert to the
compiled-in `chip-green` defaults.
