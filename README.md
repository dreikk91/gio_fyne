# CID Retranslator (Go + Gio)

Новий окремий проєкт-порт з C# Avalonia на **Go + Gio** з новим desktop UI для Windows.

## Що перенесено

- TCP server для вхідних CID (`ACK/NACK`, heartbeat, валідація, трансформація account/code).
- Relay TCP client з reconnect/backoff та метриками доставки.
- SQLite repository (devices/events/catalog/search/filter/history).
- Runtime з неблокуючими чергами, batched UI drain і захистом від перевантаження UI.
- Tabs UI: `Objects`, `Events`, `Settings`.
- Structured `Settings` sections with configurable UI font size (`UI.FontSize`, range `7..30`).
- CID account remap supports multiple configurable ranges (`CidRules.AccountRanges`).
- Device Event Journal (історія по вибраному об'єкту).
- Windows tray integration with app icon (show/hide/exit from tray).
- `X` button behavior depends on settings:
  - `UI.CloseToTray: true` -> hide to tray.
  - `UI.CloseToTray: false` -> normal process exit.
  - `UI.MinimizeToTray: true` -> minimize hides to tray.
- Load generator `cmd/cidloadgen` для stress/high-throughput сценаріїв.

## Продуктивність

Гарячий шлях прийому подій відокремлений від UI:

- вхідні події йдуть у буферизовані канали;
- UI оновлюється batched-партіями (`uiDrainBatchSize`) по таймеру;
- списки віртуалізовані через `widget.List` (Gio), щоб UI не деградував при великій кількості подій.

Це дозволяє приймати до **30k msg/s** без блокування UI (залежить від CPU/мережі/диска та режиму ACK).

## Запуск

```bash
go mod tidy
go run ./cmd/cidgio
```

Альтернативний entrypoint (сумісність):

```bash
go run ./cmd/cidwindigo
```

## Stress / Throughput тест

30k msg/s без очікування ACK:

```bash
go run ./cmd/cidloadgen --addr 127.0.0.1:20005 --rate 30000 --workers 24 --duration 30s --wait-reply=false
```

Режим з ACK/NACK перевіркою:

```bash
go run ./cmd/cidloadgen --addr 127.0.0.1:20005 --rate 5000 --workers 16 --duration 30s --wait-reply=true
```

## Build release (Windows)

```bash
go build -ldflags "-s -w -H=windowsgui" -o cidgio.exe ./cmd/cidgio
```

`rsrc_windows_amd64.syso` and `rsrc_windows_386.syso` are included, so the compiled `.exe` gets the app icon from `icon.ico`.

## CID Account Ranges

In `Settings -> CID Rules`, configure ranges using one rule per line:

```text
2000-2200:+2100
3000-3099:-100
```

Format: `From-To:Delta`
