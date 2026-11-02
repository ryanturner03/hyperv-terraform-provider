<#
.SYNOPSIS
  Install a released terraform-provider-hyperv binary into Terraform's implied
  local plugin mirror on Windows so `terraform init` finds it without CLI config.

.EXAMPLE
  .\scripts\install.ps1                  # latest release
  .\scripts\install.ps1 -Version v0.2.0  # specific release

  # one-liner
  irm https://raw.githubusercontent.com/ryanturner03/hyperv-terraform-provider/main/scripts/install.ps1 | iex; Install-HypervProvider
#>
function Install-HypervProvider {
    [CmdletBinding()]
    param(
        [string]$Version,
        [string]$Repo = 'ryanturner03/hyperv-terraform-provider',
        [string]$PluginDir = (Join-Path $env:APPDATA 'terraform.d\plugins')
    )
    $ErrorActionPreference = 'Stop'
    $providerAddr = 'registry.terraform.io/ryan/hyperv'

    if (-not $Version) {
        $Version = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest").tag_name
    }
    $ver = $Version.TrimStart('v')

    $arch = switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { 'amd64' }
        'ARM64' { 'arm64' }
        default { throw "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
    }

    $base = "https://github.com/$Repo/releases/download/v$ver"
    $zip  = "terraform-provider-hyperv_${ver}_windows_${arch}.zip"
    $sums = "terraform-provider-hyperv_${ver}_SHA256SUMS"

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Downloading $zip"
        Invoke-WebRequest -Uri "$base/$zip"  -OutFile (Join-Path $tmp $zip)
        Invoke-WebRequest -Uri "$base/$sums" -OutFile (Join-Path $tmp $sums)

        Write-Host 'Verifying checksum'
        $line = Get-Content (Join-Path $tmp $sums) | Where-Object { $_ -match "\s$([regex]::Escape($zip))$" }
        if (-not $line) { throw "$zip not listed in $sums" }
        $expected = ($line -split '\s+')[0].ToLower()
        $actual = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $zip)).Hash.ToLower()
        if ($expected -ne $actual) { throw "Checksum mismatch for $zip" }

        $dest = Join-Path $PluginDir "$providerAddr\$ver\windows_$arch"
        New-Item -ItemType Directory -Path $dest -Force | Out-Null
        Expand-Archive -Path (Join-Path $tmp $zip) -DestinationPath $dest -Force

        Write-Host @"
Installed terraform-provider-hyperv v$ver to
  $dest

Use it with:

  terraform {
    required_providers {
      hyperv = {
        source  = "ryan/hyperv"
        version = "$ver"
      }
    }
  }

Then run: terraform init
"@
    }
    finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
}

# Run immediately when invoked as a script (not dot-sourced / piped to iex).
if ($MyInvocation.InvocationName -ne '.' -and $MyInvocation.MyCommand.Path) {
    Install-HypervProvider @args
}
