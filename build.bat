chcp 65001
@echo off
echo ========================================
echo 媒体代理服务器构建脚本
echo ========================================
echo.

echo [1/4] 正在检查Go环境...
go version
if %errorlevel% neq 0 (
    echo 错误: 未找到Go环境，请先安装Go
    pause
    exit /b 1
)

echo.
echo [2/4] 正在下载依赖...
go mod tidy
if %errorlevel% neq 0 (
    echo 错误: 依赖下载失败
    pause
    exit /b 1
)

echo.
echo [3/4] 正在构建可执行文件...
go build -o media-proxy.exe .
if %errorlevel% neq 0 (
    echo 错误: 构建失败
    pause
    exit /b 1
)

echo.
echo [4/4] 正在创建目录结构...
if not exist "media" (
    mkdir media
    mkdir media\images
    mkdir media\videos
    echo 已创建media目录结构
) else (
    echo media目录已存在
)

echo.
echo ========================================
echo 构建完成！
echo ========================================
echo.
pause 