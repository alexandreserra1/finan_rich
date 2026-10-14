# Instala (ou atualiza) o finan_rich no Windows, como um aplicativo: atalho no
# menu Iniciar e na Área de Trabalho. No PowerShell:
#
#   irm https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.ps1 | iex
#
# Desinstalar (os seus dados ficam):
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.ps1))) -Uninstall
param([switch]$Uninstall)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue' # o download fica muito mais rápido sem a barra

$repo = 'alexandreserra1/finan_rich'
$dest = Join-Path $env:LOCALAPPDATA 'Programs\finan_rich'
$startMenu = Join-Path ([Environment]::GetFolderPath('Programs')) 'finan_rich.lnk'
$desktop = Join-Path ([Environment]::GetFolderPath('Desktop')) 'finan_rich.lnk'

# Fecha o finan_rich aberto: o Windows não deixa trocar um .exe em uso.
function Stop-Finan {
    try { Invoke-WebRequest -UseBasicParsing -Method Post -TimeoutSec 3 'http://127.0.0.1:8080/configuracoes/fechar' | Out-Null; Start-Sleep 1 } catch {}
    foreach ($p in Get-Process finanrich -ErrorAction SilentlyContinue) {
        try { $p.Kill() } catch {}
        $p.WaitForExit(5000) | Out-Null # o Windows só libera o .exe depois que o processo sai
    }
}

if ($Uninstall) {
    Stop-Finan
    Remove-Item -Recurse -Force $startMenu, $desktop -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force $dest
    Write-Host 'finan_rich removido. Seus dados continuam na pasta de dados (veja o README) e voltam se você instalar de novo.'
    return
}

$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64') { 'arm64' } else { 'amd64' }
$tag = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
$version = $tag.TrimStart('v')
$file = "finan_rich_${version}_windows_$arch.zip"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("finan_rich_" + [guid]::NewGuid())
New-Item -ItemType Directory $tmp | Out-Null
try {
    Write-Host "Baixando finan_rich $tag (windows/$arch)..."
    $zip = Join-Path $tmp $file
    Invoke-WebRequest -UseBasicParsing "https://github.com/$repo/releases/download/$tag/$file" -OutFile $zip
    $sums = Join-Path $tmp 'checksums.txt'
    Invoke-WebRequest -UseBasicParsing "https://github.com/$repo/releases/download/$tag/checksums.txt" -OutFile $sums
    $line = Get-Content $sums | Where-Object { $_ -match " $([regex]::Escape($file))$" } | Select-Object -First 1
    $want = if ($line) { ($line -split '\s+')[0] } else { '' }
    if (-not $want -or $want -ne (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()) {
        throw 'O arquivo baixado não confere com checksums.txt; nada foi instalado.'
    }
    Stop-Finan
    New-Item -ItemType Directory -Force $dest | Out-Null
    Expand-Archive -Force $zip $dest
    $icon = Join-Path $dest 'icon.ico'
    if (-not (Test-Path $icon)) { # versões antigas não traziam o ícone no pacote
        try { Invoke-WebRequest -UseBasicParsing "https://raw.githubusercontent.com/$repo/main/packaging/icon.ico" -OutFile $icon } catch {}
    }

    $shell = New-Object -ComObject WScript.Shell
    foreach ($path in $startMenu, $desktop) {
        $lnk = $shell.CreateShortcut($path)
        $lnk.TargetPath = Join-Path $dest 'finanrich.exe'
        $lnk.WorkingDirectory = $dest
        $lnk.WindowStyle = 7 # a janela de texto do programa abre minimizada
        $lnk.Description = 'finan_rich: finanças pessoais'
        if (Test-Path $icon) { $lnk.IconLocation = $icon }
        $lnk.Save()
    }
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host "finan_rich $tag instalado. Abra pelo atalho no menu Iniciar ou na Área de Trabalho."
Write-Host 'Para atualizar, rode o mesmo comando de novo. Seus dados ficam onde estão.'
try { Invoke-WebRequest -UseBasicParsing -TimeoutSec 2 'http://localhost:11434/api/tags' | Out-Null } catch {
    Write-Host ''
    Write-Host 'Para o consultor com IA grátis, instale também o Ollama: https://ollama.com/download'
}
