import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root

    ListModel {
        id: contactModel
        // Populated from contactsUpdated signal in real deploys.
        ListElement { id: "u1"; name: "Alice"; status: "online" }
        ListElement { id: "u2"; name: "Bob"; status: "offline" }
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 12
        spacing: 8

        Text {
            text: "Contacts"
            font.bold: true
            font.pixelSize: 18
        }

        ListView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            model: contactModel
            clip: true
            delegate: Rectangle {
                width: parent ? parent.width : 0
                height: 48
                color: "transparent"
                radius: 6
                RowLayout {
                    anchors.fill: parent
                    anchors.margins: 8
                    spacing: 10
                    Rectangle {
                        width: 28; height: 28; radius: 14
                        color: status === "online" ? "#3CA55C" : "#CCC"
                    }
                    Text { text: name; font.pixelSize: 14 }
                    Item { Layout.fillWidth: true }
                    Button {
                        text: "Chat"
                        onClicked: bridge.openConversation(id)
                    }
                }
            }
        }
    }
}