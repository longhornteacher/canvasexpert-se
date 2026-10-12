@echo off
rem Double-click to launch the CanvasExpert-SE hub (builds first if needed).
rem   webui.cmd                                the hub: Canvas setup, students, more (stays open)
rem   webui.cmd -Workspace C:\path\to\workspace start with a workspace
rem CSE_WORKSPACE may supply the explicit Install root for this Source launch.
rem Use the Quit button, Ctrl+C or close this window to stop it.
title CanvasExpert-SE hub
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\webui.ps1" %*
if errorlevel 1 (
  echo.
  echo The hub stopped with an error. Details are above.
  pause
)
