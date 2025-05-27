@echo off
echo 正在构建媒体代理服务器...
go mod tidy
go build -o media-proxy.exe .
echo 构建完成！
echo 运行 media-proxy.exe 启动服务器
pause 