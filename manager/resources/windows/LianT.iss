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

; Use a clean, predictable uninstaller filename instead of unins000.exe / unins000.dat
UninstallFilesDir={app}
UninstallFilesName=uninstall

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
