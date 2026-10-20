@echo off
rem Double-click to launch the CanvasExpert-SE hub (builds first if needed).
rem   webui.cmd                                the hub: Canvas setup, students, more (stays open)
rem   webui.cmd -Workspace C:\path\to\workspace start with a workspace
rem CSE_WORKSPACE may supply the explicit UserInstall root for this Dev launch.
rem Use the Quit button, Ctrl+C or close this window to stop it.
title CanvasExpert-SE hub
cd /d "%~dp0.."
"%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "%~dp0webui.ps1" %*
if errorlevel 1 (
  echo.
  echo The hub stopped with an error. Details are above.
  pause
)
