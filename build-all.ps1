$ErrorActionPreference = "Continue"
$root = $PSScriptRoot
$serverDir = "$root\server"
$outDir = "$root\release"
$distDir = "$serverDir\dist"

if (-not (Test-Path "$distDir\index.html")) {
    Write-Host "[ERROR] server/dist/index.html not found" -ForegroundColor Red
    exit 1
}

Remove-Item -Recurse -Force $outDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

$targets = @(
    @{OS="windows"; Arch="386";   Name="nvs-win-x32.exe";      Desc="Windows 32-bit"},
    @{OS="windows"; Arch="amd64"; Name="nvs-win-x64.exe";      Desc="Windows 64-bit"},
    @{OS="windows"; Arch="arm";   Name="nvs-win-arm32.exe";    Desc="Windows ARM32"; Goarm="7"},
    @{OS="windows"; Arch="arm64"; Name="nvs-win-arm64.exe";    Desc="Windows ARM64"},
    @{OS="linux";   Arch="386";   Name="nvs-linux-x32";        Desc="Linux 32-bit"},
    @{OS="linux";   Arch="amd64"; Name="nvs-linux-x64";        Desc="Linux 64-bit"},
    @{OS="linux";   Arch="arm";   Name="nvs-linux-arm32";      Desc="Linux ARM32(v7)"; Goarm="7"},
    @{OS="linux";   Arch="arm";   Name="nvs-linux-armv6";      Desc="Linux ARM32(v6)"; Goarm="6"},
    @{OS="linux";   Arch="arm64"; Name="nvs-linux-arm64";      Desc="Linux ARM64"},
    @{OS="linux";   Arch="mips64";Name="nvs-linux-mips64";     Desc="Linux MIPS64"},
    @{OS="linux";   Arch="riscv64";Name="nvs-linux-riscv64";   Desc="Linux RISC-V64"},
    @{OS="darwin";  Arch="amd64"; Name="nvs-mac-x64";          Desc="macOS Intel"},
    @{OS="darwin";  Arch="arm64"; Name="nvs-mac-arm64";        Desc="macOS Apple Silicon"}
)

Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  NVS Full Platform Build ($($targets.Count) targets)" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan

$ok = 0
$fail = 0

foreach ($t in $targets) {
    $env:CGO_ENABLED = "0"
    $env:GOOS = $t.OS
    $env:GOARCH = $t.Arch
    if ($t.Goarm) { $env:GOARM = $t.Goarm }
    $env:GOAMD64 = ""; $env:GO386 = ""

    $outFile = "$outDir\$($t.Name)"
    $idx = $ok + $fail + 1
    $total = $targets.Count
    Write-Host "[$idx/$total] $($t.Desc) " -NoNewline

    Push-Location $serverDir
    $err = (go build -ldflags="-s -w" -o $outFile . 2>&1)
    $rc = $LASTEXITCODE
    Pop-Location

    if ($rc -eq 0) {
        $sz = [math]::Round((Get-Item $outFile).Length / 1MB, 1)
        Write-Host "OK  (${sz} MB)" -ForegroundColor Green
        $ok++
    } else {
        Write-Host "FAIL  $err" -ForegroundColor Red
        $fail++
    }
}

Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  Result: $ok OK, $fail FAILED" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan

# x96 combined package
if ((Test-Path "$outDir\nvs-win-x32.exe") -and (Test-Path "$outDir\nvs-win-x64.exe")) {
    $x96 = "$outDir\nvs-win-x96"
    New-Item -ItemType Directory -Force -Path $x96 | Out-Null
    Copy-Item "$outDir\nvs-win-x32.exe" "$x96\nvs-server-x32.exe"
    Copy-Item "$outDir\nvs-win-x64.exe" "$x96\nvs-server-x64.exe"
    $bat = @(
        '@echo off',
        ':: NVS Launcher - Auto-detect x32/x64',
        'if "%PROCESSOR_ARCHITECTURE%"=="AMD64" (start "" "%~dp0nvs-server-x64.exe") else (start "" "%~dp0nvs-server-x32.exe")'
    )
    $bat | Out-File -Encoding ASCII "$x96\NVS.bat"
    Write-Host "x96 package: nvs-win-x96/  (x32+x64+launcher)" -ForegroundColor Green
}

Write-Host ""
Write-Host "Output: $outDir" -ForegroundColor Cyan
Write-Host "Done!" -ForegroundColor Green
