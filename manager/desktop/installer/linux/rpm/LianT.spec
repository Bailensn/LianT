Name:           LianT
Version:        1.0.0
Release:        1%{?dist}
Summary:        LianT Desktop Client

License:        AGPL-3.0
URL:            https://github.com/LensnTeam/LianT

BuildArch:      %{_arch}

Requires:       glibc

%description
LianT is a cross-platform instant messaging desktop client.
In dynamic-runtime mode the Go launcher at /opt/LianT/bin/LianT checks
/opt/LianT/runtime for a Python interpreter on first run, downloads a pinned
standalone build there if absent, pip-installs the client dependencies from
/opt/LianT/requirements.txt, and starts the client source from /opt/LianT/src.

%prep
# No source preparation needed

%build
# Build is handled by external CI scripts

%install

# Create directories:
#   /opt/LianT/bin       launcher
#   /opt/LianT/src,qml   client source (source-execution mode)
#   /opt/LianT/runtime   Python interpreter (downloaded dynamically on first run)
mkdir -p %{buildroot}/opt/LianT/bin
mkdir -p %{buildroot}/opt/LianT/resources
mkdir -p %{buildroot}/opt/LianT/runtime
mkdir -p %{buildroot}/usr/share/applications
mkdir -p %{buildroot}/usr/share/icons/hicolor/scalable/apps
mkdir -p %{buildroot}/usr/share/icons/hicolor/512x512/apps


# Launcher
install -m 755 build/LianT \
    %{buildroot}/opt/LianT/bin/LianT


# Client source (src/ + qml/); runtime is NOT bundled.
cp -r build/client/src \
    %{buildroot}/opt/LianT/
cp -r build/client/qml \
    %{buildroot}/opt/LianT/
cp build/client/requirements.txt \
    %{buildroot}/opt/LianT/requirements.txt


# Desktop entry
install -m 644 packaging/linux/LianT.desktop \
    %{buildroot}/usr/share/applications/LianT.desktop


# Icon (scalable + 512 png)
install -m 644 assets/LianT.svg \
    %{buildroot}/usr/share/icons/hicolor/scalable/apps/LianT.svg
install -m 644 packaging/linux/icons/512x512/LianT.png \
    %{buildroot}/usr/share/icons/hicolor/512x512/apps/LianT.png


%files
/opt/LianT

/usr/share/applications/LianT.desktop

/usr/share/icons/hicolor/scalable/apps/LianT.svg

/usr/share/icons/hicolor/512x512/apps/LianT.png


%post
# Ensure the per-user runtime dir is writable (runtime downloaded on first run).
if [ ! -d /opt/LianT/runtime ]; then
    mkdir -p /opt/LianT/runtime
    chmod 0777 /opt/LianT/runtime 2>/dev/null || :
fi

# Update desktop database
if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database /usr/share/applications || :
fi

# Update icon cache
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache /usr/share/icons/hicolor || :
fi


%postun
if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database /usr/share/applications || :
fi


%changelog

* Tue Aug 25 2026 Bailensn <bailensnn@gmail.com> - 1.0.0-1
- Initial release
