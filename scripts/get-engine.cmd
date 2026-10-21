@echo off
rem CanvasExpert-SE Engine fetch. Downloads the Engine files named in engine\manifest.txt,
rem checks each against the SHA-256 recorded there, and puts them in engine\.
rem
rem   get-engine.cmd              download, verify and install
rem   get-engine.cmd --stage      download and verify into engine\staged\ only; nothing live changes
rem   get-engine.cmd --base-url U fetch from U instead of the manifest's base_url (testing)
rem
rem Needs only what Windows 10 and later ship: curl.exe and certutil. No admin rights.
rem Exit codes: 0 ok, 2 bad option, 3 checksum mismatch, 4 missing tool or manifest,
rem             5 download failed, 6 the running Engine could not be replaced.
setlocal EnableExtensions EnableDelayedExpansion
set "ROOT=%~dp0"
set "ENGINE=%ROOT%engine"
set "MANIFEST=%ENGINE%\manifest.txt"
set "STAGED=%ENGINE%\staged"
set "STAGE_ONLY=0"
set "BASE_OVERRIDE="

:args
if "%~1"=="" goto argsdone
if /i "%~1"=="--stage" set "STAGE_ONLY=1" & shift & goto args
if /i "%~1"=="--base-url" set "BASE_OVERRIDE=%~2" & shift & shift & goto args
echo Unknown option %~1 1>&2
exit /b 2
:argsdone

if not exist "%MANIFEST%" (
  echo engine\manifest.txt was not found. This folder is incomplete; pull the latest version again. 1>&2
  exit /b 4
)
where curl.exe >nul 2>&1
if errorlevel 1 (
  echo curl.exe is not available on this computer, so the Engine cannot be downloaded here. 1>&2
  exit /b 4
)
where certutil.exe >nul 2>&1
if errorlevel 1 (
  echo certutil.exe is not available on this computer, so the download cannot be verified. 1>&2
  exit /b 4
)

set "VERSION="
set "BASE="
set "FILES="
for /f "usebackq eol=# tokens=1,* delims==" %%A in ("%MANIFEST%") do (
  set "K=%%A"
  if /i "!K!"=="version" set "VERSION=%%B"
  if /i "!K!"=="base_url" set "BASE=%%B"
  if /i "!K:~0,7!"=="sha256." (
    set "F=!K:~7!"
    set "HASH_!F!=%%B"
    set "FILES=!FILES! !F!"
  )
)
if defined BASE_OVERRIDE set "BASE=!BASE_OVERRIDE!"
if not defined BASE (
  echo The manifest names no download location. 1>&2
  exit /b 4
)
if not defined FILES (
  echo The manifest lists no Engine files. 1>&2
  exit /b 4
)

if not exist "%STAGED%" mkdir "%STAGED%"
set "REST=!FILES!"
:nextfile
set "CUR="
for /f "tokens=1,*" %%A in ("!REST!") do (
  set "CUR=%%A"
  set "REST=%%B"
)
if not defined CUR goto filesdone
call :fetch "!CUR!"
if errorlevel 1 exit /b %errorlevel%
goto nextfile
:filesdone

if "%STAGE_ONLY%"=="1" (
  echo Engine %VERSION% is downloaded and verified in engine\staged; nothing live was changed.
  exit /b 0
)

rem Replace the live files. A running cse.exe can be renamed but not overwritten, so move it
rem aside first; the previous copy is kept as cse.exe.previous.
if exist "%ENGINE%\cse.exe" (
  if exist "%ENGINE%\cse.exe.previous" del /f /q "%ENGINE%\cse.exe.previous" >nul 2>&1
  ren "%ENGINE%\cse.exe" cse.exe.previous >nul 2>&1
  if exist "%ENGINE%\cse.exe" (
    echo The current Engine could not be replaced. Close CanvasExpert and any app using it, then run this again. 1>&2
    exit /b 6
  )
)
set "REST=!FILES!"
:nextmove
set "CUR="
for /f "tokens=1,*" %%A in ("!REST!") do (
  set "CUR=%%A"
  set "REST=%%B"
)
if not defined CUR goto movesdone
move /y "%STAGED%\!CUR!" "%ENGINE%\!CUR!" >nul
if errorlevel 1 (
  echo Could not place !CUR! in engine\. 1>&2
  exit /b 6
)
goto nextmove
:movesdone
rmdir "%STAGED%" >nul 2>&1
echo CanvasExpert-SE Engine %VERSION% is installed in engine\.
exit /b 0

:fetch
set "NAME=%~1"
set "WANT=!HASH_%NAME%!"
if not defined WANT (
  echo The manifest has no checksum for %NAME%. 1>&2
  exit /b 4
)
if exist "%STAGED%\%NAME%" (
  call :hash "%STAGED%\%NAME%"
  if /i "!GOT!"=="!WANT!" exit /b 0
  del /f /q "%STAGED%\%NAME%" >nul 2>&1
)
curl.exe -fsSL --retry 2 --connect-timeout 20 -o "%STAGED%\%NAME%.part" "!BASE!/%NAME%"
if errorlevel 1 (
  del /f /q "%STAGED%\%NAME%.part" >nul 2>&1
  echo Could not download %NAME% from !BASE!. Check the internet connection, or ask for the ZIP install. 1>&2
  exit /b 5
)
call :hash "%STAGED%\%NAME%.part"
if /i not "!GOT!"=="!WANT!" (
  del /f /q "%STAGED%\%NAME%.part" >nul 2>&1
  echo The downloaded %NAME% does not match its expected checksum, so it was discarded. 1>&2
  exit /b 3
)
move /y "%STAGED%\%NAME%.part" "%STAGED%\%NAME%" >nul
exit /b 0

:hash
set "GOT="
for /f "skip=1 delims=" %%H in ('certutil -hashfile "%~1" SHA256') do if not defined GOT set "GOT=%%H"
set "GOT=!GOT: =!"
exit /b 0
