; LianT Windows installer configuration (Inno Setup).
; Source-execution + dynamic-runtime mode: ships the launcher exe + client source.
; The Python interpreter is downloaded by the launcher into .\runtime on first run.

#define MyAppName "LianT"
#define MyAppVersion "0.0.0"
#define MyAppPublisher "LensnTeam"
#define MyAppExeName "LianT.exe"

[Setup]
AppId={{8B7EDE17-5F8E-4F2E-9A1C-2D77E5B4B3F0}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\LianT
DefaultGroupName=LianT
DisableProgramGroupPage=yes
OutputBaseFilename=LianTSetup
SetupIconFile=LianT.ico
UninstallDisplayIcon={app}\LianT.exe
Compression=lzma2
SolidCompression=yes
WizardStyle=modern

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "chinesesimplified"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
; Launcher executable
Source: "..\..\..\..\dist\launcher\LianT.exe"; DestDir: "{app}"; Flags: ignoreversion
; Client source tree (source-execution mode). No runtime is bundled on purpose.
Source: "..\..\..\..\client\src\*"; DestDir: "{app}\client\src"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "..\..\..\..\client\qml\*"; DestDir: "{app}\client\qml"; Flags: ignoreversion recursesubdirs createallsubdirs
; Python dependencies are resolved dynamically on first launch.
Source: "..\..\..\..\client\requirements.txt"; DestDir: "{app}\client"; Flags: ignoreversion

[Icons]
Name: "{group}\LianT"; Filename: "{app}\LianT.exe"
Name: "{autodesktop}\LianT"; Filename: "{app}\LianT.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\LianT.exe"; Description: "{cm:LaunchProgram,LianT}"; Flags: nowait postinstall skipifsilent