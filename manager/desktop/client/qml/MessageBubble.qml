import QtQuick

// Reusable message bubble, used by Chat.qml-style views.
// In this codebase Chat.qml draws bubbles inline; this component is provided
// as a reusable primitive for other surfaces (e.g. search results, replies).
Rectangle {
    id: root

    property string direction: "in"   // "in" | "out"
    property string sender: ""
    property string body: ""
    property color bubbleColor: direction === "out" ? "#C8E6F0" : "white"
    property color textColor: "#222"

    width: Math.min(body.length * 7 + 30, 480)
    height: 30
    radius: 10
    color: bubbleColor
    border.color: "#E0E0E0"

    Text {
        anchors.centerIn: parent
        text: root.body
        color: root.textColor
        font.pixelSize: 13
    }
}