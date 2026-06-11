param(
  [string]$Hour = "16",
  [string]$Minute = "00"
)

$BotDir = $PSScriptRoot
$Node = (Get-Command node).Source
$TaskName = "ClaudeDailyMessage"

$Action = New-ScheduledTaskAction -Execute $Node -Argument "`"$BotDir\send-message.js`"" -WorkingDirectory $BotDir
$Trigger = New-ScheduledTaskTrigger -Daily -At "$Hour`:$Minute"
$Settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -AllowStartIfOnBatteries

Register-ScheduledTask -TaskName $TaskName -Action $Action -Trigger $Trigger -Settings $Settings -Force | Out-Null

Write-Host "Scheduled task '$TaskName' created."
Write-Host "Runs daily at ${Hour}:${Minute} (laptop local time)."
Write-Host "Requires laptop ON at that time."
Write-Host "Uses your local auth.json (already working)."
Write-Host ""
Write-Host "Test now: cd `"$BotDir`"; npm run send"
