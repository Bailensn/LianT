; LianT — Windows 安装包（Inno Setup）
; 版本由 CI 注入：搜索替换 `#define AppVersion "..."" 行即可

#define AppName "LianT"
#define AppVersion "1.0.0"
#define AppPublisher "LianT Team"
#define AppExeName "LianT.exe"

[Setup]
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL=https://github.com/bailensn/LianT
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}

; 产物输出到 manager/release/（本文件位于 manager/resources/windows/，向上两级到 manager/，再进 release/）
OutputDir=..\..\release
OutputBaseFilename=LianT_Setup

SetupIconFile=LianT.ico
UninstallDisplayIcon={app}\{#AppExeName}

Compression=lzma2/ultra
SolidCompression=yes
WizardStyle=modern

; 当前用户安装，无需管理员权限（与之前 cargo-packager 的 currentUser 对齐）
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64

DisableProgramGroupPage=yes
UninstallDisplayIcon={app}\{#AppExeName}

[Languages]
Name: "chinesesimplified"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
; 主二进制（相对于本 .iss 文件所在目录）
Source: "..\..\desktop\target\release\{#AppExeName}"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{group}\卸载 {#AppName}"; Filename: "{app}\uninstall.exe"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "创建桌面快捷方式"; GroupDescription: "附加图标:"; Flags: unchecked

[Run]
; 安装完成后可选启动
Filename: "{app}\{#AppExeName}"; \
  Description: "现在运行 {#AppName}"; \
  Flags: nowait postinstall skipifsilent

[Code]
procedure CurStepChanged(CurStep: TSetupStep);
var
  OldUninstallExe: String;
  NewUninstallExe: String;
  UninstallKey: String;
begin
  // 安装完成后，把默认的 unins000.exe 重命名为 uninstall.exe
  // 避免安装目录下出现 unins000 / unins001 这类自动编号文件名
  if CurStep = ssPostInstall then
  begin
    OldUninstallExe := ExpandConstant('{uninstallexe}');
    NewUninstallExe := ExpandConstant('{app}\uninstall.exe');

    if FileExists(OldUninstallExe) then
    begin
      if FileExists(NewUninstallExe) then
        DeleteFile(NewUninstallExe);

      RenameFile(OldUninstallExe, NewUninstallExe);

      // 更新"添加或删除程序"里的卸载路径
      UninstallKey := 'Software\Microsoft\Windows\CurrentVersion\Uninstall\' +
        '{#AppName}_is1';

      RegWriteStringValue(HKCU, UninstallKey,
        'UninstallString', '"' + NewUninstallExe + '"');
      RegWriteStringValue(HKCU, UninstallKey,
        'QuietUninstallString', '"' + NewUninstallExe + '" /SILENT');
      RegWriteStringValue(HKCU, UninstallKey,
        'ModifyPath', '"' + NewUninstallExe + '" /MODIFY');
    end;
  end;
end;
