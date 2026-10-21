@echo off
rem CanvasExpert-SE workspace launcher. Runs the Engine against THIS workspace:
rem the copy in this folder's engine\ if there is one, otherwise the Engine the
rem hub last ran from on this computer (for a workspace the hub created elsewhere).
rem Usage:  .\cse.cmd COMMAND [ARGS]     e.g.  .\cse.cmd --help   .\cse.cmd setup
setlocal
set "CSE_EXE=%~dp0engine\cse.exe"
if exist "%CSE_EXE%" goto run
set "CSE_POINTER=%LOCALAPPDATA%\CanvasExpert-SE\program.txt"
if not exist "%CSE_POINTER%" goto missing
set /p CSE_EXE=<"%CSE_POINTER%"
if not exist "%CSE_EXE%" goto missing
:run
"%CSE_EXE%" --workspace "%~dp0." %*
exit /b %ERRORLEVEL%
:missing
echo The CanvasExpert-SE program was not found. 1>&2
echo This folder has no engine\cse.exe, and the hub has not run on this computer. 1>&2
echo Unzip the full CanvasExpert-SE download again. 1>&2
exit /b 4
