@echo off
chcp 65001 >nul
echo ============================================
echo   NVS - Build
echo ============================================
echo.

cd /d "%~dp0"

echo [1/3] Build frontend...
cd web
call npm run build
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Frontend build failed!
    pause
    exit /b 1
)
cd ..

echo.
echo [2/3] Sync dist -^> server/dist + Build Go...
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
echo [3/3] Copy exe to root...
copy /Y "server\nvs-server.exe" "nvs-server.exe" >nul

echo.
echo ============================================
echo   Build succeeded!
echo   Output: nvs-server.exe  (embedded frontend + backend)
echo   Run:   nvs-server.exe
echo   Visit: http://localhost:8080
echo ============================================
pause
