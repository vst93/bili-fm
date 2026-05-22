param(
  [string]$Version,
  [string]$IdentityName = "biliFM",
  [string]$Publisher = "CN=vst",
  [string]$PublisherDisplayName = "vst",
  [string]$DisplayName = "bili-FM",
  [string]$Description = "bili-FM"
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Drawing

$workspace = if ($env:GITHUB_WORKSPACE) { $env:GITHUB_WORKSPACE } else { (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path }
$buildRoot = Join-Path $workspace "build"
$packageRoot = Join-Path $buildRoot "msix"
$assetsDir = Join-Path $packageRoot "Assets"
$binDir = Join-Path $buildRoot "bin"
$exeName = "$DisplayName.exe"
$exePath = Join-Path $binDir $exeName
$templatePath = Join-Path $PSScriptRoot "AppxManifest.xml.template"
$manifestPath = Join-Path $packageRoot "AppxManifest.xml"
$sourceIcon = Join-Path $buildRoot "appicon.png"
$msixPath = Join-Path $binDir "$DisplayName-windows-amd64.msix"

if (-not $Version) {
  if ($env:GITHUB_REF_NAME -match '^v?(\d+)\.(\d+)\.(\d+)$') {
    $Version = "$($Matches[1]).$($Matches[2]).$($Matches[3]).0"
  } else {
    $Version = "1.0.0.0"
  }
}

if ($Version -notmatch '^\d+\.\d+\.\d+\.\d+$') {
  throw "Version must be in a.b.c.d format: $Version"
}

if ([string]::IsNullOrWhiteSpace($IdentityName)) {
  throw "IdentityName is required"
}

if ([string]::IsNullOrWhiteSpace($Publisher)) {
  throw "Publisher is required"
}

if ([string]::IsNullOrWhiteSpace($PublisherDisplayName)) {
  throw "PublisherDisplayName is required"
}

if ([string]::IsNullOrWhiteSpace($DisplayName)) {
  throw "DisplayName is required"
}

if ([string]::IsNullOrWhiteSpace($Description)) {
  throw "Description is required"
}

if (-not (Test-Path $exePath)) {
  throw "Windows executable not found: $exePath"
}

if (-not (Test-Path $templatePath)) {
  throw "Manifest template not found: $templatePath"
}

if (-not (Test-Path $sourceIcon)) {
  throw "Source icon not found: $sourceIcon"
}

Remove-Item $packageRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $packageRoot, $assetsDir | Out-Null
Copy-Item $exePath (Join-Path $packageRoot $exeName) -Force

function New-SquarePng {
  param(
    [string]$Source,
    [string]$Destination,
    [int]$Width,
    [int]$Height = $Width
  )

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

New-SquarePng -Source $sourceIcon -Destination (Join-Path $assetsDir "StoreLogo.png") -Width 50
New-SquarePng -Source $sourceIcon -Destination (Join-Path $assetsDir "Square44x44Logo.png") -Width 44
New-SquarePng -Source $sourceIcon -Destination (Join-Path $assetsDir "Square150x150Logo.png") -Width 150
New-SquarePng -Source $sourceIcon -Destination (Join-Path $assetsDir "Wide310x150Logo.png") -Width 310 -Height 150
New-SquarePng -Source $sourceIcon -Destination (Join-Path $assetsDir "Square310x310Logo.png") -Width 310

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
  Sort-Object FullName -Descending |
  Select-Object -First 1 -ExpandProperty FullName

if (-not $makeappx) {
  throw "makeappx.exe not found"
}

& $makeappx pack /d $packageRoot /p $msixPath /o
if ($LASTEXITCODE -ne 0) {
  throw "makeappx pack failed"
}

Write-Host "Created $msixPath"
