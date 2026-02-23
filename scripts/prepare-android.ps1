# prepare-android.ps1
# This script prepares the necessary binaries for building the Android app locally.
# It cross-compiles the Go server and downloads the corresponding yt-dlp binaries.

$ErrorActionPreference = "Stop"

$JniLibsBase = Join-Path $PSScriptRoot "..\android\app\src\main\jniLibs"
$AssetsBin = Join-Path $PSScriptRoot "..\android\app\src\main\assets\bin"
if (-not (Test-Path $AssetsBin)) {
    New-Item -ItemType Directory -Path $AssetsBin -Force | Out-Null
}

$ABIs = @{
    "arm64-v8a" = "arm64"
    "armeabi-v7a" = "arm"
    "x86_64" = "amd64"
    "x86" = "386"
}

$YtDlpUrls = @{
    "arm64-v8a" = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux_aarch64"
    "armeabi-v7a" = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux_armv7l"
    "x86_64" = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux"
}

Write-Host "Building Go server for Android ABIs..."

foreach ($abi in $ABIs.Keys) {
    $goArch = $ABIs[$abi]
    $abiDir = Join-Path $JniLibsBase $abi
    if (-not (Test-Path $abiDir)) {
        New-Item -ItemType Directory -Path $abiDir -Force | Out-Null
    }
    $outFile = Join-Path $abiDir "libytdlpweb.so"
    
    Write-Host "Building for $abi ($goArch)..."
    
    $env:GOOS = "linux"
    $env:GOARCH = $goArch
    $env:CGO_ENABLED = "0"
    
    if ($goArch -eq "arm") {
        $env:GOARM = "7"
    } else {
        Remove-Item Env:\GOARM -ErrorAction SilentlyContinue
    }
    
    go build -ldflags="-s -w" -trimpath -o $outFile .

    if ($YtDlpUrls.ContainsKey($abi)) {
        $ytdlpFile = Join-Path $AssetsBin "yt-dlp_$abi"
        if (-not (Test-Path $ytdlpFile)) {
            Write-Host "Downloading yt-dlp for $abi..."
            Invoke-WebRequest -Uri $YtDlpUrls[$abi] -OutFile $ytdlpFile
        } else {
            Write-Host "yt-dlp for $abi already exists, skipping download."
        }
    }
}

Write-Host "`nAndroid asset preparation complete!"
Write-Host "You can now build the project in Android Studio."
