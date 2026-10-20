@echo off
rem Registers the canvasexpert-se MCP server with Claude Desktop (Cowork).
rem Edit INSTALL if your CE-SE UserInstall folder is elsewhere.
set "INSTALL=C:\Users\adamb\CanvasExpert-SE"

if not exist "%INSTALL%\context\courses.md" (
  echo ERROR: "%INSTALL%" is not a CE-SE UserInstall: context\courses.md is missing.
  goto done
)
if not exist "%INSTALL%\engine\cse.exe" (
  echo ERROR: "%INSTALL%\engine\cse.exe" is missing.
  goto done
)

"%INSTALL%\engine\cse.exe" --workspace "%INSTALL%" install --client cowork
echo.
echo Follow the setup result above. Approve Claude's extension prompt and restart only if needed.

:done
echo.
pause
