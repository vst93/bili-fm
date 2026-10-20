; Tauri 2.0.37 -> mygo 迁移器（Windows）。
;
; 由旧 Tauri updater 下载并执行。它不运行 Tauri 的卸载器（卸载器可能带
; “删除应用数据”复选框，静默模式行为不可靠），只做三件事：
;   1. 关闭旧进程，删除旧的 Tauri 安装目录 / 卸载项 / 快捷方式；
;   2. 静默安装 release 里嵌入的 mygo NSIS 安装包；
;   3. 启动新的 mygo。
;
; 关键约束：**绝不删除 %APPDATA%\bili-fm、%APPDATA%\com.bili-fm.app
; 或 %LOCALAPPDATA%\com.bili-fm.app**。登录态、歌单、Tauri WebView 数据都
; 在这里；mygo 首次启动会走 store.Migrate。

!ifndef MYGO_SETUP
  !error "MYGO_SETUP is not defined"
!endif
!ifndef OUT
  !error "OUT is not defined"
!endif
!ifndef VER
  !define VER "unknown"
!endif

Unicode true
Name "bili-FM ${VER} (native migration)"
OutFile "${OUT}"
RequestExecutionLevel user
SilentInstall silent
ShowUninstDetails nevershow

Section "Migrate to mygo"
  ; 旧 Tauri 进程和新进程都先关掉，避免文件占用。
  nsExec::ExecToLog 'taskkill /F /IM bili-fm.exe'
  nsExec::ExecToLog 'taskkill /F /IM bili-FM.exe'
  Sleep 800

  ; Tauri v2 NSIS 默认 per-user 安装在 %LOCALAPPDATA%\bili-FM。
  ; 只删安装目录，不碰 AppData。
  RMDir /r "$LOCALAPPDATA\bili-FM"

  ; 旧卸载项、开始菜单 / 桌面快捷方式（Tauri 的产品名是 bili-FM）。
  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\bili-FM"
  Delete "$SMPROGRAMS\bili-FM.lnk"
  RMDir /r "$SMPROGRAMS\bili-FM"
  Delete "$DESKTOP\bili-FM.lnk"

  ; 嵌入的 mygo 安装包（构建时由 release-mygo 的 bridge job 传入）。
  ; 它装到 %LOCALAPPDATA%\Programs\bili-FM，不会覆盖上面的 AppData。
  InitPluginsDir
  File /oname=$PLUGINSDIR\mygo-setup.exe "${MYGO_SETUP}"
  ExecWait '"$PLUGINSDIR\mygo-setup.exe" /S' $0

  ; 新安装包成功后启动；失败时也尝试一次（例如安装器被 UAC/安全软件拦截）。
  IfFileExists "$LOCALAPPDATA\Programs\bili-FM\bili-FM.exe" 0 +2
    Exec '"$LOCALAPPDATA\Programs\bili-FM\bili-FM.exe"'
SectionEnd
