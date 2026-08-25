import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root

    property string currentConv: "u1"
    property string draftText: ""

    Connections {
        target: bridge
        function onMessageReceived(convId, payload) {
            if (convId === root.currentConv) {
                var obj = JSON.parse(payload)
                messageModel.append({
                    fromMe: obj.sender_id === bridge.selfUserId(),
                    sender: obj.sender_name || obj.sender_id,
                    body: obj.content || ""
                })
            }
        }
    }

    ListModel { id: messageModel }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 12
        spacing: 8

        RowLayout {
            Text { text: "Conversation"; font.bold: true; font.pixelSize: 18 }
            Item { Layout.fillWidth: true }
            Text { text: root.currentConv; color: "#888"; font.pixelSize: 12 }
        }

        ListView {
            id: msgList
            Layout.fillWidth: true
            Layout.fillHeight: true
            model: messageModel
            clip: true
            spacing: 6
            delegate: RowLayout {
                width: msgList.width
                LayoutMirroring.enabled: fromMe
                LayoutMirroring.childrenInherit: true
                Item { Layout.preferredWidth: 40 }
                Rectangle {
                    Layout.preferredWidth: Math.min(body.length * 7 + 30, msgList.width * 0.7)
                    Layout.preferredHeight: 34
                    color: fromMe ? "#C8E6F0" : "white"
                    radius: 10
                    border.color: "#E0E0E0"
                    Text {
                        anchors.centerIn: parent
                        text: body
                        font.pixelSize: 14
                    }
                }
                Item { Layout.fillWidth: true }
            }
        }

        RowLayout {
            TextField {
                id: input
                Layout.fillWidth: true
                placeholderText: "Type a message…"
                text: root.draftText
            }
            Button {
                text: "Send"
                onClicked: {
                    if (input.text.trim().length === 0) return
                    bridge.sendMessage(root.currentConv, input.text)
                    input.text = ""
                }
            }
        }
    }
}