<#
.SYNOPSIS
Downloads the local U2-Net-P ONNX model (Apache-2.0) and Windows ONNX Runtime
CPU shared library (MIT) for WeevCrafts AI Image Studio.

.DESCRIPTION
All AI inference runs 100% locally on CPU. WeevCrafts never downloads models at
runtime. Run this setup script once before launching the server if you want the
pretrained U2-Net-P neural segmenter enabled (otherwise the built-in pure-Go CV
segmenter runs automatically as a lightweight fallback).
#>

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$ModelsDir = Join-Path $Root "models"
New-Item -ItemType Directory -Force -Path $ModelsDir | Out-Null

$ModelDest = Join-Path $ModelsDir "u2netp.onnx"
$ModelUrl = "https://github.com/danielgatis/rembg/releases/download/v0.0.0/u2netp.onnx"

if (-not (Test-Path $ModelDest)) {
    Write-Host "[+] Downloading U2-Net-P ONNX weights (~4.7 MB, Apache-2.0)..."
    Invoke-WebRequest -Uri $ModelUrl -OutFile $ModelDest -UseBasicParsing
} else {
    Write-Host "[=] u2netp.onnx already exists at $ModelDest"
}

$ISNetDest = Join-Path $ModelsDir "isnet-general-use.onnx"
$ISNetUrl = "https://github.com/danielgatis/rembg/releases/download/v0.0.0/isnet-general-use.onnx"

if (-not (Test-Path $ISNetDest)) {
    Write-Host "[+] Downloading IS-Net General Use ONNX weights (~176 MB, Apache-2.0)..."
    Invoke-WebRequest -Uri $ISNetUrl -OutFile $ISNetDest -UseBasicParsing
} else {
    Write-Host "[=] isnet-general-use.onnx already exists at $ISNetDest"
}

$OrtDllDest = Join-Path $ModelsDir "onnxruntime.dll"
$OrtVersion = "1.24.4"
$OrtZipUrl = "https://api.nuget.org/v3-flatcontainer/microsoft.ml.onnxruntime/$OrtVersion/microsoft.ml.onnxruntime.$OrtVersion.nupkg"

if (-not (Test-Path $OrtDllDest)) {
    $TempZip = Join-Path $env:TEMP "microsoft.ml.onnxruntime.$OrtVersion.zip"
    $TempExtract = Join-Path $env:TEMP "microsoft.ml.onnxruntime.$OrtVersion"
    Write-Host "[+] Downloading Microsoft.ML.OnnxRuntime CPU v$OrtVersion (MIT)..."
    Invoke-WebRequest -Uri $OrtZipUrl -OutFile $TempZip -UseBasicParsing
    if (Test-Path $TempExtract) { Remove-Item -Recurse -Force $TempExtract }
    Expand-Archive -Path $TempZip -DestinationPath $TempExtract -Force
    $NativeDir = Join-Path $TempExtract "runtimes\win-x64\native"
    $FoundDll = Get-ChildItem -Path $NativeDir -Filter "onnxruntime.dll" -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $FoundDll) {
        throw "onnxruntime.dll was not found inside the downloaded package."
    }
    Copy-Item -Path (Join-Path $NativeDir "*.dll") -Destination $ModelsDir -Force
    Remove-Item -Force $TempZip -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force $TempExtract -ErrorAction SilentlyContinue
} else {
    Write-Host "[=] onnxruntime.dll already exists at $OrtDllDest"
}

Write-Host ""
Write-Host "[+] Local AI Image Studio model setup complete:"
Write-Host "    Model:   $ModelDest"
Write-Host "    Runtime: $OrtDllDest"
