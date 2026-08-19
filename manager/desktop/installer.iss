; 脚本名：installer.iss
; 位置：manager/desktop/installer.iss

[Setup]
AppName=LianT
AppVersion=1.0.0
AppPublisher=LianT Team
DefaultDirName={pf}\LianT
DefaultGroupName=LianT
OutputDir=Output
OutputBaseFilename=LianT
SetupIconFile=..\resources\windows\LianT.ico
Compression=lzma
SolidCompression=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64

[Files]
; Compose Desktop createDistributable 输出目录
Source: "desktopApp\build\compose\binaries\main\app\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
; 开始菜单快捷方式
Name: "{group}\LianT"; Filename: "{app}\LianT.exe"; IconFilename: "{app}\LianT.exe"

; 桌面快捷方式
Name: "{autodesktop}\LianT"; Filename: "{app}\LianT.exe"; IconFilename: "{app}\LianT.exe"

[Run]
; 不自动启动