import QtQuick
import QtQuick.Controls

Rectangle {
    id: root
    color: "white"

    Rectangle {
        id: sidebar
        width: 220
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        color: "#F0F1F4"

        Column {
            anchors.fill: parent
            anchors.margins: 8

            Rectangle {
                width: parent.width
                height: 48
                color: "transparent"
                Row {
                    spacing: 8
                    Text { text: "LianT"; font.bold: true; font.pixelSize: 16; verticalAlignment: Text.AlignVCenter }
                    Text { text: "· " + bridge.appVersion(); color: "#888"; font.pixelSize: 11; verticalAlignment: Text.AlignVCenter }
                }
            }

            Button {
                width: parent.width
                text: "Contacts"
                onClicked: mainHost.currentIndex = 0
            }
            Button {
                width: parent.width
                text: "Chat"
                onClicked: mainHost.currentIndex = 1
            }
            Button {
                width: parent.width
                text: "Settings"
                onClicked: mainHost.currentIndex = 2
            }

            Item { height: 12; width: 1 }

            Text {
                text: "Signed in as " + bridge.selfUserId()
                color: "#999"
                font.pixelSize: 11
                wrapMode: Text.WordWrap
            }
        }
    }

    Rectangle {
        anchors.left: sidebar.right
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.right: parent.right
        color: "#FAFAFB"

        SwipeView {
            id: mainHost
            anchors.fill: parent
            currentIndex: 1

            Item { ContactList { anchors.fill: parent } }
            Item { Chat { anchors.fill: parent } }
            Item { Settings { anchors.fill: parent } }
        }
    }
}