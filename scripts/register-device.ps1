<#
.SYNOPSIS
  Interactive helper to register a ZKTeco device with the gateway via its API.
  Windows counterpart of scripts/register-device.sh.

.DESCRIPTION
  You need the HR device credential first: an HR admin creates it with
    POST /hrm/devices        (on the ERP HR service)
  which returns a deviceIdentifier (X-Device-Id) and a one-time apiKey
  (X-Device-Key). Paste those when prompted.

.EXAMPLE
  .\scripts\register-device.ps1
#>
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'

function Ask([string]$Prompt, [string]$Default) {
    $suffix = if ($Default) { " [$Default]" } else { '' }
    $value = Read-Host "$Prompt$suffix"
    if ([string]::IsNullOrWhiteSpace($value)) { return $Default }
    return $value.Trim()
}

$base   = Ask 'Gateway base URL' 'http://localhost:8080'
$apiKey = Ask 'Management API key (MGMT_API_KEY)'
$name   = Ask 'Device name (e.g. Main Gate)'
$mode   = Ask 'Mode [both/pull/push]' 'both'
$ip     = Ask 'Device IP (for pull, blank to skip)'
$sn     = Ask 'Device serial number (for push, blank to skip)'
$port   = [int](Ask 'Device TCP port' '4370')
$hrId   = Ask 'HR device id (X-Device-Id)'

# Read the HR key without echoing it, then hand the plaintext straight to the
# request body. The gateway encrypts it at rest (AES-256-GCM) on arrival.
$hrKeySecure = Read-Host 'HR device key (X-Device-Key, hidden)' -AsSecureString
$hrKey = [System.Net.NetworkCredential]::new('', $hrKeySecure).Password

$payload = @{
    name         = $name
    mode         = $mode
    ipAddress    = $ip
    serialNumber = $sn
    port         = $port
    hrDeviceId   = $hrId
    hrDeviceKey  = $hrKey
} | ConvertTo-Json

Write-Host 'Registering...'
try {
    $response = Invoke-RestMethod -Method Post -Uri "$base/devices" `
        -Headers @{ 'X-API-Key' = $apiKey } `
        -ContentType 'application/json' `
        -Body $payload
    $response | ConvertTo-Json -Depth 6
} catch {
    Write-Host "Registration failed: $($_.Exception.Message)" -ForegroundColor Red
    # Surface the gateway's own error body, which carries the useful message.
    if ($_.ErrorDetails.Message) { Write-Host $_.ErrorDetails.Message }
    exit 1
} finally {
    $hrKey = $null
    [System.GC]::Collect()
}
