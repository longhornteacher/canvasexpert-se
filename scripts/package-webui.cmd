@echo off
rem Double-click to open the CanvasExpert-SE hub for this folder.
title CanvasExpert-SE hub
"%~dp0engine\cse.exe" --workspace "%~dp0." ui
if errorlevel 1 (
  echo.
  echo The hub stopped with an error. Details are above.
  pause
)
