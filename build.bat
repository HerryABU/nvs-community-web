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
if exist "server\dist" rmdir /S /Q "server\dist"
xcopy /E /I /Y "dist" "server\dist" >nul
cd server
go build -ldflags="-s -w" -o nvs-server.exe .
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Go build failed!
    cd ..
    pause
    exit /b 1
)
cd ..

echo.
echo [3/3] Copy exe to project root...
copy /Y "server\nvs-server.exe" "nvs-server.exe" >nul

echo.
echo [3.5/3] Compress with UPX...
set UPX_EXE=..\upx\upx-5.2.0-win64\upx.exe
if exist "nvs-server.exe" (
    if exist "%UPX_EXE%" (
        "%UPX_EXE%" --best --force nvs-server.exe
        echo UPX compression done.
    ) else (
        echo UPX not found, skipping compression.
    )
) else (
    echo nvs-server.exe not found for compression
)

echo.
echo ============================================
echo   Build succeeded!
echo   Output: nvs-server.exe  (embedded frontend + backend, UPX compressed)
echo   Assets: dist\          (frontend output, project root)
echo   Run:    nvs-server.exe
echo   Visit:  http://localhost:8080
echo ============================================
pause