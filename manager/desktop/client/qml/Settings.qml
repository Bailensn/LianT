import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 16

        Text { text: "Settings"; font.bold: true; font.pixelSize: 18 }

        GridLayout {
            columns: 2
            rowSpacing: 10
            columnSpacing: 12

            Text { text: "Server"; Layout.alignment: Qt.AlignLeft }
            TextField {
                text: "https://api.liant.dev"
                Layout.fillWidth: true
            }

            Text { text: "Auto update"; Layout.alignment: Qt.AlignLeft }
            Switch { }

            Text { text: "Start with system"; Layout.alignment: Qt.AlignLeft }
            Switch { }
        }

        Item { Layout.fillHeight: true }

        Button {
            text: "About"
            Layout.alignment: Qt.AlignHCenter
            onClicked: aboutDialog.open()
        }
    }

    Dialog {
        id: aboutDialog
        title: "About LianT"
        standardButtons: Dialog.Ok
        modal: true
        anchors.centerIn: parent

        Column {
            spacing: 8
            Text { text: "LianT"; font.bold: true; font.pixelSize: 20 }
            Text { text: "Cross-platform instant messaging desktop client."; font.pixelSize: 13; color: "#666" }
            Text { text: "Version " + bridge.appVersion(); font.pixelSize: 12; color: "#888" }
        }
    }
}