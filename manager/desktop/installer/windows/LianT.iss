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
Name: "chinesesimplified"; MessagesFile: "ChineseSimplified.isl"

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

; ---------------------------------------------------------------------------
; 卸载清理。runtime/ 是 launcher 首次运行按需下载的 Python 环境，不在
; [Files] 里，Inno 不会自动删除——若遗漏，卸载后会在安装目录残留数百 MB。
; unins000.* 由 Inno 在安装时生成(卸载程序 + 数据)，此处保证卸载干净。
; ---------------------------------------------------------------------------
[UninstallDelete]
Type: filesandordirs; Name: "{app}\client\runtime"
Type: filesandordirs; Name: "{app}\client\__pycache__"
Type: filesandordirs; Name: "{app}\client\src\__pycache__"
Type: filesandordirs; Name: "{app}\client\qml\__pycache__"
Type: dirifempty; Name: "{app}\client"
Type: dirifempty; Name: "{app}"