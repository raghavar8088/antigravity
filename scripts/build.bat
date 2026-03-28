@echo off
echo ==============================================
echo ANTIGRAVITY BUILD TOOLCHAIN
echo ==============================================

echo [1] Updating Go Dependencies...
cd engine
go mod tidy
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Failed to run 'go mod tidy'. Is Go installed?
    exit /b %ERRORLEVEL%
)

echo [2] Compiling Golang Live Trading Engine...
go build -o ../bin/antigravity.exe cmd/antigravity/main.go
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Go compilation failed!
    exit /b %ERRORLEVEL%
)

echo [3] Compiling Go Backtesting Engine...
go build -o ../bin/backtest.exe cmd/backtest/main.go

cd ..

echo [4] Compiling Protobuf Definitions for Python...
cd infrastructure\ai
python -m grpc_tools.protoc -I./proto --python_out=. --grpc_python_out=. ./proto/strategy.proto
if %ERRORLEVEL% NEQ 0 (
    echo [WARNING] Python grpc_tools missing. Please run: pip install -r requirements.txt
)

cd ..\..

echo [5] Installing React Dashboard Dependencies...
cd client
call npm install
cd ..

echo ==============================================
echo [SUCCESS] Antigravity Architectural Build Complete!
echo Exectuables rendered in \bin\ directory.
echo ==============================================
pause
