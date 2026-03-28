@echo off
set GIT_EXE="C:\Program Files\Git\cmd\git.exe"

%GIT_EXE% config --global user.email "bot@antigravity.local"
%GIT_EXE% config --global user.name "AutoBot"

%GIT_EXE% init
%GIT_EXE% add .
%GIT_EXE% commit -m "Automated production platform setup"
%GIT_EXE% branch -M main

%GIT_EXE% remote add origin https://github.com/raghavar8088/antigravity.git
%GIT_EXE% push -u origin main
