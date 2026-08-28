<#
.SYNOPSIS
  ZKTeco biometric gateway - operator shortcuts for Windows. The PowerShell
  counterpart of the Makefile; run it from the repo root.

.DESCRIPTION
  Two deployment paths are supported on Windows, and this script drives both:

  Docker Desktop (same stack as the Raspberry Pi - gateway + web console +
  MariaDB + phpMyAdmin):
      .\make.ps1 init          Scaffold .env and env\*.env from the .example files
      .\make.ps1 preflight     Sanity-check config before starting
      .\make.ps1 up            Build and start the whole stack (detached)
      .\make.ps1 down          Stop the stack (data volumes are preserved)
      .\make.ps1 restart       Restart the gateway container only
      .\make.ps1 build         Rebuild the gateway image
      .\make.ps1 logs          Follow gateway logs   (-Service mariadb to pick another)
      .\make.ps1 ps            Container status

  Native Windows Service (no Docker; bring your own MariaDB):
      .\make.ps1 native-build      Compile dist\windows\zkbio.exe + scaffold its .env
      .\make.ps1 native-preflight  Sanity-check dist\windows\.env
      .\make.ps1 service-install   Register the Windows Service (needs elevation)
      .\make.ps1 service-start     Start it
      .\make.ps1 service-status    Query it
      .\make.ps1 service-stop      Stop it
      .\make.ps1 service-uninstall Remove it (needs elevation)

  Either path:
      .\make.ps1 register-device   Interactive: register a device via the API

.EXAMPLE
  .\make.ps1 up

.EXAMPLE
  .\make.ps1 native-build -Arch arm64
#>
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet('help', 'init', 'preflight', 'up', 'down', 'restart', 'build', 'logs', 'ps', 'status',
        'register-device', 'native-build', 'native-preflight',
        'service-install', 'service-uninstall', 'service-start', 'service-stop', 'service-status')]
    [string]$Target = 'help',

    # logs: which compose service to follow.
    [string]$Service = 'zkbio',

    # native-build: target architecture. amd64 for ordinary PCs, arm64 for
    # Windows-on-ARM machines.
    [ValidateSet('amd64', 'arm64')]
    [string]$Arch = 'amd64',

    # service-install: services the gateway must wait for, e.g. a local MariaDB.
    [string]$DependsOn = ''
)

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

$DistDir = Join-Path $PSScriptRoot 'dist\windows'
$ExePath = Join-Path $DistDir 'zkbio.exe'

# Args are passed as an explicit array: a function parameter would swallow
# docker's own switches (-d, -f) as PowerShell parameter names.
function Invoke-Compose([string[]]$ComposeArgs) {
    & docker compose @ComposeArgs
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose $($ComposeArgs -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Require-Exe {
    if (-not (Test-Path $ExePath)) {
        throw "$ExePath not found - run '.\make.ps1 native-build' first."
    }
}

# The service subcommands talk to the Service Control Manager, which needs
# elevation for install/uninstall. Fail with a clear message rather than a raw
# "Access is denied" from deep inside the binary.
function Require-Elevation {
    $identity  = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw 'This target changes the Windows Service registration - run PowerShell as Administrator.'
    }
}

function Copy-IfMissing($from, $to) {
    if (-not (Test-Path $to)) {
        Copy-Item $from $to
        Write-Host "  created $to"
    }
}

switch ($Target) {

    'help' { Get-Help $PSCommandPath -Detailed; break }

    'init' {
        Copy-IfMissing '.env.example'                 '.env'
        Copy-IfMissing 'env\zkbio.env.example'        'env\zkbio.env'
        Copy-IfMissing 'env\db.env.example'           'env\db.env'
        Copy-IfMissing 'env\phpmyadmin.env.example'   'env\phpmyadmin.env'
        Write-Host 'Copied .env and env\*.env - now edit them and replace every CHANGEME value.'
        break
    }

    'preflight' { & (Join-Path $PSScriptRoot 'scripts\preflight.ps1'); break }

    'up' {
        & (Join-Path $PSScriptRoot 'scripts\preflight.ps1')
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        Invoke-Compose @('up', '-d', '--build')
        # Ports come from .env (compose reads it itself; PowerShell does not).
        $ports = @{ WEBUI_PORT = '8082'; ZKBIO_PORT = '8080'; PHPMYADMIN_PORT = '8081' }
        if (Test-Path '.env') {
            foreach ($line in Get-Content '.env') {
                if ($line -match '^\s*(WEBUI_PORT|ZKBIO_PORT|PHPMYADMIN_PORT)\s*=\s*(\S+)') {
                    $ports[$Matches[1]] = $Matches[2]
                }
            }
        }
        Write-Host "Web console: http://localhost:$($ports.WEBUI_PORT)"
        Write-Host "Gateway:     http://localhost:$($ports.ZKBIO_PORT)"
        Write-Host "phpMyAdmin:  http://localhost:$($ports.PHPMYADMIN_PORT)"
        break
    }

    'down'    { Invoke-Compose @('down'); break }
    'restart' { Invoke-Compose @('restart', 'zkbio'); break }
    'build'   { Invoke-Compose @('build', 'zkbio'); break }
    'logs'    { Invoke-Compose @('logs', '-f', $Service); break }
    { $_ -in 'ps', 'status' } { Invoke-Compose @('ps'); break }

    'register-device' { & (Join-Path $PSScriptRoot 'scripts\register-device.ps1'); break }

    'native-build' {
        if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
            throw 'Go toolchain not found on PATH - install Go 1.25+ from https://go.dev/dl/'
        }
        New-Item -ItemType Directory -Force -Path $DistDir | Out-Null
        # The service chdirs to the executable's directory on start, so .env and
        # the relative LOG_DIR both resolve from dist\windows.
        New-Item -ItemType Directory -Force -Path (Join-Path $DistDir 'logs') | Out-Null

        Write-Host "Building zkbio.exe (windows/$Arch)..."
        Push-Location (Join-Path $PSScriptRoot 'src')
        try {
            $env:GOOS = 'windows'; $env:GOARCH = $Arch; $env:CGO_ENABLED = '0'
            & go build -trimpath -ldflags '-s -w' -o $ExePath .
            if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
        } finally {
            Pop-Location
            Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
        }

        Copy-Item 'windows\zkbio.env.example' (Join-Path $DistDir '.env.example') -Force
        Copy-IfMissing (Join-Path $DistDir '.env.example') (Join-Path $DistDir '.env')

        Write-Host ""
        Write-Host "Built $ExePath"
        Write-Host "Next: edit $(Join-Path $DistDir '.env'), then run (as Administrator):"
        Write-Host "  .\make.ps1 native-preflight"
        Write-Host "  .\make.ps1 service-install"
        break
    }

    'native-preflight' {
        & (Join-Path $PSScriptRoot 'scripts\preflight.ps1') -EnvFile (Join-Path $DistDir '.env')
        exit $LASTEXITCODE
    }

    'service-install' {
        Require-Exe; Require-Elevation
        & (Join-Path $PSScriptRoot 'scripts\preflight.ps1') -EnvFile (Join-Path $DistDir '.env')
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        if ($DependsOn) { & $ExePath service install -depends $DependsOn }
        else            { & $ExePath service install }
        exit $LASTEXITCODE
    }

    'service-uninstall' { Require-Exe; Require-Elevation; & $ExePath service uninstall; exit $LASTEXITCODE }
    'service-start'     { Require-Exe; & $ExePath service start;  exit $LASTEXITCODE }
    'service-stop'      { Require-Exe; & $ExePath service stop;   exit $LASTEXITCODE }
    'service-status'    { Require-Exe; & $ExePath service status; exit $LASTEXITCODE }
}
