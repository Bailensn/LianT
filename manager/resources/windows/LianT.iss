; LianT — Windows installer (Inno Setup)
; Version is injected by CI: replace the `#define AppVersion "..."` line.

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

; Output goes to manager/release/ (this file is at manager/resources/windows/)
OutputDir=..\..\release
OutputBaseFilename=LianT_Setup

SetupIconFile=LianT.ico
UninstallDisplayIcon={app}\{#AppExeName}

Compression=lzma2/ultra
SolidCompression=yes
WizardStyle=modern

; Per-user install, no admin rights required
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64

DisableProgramGroupPage=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "..\..\desktop\target\release\{#AppExeName}"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{group}\Uninstall {#AppName}"; Filename: "{app}\uninstall.exe"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional icons:"; Flags: unchecked

[Run]
Filename: "{app}\{#AppExeName}"; \
  Description: "Launch {#AppName} now"; \
  Flags: nowait postinstall skipifsilent

[Code]
// Inno Setup always writes its uninstaller as unins000.exe / unins000.dat.
// After install completes, rename them to the predictable uninstall.exe /
// uninstall.dat and point the registry uninstall entry at the new name.
procedure RenameUninstaller;
var
  Uninstaller: String;
  DataFile: String;
  NewUninstaller: String;
  NewDataFile: String;
  UninstallPath: String;
  Failed: Boolean;
begin
  Uninstaller := ExpandConstant('{app}\unins000.exe');
  DataFile    := ExpandConstant('{app}\unins000.dat');
  NewUninstaller := ExpandConstant('{app}\uninstall.exe');
  NewDataFile    := ExpandConstant('{app}\uninstall.dat');
  Failed := False;

  if FileExists(Uninstaller) then
  begin
    if not RenameFile(Uninstaller, NewUninstaller) then
      Failed := True;
  end;
  if FileExists(DataFile) then
  begin
    if not RenameFile(DataFile, NewDataFile) then
      Failed := True;
  end;

  // Update the registry uninstall entry so it points to the renamed exe.
  UninstallPath := ExpandConstant('Software\Microsoft\Windows\CurrentVersion\Uninstall\') + '{#AppName}_is1';
  if not Failed then
  begin
    RegWriteStringValue(HKCU, UninstallPath, 'UninstallString', '"' + NewUninstaller + '"');
    RegWriteStringValue(HKCU, UninstallPath, 'QuietUninstallString', '"' + NewUninstaller + '" /VERYSILENT /NORESTART');
    RegWriteStringValue(HKCU, UninstallPath, 'ModifyPath', '"' + NewUninstaller + '"');
    RegWriteStringValue(HKCU, UninstallPath, 'DisplayIcon', NewUninstaller);
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
    RenameUninstaller;
end;
