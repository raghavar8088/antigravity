param(
  [Parameter(Mandatory = $true)]
  [string]$LightsailIp,

  [string]$User = "ubuntu",
  [string]$RemoteDir = "/home/ubuntu/claude-bot",
  [string]$KeyPath = ""
)

$ErrorActionPreference = "Stop"
$Source = $PSScriptRoot

if (-not $KeyPath) {
  $defaultKey = Join-Path (Split-Path $Source -Parent) "LightsailDefaultKey-ap-south-1.pem"
  if (Test-Path $defaultKey) {
    $KeyPath = $defaultKey
  }
}

$sshArgs = @()
$scpArgs = @()
if ($KeyPath) {
  $sshArgs += @("-i", $KeyPath, "-o", "StrictHostKeyChecking=accept-new")
  $scpArgs += @("-i", $KeyPath, "-o", "StrictHostKeyChecking=accept-new")
  Write-Host "Using SSH key: $KeyPath"
}

Write-Host "Deploying claude-bot to ${User}@${LightsailIp}:${RemoteDir}"

ssh @sshArgs "${User}@${LightsailIp}" "mkdir -p $RemoteDir"

scp @scpArgs -r `
  "$Source\package.json" `
  "$Source\send-message.js" `
  "$Source\save-auth.js" `
  "$Source\save-auth-server.js" `
  "$Source\capture-auth-cdp.js" `
  "$Source\messages.json" `
  "$Source\.env.example" `
  "$Source\setup-lightsail.sh" `
  "$Source\install-cron.sh" `
  "$Source\lib" `
  "${User}@${LightsailIp}:${RemoteDir}/"

if (Test-Path "$Source\auth.json") {
  scp @scpArgs "$Source\auth.json" "${User}@${LightsailIp}:${RemoteDir}/auth.json"
  Write-Host "Uploaded auth.json"
} else {
  Write-Host "auth.json not found locally. Run 'npm run save-auth' first, then redeploy."
}

if (Test-Path "$Source\.env") {
  scp @scpArgs "$Source\.env" "${User}@${LightsailIp}:${RemoteDir}/.env"
  Write-Host "Uploaded .env"
}

Write-Host ""
Write-Host "Files copied. On Lightsail run:"
Write-Host "  cd $RemoteDir && chmod +x setup-lightsail.sh install-cron.sh && ./setup-lightsail.sh"
Write-Host "  npm run send"
Write-Host "  ./install-cron.sh"
