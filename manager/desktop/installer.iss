; 脚本名：installer.iss
; 位置：manager/desktop/installer.iss
; 对应CI里的 iscc manager/desktop/installer.iss

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
; 核心：把CMP打出来的裸目录（含JVM运行时+你的程序）全拷到安装目录
Source: "..\desktopApp\build\compose\binaries\main\app\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
; 开始菜单快捷方式
Name: "{group}\LianT"; Filename: "{app}\LianT.exe"; IconFilename: "{app}\LianT.exe"
; 桌面快捷方式
Name: "{autodesktop}\LianT"; Filename: "{app}\LianT.exe"; IconFilename: "{app}\LianT.exe"

[Run]
; 安装完不自动启动，只留快捷方式
; 卸载程序自动生成，不用你管
