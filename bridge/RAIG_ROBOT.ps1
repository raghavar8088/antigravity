# RAIG_ROBOT.ps1 - Automated Connection Helper
$ErrorActionPreference = "SilentlyContinue"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host "`n[RAIG] INITIALIZING ROBOT CONNECTION PROTOCOLS..." -ForegroundColor Cyan

# 1. Kill potentially stuck chrome processes
Write-Host "[!] Closing existing Chrome instances..." -ForegroundColor Yellow
taskkill /F /IM chrome.exe /T 2>$null
Start-Sleep -Seconds 2

# 2. Start Chrome with correct flag and a Dedicated Robot Profile
Write-Host "[+] Launching Chrome in 'Bulletproof Robot Mode'..." -ForegroundColor Green
$chromePath = "C:\Program Files\Google\Chrome\Application\chrome.exe"
if (-not (Test-Path $chromePath)) {
    $chromePath = "$env:LOCALAPPDATA\Google\Chrome\Application\chrome.exe"
}

# Use a temporary profile to ensure the debugging port ALWAYS opens
$tempProfile = "$env:TEMP\raig_robot_profile"
Start-Process $chromePath -ArgumentList "--remote-debugging-port=9222", "--user-data-dir=$tempProfile", "--no-first-run"

Write-Host "`nSTEP 1: In the new window, go to https://chatgpt.com" -ForegroundColor White
Write-Host "STEP 2: Log in (You only need to do this once for this profile)." -ForegroundColor White
Write-Host "`n[?] Press any key once you are ready, and I will link the Robot..." -ForegroundColor Cyan
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")

# 3. Start the bridge with Auto-Detection
Write-Host "`n[SCAN] RAIG: SCANNING FOR ACTIVE ENGINE..." -ForegroundColor Cyan

$localUrl = "http://localhost:8080/health"
$cloudUrl = "https://antigravity-x7he.onrender.com"

try {
    Invoke-WebRequest -Uri $localUrl -TimeoutSec 2 -UseBasicParsing > $null
    Write-Host "[OK] LOCAL ENGINE DETECTED! Linking Robot to your computer..." -ForegroundColor Green
    $env:ENGINE_URL = "http://localhost:8080"
} catch {
    Write-Host "[NW] LOCAL ENGINE NOT FOUND. Linking Robot to CLOUD (Render)..." -ForegroundColor Yellow
    $env:ENGINE_URL = $cloudUrl
}

Push-Location $scriptDir
try {
    node .\bridge.js
} finally {
    Pop-Location
}
