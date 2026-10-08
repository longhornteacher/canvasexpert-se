@echo off
rem Double-click to open the CanvasExpert-SE hub. This also tells this computer
rem where the program is, so a workspace's cse.cmd can find it.
rem   webui.cmd                                   open the hub (stays open)
rem   webui.cmd --workspace C:\path\to\workspace  start with a workspace
title CanvasExpert-SE hub
"%~dp0cse.exe" link >nul
"%~dp0cse.exe" %* ui
if errorlevel 1 (
  echo.
  echo The hub stopped with an error. Details are above.
  pause
)
