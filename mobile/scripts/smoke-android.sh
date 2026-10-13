#!/usr/bin/env bash
# Teste de fumaça do app Android num emulador (CI): instala o APK, abre o app
# e confere que a tela inicial aparece (o WebView carregou as telas e as
# regras em WebAssembly) e que o app não fechou com erro.
set -euo pipefail
APK=${1:-android/app/build/outputs/apk/debug/app-debug.apk}
PKG=io.github.alexandreserra1.finanrich

adb wait-for-device
adb install -r "$APK"
adb logcat -c
adb shell am start -W -n "$PKG/.MainActivity"

for i in $(seq 1 45); do
  sleep 2
  adb shell uiautomator dump /sdcard/ui.xml >/dev/null 2>&1 || continue
  if adb shell cat /sdcard/ui.xml | grep -q "ainda não está conectado"; then
    echo "OK: a tela inicial do app apareceu (${i}x2 s)"
    adb shell pidof "$PKG" >/dev/null || { echo "FALHA: o app não está rodando"; exit 1; }
    # Só conta erro fatal do próprio app (o uiautomator, usado acima, às vezes
    # registra erros dele mesmo no logcat).
    if adb logcat -d -b crash | grep -A3 "FATAL EXCEPTION" | grep "Process: $PKG" ; then
      echo "FALHA: o app fechou com erro"; exit 1
    fi
    exit 0
  fi
done
echo "FALHA: a tela inicial não apareceu em 90 s"
adb shell cat /sdcard/ui.xml | head -c 3000 || true
adb logcat -d | grep -iE "chromium|Capacitor|$PKG" | tail -80 || true
exit 1
