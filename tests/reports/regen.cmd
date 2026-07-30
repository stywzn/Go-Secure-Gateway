@echo off
REM Regenerate the single-file Allure report offline (double-click to run).
REM Uses the bundled Allure CLI (tests\tools\allure-2.29.0) + local Java.
setlocal
cd /d "%~dp0"

set "OUT=%TEMP%\allure-regen-out"
echo [1/2] Generating report into temp dir...
call "..\tools\allure-2.29.0\bin\allure.bat" generate "allure-results" --clean --single-file -o "%OUT%"
if errorlevel 1 (
  echo FAILED: ensure Java is installed and tests\tools\allure-2.29.0 exists.
  exit /b 1
)

echo [2/2] Copying to allure-report.html ...
copy /Y "%OUT%\index.html" "allure-report.html" >nul
echo.
echo Done: %~dp0allure-report.html  (double-click to view offline)
endlocal
