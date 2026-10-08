@echo off
rem CanvasExpert-SE workspace launcher. Runs this machine's linked program
rem against THIS workspace. The program is not stored in this synced folder.
rem Usage:  .\cse.cmd COMMAND [ARGS]     e.g.  .\cse.cmd --help   .\cse.cmd setup
setlocal
set "CSE_POINTER=%LOCALAPPDATA%\CanvasExpert-SE\program.txt"
if not exist "%CSE_POINTER%" goto missing
set /p CSE_EXE=<"%CSE_POINTER%"
if not exist "%CSE_EXE%" goto missing
"%CSE_EXE%" --workspace "%~dp0." %*
exit /b %ERRORLEVEL%
:missing
echo The CanvasExpert-SE program is not linked on this machine. 1>&2
echo Open the CanvasExpert-SE program folder and double-click webui.cmd once. 1>&2
exit /b 4
