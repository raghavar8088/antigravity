# Re-clone / refresh all external integration repos as shallow clones.
# Usage: pwsh integrations/sync.ps1   (run from repo root)
$ErrorActionPreference = "Stop"
$root = Join-Path $PSScriptRoot ""

$repos = @(
    @{ Path = "01-exchange-connectivity/ccxt";                 Url = "https://github.com/ccxt/ccxt.git" },
    @{ Path = "02-fix-protocol/quickfix-go";                   Url = "https://github.com/quickfixgo/quickfix.git" },
    @{ Path = "03-indicators-go/indicator";                    Url = "https://github.com/cinar/indicator.git" },
    @{ Path = "04-backtest-execution-engine/nautilus_trader";  Url = "https://github.com/nautechsystems/nautilus_trader.git" },
    @{ Path = "05-quant-ml-research/qlib";                      Url = "https://github.com/microsoft/qlib.git" },
    @{ Path = "05-quant-ml-research/finrl-trading";            Url = "https://github.com/AI4Finance-Foundation/FinRL-Trading.git" },
    @{ Path = "06-timeseries-db/questdb";                      Url = "https://github.com/questdb/questdb.git" },
    @{ Path = "07-realtime-messaging/centrifuge";              Url = "https://github.com/centrifugal/centrifuge.git" }
)

foreach ($r in $repos) {
    $dest = Join-Path $root $r.Path
    if (Test-Path (Join-Path $dest ".git")) {
        Write-Host "Updating $($r.Path)..." -ForegroundColor Cyan
        git -C $dest pull --depth 1 --ff-only
    } else {
        Write-Host "Cloning $($r.Path)..." -ForegroundColor Green
        git clone --depth 1 $r.Url $dest
    }
}
Write-Host "Done." -ForegroundColor Green
