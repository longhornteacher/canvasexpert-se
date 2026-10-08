@echo off
rem Double-click to launch the CanvasExpert-SE web pages (builds first if needed).
rem   webui.cmd                    Canvas connection page
rem   webui.cmd students -Workspace C:\path\to\workspace
rem CSE_WORKSPACE may supply the workspace. Ctrl+C or closing this window stops the page.
title CanvasExpert-SE web UI
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\webui.ps1" %*
if errorlevel 1 (
  echo.
  echo The web UI stopped with an error. Details are above.
  pause
)
