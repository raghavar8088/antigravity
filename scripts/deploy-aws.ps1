# Deploy the Go engine to AWS via SSH (PowerShell / Windows OpenSSH).
# Reads connection details from scripts/aws-deploy.env, then runs
# scripts/update-aws-engine.sh on the server (git pull origin main + docker rebuild).
#
# Usage (from anywhere):
#   powershell -ExecutionPolicy Bypass -File scripts\deploy-aws.ps1
#   # or, if scripts can run:  .\scripts\deploy-aws.ps1

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot  = Split-Path -Parent $ScriptDir
$EnvFile   = Join-Path $ScriptDir "aws-deploy.env"

if (-not (Test-Path $EnvFile)) {
    Write-Error "Config not found: $EnvFile. Fill in your AWS SSH details there first."
}

# Parse the simple KEY=VALUE env file (ignores blank lines and # comments).
$cfg = @{}
foreach ($line in Get-Content $EnvFile) {
    $trimmed = $line.Trim()
    if ($trimmed -eq "" -or $trimmed.StartsWith("#")) { continue }
    $idx = $trimmed.IndexOf("=")
    if ($idx -lt 1) { continue }
    $key = $trimmed.Substring(0, $idx).Trim()
    $val = $trimmed.Substring($idx + 1).Trim()
    $cfg[$key] = $val
}

$AwsHost    = $cfg["AWS_HOST"]
$AwsUser    = if ($cfg["AWS_USER"]) { $cfg["AWS_USER"] } else { "ubuntu" }
$AwsKeyPath = $cfg["AWS_KEY_PATH"]
$RemoteDir  = if ($cfg["AWS_REMOTE_DIR"]) { $cfg["AWS_REMOTE_DIR"] } else { "~/antigravity" }
$SshPort    = if ($cfg["AWS_SSH_PORT"]) { $cfg["AWS_SSH_PORT"] } else { "22" }

if (-not $AwsHost)    { Write-Error "Set AWS_HOST in $EnvFile" }
if (-not $AwsKeyPath) { Write-Error "Set AWS_KEY_PATH in $EnvFile" }

# Resolve a relative key path from the repo root.
if (-not ([System.IO.Path]::IsPathRooted($AwsKeyPath))) {
    $AwsKeyPath = Join-Path $RepoRoot $AwsKeyPath
}
if (-not (Test-Path $AwsKeyPath)) {
    Write-Error "SSH key not found at: $AwsKeyPath"
}

# Windows OpenSSH refuses keys readable by other users. Lock the key to the
# current user (idempotent — safe to run every deploy).
$me = "$env:USERDOMAIN\$env:USERNAME"
icacls $AwsKeyPath /inheritance:r          | Out-Null
icacls $AwsKeyPath /remove "NT AUTHORITY\Authenticated Users" "BUILTIN\Users" "Everyone" 2>$null | Out-Null
icacls $AwsKeyPath /grant:r "${me}:R"      | Out-Null

Write-Host "==> Deploying engine to ${AwsUser}@${AwsHost}:${SshPort} (${RemoteDir})" -ForegroundColor Cyan

ssh -i "$AwsKeyPath" -p "$SshPort" `
    -o StrictHostKeyChecking=accept-new `
    "${AwsUser}@${AwsHost}" `
    "cd $RemoteDir && bash scripts/update-aws-engine.sh"

if ($LASTEXITCODE -ne 0) {
    Write-Error "Deploy failed (ssh exit code $LASTEXITCODE)."
}

Write-Host "==> Done." -ForegroundColor Green
