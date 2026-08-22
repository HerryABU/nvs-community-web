@echo off
chcp 65001 >nul
echo ============================================
echo   NVS - Build (frontend + Go embed)
echo ============================================
echo.

cd /d "%~dp0"

echo [1/3] Build frontend -^> project root dist\
cd web
call npm run build
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Frontend build failed!
    pause
    exit /b 1
)
cd ..

REM Verify frontend output exists in project root
if not exist "dist\index.html" (
    echo [ERROR] dist\index.html not found in project root!
    echo        Check vite.config.ts → build.outDir
    pause
    exit /b 1
)

echo.
echo [2/3] Sync frontend dist -^> server\dist + Build Go...
REM 删除前端产物中的 .gz / .br 副本：后端用 gin-contrib/gzip 实时压缩响应，
REM 这些静态压缩文件对嵌入的可执行文件是纯冗余，会显著增大二进制体积。
echo      [clean] removing redundant .gz / .br from dist...
del /S /Q "dist\*.gz" "dist\*.br" >nul 2>&1

if exist "server\dist" rmdir /S /Q "server\dist"
xcopy /E /I /Y "dist" "server\dist" >nul
cd server
go build -ldflags="-s -w" -trimpath -o nvs-server.exe .
if %ERRORLEVEL% NEQ 0 (
    echo [  ERROR] Go build failed!
    cd ..
    pause
    exit /b 1
)
cd ..

echo.
echo [3/3] Copy exe to project root...
copy /Y "server\nvs-server.exe" "nvs-server.exe" >nul

echo.
echo ============================================
echo   Build succeeded!
echo   Output: nvs-server.exe  (embedded frontend + backend, no UPX)
echo   Assets: dist\          (frontend output, project root)
echo   Run:    nvs-server.exe
echo   Visit:  http://localhost:8080
echo ============================================

echo.
echo ============================================
echo   Build succeeded!
echo   Output: nvs-server.exe  (embedded frontend + backend, UPX compressed)
echo   Assets: dist\          (frontend output, project root)
echo   Run:    nvs-server.exe
echo   Visit:  http://localhost:8080
echo ============================================
pause