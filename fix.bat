@echo off
set GIT_EXE="C:\Program Files\Git\cmd\git.exe"

%GIT_EXE% add .
%GIT_EXE% commit -m "Fix unused imports to resolve Render Docker build crash"
%GIT_EXE% push origin main
