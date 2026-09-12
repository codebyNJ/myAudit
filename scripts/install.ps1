# Install myAudit desktop app from the latest GitHub Release (Windows).
# Usage: irm https://raw.githubusercontent.com/codebyNJ/myAudit/main/scripts/install.ps1 | iex
$ErrorActionPreference = "Stop"

$Repo = if ($env:MYAUDIT_INSTALL_REPO) { $env:MYAUDIT_INSTALL_REPO } else { "codebyNJ/myAudit" }
$Version = if ($env:MYAUDIT_VERSION) { $env:MYAUDIT_VERSION } else { "latest" }
$Api = "https://api.github.com/repos/$Repo/releases/$Version"

Write-Host "==> fetching release metadata from $Repo"
$Release = Invoke-RestMethod -Uri $Api -Headers @{ "Accept" = "application/vnd.github+json" }

$Asset = $Release.assets | Where-Object { $_.name -like "myAudit-*-Windows-x86_64-setup.exe" } | Select-Object -First 1
if (-not $Asset) {
    $Asset = $Release.assets | Where-Object { $_.name -like "myAudit-*-Windows-x86_64.msi" } | Select-Object -First 1
}
if (-not $Asset) {
    Write-Error "no Windows installer found — see https://github.com/$Repo/releases"
}

$Dest = Join-Path $env:TEMP $Asset.name
Write-Host "==> downloading $($Asset.name)"
Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $Dest

Write-Host "==> launching installer"
Start-Process -FilePath $Dest -Wait
Write-Host "==> done"
