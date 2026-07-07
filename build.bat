@echo off
chcp 65001 >nul
echo ============================================
echo   NVS (网络小说平台) 一键构建脚本
echo ============================================
echo.

REM 进入脚本所在目录
cd /d "%~dp0"

echo [1/3] 构建前端...
cd web
call npm run build
if %ERRORLEVEL% NEQ 0 (
    echo [错误] 前端构建失败!
    pause
    exit /b 1
)
cd ..

echo.
echo [2/3] 编译 Go 后端 (含前端)...
cd server
go build -ldflags="-s -w" -o nvs-server.exe .
if %ERRORLEVEL% NEQ 0 (
    echo [错误] Go 编译失败!
    cd ..
    pause
    exit /b 1
)
cd ..

echo.
echo [3/3] 拷贝 exe 到项目根目录...
copy /Y "server\nvs-server.exe" "nvs-server.exe" >nul

echo.
echo ============================================
echo   构建成功!
echo   输出文件: nvs-server.exe
echo   运行: nvs-server.exe
echo   访问: http://localhost:8080
echo ============================================
pause
