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

# 打包目标清单。
#
# ⚠️ 以下 3 个平台**编不出来**，已移除，不要加回来（2026-10-31 实测）：
#   windows/386  —— modernc.org/sqlite 没有 386 的 C 转译实现
#                   （undefined: sqlite3_index_constraint …），纯 Go sqlite 的固有限制
#   windows/arm  —— Go 工具链本身不支持该 GOOS/GOARCH 组合
#                   （go: unsupported GOOS/GOARCH pair windows/arm）
#   linux/mips64 —— modernc.org/libc 没有 mips64 实现
#                   （build constraints exclude all Go files in …/libc/errno 等）
# 这些是**依赖选型**（纯 Go sqlite → 无需 cgo，但平台矩阵随之收窄）决定的，
# 不是配置问题：改 GOMIPS/cgo 也解决不了。留在列表里只会让每次打包都刷 3 个 FAIL。
$targets = @(
    @{OS="windows"; Arch="amd64"; Name="nvs-win-x64.exe";      Desc="Windows 64-bit"},
    @{OS="windows"; Arch="arm64"; Name="nvs-win-arm64.exe";    Desc="Windows ARM64"},
    @{OS="linux";   Arch="386";   Name="nvs-linux-x32";        Desc="Linux 32-bit"},
    @{OS="linux";   Arch="amd64"; Name="nvs-linux-x64";        Desc="Linux 64-bit"},
    @{OS="linux";   Arch="arm";   Name="nvs-linux-arm32";      Desc="Linux ARM32(v7)"; Goarm="7"},
    @{OS="linux";   Arch="arm";   Name="nvs-linux-armv6";      Desc="Linux ARM32(v6)"; Goarm="6"},
    @{OS="linux";   Arch="arm64"; Name="nvs-linux-arm64";      Desc="Linux ARM64"},
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
        # UPX compress (skip macOS Mach-O and Windows ARM64 PE)
        $upxExe = "$root\..\upx\upx-5.2.0-win64\upx.exe"
        $skipUpx = ($t.OS -eq "darwin") -or ($t.OS -eq "windows" -and $t.Arch -eq "arm64")
        if (-not $skipUpx -and (Test-Path $upxExe)) {
            $upxResult = & $upxExe --best --force $outFile 2>&1 | Out-String
            if ($LASTEXITCODE -ne 0) {
                Write-Host "UPX skipped (unsupported format)" -ForegroundColor Yellow
            }
        } elseif ($skipUpx) {
            # silently skip unsupported platforms
        }
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

# x32/x64 合并包已取消：windows/386 编不出来（见上方说明），
# 没有 x32 产物就没有「自动选 32/64 位」的启动器可做。
# 用户直接用 nvs-win-x64.exe（64 位 Windows 占绝对多数）。

Write-Host ""
Write-Host "Output: $outDir" -ForegroundColor Cyan
Write-Host "Done!" -ForegroundColor Green
