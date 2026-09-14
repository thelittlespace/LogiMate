@echo off
setlocal
cd /d "%~dp0"
echo LogiMate Build 018 - verified Go 1.27.1 release build
echo.
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0Build-Release-Go1271.ps1"
set EXITCODE=%ERRORLEVEL%
echo.
if not "%EXITCODE%"=="0" (
  echo RELEASE BUILD FAILED with exit code %EXITCODE%.
) else (
  echo RELEASE BUILD PASSED.
  echo Output: LogiMate-0.0.1-alpha-Build018-Go1.27.1-Release.zip
)
pause
exit /b %EXITCODE%
