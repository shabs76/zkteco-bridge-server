<#
.SYNOPSIS
  Fail fast on obvious misconfiguration before starting the gateway. Windows
  counterpart of scripts/preflight.sh.

.DESCRIPTION
  Two modes, matching the two ways the gateway is deployed on Windows:

    .\scripts\preflight.ps1
        Docker Desktop stack — checks .env + env\*.env, the same rules as the
        bash version.

    .\scripts\preflight.ps1 -EnvFile dist\windows\.env
        Native service install — checks the single .env that sits next to
        zkbio.exe.

.EXAMPLE
  .\scripts\preflight.ps1
#>
[CmdletBinding()]
param(
    # Check one flat .env (native install) instead of the compose env\ layout.
    [string]$EnvFile
)

$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

$script:Failed = $false
function Note($msg) { Write-Host "  - $msg" }
function Fail($msg) { Note $msg; $script:Failed = $true }

# Reads KEY=VALUE out of a .env, ignoring comments. Returns $null if absent.
function Get-EnvValue([string]$Path, [string]$Key) {
    if (-not (Test-Path $Path)) { return $null }
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -match "^\s*$([regex]::Escape($Key))\s*=(.*)$") {
            return $Matches[1].Trim()
        }
    }
    return $null
}

Write-Host 'Preflight checks:'

if ($EnvFile) {
    # ── Native install ──────────────────────────────────────────────────────
    if (-not (Test-Path $EnvFile)) {
        Fail "MISSING $EnvFile  (copy windows\zkbio.env.example next to zkbio.exe as .env)"
        Write-Host 'Preflight failed.'; exit 1
    }
    $files = @($EnvFile)
    $dbPassKey = $null   # native uses an externally-managed MariaDB
} else {
    # ── Docker stack ────────────────────────────────────────────────────────
    foreach ($f in @('.env', 'env\zkbio.env', 'env\db.env', 'env\phpmyadmin.env')) {
        if (-not (Test-Path $f)) { Fail "MISSING $f  (run '.\make.ps1 init' first)" }
    }
    if ($script:Failed) { Write-Host 'Preflight failed.'; exit 1 }
    $files = @('.env') + (Get-ChildItem 'env\*.env' | ForEach-Object { $_.FullName })
    $EnvFile = 'env\zkbio.env'
    $dbPassKey = 'ERP_ZKBIO_DB_PASSWORD'
}

# No leftover CHANGEME placeholders. Only the real config files are scanned —
# the *.env.example templates keep their CHANGEME placeholders by design.
$withPlaceholders = $files | Where-Object { Select-String -LiteralPath $_ -Pattern 'CHANGEME' -Quiet }
if ($withPlaceholders) {
    Fail 'CHANGEME placeholders still present - set real values:'
    $withPlaceholders | ForEach-Object { Write-Host "      $_" }
}

# Passwords must match between the two files (docker stack only).
if ($dbPassKey) {
    $zkPass = Get-EnvValue 'env\zkbio.env' 'MYSQL_PASS'
    $dbPass = Get-EnvValue 'env\db.env'    $dbPassKey
    if ($zkPass -and $zkPass -ne $dbPass) {
        Fail "MYSQL_PASS (env\zkbio.env) does not match $dbPassKey (env\db.env)"
    }
}

# HR ingest URL sanity.
$hrUrl = Get-EnvValue $EnvFile 'HR_INGEST_URL'
if ($hrUrl -notmatch '^https?://') {
    Fail "HR_INGEST_URL looks unset or malformed: '$hrUrl'"
} elseif ($hrUrl -notmatch '/device/attendance/logs$') {
    Note "HR_INGEST_URL should end in /device/attendance/logs (got '$hrUrl')"
}

# Encryption key presence.
if (-not (Get-EnvValue $EnvFile 'CRED_ENC_KEY')) {
    Note 'CRED_ENC_KEY is empty - HR device keys will NOT be encrypted at rest'
}

# Timezone must be an IANA name, not a Windows one. The gateway embeds the IANA
# tz database, so "Africa/Dar_es_Salaam" works on Windows - but "E. Africa
# Standard Time" does not, and would silently fall back to UTC.
$tz = Get-EnvValue $EnvFile 'TZ'
if ($tz -and $tz -match '\s') {
    Fail "TZ must be an IANA zone name like 'Africa/Dar_es_Salaam', not a Windows one (got '$tz')"
}

if ($script:Failed) { Write-Host 'Preflight failed.'; exit 1 }
Write-Host 'Preflight OK.'
# Explicit: callers gate on $LASTEXITCODE, which would otherwise hold a stale
# value from whatever native command ran last.
exit 0
