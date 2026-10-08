@echo off
rem Launch the CanvasSidekick local web pages from the repo root.
rem   webui                       connect Canvas (setup page)
rem   webui setup                 same
rem   webui students -Workspace C:\path\to\workspace
rem Ctrl+C closes the page. CSE_WORKSPACE may supply the workspace.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\webui.ps1" %*
