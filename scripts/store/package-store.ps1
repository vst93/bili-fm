param(
  [Parameter(Mandatory = $true)][string]$Version,
  [string]$IdentityName = "vvv.bilifm",
  [string]$Publisher = "CN=21D233E1-6650-4700-A43E-EDA723C43F96",
  [string]$PublisherDisplayName = "vvv_",
  [string]$DisplayName = "bili-FM",
  [string]$Description = "bili-FM",
  [string]$ExeDir = "app/build/windows-amd64",
  [string]$Icon = "app/resources/icon.png"
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Drawing

$workspace = if ($env:GITHUB_WORKSPACE) { $env:GITHUB_WORKSPACE } else { (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path }
$buildRoot = Join-Path $workspace "build"
$packageRoot = Join-Path $buildRoot "msix"
$assetsDir = Join-Path $packageRoot "Assets"
$binDir = Join-Path $workspace $ExeDir
$exeName = "$DisplayName.exe"
$exePath = Join-Path $binDir $exeName
$templatePath = Join-Path $PSScriptRoot "AppxManifest.xml.template"
$iconPath = Join-Path $workspace $Icon
$manifestPath = Join-Path $packageRoot "AppxManifest.xml"
$msixPath = Join-Path $buildRoot "msix/$DisplayName-windows-amd64.msix"

# Microsoft Store requires a.b.c.d, all numeric. GitHub tags may be 3.1.0 or
# 3.1.0-preview.x; Store packages must use a stable 3.1.0.0.
if ($Version -match '^(\d+)\.(\d+)\.(\d+)$') {
  $Version = "$Version.0"
}
if ($Version -notmatch '^\d+\.\d+\.\d+\.\d+$') {
  throw "Store version must be numeric a.b.c.d (no -preview suffix): $Version"
}

if (-not (Test-Path $exePath)) { throw "Windows executable not found: $exePath" }
if (-not (Test-Path $templatePath)) { throw "Manifest template not found: $templatePath" }
if (-not (Test-Path $iconPath)) { throw "Store icon not found: $iconPath" }

Remove-Item $packageRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $packageRoot, $assetsDir | Out-Null

# mygo build's output directory contains the exe, its embedded resources, and
# the NSIS setup produced by the same build. The Store MSIX must **not** contain
# the NSIS installer (Store policy and size): copy the exe and its non-exe
# resources only.
Copy-Item $exePath $packageRoot -Force
Get-ChildItem $binDir -File | Where-Object { $_.Name -ne $exeName -and $_.Extension -ne ".exe" } |
  Copy-Item -Destination $packageRoot -Force

function New-SquarePng {
  param([string]$Source, [string]$Destination, [int]$Width, [int]$Height = 0)
  if ($Height -le 0) { $Height = $Width }
  $sourceImage = [System.Drawing.Image]::FromFile($Source)
  try {
    $bitmap = New-Object System.Drawing.Bitmap $Width, $Height
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    try {
      $graphics.Clear([System.Drawing.Color]::Transparent)
      $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
      $graphics.DrawImage($sourceImage, 0, 0, $Width, $Height)
      $bitmap.Save($Destination, [System.Drawing.Imaging.ImageFormat]::Png)
    } finally {
      $graphics.Dispose()
      $bitmap.Dispose()
    }
  } finally {
    $sourceImage.Dispose()
  }
}

New-SquarePng -Source $iconPath -Destination (Join-Path $assetsDir "StoreLogo.png") -Width 50
New-SquarePng -Source $iconPath -Destination (Join-Path $assetsDir "Square44x44Logo.png") -Width 44
New-SquarePng -Source $iconPath -Destination (Join-Path $assetsDir "Square150x150Logo.png") -Width 150
New-SquarePng -Source $iconPath -Destination (Join-Path $assetsDir "Wide310x150Logo.png") -Width 310 -Height 150
New-SquarePng -Source $iconPath -Destination (Join-Path $assetsDir "Square310x310Logo.png") -Width 310

$manifest = Get-Content $templatePath -Raw
$manifest = $manifest.Replace("{{IDENTITY_NAME}}", $IdentityName)
$manifest = $manifest.Replace("{{PUBLISHER}}", $Publisher)
$manifest = $manifest.Replace("{{VERSION}}", $Version)
$manifest = $manifest.Replace("{{DISPLAY_NAME}}", $DisplayName)
$manifest = $manifest.Replace("{{PUBLISHER_DISPLAY_NAME}}", $PublisherDisplayName)
$manifest = $manifest.Replace("{{DESCRIPTION}}", $Description)
$manifest = $manifest.Replace("{{EXECUTABLE_NAME}}", $exeName)
Set-Content -Path $manifestPath -Value $manifest -Encoding utf8

$makeappx = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin" -Recurse -Filter makeappx.exe |
  Sort-Object FullName -Descending | Select-Object -First 1 -ExpandProperty FullName
if (-not $makeappx) { throw "makeappx.exe not found (Windows SDK)" }

& $makeappx pack /d $packageRoot /p $msixPath /o
if ($LASTEXITCODE -ne 0) { throw "makeappx pack failed" }
Write-Host "Created $msixPath (version $Version)"
