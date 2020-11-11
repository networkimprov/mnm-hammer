:: Copyright 2020 Liam Breck
:: Published at https://github.com/networkimprov/mnm-hammer
::
:: This Source Code Form is subject to the terms of the Mozilla Public
:: License, v. 2.0. If a copy of the MPL was not distributed with this
:: file, You can obtain one at http://mozilla.org/MPL/2.0/

@echo off

net file 1>nul 2>nul
if %errorlevel% neq 0 (
   echo Welcome to mnm!
   echo Requesting admin privileges...
   powershell -noprofile -command start-process -filepath "%0" -verb runas || goto error
   exit /b
)
echo Welcome to mnm!
echo Requesting admin privileges... OK

cd /d "%~dp0"
setlocal EnableDelayedExpansion

if not exist "store" (
   set findStore=get-item ..\mnm-*-v0.*.?\store ^| ^
                 sort -property LastWriteTime ^| ^
                 select -last 1 -expandproperty FullName
   set findStore=powershell -noprofile -command "!findStore!"
   !findStore! >nul || !findStore! || goto error
   for /f "delims=" %%V in ('!findStore!') do set found=%%V
   if defined found (
      echo It looks like you're updating from !found:~0,-6!.
      echo Press U to update, N to start anew, or Q to quit.
      choice /c unq
      if !errorlevel! equ 3 exit /b
      if !errorlevel! equ 1 (
         echo Moving !found!
         move "!found!" .\store >nul || goto error
      )
      echo:
   )
)

netsh advfirewall firewall delete rule name=mnm-hammer >nul
netsh advfirewall firewall add rule action=allow protocol=tcp enable=yes direction=in ^
      profile=domain,private,public name=mnm-hammer program="%~dp0mnm-hammer.exe" >nul || goto error
netsh advfirewall firewall add rule action=allow protocol=tcp enable=yes direction=out ^
      profile=domain,private,public name=mnm-hammer program="%~dp0mnm-hammer.exe" >nul || goto error

:loop
   mnm-hammer.exe -http :8123
   echo:
   echo Press R to restart, or Q to quit.
   choice /c rq
   if !errorlevel! equ 2 exit /b
   echo:
goto loop

:error
echo Stopped by error. Press any key to quit.
pause >nul
exit /b 1
